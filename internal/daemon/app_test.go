package daemon

import (
	"testing"
	"time"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/stretchr/testify/assert"
)

func intPtr(i int) *int {
	return &i
}

func TestNewAppReplicaController(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "sleep",
			Args:    []string{"1"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(2),
		},
	}

	app, err := NewAppReplicaController(nil, cfg, 2)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Len(t, app.cmds, 2)
}

func TestAppReplicaController_TemplateSupport(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "server",
			Args:    []string{"--port={{index}}", "--name=test_{{index}}"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(2),
		},
	}

	app, err := NewAppReplicaController(nil, cfg, 2)
	assert.NoError(t, err)
	assert.NotNil(t, app)
	assert.Len(t, app.cmds, 2)

	// Test that commands were created (but we can't directly verify the command processing without integration test)
	// Testing the functionality would require mocking or integration testing
}

func TestAppReplicaController_Start(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "sleep",
			Args:    []string{"1"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(1),
		},
	}

	app, err := NewAppReplicaController(nil, cfg, 1)
	assert.NoError(t, err)
	assert.NotNil(t, app)

	// Test start (just make sure it doesn't panic)
	app.Start()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)
}

func TestAppReplicaController_Stop(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "sleep",
			Args:    []string{"1"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(1),
		},
	}

	app, err := NewAppReplicaController(nil, cfg, 1)
	assert.NoError(t, err)
	assert.NotNil(t, app)

	// Test stop (just make sure it doesn't panic)
	app.Stop()
}

func TestAppReplicaController_Restart(t *testing.T) {
	cfg := config.AppConfig{
		Name: "test-app",
		Command: config.CommandConfig{
			Command: "sleep",
			Args:    []string{"1"},
		},
		ReplicaPolicy: config.ReplicaPolicy{
			Static: intPtr(1),
		},
	}

	app, err := NewAppReplicaController(nil, cfg, 1)
	assert.NoError(t, err)
	assert.NotNil(t, app)

	// Test restart (just make sure it doesn't panic)
	err = app.Restart()
	assert.NoError(t, err)
}
