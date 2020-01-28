package config

import (
	"time"

	"github.com/spf13/afero"

	sharedcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
)

// Config modified for quarks-job
type Config struct {
	*sharedcfg.Config
	ServiceAccount string
}

// NewDefaultConfig returns a new Config for a manager of controllers
func NewDefaultConfig(fs afero.Fs) *Config {
	return &Config{
		Config: &sharedcfg.Config{
			MeltdownDuration:     sharedcfg.MeltdownDuration,
			MeltdownRequeueAfter: sharedcfg.MeltdownRequeueAfter,
			Fs:                   fs,
		},
	}
}

// NewConfigWithTimeout returns a default config, with a context timeout
func NewConfigWithTimeout(timeout time.Duration) *Config {
	return &Config{
		Config: &sharedcfg.Config{
			CtxTimeOut:           timeout,
			MeltdownDuration:     sharedcfg.MeltdownDuration,
			MeltdownRequeueAfter: sharedcfg.MeltdownRequeueAfter,
		},
	}
}
