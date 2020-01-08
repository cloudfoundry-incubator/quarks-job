package quarksjob

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/fsnotify.v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned"
	podutil "code.cloudfoundry.org/quarks-utils/pkg/pod"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// OutputPersistor creates a kubernetes secret for each container in the in the qJob pod.
type OutputPersistor struct {
	log                  *zap.SugaredLogger
	namespace            string
	podName              string
	clientSet            kubernetes.Interface
	versionedClientSet   versioned.Interface
	outputFilePathPrefix string
}

// NewOutputPersistor returns a persist output interface which can create kubernetes secrets.
func NewOutputPersistor(log *zap.SugaredLogger, namespace string, podName string, clientSet kubernetes.Interface, versionedClientSet versioned.Interface, outputFilePathPrefix string) *OutputPersistor {
	return &OutputPersistor{
		log:                  log,
		namespace:            namespace,
		podName:              podName,
		clientSet:            clientSet,
		versionedClientSet:   versionedClientSet,
		outputFilePathPrefix: outputFilePathPrefix,
	}
}

// Persist converts the output files of each container
// in the pod related to an qJob into a kubernetes secret.
func (po *OutputPersistor) Persist() error {
	// Fetch the pod
	pod, err := po.clientSet.CoreV1().Pods(po.namespace).Get(po.podName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to fetch pod %s", po.podName)
	}

	// Fetch the qJob
	qJobName := pod.GetLabels()[qjv1a1.LabelQJobName]

	qJobClient := po.versionedClientSet.QuarksjobV1alpha1().QuarksJobs(po.namespace)
	qJob, err := qJobClient.Get(qJobName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to fetch qJob")
	}

	// Persist output if needed
	if !reflect.DeepEqual(qjv1a1.Output{}, qJob.Spec.Output) && qJob.Spec.Output != nil {
		err = po.persistPod(pod, qJob)
		if err != nil {
			return err
		}
	}
	return nil
}

// persistPod starts goroutine for creating secrets for each output found in our containers
func (po *OutputPersistor) persistPod(pod *corev1.Pod, qJob *qjv1a1.QuarksJob) error {
	errorContainerChannel := make(chan error)

	// Loop over containers and create go routine
	for containerIndex, container := range pod.Spec.Containers {
		if container.Name == "output-persist" {
			continue
		}

		filesToSecrets, found := qJob.Spec.Output.OutputMap[container.Name]
		if !found {
			continue
		}

		go po.persistContainer(qJob, containerIndex, container, filesToSecrets, errorContainerChannel)
	}

	// wait for all container go routines
	for i := 0; i < len(pod.Spec.Containers)-1; i++ {
		err := <-errorContainerChannel
		if err != nil {
			return err
		}
	}
	return nil
}

// persistContainer converts json output file
// of the specified container into a secret
func (po *OutputPersistor) persistContainer(
	qJob *qjv1a1.QuarksJob,
	containerIndex int,
	container corev1.Container,
	filesToSecrets qjv1a1.FilesToSecrets,
	errorContainerChannel chan<- error,
) {
	prefix := filepath.Join(po.outputFilePathPrefix, container.Name)
	filePaths := filesToSecrets.PrefixedPaths(prefix)
	po.log.Debugf("container '%s': expects outputs in %v", container.Name, filePaths)

	containerIndex, err := po.checkForOutputFiles(filePaths, containerIndex, container.Name)
	if err != nil {
		errorContainerChannel <- err
	}
	if containerIndex != -1 {
		exitCode, err := po.getContainerExitCode(containerIndex)
		if err != nil {
			errorContainerChannel <- err
		}
		if exitCode == 0 || (exitCode == 1 && qJob.Spec.Output.WriteOnFailure) {
			for fileName, options := range filesToSecrets {
				filePath := filepath.Join(prefix, fileName)
				po.log.Debugf("container '%s': creating '%s' from '%s'", container.Name, options.Name, filePath)
				err := po.createSecret(
					qJob,
					container,
					filePath,
					options,
				)
				if err != nil {
					errorContainerChannel <- err
				}
			}
		}
	}
	errorContainerChannel <- err
}

// getContainerExitCode returns the exit code of the container
func (po *OutputPersistor) getContainerExitCode(containerIndex int) (int, error) {
	// Wait until the container gets into terminated state
	for {
		pod, err := po.clientSet.CoreV1().Pods(po.namespace).Get(po.podName, metav1.GetOptions{})
		if err != nil {
			return -1, errors.Wrapf(err, "failed to fetch pod %s", po.podName)
		}
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name == pod.Spec.Containers[containerIndex].Name && containerStatus.State.Terminated != nil {
				return int(containerStatus.State.Terminated.ExitCode), nil
			}
		}
	}
}

// checkForOutputFiles waits for the output json file to be created
// in the container
func (po *OutputPersistor) checkForOutputFiles(filePaths []string, containerIndex int, containerName string) (int, error) {
	seen := newSeen(filePaths)
	seen.checkAll()
	if seen.complete() {
		po.log.Debugf("container '%s': exit early, files already existed", containerName)
		return containerIndex, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return -1, err
	}
	defer watcher.Close()

	createEventFileChannel := make(chan int)
	errorEventFileChannel := make(chan error)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					continue
				}
				po.log.Debugf("container '%s': new event for %s", containerName, event.Name)
				if event.Op == fsnotify.Create && seen.requires(event.Name) {
					po.log.Debugf("container '%s': event for %s", containerName, event.Name)
					seen.done(event.Name)
				}
				if seen.complete() {
					createEventFileChannel <- containerIndex
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					continue
				}
				errorEventFileChannel <- err
			}
		}
	}()

	err = watcher.Add(filepath.Join(po.outputFilePathPrefix, containerName))
	if err != nil {
		return -1, err
	}

	select {
	case containerIndex := <-createEventFileChannel:
		return containerIndex, nil
	case err := <-errorEventFileChannel:
		return -1, err
	}
}

type seen map[string]bool

func newSeen(files []string) seen {
	s := map[string]bool{}
	for _, file := range files {
		s[file] = false
	}
	return s
}

func (s seen) requires(key string) bool {
	if _, found := s[key]; found {
		return true
	}
	return false
}

func (s seen) done(file string) {
	s[file] = true
}

func (s seen) complete() bool {
	for _, result := range s {
		if !result {
			return false
		}
	}
	return true
}

func (s seen) checkAll() {
	for file := range s {
		if fileExists(file) {
			s.done(file)
		}
	}
}

// fileExists checks if the file exists
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// createSecret converts the output file into json and creates a secret for a given container
func (po *OutputPersistor) createSecret(
	qJob *qjv1a1.QuarksJob,
	container corev1.Container,
	filePath string,
	options qjv1a1.SecretOptions,
) error {
	// Fetch json from file
	file, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.Wrapf(err, "unable to read file %s in container %s in pod %s", filePath, container.Name, po.podName)
	}
	var data map[string]string
	err = json.Unmarshal([]byte(file), &data)
	if err != nil {
		return errors.Wrapf(err, "failed to convert output file %s into json for creating secret %s in pod %s",
			filePath, options.Name, po.podName)
	}

	// Create secret for the output file to persist
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.Name,
			Namespace: po.namespace,
		},
	}

	secretLabels := map[string]string{}
	for k, v := range qJob.Spec.Output.SecretLabels {
		secretLabels[k] = v
	}
	for k, v := range options.AdditionalSecretLabels {
		secretLabels[k] = v
	}
	secretLabels[qjv1a1.LabelPersistentSecretContainer] = container.Name
	if id, ok := podutil.LookupEnv(container.Env, qjv1a1.RemoteIDKey); ok {
		secretLabels[qjv1a1.LabelRemoteID] = id
	}

	if options.Versioned {
		ownerName := qJob.GetName()
		ownerID := qJob.GetUID()
		sourceDescription := "created by quarksJob"

		store := versionedsecretstore.NewClientsetVersionedSecretStore(po.clientSet)
		err = store.Create(context.Background(), po.namespace, ownerName, ownerID, options.Name, data, secretLabels, sourceDescription)
		if err != nil {
			if !versionedsecretstore.IsSecretIdenticalError(err) {
				return errors.Wrapf(err, "could not persist qJob's %s output to a secret", qJob.GetName())
			}
			// No-op. the latest version is identical to the one we have
			return nil
		}
	} else {
		secret.StringData = data
		secret.Labels = secretLabels
		_, err = po.clientSet.CoreV1().Secrets(po.namespace).Create(secret)

		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// If it exists update it
				_, err = po.clientSet.CoreV1().Secrets(po.namespace).Update(secret)
				if err != nil {
					return errors.Wrapf(err, "failed to update secret %s for container %s in pod %s.", options.Name, container.Name, po.podName)
				}
			} else {
				return errors.Wrapf(err, "failed to create secret %s for container %s in pod %s.", options.Name, container.Name, po.podName)
			}
		}

	}
	return nil
}
