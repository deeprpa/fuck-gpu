package gateway

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/gin-gonic/gin"
	"github.com/ygpkg/yg-go/logs"
)

type Gateway struct {
	ctx        context.Context
	cfg        *config.GatewayConfig
	backends   map[string][]*Backend
	mu         sync.RWMutex
	httpServer *http.Server
}

type Backend struct {
	URL        string
	AppName    string
	ReplicaIdx int
	PathPrefix string
}

func NewGateway(ctx context.Context, cfg *config.GatewayConfig, backends map[string][]*Backend) *Gateway {
	return &Gateway{
		ctx:      ctx,
		cfg:      cfg,
		backends: backends,
	}
}

func (g *Gateway) Start() error {
	if g.cfg == nil || !g.cfg.Enable {
		logs.InfoContextf(g.ctx, "Gateway is disabled")
		return nil
	}

	if g.cfg.ListenAddr == "" {
		g.cfg.ListenAddr = ":8080"
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger())

	// All requests go through the proxy handler which handles routing
	router.Any("/", func(c *gin.Context) {
		g.serveRequest(c)
	})

	g.httpServer = &http.Server{
		Addr:    g.cfg.ListenAddr,
		Handler: router,
	}

	go func() {
		logs.Infof("Gateway starting on %s", g.cfg.ListenAddr)
		if err := g.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logs.Errorf("Gateway server error: %s", err)
		}
	}()

	logs.Infof("Gateway started successfully")
	return nil
}

func (g *Gateway) serveRequest(c *gin.Context) {
	path := c.Request.URL.Path

	// Handle /health
	if path == "/health" || path == "/health/" {
		c.JSON(200, gin.H{"status": "ok"})
		return
	}

	// Handle /backends
	if path == "/backends" || path == "/backends/" {
		g.mu.RLock()
		defer g.mu.RUnlock()

		var backends []map[string]interface{}
		for appName, appBackends := range g.backends {
			for _, b := range appBackends {
				backends = append(backends, map[string]interface{}{
					"app_name":    appName,
					"replica_idx": b.ReplicaIdx,
					"url":         b.URL,
					"path_prefix": b.PathPrefix,
				})
			}
		}
		c.JSON(200, gin.H{"backends": backends})
		return
	}

	// Proxy to backend
	g.proxyRequest(c)
}

func (g *Gateway) proxyRequest(c *gin.Context) {
	path := c.Request.URL.Path

	g.mu.RLock()
	defer g.mu.RUnlock()

	// Try to find matching backend by path prefix
	var targetBackend *Backend
	for _, backends := range g.backends {
		for _, b := range backends {
			if b.PathPrefix != "" && len(path) > len(b.PathPrefix) && path[:len(b.PathPrefix)] == b.PathPrefix {
				targetBackend = b
				break
			}
		}
		if targetBackend != nil {
			break
		}
	}

	// If no path prefix match, use first available backend
	if targetBackend == nil {
		for _, backends := range g.backends {
			if len(backends) > 0 {
				targetBackend = backends[0]
				break
			}
		}
	}

	if targetBackend == nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "no backend available"})
		return
	}

	// Forward request to backend
	targetURL := targetBackend.URL + path
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	logs.Debugf("Proxying %s %s to %s", c.Request.Method, path, targetURL)

	// Simple proxy - just redirect to the backend
	c.Redirect(http.StatusFound, targetURL)
}

func (g *Gateway) Stop() error {
	if g.httpServer != nil {
		return g.httpServer.Close()
	}
	return nil
}

func (g *Gateway) UpdateBackends(backends map[string][]*Backend) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.backends = backends
	logs.Infof("Gateway backends updated")
}

// GenerateBackends generates backend URLs based on app configuration
func GenerateBackends(apps []config.AppConfig) map[string][]*Backend {
	backends := make(map[string][]*Backend)

	for _, app := range apps {
		if app.WebApp == nil {
			continue
		}

		replicaCount := 1
		if app.ReplicaPolicy.Static != nil && *app.ReplicaPolicy.Static > 0 {
			replicaCount = *app.ReplicaPolicy.Static
		}

		// For dynamic scheduling, we need to estimate - use max_replicas or default to 1
		if app.ReplicaPolicy.MaxReplicas != nil && *app.ReplicaPolicy.MaxReplicas > replicaCount {
			replicaCount = *app.ReplicaPolicy.MaxReplicas
		}

		basePort := app.WebApp.Port
		pathPrefix := app.WebApp.PathPrefix
		if pathPrefix == "" {
			pathPrefix = "/" + app.Name
		}

		for i := 0; i < replicaCount; i++ {
			port := basePort + i
			backend := &Backend{
				URL:        fmt.Sprintf("http://localhost:%d", port),
				AppName:    app.Name,
				ReplicaIdx: i,
				PathPrefix: pathPrefix,
			}
			backends[app.Name] = append(backends[app.Name], backend)
		}

		logs.Infof("Generated %d backends for app %s with base port %d", replicaCount, app.Name, basePort)
	}

	return backends
}
