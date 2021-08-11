module code.cloudfoundry.org/quarks-job

require (
	code.cloudfoundry.org/quarks-utils v0.0.3-0.20210303091853-3b41f4b87e33
	github.com/go-logr/logr v0.4.0
	github.com/mitchellh/mapstructure v1.3.2 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/afero v1.4.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.0
	go.uber.org/zap v1.18.1
	gopkg.in/fsnotify.v1 v1.4.7
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	sigs.k8s.io/controller-runtime v0.9.6
)

go 1.15
