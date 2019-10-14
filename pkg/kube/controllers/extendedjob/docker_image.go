package extendedjob

import (
	"fmt"

	"code.cloudfoundry.org/quarks-utils/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

// operatorDockerImage is the location of the operators own docker image
var operatorDockerImage = ""
var operatorImagePullPolicy = corev1.PullIfNotPresent

// SetupOperatorDockerImage initializes the package scoped variable
func SetupOperatorDockerImage(org, repo, tag string) (err error) {
	operatorDockerImage, err = names.GetDockerSourceName(org, repo, tag)
	return
}

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return operatorDockerImage
}

// SetupOperatorImagePullPolicy sets the pull policy
func SetupOperatorImagePullPolicy(pullPolicy string) error {
	switch pullPolicy {
	case string(corev1.PullAlways):
		operatorImagePullPolicy = corev1.PullAlways
	case string(corev1.PullNever):
		operatorImagePullPolicy = corev1.PullNever
	case string(corev1.PullIfNotPresent):
		operatorImagePullPolicy = corev1.PullIfNotPresent
	default:
		return fmt.Errorf("invalid image pull policy '%s'", pullPolicy)
	}
	return nil
}

// GetOperatorImagePullPolicy returns the image pull policy to be used for generated pods
func GetOperatorImagePullPolicy() corev1.PullPolicy {
	return operatorImagePullPolicy
}
