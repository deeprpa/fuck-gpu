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

	r := httptest.NewRecorder()
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
