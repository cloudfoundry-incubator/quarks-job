package v1alpha1

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/quarks-job/pkg/kube/apis"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

var (
	// LabelPersistentSecretContainer is a label used for persisted secrets,
	// identifying the container that created them
	LabelPersistentSecretContainer = fmt.Sprintf("%s/container-name", apis.GroupName)
	// LabelInstanceGroup is a label for persisted secrets, identifying
	// the instance group they belong to
	LabelInstanceGroup = fmt.Sprintf("%s/instance-group", apis.GroupName)

	// LabelQuarksJob key for label used to identify quarksJob.
	// Value is set to true if the batchv1.Job is from an QuarksJob
	LabelQuarksJob = fmt.Sprintf("%s/quarks-job", apis.GroupName)
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

const (
	// TriggerManual is the default for errand jobs, change to TriggerNow to run them
	TriggerManual Strategy = "manual"
	// TriggerNow instructs the controller to run the job now,
	// resets to TriggerManual after starting the job
	TriggerNow Strategy = "now"
	// TriggerOnce jobs run only once, when created, then switches to TriggerDone
	TriggerOnce Strategy = "once"
	// TriggerDone jobs are no longer triggered. It's the final state for TriggerOnce strategies
	TriggerDone Strategy = "done"
)

// Trigger decides how to trigger the QuarksJob
type Trigger struct {
	Strategy Strategy `json:"strategy"`
}

// Output contains options to persist job output
type Output struct {
	NamePrefix     string            `json:"namePrefix"`           // the secret name will be <NamePrefix><container name>
	OutputType     string            `json:"outputType,omitempty"` // only json is supported for now
	SecretLabels   map[string]string `json:"secretLabels,omitempty"`
	WriteOnFailure bool              `json:"writeOnFailure,omitempty"`
	Versioned      bool              `json:"versioned,omitempty"`
}

// QuarksJobStatus defines the observed state of QuarksJob
type QuarksJobStatus struct {
	LastReconcile *metav1.Time `json:"lastReconcile"`
	Nodes         []string     `json:"nodes"`
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
