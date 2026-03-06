package daemon

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/stretchr/testify/assert"
)

func TestRuntimeInstance_RestartInterval_DefaultAndConfigured(t *testing.T) {
	c1 := &RuntimeInstance{cfg: config.AppConfig{}}
	assert.Equal(t, defaultRestartInterval, c1.restartInterval())

	custom := 2 * time.Second
	c2 := &RuntimeInstance{cfg: config.AppConfig{RestartPolicy: config.RestartPolicy{Interval: &custom}}}
	assert.Equal(t, 2*time.Second, c2.restartInterval())
}

func TestRuntimeInstance_CanRestart_ByMaxRetries(t *testing.T) {
	cUnlimited := &RuntimeInstance{cfg: config.AppConfig{}}
	cUnlimited.retryTimes = 100
	assert.True(t, cUnlimited.canRestart())

	zero := 0
	cZero := &RuntimeInstance{cfg: config.AppConfig{RestartPolicy: config.RestartPolicy{MaxRetries: &zero}}}
	assert.False(t, cZero.canRestart())

	two := 2
	c := &RuntimeInstance{cfg: config.AppConfig{RestartPolicy: config.RestartPolicy{MaxRetries: &two}}}
	c.retryTimes = 0
	assert.True(t, c.canRestart())
	c.retryTimes = 1
	assert.True(t, c.canRestart())
	c.retryTimes = 2
	assert.False(t, c.canRestart())
}

func TestRuntimeInstance_Restart_NotifyPermanentExitWhenMaxRetriesReached(t *testing.T) {
	zero := 0
	notified := false
	c := &RuntimeInstance{
		cfg:           config.AppConfig{RestartPolicy: config.RestartPolicy{MaxRetries: &zero}},
		ctx:           context.TODO(),
		cmd:           &exec.Cmd{Process: &os.Process{Pid: 99999}},
		chExitRoutine: make(chan struct{}),
		onPermanentExit: func(inst *RuntimeInstance) {
			notified = inst != nil
		},
	}

	err := c.restart()
	assert.NoError(t, err)
	assert.True(t, notified)
}
