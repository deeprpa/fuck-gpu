package gateway

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/gin-gonic/gin"
	"github.com/ygpkg/yg-go/logs"
)

type Gateway struct {
	ctx        context.Context
	cfg        *config.GatewayConfig
	backends   map[string][]*Backend
	rrCursor   map[string]int
	health     map[string]*backendHealthState
	healthStop map[string]chan struct{}
	healthWg   sync.WaitGroup
	mu         sync.RWMutex
	httpServer *http.Server
}

type Backend struct {
	URL         string
	AppName     string
	ReplicaIdx  int
	PathPrefix  string
	HealthCheck *config.GatewayHealthCheckConfig
}

type backendHealthState struct {
	initialized   bool
	healthy       bool
	consecSuccess int
	consecFailure int
}

const (
	defaultHealthCheckPath               = "/health"
	defaultHealthCheckInterval           = 2 * time.Second
	defaultHealthCheckTimeout            = 1 * time.Second
	defaultHealthCheckHealthyThreshold   = 1
	defaultHealthCheckUnhealthyThreshold = 1
)

func NewGateway(ctx context.Context, cfg *config.GatewayConfig, backends map[string][]*Backend) *Gateway {
	return &Gateway{
		ctx:        ctx,
		cfg:        cfg,
		backends:   backends,
		rrCursor:   map[string]int{},
		health:     map[string]*backendHealthState{},
		healthStop: map[string]chan struct{}{},
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

	g.mu.Lock()
	g.restartHealthWorkersLocked()
	g.mu.Unlock()

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
					"healthy":     g.isBackendHealthyLocked(b),
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
		healthyBackends := g.filterHealthyBackendsLocked(appBackends)
		if len(healthyBackends) > 0 {
			idx := g.rrCursor[matchedApp] % len(healthyBackends)
			targetBackend = healthyBackends[idx]
			g.rrCursor[matchedApp] = (idx + 1) % len(healthyBackends)
		}
	}

	if targetBackend == nil {
		g.mu.Unlock()
		c.JSON(http.StatusBadGateway, gin.H{"error": "no backend available"})
		return
	}
	g.mu.Unlock()

	// Forward request to backend
	targetURL := targetBackend.URL + path
	if c.Request.URL.RawQuery != "" {
		targetURL += "?" + c.Request.URL.RawQuery
	}

	logs.Debugf("Proxying %s %s to %s", c.Request.Method, path, targetURL)
	target, err := url.Parse(targetBackend.URL)
	if err != nil {
		logs.Errorf("invalid backend url %s: %v", targetBackend.URL, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid backend url"})
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
		logs.Errorf("proxy request failed, backend=%s, err=%v", targetBackend.URL, proxyErr)
		rw.WriteHeader(http.StatusBadGateway)
	}

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func (g *Gateway) filterHealthyBackendsLocked(backends []*Backend) []*Backend {
	if len(backends) == 0 {
		return nil
	}

	result := make([]*Backend, 0, len(backends))
	for _, backend := range backends {
		if backend == nil {
			continue
		}
		if backend.HealthCheck == nil {
			result = append(result, backend)
			continue
		}
		state, ok := g.health[backendKey(backend)]
		if !ok || !state.initialized || !state.healthy {
			continue
		}
		result = append(result, backend)
	}

	return result
}

func (g *Gateway) Stop() error {
	g.mu.Lock()
	g.stopHealthWorkersLocked()
	g.mu.Unlock()
	g.healthWg.Wait()

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
	g.restartHealthWorkersLocked()
	logs.Infof("Gateway backends updated")
}

func (g *Gateway) RemoveBackend(appName string, replicaIdx int) {
	g.mu.Lock()
	defer g.mu.Unlock()

	removed := false
	for key, backendList := range g.backends {
		if len(backendList) == 0 {
			continue
		}

		filtered := backendList[:0]
		for _, backend := range backendList {
			if backend == nil {
				continue
			}
			if backend.AppName == appName && backend.ReplicaIdx == replicaIdx {
				removed = true
				continue
			}
			filtered = append(filtered, backend)
		}

		if len(filtered) == 0 {
			delete(g.backends, key)
			delete(g.rrCursor, key)
			continue
		}

		g.backends[key] = filtered
		if cursor, ok := g.rrCursor[key]; ok && cursor >= len(filtered) {
			g.rrCursor[key] = 0
		}
	}

	if removed {
		logs.Warnf("Gateway backend removed, app=%s replica_idx=%d", appName, replicaIdx)
	}
	g.restartHealthWorkersLocked()
}

func (g *Gateway) restartHealthWorkersLocked() {
	g.stopHealthWorkersLocked()
	g.startHealthWorkersLocked()
}

func (g *Gateway) stopHealthWorkersLocked() {
	for key, stopCh := range g.healthStop {
		close(stopCh)
		delete(g.healthStop, key)
	}
	g.health = map[string]*backendHealthState{}
}

func (g *Gateway) startHealthWorkersLocked() {
	for _, group := range g.backends {
		for _, backend := range group {
			if backend == nil || backend.HealthCheck == nil {
				continue
			}

			key := backendKey(backend)
			if _, exists := g.healthStop[key]; exists {
				continue
			}

			stopCh := make(chan struct{})
			g.healthStop[key] = stopCh
			g.health[key] = &backendHealthState{}

			backendSnapshot := *backend
			g.healthWg.Add(1)
			go g.runHealthCheckLoop(key, backendSnapshot, stopCh)
		}
	}
}

func backendKey(backend *Backend) string {
	if backend == nil {
		return ""
	}
	return fmt.Sprintf("%s|%d|%s|%s", backend.AppName, backend.ReplicaIdx, backend.PathPrefix, backend.URL)
}

func (g *Gateway) runHealthCheckLoop(key string, backend Backend, stopCh <-chan struct{}) {
	defer g.healthWg.Done()

	hc := backend.HealthCheck
	path := defaultHealthCheckPath
	interval := defaultHealthCheckInterval
	timeout := defaultHealthCheckTimeout
	healthyThreshold := defaultHealthCheckHealthyThreshold
	unhealthyThreshold := defaultHealthCheckUnhealthyThreshold
	if hc != nil {
		if strings.TrimSpace(hc.Path) != "" {
			path = strings.TrimSpace(hc.Path)
		}
		if hc.Interval != nil && *hc.Interval > 0 {
			interval = *hc.Interval
		}
		if hc.Timeout != nil && *hc.Timeout > 0 {
			timeout = *hc.Timeout
		}
		if hc.HealthyThreshold != nil && *hc.HealthyThreshold > 0 {
			healthyThreshold = *hc.HealthyThreshold
		}
		if hc.UnhealthyThreshold != nil && *hc.UnhealthyThreshold > 0 {
			unhealthyThreshold = *hc.UnhealthyThreshold
		}
	}

	check := func() {
		healthy := probeBackendHealth(backend.URL, path, timeout)

		g.mu.Lock()
		defer g.mu.Unlock()

		state, ok := g.health[key]
		if !ok {
			return
		}

		if healthy {
			state.consecSuccess++
			state.consecFailure = 0
			if state.consecSuccess >= healthyThreshold {
				state.initialized = true
				state.healthy = true
			}
			return
		}

		state.consecFailure++
		state.consecSuccess = 0
		if state.consecFailure >= unhealthyThreshold {
			state.initialized = true
			state.healthy = false
		}
	}

	check()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			check()
		case <-stopCh:
			return
		}
	}
}

func probeBackendHealth(baseURL, path string, timeout time.Duration) bool {
	if path == "" {
		path = defaultHealthCheckPath
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	target, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	target.Path = path

	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodGet, target.String(), nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest
}

func (g *Gateway) isBackendHealthyLocked(backend *Backend) bool {
	if backend == nil {
		return false
	}
	if backend.HealthCheck == nil {
		return true
	}
	state, ok := g.health[backendKey(backend)]
	if !ok || !state.initialized {
		return false
	}
	return state.healthy
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
					URL:         backendURL,
					AppName:     app.Name,
					ReplicaIdx:  i,
					PathPrefix:  pathPrefix,
					HealthCheck: rule.HealthCheck,
				})
			}
		}

		logs.Infof("Generated gateway backends for app %s with replica count %d", app.Name, replicaCount)
	}

	return backends
}
