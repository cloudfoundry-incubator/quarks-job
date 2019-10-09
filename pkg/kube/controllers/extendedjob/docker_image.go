package extendedjob

import "code.cloudfoundry.org/quarks-utils/pkg/names"

// operatorDockerImage is the location of the operators own docker image
var operatorDockerImage = ""

// SetupOperatorDockerImage initializes the package scoped variable
func SetupOperatorDockerImage(org, repo, tag string) (err error) {
	operatorDockerImage, err = names.GetDockerSourceName(org, repo, tag)
	return
}

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return operatorDockerImage
}
