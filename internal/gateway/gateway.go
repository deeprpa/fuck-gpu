package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/gin-gonic/gin"
	"github.com/ygpkg/yg-go/logs"
)

type Gateway struct {
	ctx        context.Context
	cfg        *config.GatewayConfig
	backends   map[string][]*Backend
	rrCursor   map[string]int
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
		rrCursor: map[string]int{},
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
	router.Any("/*any", func(c *gin.Context) {
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
		for _, appBackends := range g.backends {
			for _, b := range appBackends {
				backends = append(backends, map[string]interface{}{
					"app_name":    b.AppName,
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

	g.mu.Lock()
	defer g.mu.Unlock()

	// Try to find matching backend by path prefix
	var matchedApp string
	var matchedPrefix string
	for appName, backends := range g.backends {
		if len(backends) == 0 {
			continue
		}
		prefix := backends[0].PathPrefix
		if prefix != "" && matchPathPrefix(path, prefix) {
			if len(prefix) > len(matchedPrefix) {
				matchedPrefix = prefix
				matchedApp = appName
			}
		}
	}

	if matchedApp == "" {
		for appName, backends := range g.backends {
			if len(backends) > 0 {
				matchedApp = appName
				break
			}
		}
	}

	var targetBackend *Backend
	if matchedApp != "" {
		appBackends := g.backends[matchedApp]
		if len(appBackends) > 0 {
			idx := g.rrCursor[matchedApp] % len(appBackends)
			targetBackend = appBackends[idx]
			g.rrCursor[matchedApp] = (idx + 1) % len(appBackends)
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
	g.rrCursor = map[string]int{}
	logs.Infof("Gateway backends updated")
}

func matchPathPrefix(path, prefix string) bool {
	if prefix == "" || path == "" {
		return false
	}
	if path == prefix {
		return true
	}
	if strings.HasPrefix(path, prefix+"/") {
		return true
	}
	return false
}

// GenerateBackends generates backend URLs based on app configuration
func GenerateBackends(apps []config.AppConfig) map[string][]*Backend {
	backends := make(map[string][]*Backend)

	for _, app := range apps {
		if len(app.GatewayBackends) == 0 {
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

		for _, rule := range app.GatewayBackends {
			pathPrefix := strings.TrimSpace(rule.PathPrefix)
			backendTpl := strings.TrimSpace(rule.Backend)
			if pathPrefix == "" || backendTpl == "" {
				continue
			}

			groupKey := fmt.Sprintf("%s|%s", app.Name, pathPrefix)
			for i := 0; i < replicaCount; i++ {
				backendURL := strings.ReplaceAll(backendTpl, "{{index}}", fmt.Sprintf("%d", i))
				if !strings.HasPrefix(backendURL, "http://") && !strings.HasPrefix(backendURL, "https://") {
					backendURL = "http://" + backendURL
				}
				backends[groupKey] = append(backends[groupKey], &Backend{
					URL:        backendURL,
					AppName:    app.Name,
					ReplicaIdx: i,
					PathPrefix: pathPrefix,
				})
			}
		}

		logs.Infof("Generated gateway backends for app %s with replica count %d", app.Name, replicaCount)
	}

	return backends
}
