package environment

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// Machine produces and destroys resources for tests
type Machine struct {
	machine.Machine

	VersionedClientset *versioned.Clientset
}

// GetQuarksJob gets an QuarksJob custom resource
func (m *Machine) GetQuarksJob(namespace string, name string) (*qjv1a1.QuarksJob, error) {
	client := m.VersionedClientset.QuarksjobV1alpha1().QuarksJobs(namespace)
	d, err := client.Get(context.Background(), name, metav1.GetOptions{})
	return d, err
}

// CreateQuarksJob creates an QuarksJob
func (m *Machine) CreateQuarksJob(namespace string, job qjv1a1.QuarksJob) (*qjv1a1.QuarksJob, machine.TearDownFunc, error) {
	client := m.VersionedClientset.QuarksjobV1alpha1().QuarksJobs(namespace)
	d, err := client.Create(context.Background(), &job, metav1.CreateOptions{})
	return d, func() error {
		pods, err := m.Clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: labels.Set(map[string]string{
				qjv1a1.LabelQJobName: job.Name,
			}).String(),
		})
		if err != nil {
			return err
		}

		err = client.Delete(context.Background(), job.GetName(), metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		for _, pod := range pods.Items {
			err = m.Clientset.CoreV1().Pods(namespace).Delete(context.Background(), pod.GetName(), metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}

		return nil
	}, err
}

// UpdateQuarksJob updates an quarks job
func (m *Machine) UpdateQuarksJob(namespace string, qJob qjv1a1.QuarksJob) error {
	client := m.VersionedClientset.QuarksjobV1alpha1().QuarksJobs(namespace)
	_, err := client.Update(context.Background(), &qJob, metav1.UpdateOptions{})
	return err
}

// WaitForQuarksJobDeletion blocks until the quarks job is deleted
func (m *Machine) WaitForQuarksJobDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		found, err := m.QuarksJobExists(namespace, name)
		return !found, err
	})
}

// QuarksJobExists returns true if quarks job with that name exists
func (m *Machine) QuarksJobExists(namespace string, name string) (bool, error) {
	_, err := m.GetQuarksJob(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for quarks job by name: %s", name)
	}

	return true, nil
}

// CollectJobs waits for n jobs with specified labels.
// It fails after the timeout.
func (m *Machine) CollectJobs(namespace string, labels string, n int) ([]batchv1.Job, error) {
	found := map[string]batchv1.Job{}
	err := wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for jobs by label: %s", labels)
		}

		for _, job := range jobs.Items {
			found[job.GetName()] = job
		}
		return len(found) >= n, nil
	})

	if err != nil {
		return nil, err
	}

	jobs := []batchv1.Job{}
	for _, job := range found {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// JobExists returns true if job with that name exists
func (m *Machine) JobExists(namespace string, name string) (bool, error) {
	_, err := m.Clientset.BatchV1().Jobs(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for job by name: %s", name)
	}

	return true, nil
}

// WaitForJobExists polls until a short timeout is reached or a job is found
// It returns true only if a job is found
func (m *Machine) WaitForJobExists(namespace string, labels string) (bool, error) {
	found := false
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for jobs by label: %s", labels)
		}

		found = len(jobs.Items) != 0
		return found, err
	})

	if err != nil && strings.Contains(err.Error(), "timed out waiting for the condition") {
		err = nil
	}

	return found, err
}

// WaitForJobDeletion blocks until the batchv1.Job is deleted
func (m *Machine) WaitForJobDeletion(namespace string, name string) error {
	return wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		found, err := m.JobExists(namespace, name)
		return !found, err
	})
}
