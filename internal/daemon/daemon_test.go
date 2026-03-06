package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ygpkg/yg-go/lifecycle"
)

func TestDaemonSchedule_ManagesStaticAndDynamicApps(t *testing.T) {
	cfg := &config.MainConfig{
		Global: config.GlobalConfig{
			AllocatableResource: &config.Resource{GPUMemory: config.NewMemorySize("16G")},
		},
		Apps: []config.AppConfig{
			{
				Name: "llm-qwen3",
				Command: config.CommandConfig{
					Command: "sleep",
					Args:    []string{"2"},
				},
				ReplicaPolicy: config.ReplicaPolicy{
					Require:     &config.Resource{GPUMemory: config.NewMemorySize("4G")},
					MaxReplicas: intPtr(2),
					MinReplicas: intPtr(1),
				},
			},
			{
				Name: "echo-app",
				Command: config.CommandConfig{
					Command: "sleep",
					Args:    []string{"2"},
				},
				ReplicaPolicy: config.ReplicaPolicy{
					Static: intPtr(3),
					Require: &config.Resource{
						GPUMemory: 0,
					},
				},
			},
		},
	}

	d := &Daemon{
		ctx:  context.TODO(),
		cfg:  cfg,
		apps: map[string]*AppReplicaController{},
		InitStatus: &EnvStatus{
			Resource: *cfg.Global.AllocatableResource,
		},
		CurrentStatus: &EnvStatus{
			Resource: *cfg.Global.AllocatableResource,
		},
	}

	require.NoError(t, d.schedule())
	defer stopAllDaemonApps(d)

	require.Len(t, d.apps, 2)
	assert.Len(t, d.apps["llm-qwen3"].cmds, 2)
	assert.Len(t, d.apps["echo-app"].cmds, 3)

	for _, app := range d.apps {
		for _, cmd := range app.cmds {
			require.Eventually(t, func() bool {
				return cmd.cmd != nil && cmd.cmd.Process != nil && cmd.cmd.ProcessState == nil
			}, 500*time.Millisecond, 20*time.Millisecond)
		}
	}
}

func TestDaemonSchedule_DynamicWithoutRequireDoesNotBlock(t *testing.T) {
	cfg := &config.MainConfig{
		Global: config.GlobalConfig{
			AllocatableResource: &config.Resource{GPUMemory: config.NewMemorySize("16G")},
		},
		Apps: []config.AppConfig{
			{
				Name: "dynamic-no-require",
				Command: config.CommandConfig{
					Command: "echo",
					Args:    []string{"hello"},
				},
				ReplicaPolicy: config.ReplicaPolicy{},
			},
		},
	}

	d := &Daemon{
		ctx:  context.TODO(),
		cfg:  cfg,
		apps: map[string]*AppReplicaController{},
		InitStatus: &EnvStatus{
			Resource: *cfg.Global.AllocatableResource,
		},
		CurrentStatus: &EnvStatus{
			Resource: *cfg.Global.AllocatableResource,
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- d.schedule()
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("schedule should return quickly for dynamic app without resource requirement")
	}

	assert.Len(t, d.apps, 0)
}

func TestDaemonRun_InitializesAppsAndGateway(t *testing.T) {
	cfg := &config.MainConfig{
		Gateway: config.GatewayConfig{
			Enable:     true,
			ListenAddr: "127.0.0.1:18080",
		},
		Global: config.GlobalConfig{
			AllocatableResource: &config.Resource{GPUMemory: config.NewMemorySize("8G")},
		},
		Apps: []config.AppConfig{
			{
				Name: "llm-qwen3",
				Command: config.CommandConfig{
					Command: "sleep",
					Args:    []string{"2"},
				},
				ReplicaPolicy: config.ReplicaPolicy{
					Require:     &config.Resource{GPUMemory: config.NewMemorySize("4G")},
					MaxReplicas: intPtr(1),
				},
				GatewayBackends: []config.GatewayBackendConfig{
					{PathPrefix: "/qwen3", Backend: "127.0.0.1:808{{index}}"},
				},
			},
		},
	}

	lc := lifecycle.New()
	d, err := NewDaemon(lc, cfg)
	require.NoError(t, err)
	require.NotNil(t, d.Gateway)

	require.NoError(t, d.Run())
	defer func() {
		stopAllDaemonApps(d)
		if d.Gateway != nil {
			_ = d.Gateway.Stop()
		}
	}()

	assert.Len(t, d.apps, 1)
	require.Eventually(t, func() bool {
		app := d.apps["llm-qwen3"]
		if app == nil || len(app.cmds) == 0 {
			return false
		}
		cmd := app.cmds[0]
		return cmd.cmd != nil && cmd.cmd.Process != nil && cmd.cmd.ProcessState == nil
	}, 500*time.Millisecond, 20*time.Millisecond)
}

func TestDaemonClose_StopsManagedProcesses(t *testing.T) {
	cfg := &config.MainConfig{
		Global: config.GlobalConfig{
			AllocatableResource: &config.Resource{GPUMemory: config.NewMemorySize("4G")},
		},
		Apps: []config.AppConfig{
			{
				Name: "close-check-app",
				Command: config.CommandConfig{
					Command: "sleep",
					Args:    []string{"5"},
				},
				ReplicaPolicy: config.ReplicaPolicy{
					Static: intPtr(1),
				},
			},
		},
	}

	lc := lifecycle.New()
	d, err := NewDaemon(lc, cfg)
	require.NoError(t, err)
	require.NoError(t, d.Run())

	app := d.apps["close-check-app"]
	require.NotNil(t, app)
	require.Len(t, app.cmds, 1)

	cmd := app.cmds[0]
	require.Eventually(t, func() bool {
		return cmd.cmd != nil && cmd.cmd.Process != nil && cmd.cmd.ProcessState == nil
	}, 500*time.Millisecond, 20*time.Millisecond)

	require.NoError(t, d.Close())
	require.Eventually(t, func() bool {
		select {
		case <-cmd.chExitRoutine:
			return true
		default:
			return false
		}
	}, 500*time.Millisecond, 20*time.Millisecond)
}

func stopAllDaemonApps(d *Daemon) {
	for _, app := range d.apps {
		if app != nil {
			app.Stop()
		}
	}
}
