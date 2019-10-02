package environment

import (
	"strings"
	"time"

	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	utils "code.cloudfoundry.org/cf-operator/integration/environment"

	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
)

// GetExtendedJob gets an ExtendedJob custom resource
func (m *Machine) GetExtendedJob(namespace string, name string) (*ejv1.ExtendedJob, error) {
	client := m.VersionedClientset.ExtendedjobV1alpha1().ExtendedJobs(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	return d, err
}

// CreateExtendedJob creates an ExtendedJob
func (m *Machine) CreateExtendedJob(namespace string, job ejv1.ExtendedJob) (*ejv1.ExtendedJob, utils.TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedjobV1alpha1().ExtendedJobs(namespace)
	d, err := client.Create(&job)
	return d, func() error {
		pods, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: labels.Set(map[string]string{
				ejv1.LabelEJobName: job.Name,
			}).String(),
		})
		if err != nil {
			return err
		}

		err = client.Delete(job.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		for _, pod := range pods.Items {
			err = m.Clientset.CoreV1().Pods(namespace).Delete(pod.GetName(), &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}

		return nil
	}, err
}

// UpdateExtendedJob updates an extended job
func (m *Machine) UpdateExtendedJob(namespace string, eJob ejv1.ExtendedJob) error {
	client := m.VersionedClientset.ExtendedjobV1alpha1().ExtendedJobs(namespace)
	_, err := client.Update(&eJob)
	return err
}

// WaitForExtendedJobDeletion blocks until the CR job is deleted
func (m *Machine) WaitForExtendedJobDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.ExtendedJobExists(namespace, name)
		return !found, err
	})
}

// ExtendedJobExists returns true if extended job with that name exists
func (m *Machine) ExtendedJobExists(namespace string, name string) (bool, error) {
	_, err := m.GetExtendedJob(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for extended job by name: %s", name)
	}

	return true, nil
}

// CollectJobs waits for n jobs with specified labels.
// It fails after the timeout.
func (m *Machine) CollectJobs(namespace string, labels string, n int) ([]batchv1.Job, error) {
	found := map[string]batchv1.Job{}
	err := wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{
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
	_, err := m.Clientset.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
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
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{
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
