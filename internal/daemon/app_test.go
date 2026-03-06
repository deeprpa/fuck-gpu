package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(i int) *int {
	return &i
}

func TestNewAppReplicaController(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "sleep",
			Args:    []string{"2"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(2),
		},
	}

	app, err := NewAppReplicaController(context.TODO(), cfg, 2)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Len(t, app.cmds, 2)
	app.Stop()
}

func TestAppReplicaController_TemplateSupport(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "echo",
			Args:    []string{"instance={{index}}"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(2),
		},
	}

	app, err := NewAppReplicaController(context.TODO(), cfg, 2)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Len(t, app.cmds, 2)
	require.Equal(t, "instance=0", app.cmds[0].cmd.Args[1])
	require.Equal(t, "instance=1", app.cmds[1].cmd.Args[1])
	app.Stop()
}

func TestAppReplicaController_StartStopLifecycle(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "sleep",
			Args:    []string{"2"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(2),
		},
	}

	app, err := NewAppReplicaController(context.TODO(), cfg, 2)
	require.NoError(t, err)
	app.Start()

	for _, cmd := range app.cmds {
		require.Eventually(t, func() bool {
			return cmd.cmd != nil && cmd.cmd.Process != nil && cmd.cmd.ProcessState == nil
		}, 500*time.Millisecond, 20*time.Millisecond)
	}

	app.Stop()
	for _, cmd := range app.cmds {
		require.Eventually(t, func() bool {
			select {
			case <-cmd.chExitRoutine:
				return true
			default:
				return false
			}
		}, 500*time.Millisecond, 20*time.Millisecond)
	}
}

func TestAppReplicaController_RestartLifecycle(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "sleep",
			Args:    []string{"2"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(1),
		},
	}

	app, err := NewAppReplicaController(context.TODO(), cfg, 1)
	require.NoError(t, err)
	app.Start()

	require.Eventually(t, func() bool {
		return app.cmds[0].cmd != nil && app.cmds[0].cmd.Process != nil && app.cmds[0].cmd.ProcessState == nil
	}, 500*time.Millisecond, 20*time.Millisecond)
	oldPID := app.cmds[0].cmd.Process.Pid

	require.NoError(t, app.Restart())
	require.Eventually(t, func() bool {
		return app.cmds[0].cmd != nil && app.cmds[0].cmd.Process != nil && app.cmds[0].cmd.ProcessState == nil
	}, 500*time.Millisecond, 20*time.Millisecond)
	newPID := app.cmds[0].cmd.Process.Pid
	assert.NotEqual(t, oldPID, newPID)

	app.Stop()
}
