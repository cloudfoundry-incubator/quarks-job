package config

import (
	"time"

	"github.com/spf13/afero"

	sharedcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
)

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

func NewConfigWithTimeout(timeout time.Duration) *Config {
	return &Config{
		Config: &sharedcfg.Config{
			CtxTimeOut:           timeout,
			MeltdownDuration:     sharedcfg.MeltdownDuration,
			MeltdownRequeueAfter: sharedcfg.MeltdownRequeueAfter,
		},
	}
}
