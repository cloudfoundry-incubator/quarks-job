module code.cloudfoundry.org/quarks-job

require (
	code.cloudfoundry.org/quarks-utils v0.0.2-0.20200925150425-86012dcc3927
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.8.1
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.6.1
	go.uber.org/zap v1.13.0
	golang.org/x/tools v0.0.0-20200708183856-df98bc6d456c // indirect
	gopkg.in/fsnotify.v1 v1.4.7
	k8s.io/api v0.18.9
	k8s.io/apiextensions-apiserver v0.18.9
	k8s.io/apimachinery v0.18.9
	k8s.io/client-go v0.18.9
	sigs.k8s.io/controller-runtime v0.6.3
)

go 1.14
