package environment

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util"

	"code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned"
)

// Machine produces and destroys resources for tests
type Machine struct {
	pollTimeout  time.Duration
	pollInterval time.Duration

	Clientset          *kubernetes.Clientset
	VersionedClientset *versioned.Clientset
}

// TearDownFunc tears down the resource
type TearDownFunc func() error

// ChanResult holds different fields that can be
// sent through a channel
type ChanResult struct {
	Error error
}

// CreateNamespace creates a namespace, it doesn't return an error if the namespace exists
func (m *Machine) CreateNamespace(namespace string) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Namespaces()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := client.Create(ns)
	if apierrors.IsAlreadyExists(err) {
		err = nil
	}
	return func() error {
		b := metav1.DeletePropagationBackground
		err := client.Delete(ns.GetName(), &metav1.DeleteOptions{
			// this is run in aftersuite before failhandler, so let's keep the namespace for a few seconds
			GracePeriodSeconds: util.Int64(5),
			PropagationPolicy:  &b,
		})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// TearDownAll calls all passed in tear down functions in order
func (m *Machine) TearDownAll(funcs []TearDownFunc) error {
	var messages string
	for _, f := range funcs {
		err := f()
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}
	}
	if messages != "" {
		return errors.New(messages)
	}
	return nil
}

// CreateConfigMap creates a ConfigMap and returns a function to delete it
func (m *Machine) CreateConfigMap(namespace string, configMap corev1.ConfigMap) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().ConfigMaps(namespace)
	_, err := client.Create(&configMap)
	return func() error {
		err := client.Delete(configMap.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// UpdateConfigMap updates a ConfigMap and returns a function to delete it
func (m *Machine) UpdateConfigMap(namespace string, configMap corev1.ConfigMap) (*corev1.ConfigMap, TearDownFunc, error) {
	client := m.Clientset.CoreV1().ConfigMaps(namespace)
	cm, err := client.Update(&configMap)
	return cm, func() error {
		err := client.Delete(configMap.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// GetConfigMap gets a ConfigMap by name
func (m *Machine) GetConfigMap(namespace string, name string) (*corev1.ConfigMap, error) {
	configMap, err := m.Clientset.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return &corev1.ConfigMap{}, errors.Wrapf(err, "failed to query for configMap by name: %v", name)
	}

	return configMap, nil
}

// CreateSecret creates a secret and returns a function to delete it
func (m *Machine) CreateSecret(namespace string, secret corev1.Secret) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	_, err := client.Create(&secret)
	return func() error {
		err := client.Delete(secret.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// UpdateSecret updates a secret and returns a function to delete it
func (m *Machine) UpdateSecret(namespace string, secret corev1.Secret) (*corev1.Secret, TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	s, err := client.Update(&secret)
	return s, func() error {
		err := client.Delete(secret.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// PodsDeleted returns true if the all pods are deleted
func (m *Machine) PodsDeleted(namespace string) (bool, error) {
	podList, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	if len(podList.Items) == 0 {
		return true, nil
	}
	return false, nil
}
