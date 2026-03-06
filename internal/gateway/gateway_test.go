package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closeNotifyRecorder struct {
	*httptest.ResponseRecorder
	closeCh chan bool
}

func newCloseNotifyRecorder() *closeNotifyRecorder {
	return &closeNotifyRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeCh:          make(chan bool, 1),
	}
}

func (r *closeNotifyRecorder) CloseNotify() <-chan bool {
	return r.closeCh
}

func TestGatewayProxyRequest_ForwardsWithoutRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "ok")
		_, _ = fmt.Fprintf(w, "backend-path=%s query=%s", r.URL.Path, r.URL.RawQuery)
	}))
	defer backend.Close()

	g := NewGateway(context.TODO(), &config.GatewayConfig{Enable: true}, map[string][]*Backend{
		"llm-qwen3|/qwen3": {
			{
				URL:        backend.URL,
				AppName:    "llm-qwen3",
				ReplicaIdx: 0,
				PathPrefix: "/qwen3",
			},
		},
	})

	r := newCloseNotifyRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest(http.MethodGet, "/qwen3/chat?x=1", nil)

	g.proxyRequest(c)

	require.Equal(t, http.StatusOK, r.Code)
	assert.Equal(t, "ok", r.Header().Get("X-Backend"))
	body, err := io.ReadAll(r.Result().Body)
	require.NoError(t, err)
	assert.Equal(t, "backend-path=/qwen3/chat query=x=1", string(body))
	assert.NotEqual(t, http.StatusFound, r.Code)
}

func TestGatewayRemoveBackend(t *testing.T) {
	g := NewGateway(context.TODO(), &config.GatewayConfig{Enable: true}, map[string][]*Backend{
		"llm-qwen3|/qwen3": {
			{URL: "http://127.0.0.1:8090", AppName: "llm-qwen3", ReplicaIdx: 0, PathPrefix: "/qwen3"},
			{URL: "http://127.0.0.1:8091", AppName: "llm-qwen3", ReplicaIdx: 1, PathPrefix: "/qwen3"},
		},
		"llm-other|/other": {
			{URL: "http://127.0.0.1:9000", AppName: "llm-other", ReplicaIdx: 0, PathPrefix: "/other"},
		},
	})

	g.RemoveBackend("llm-qwen3", 1)

	require.Len(t, g.backends["llm-qwen3|/qwen3"], 1)
	assert.Equal(t, 0, g.backends["llm-qwen3|/qwen3"][0].ReplicaIdx)
	require.Len(t, g.backends["llm-other|/other"], 1)
}

func TestGatewayProxyRequest_OnlyUsesHealthyBackends(t *testing.T) {
	gin.SetMode(gin.TestMode)

	healthyBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend", "healthy")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer healthyBackend.Close()

	hc := &config.GatewayHealthCheckConfig{}
	unhealthy := &Backend{URL: "http://127.0.0.1:1", AppName: "llm-qwen3", ReplicaIdx: 0, PathPrefix: "/qwen3", HealthCheck: hc}
	healthy := &Backend{URL: healthyBackend.URL, AppName: "llm-qwen3", ReplicaIdx: 1, PathPrefix: "/qwen3", HealthCheck: hc}

	g := NewGateway(context.TODO(), &config.GatewayConfig{Enable: true}, map[string][]*Backend{
		"llm-qwen3|/qwen3": {unhealthy, healthy},
	})
	g.health[backendKey(unhealthy)] = &backendHealthState{initialized: true, healthy: false}
	g.health[backendKey(healthy)] = &backendHealthState{initialized: true, healthy: true}

	r := newCloseNotifyRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest(http.MethodGet, "/qwen3/chat", nil)

	g.proxyRequest(c)

	require.Equal(t, http.StatusOK, r.Code)
	assert.Equal(t, "healthy", r.Header().Get("X-Backend"))
}

func TestGatewayProxyRequest_HealthUnknownReturnsBadGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hc := &config.GatewayHealthCheckConfig{}
	g := NewGateway(context.TODO(), &config.GatewayConfig{Enable: true}, map[string][]*Backend{
		"llm-qwen3|/qwen3": {
			{URL: "http://127.0.0.1:8091", AppName: "llm-qwen3", ReplicaIdx: 1, PathPrefix: "/qwen3", HealthCheck: hc},
		},
	})

	r := newCloseNotifyRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest(http.MethodGet, "/qwen3/chat", nil)

	g.proxyRequest(c)

	require.Equal(t, http.StatusBadGateway, r.Code)
}
