package daemon

import (
	"os"
	"os/exec"
	"testing"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/stretchr/testify/assert"
)

func TestBuildGatewayBackends_FromAppGatewayBackends(t *testing.T) {
	d := &Daemon{
		apps: map[string]*AppReplicaController{
			"llm-qwen3": {
				appCfg: config.AppConfig{
					Name: "llm-qwen3",
					GatewayBackends: []config.GatewayBackendConfig{
						{
							PathPrefix: "/qwen3",
							Backend:    "127.0.0.1:808{{index}}",
						},
					},
				},
				cmds: []*RuntimeInstance{
					{Index: 0, cmd: &exec.Cmd{Process: &os.Process{Pid: 1000}}},
					{Index: 1, cmd: &exec.Cmd{Process: &os.Process{Pid: 1001}}},
				},
			},
		},
	}

	backends := d.buildGatewayBackends()
	assert.Len(t, backends, 1)

	key := "llm-qwen3|/qwen3"
	assert.Contains(t, backends, key)
	assert.Len(t, backends[key], 2)

	assert.Equal(t, "http://127.0.0.1:8080", backends[key][0].URL)
	assert.Equal(t, "http://127.0.0.1:8081", backends[key][1].URL)
	assert.Equal(t, "/qwen3", backends[key][0].PathPrefix)
	assert.Equal(t, "llm-qwen3", backends[key][0].AppName)
}

func TestBuildGatewayBackends_UseActiveReplicaIndex(t *testing.T) {
	d := &Daemon{
		apps: map[string]*AppReplicaController{
			"llm-qwen3": {
				appCfg: config.AppConfig{
					Name: "llm-qwen3",
					GatewayBackends: []config.GatewayBackendConfig{
						{
							PathPrefix: "/qwen3",
							Backend:    "127.0.0.1:809{{index}}",
						},
					},
				},
				cmds: []*RuntimeInstance{
					{Index: 0, cmd: &exec.Cmd{Process: &os.Process{Pid: 1000}}},
					{Index: 1},
					{Index: 2, cmd: &exec.Cmd{Process: &os.Process{Pid: 1002}}},
				},
			},
		},
	}

	backends := d.buildGatewayBackends()
	key := "llm-qwen3|/qwen3"
	assert.Contains(t, backends, key)
	assert.Len(t, backends[key], 2)
	assert.Equal(t, 0, backends[key][0].ReplicaIdx)
	assert.Equal(t, "http://127.0.0.1:8090", backends[key][0].URL)
	assert.Equal(t, 2, backends[key][1].ReplicaIdx)
	assert.Equal(t, "http://127.0.0.1:8092", backends[key][1].URL)
}

func TestNormalizeGatewayBackendURL(t *testing.T) {
	assert.Equal(t, "http://127.0.0.1:8080", normalizeGatewayBackendURL("127.0.0.1:8080"))
	assert.Equal(t, "http://127.0.0.1:8080", normalizeGatewayBackendURL(" http://127.0.0.1:8080 "))
	assert.Equal(t, "https://127.0.0.1:8443", normalizeGatewayBackendURL("https://127.0.0.1:8443"))
}
