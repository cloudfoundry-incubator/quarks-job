package environment

import (
	"time"

	"go.uber.org/zap/zaptest/observer"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	pollInterval = 1 * time.Second
)

// WaitForLogMsg searches zap test logs for at least one occurrence of msg.
// When using this, tests should use FlushLog() to remove log messages from
// other tests.
func (m *Machine) WaitForLogMsg(logs *observer.ObservedLogs, msg string) error {
	return wait.PollImmediate(pollInterval, m.pollTimeout, func() (bool, error) {
		n := logs.FilterMessageSnippet(msg).Len()
		return n > 0, nil
	})
}
