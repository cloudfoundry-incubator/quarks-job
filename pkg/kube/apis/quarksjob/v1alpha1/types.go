package v1alpha1

import (
	"fmt"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/quarks-job/pkg/kube/apis"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

var (
	// LabelNamespace key for label on a namespace to indicate that cf-operator is monitoring it.
	// Can be used as an ID, to keep operators in a cluster from intefering with each other.
	LabelNamespace = fmt.Sprintf("%s/monitored", apis.GroupName)

	// LabelServiceAccount key for label on a namespace, which names the
	// service account, that will be injected to capture job output into a
	// secret
	LabelServiceAccount = fmt.Sprintf("%s/qjob-service-account", apis.GroupName)

	// LabelPersistentSecretContainer is a label used for persisted secrets,
	// identifying the container that created them
	LabelPersistentSecretContainer = fmt.Sprintf("%s/container-name", apis.GroupName)

	// LabelRemoteID is a label for persisted secrets, identifying
	// the remote resource they belong to
	LabelRemoteID = fmt.Sprintf("%s/remote-id", apis.GroupName)

	// LabelQJobName key for label on a batchv1.Job's pod, which is set to the QuarksJob's name
	LabelQJobName = fmt.Sprintf("%s/qjob-name", apis.GroupName)
	// LabelTriggeringPod key for label, which is set to the UID of the pod that triggered an QuarksJob
	LabelTriggeringPod = fmt.Sprintf("%s/triggering-pod", apis.GroupName)
)

// QuarksJobSpec defines the desired state of QuarksJob
type QuarksJobSpec struct {
	Output               *Output                 `json:"output,omitempty"`
	Trigger              Trigger                 `json:"trigger"`
	Template             batchv1.JobTemplateSpec `json:"template"`
	UpdateOnConfigChange bool                    `json:"updateOnConfigChange"`
}

// Strategy describes the trigger strategy
type Strategy string

// PersistenceMethod describes the secret persistence implemention style
type PersistenceMethod string

const (
	// RemoteIDKey is the key for the ENV variable which is copied to the
	// output secrets label `LabelReferencedJobName`
	// This env can be set on each container, which is supposed to generate output.
	RemoteIDKey = "REMOTE_ID"

	// TriggerManual is the default for errand jobs, change to TriggerNow to run them
	TriggerManual Strategy = "manual"
	// TriggerNow instructs the controller to run the job now,
	// resets to TriggerManual after starting the job
	TriggerNow Strategy = "now"
	// TriggerOnce jobs run only once, when created, then switches to TriggerDone
	TriggerOnce Strategy = "once"
	// TriggerDone jobs are no longer triggered. It's the final state for TriggerOnce strategies
	TriggerDone Strategy = "done"

	// PersistOneToOne results in one secret per input file using the provided
	// name as the secret name
	PersistOneToOne PersistenceMethod = "one-to-one"

	// PersistUsingFanOut results in one secret per key/value pair found in the
	// provided input file and the name being used as a prefix for the secret
	PersistUsingFanOut PersistenceMethod = "fan-out"
)

// Trigger decides how to trigger the QuarksJob
type Trigger struct {
	Strategy Strategy `json:"strategy"`
}

// SecretOptions specify the name of the output secret and if it's versioned
type SecretOptions struct {
	Name                        string            `json:"name,omitempty"`
	AdditionalSecretLabels      map[string]string `json:"secretLabels,omitempty"`
	AdditionalSecretAnnotations map[string]string `json:"secretAnnotations,omitempty"`
	Versioned                   bool              `json:"versioned,omitempty"`
	PersistenceMethod           PersistenceMethod `json:"persistencemethod,omitempty"`
}

// FanOutName returns the name of the secret for PersistenceMethod 'fan-out'
func (so SecretOptions) FanOutName(key string) string {
	return so.Name + "-" + key
}

// FilesToSecrets maps file names to secret names
type FilesToSecrets map[string]SecretOptions

// OutputMap has FilesToSecrets mappings for every container
type OutputMap map[string]FilesToSecrets

// Output contains options to persist job output to secrets
type Output struct {
	// OutputMap allows for for additional output files per container.
	// Each filename maps to a set of options.
	OutputMap OutputMap `json:"outputMap"`

	// OutputType only JSON is supported for now
	OutputType string `json:"outputType,omitempty"`

	// SecretLabels are copied onto the newly created secrets
	SecretLabels   map[string]string `json:"secretLabels,omitempty"`
	WriteOnFailure bool              `json:"writeOnFailure,omitempty"`
}

// QuarksJobStatus defines the observed state of QuarksJob
type QuarksJobStatus struct {
	LastReconcile *metav1.Time `json:"lastReconcile"`
	Completed     bool         `json:"completed"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuarksJob is the Schema for the QuarksJobs API
// +k8s:openapi-gen=true
type QuarksJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuarksJobSpec   `json:"spec,omitempty"`
	Status QuarksJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuarksJobList contains a list of QuarksJob
type QuarksJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuarksJob `json:"items"`
}

// ToBeDeleted checks whether this QuarksJob has been marked for deletion
func (q *QuarksJob) ToBeDeleted() bool {
	// IsZero means that the object hasn't been marked for deletion
	return !q.GetDeletionTimestamp().IsZero()
}

// IsAutoErrand returns true if this quarks job is an auto errand
func (q *QuarksJob) IsAutoErrand() bool {
	return q.Spec.Trigger.Strategy == TriggerOnce || q.Spec.Trigger.Strategy == TriggerDone
}

// GetNamespacedName returns the resource name with its namespace
func (q *QuarksJob) GetNamespacedName() string {
	return fmt.Sprintf("%s/%s", q.Namespace, q.Name)
}

// IsMonitoredNamespace returns true if the namespace has all the necessary
// labels and should be included in controller watches.
func IsMonitoredNamespace(n *corev1.Namespace, id string) bool {
	if _, ok := n.Labels[LabelServiceAccount]; !ok {
		return false
	}
	if value, ok := n.Labels[LabelNamespace]; ok && value == id {
		return true
	}
	return false
}

// NewFileToSecret returns a FilesToSecrets with just one mapping
func NewFileToSecret(fileName string, secretName string, versioned bool, annotations map[string]string, labels map[string]string) FilesToSecrets {
	return FilesToSecrets{
		fileName: SecretOptions{
			Name:                        secretName,
			Versioned:                   versioned,
			PersistenceMethod:           PersistOneToOne,
			AdditionalSecretAnnotations: annotations,
			AdditionalSecretLabels:      labels,
		},
	}
}

// NewFileToSecrets uses a fan out style and creates one secret per key/value
// pair in the given input file
func NewFileToSecrets(fileName string, secretName string, versioned bool, annotations map[string]string, labels map[string]string) FilesToSecrets {
	return FilesToSecrets{
		fileName: SecretOptions{
			Name:                        secretName,
			Versioned:                   versioned,
			PersistenceMethod:           PersistUsingFanOut,
			AdditionalSecretAnnotations: annotations,
			AdditionalSecretLabels:      labels,
		},
	}
}

// PrefixedPaths returns all output file names, prefixed with the `prefix`
func (f FilesToSecrets) PrefixedPaths(prefix string) []string {
	paths := make([]string, 0, len(f))
	for fileName := range f {
		paths = append(paths, filepath.Join(prefix, fileName))
	}
	return paths
}
