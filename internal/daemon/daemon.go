package daemon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/deeprpa/fuck-gpu/internal/gateway"
	"github.com/deeprpa/fuck-gpu/pkgs/gpucollect"
	"github.com/ygpkg/yg-go/lifecycle"
	"github.com/ygpkg/yg-go/logs"
)

// Daemon background daemon
type Daemon struct {
	ctx context.Context
	cfg *config.MainConfig
	lc  *lifecycle.LifeCycle

	apps map[string]*AppReplicaController

	// InitStatus 初始化的状态
	InitStatus *EnvStatus

	// CurrentStatus 当前状态
	CurrentStatus *EnvStatus

	// Gateway 网关
	Gateway *gateway.Gateway
}

// EnvStatus 环境状态
type EnvStatus struct {
	Resource config.Resource
}

// NewDaemon Create a new daemon instance
func NewDaemon(lc *lifecycle.LifeCycle, cfg *config.MainConfig) (*Daemon, error) {
	baseCtx := context.TODO()
	if lc != nil {
		baseCtx = lc.Context()
	}

	d := &Daemon{
		ctx:  logs.WithContextFields(baseCtx),
		cfg:  cfg,
		lc:   lc,
		apps: map[string]*AppReplicaController{},
	}
	if lc != nil {
		lc.AddCloser(d)
	}

	// Initialize Gateway if enabled
	if cfg.Gateway.Enable {
		d.Gateway = gateway.NewGateway(d.ctx, &cfg.Gateway, map[string][]*gateway.Backend{})
		logs.InfoContextf(d.ctx, "Gateway initialized")
	}

	return d, nil
}

// Close stops all managed components and is wired into lifecycle shutdown.
func (d *Daemon) Close() error {
	for _, app := range d.apps {
		if app == nil {
			continue
		}
		app.Stop()
	}

	if d.Gateway != nil {
		if err := d.Gateway.Stop(); err != nil {
			return err
		}
	}

	return nil
}

func (d *Daemon) Run() error {
	if err := d.loadCurrentStatus(); err != nil {
		logs.ErrorContextf(d.ctx, "load current env status failed, %s", err)
		return err
	}

	if err := d.schedule(); err != nil {
		logs.ErrorContextf(d.ctx, "schedule failed, %s", err)
		return err
	}

	if d.Gateway != nil {
		d.updateGatewayBackendsWithDelay()
	}

	// Start Gateway if enabled
	if d.Gateway != nil {
		if err := d.Gateway.Start(); err != nil {
			logs.ErrorContextf(d.ctx, "start gateway failed, %s", err)
			return err
		}
	}

	return nil
}

// loadCurrentStatus 加载当前环境状态
func (d *Daemon) loadCurrentStatus() error {
	globalCfg := d.cfg.Global
	if globalCfg.AllocatableResource != nil {
		d.InitStatus = &EnvStatus{Resource: *d.cfg.Global.AllocatableResource}
		d.CurrentStatus = &EnvStatus{Resource: *d.cfg.Global.AllocatableResource}
		return nil
	}

	gpuinfos, err := gpucollect.GetNvidiaGPUMemory()
	if err != nil {
		logs.ErrorContextf(d.ctx, "failed to get gpu memory info: %v", err)
		return err
	}
	if len(gpuinfos) == 0 {
		logs.WarnContextf(d.ctx, "no gpu found")
		return nil
	}

	total := &gpucollect.GPUInfo{}
	for _, gpuinfo := range gpuinfos {
		logs.InfoContextf(d.ctx, "gpu memory info: %v", gpuinfo)
		total.MemoryFree += gpuinfo.MemoryFree
		total.MemoryTotal += gpuinfo.MemoryTotal
		total.MemoryUsed += gpuinfo.MemoryUsed
	}

	logs.InfoContextf(d.ctx, "total gpu memory free: %v", total.MemoryFree)
	d.InitStatus = &EnvStatus{Resource: config.Resource{GPUMemory: total.MemoryFree}}
	d.CurrentStatus = &EnvStatus{Resource: config.Resource{GPUMemory: total.MemoryFree}}
	return nil
}

// schedule 调度
func (d *Daemon) schedule() error {
	if d.InitStatus == nil {
		logs.WarnContextf(d.ctx, "not initialized yet")
		return nil
	}

	logs.InfoContextf(d.ctx, "Starting schedule process with %d apps", len(d.cfg.Apps))
	logs.InfoContextf(d.ctx, "Available GPU memory: %v", d.InitStatus.Resource.GPUMemory)

	apps := map[string]*AppReplicaController{}
	needScheApps := map[string]struct{}{}

	for _, appCfg := range d.cfg.Apps {
		if appCfg.ReplicaPolicy.Static != nil && *appCfg.ReplicaPolicy.Static > 0 {
			logs.InfoContextf(d.ctx, "Creating %d static replicas for app: %s", *appCfg.ReplicaPolicy.Static, appCfg.Name)
			app, err := NewAppReplicaController(d.ctx, appCfg, *appCfg.ReplicaPolicy.Static)
			if err != nil {
				logs.ErrorContextf(d.ctx, "create app replica controller for app %s failed, %s", appCfg.Name, err)
				return err
			}
			app.SetDaemon(d)
			apps[appCfg.Name] = app
			continue
		}

		if appCfg.ReplicaPolicy.Static != nil && *appCfg.ReplicaPolicy.Static == 0 {
			logs.InfoContextf(d.ctx, "Skipping app %s with static 0 replicas", appCfg.Name)
			continue
		}

		logs.InfoContextf(d.ctx, "Adding app %s to dynamic scheduling", appCfg.Name)
		needScheApps[appCfg.Name] = struct{}{}
	}

	logs.InfoContextf(d.ctx, "Dynamic scheduling apps count: %d", len(needScheApps))
	dynamicPlan := map[string]int{}

	if len(needScheApps) > 0 {
		freeSize := d.InitStatus.Resource.GPUMemory
	LOOP:
		for {
			if len(needScheApps) == 0 {
				break
			}

			progress := false
			for _, appCfg := range d.cfg.Apps {
				if _, ok := needScheApps[appCfg.Name]; !ok {
					continue
				}

				if _, ok := dynamicPlan[appCfg.Name]; !ok {
					dynamicPlan[appCfg.Name] = 0
				}

				repPol := appCfg.ReplicaPolicy
				if repPol.Require != nil && repPol.Require.GPUMemory > 0 {
					requireMem := repPol.Require.GPUMemory
					if repPol.MaxReplicas != nil && dynamicPlan[appCfg.Name] >= *repPol.MaxReplicas {
						delete(needScheApps, appCfg.Name)
						progress = true
						continue
					}

					freeSize -= requireMem
					if freeSize < 0 {
						logs.WarnContextf(d.ctx, "Not enough resources to schedule more instances for app %s, free: %v, need: %v", appCfg.Name, freeSize+requireMem, requireMem)
						break LOOP
					}

					dynamicPlan[appCfg.Name]++
					progress = true
					logs.InfoContextf(d.ctx, "Scheduled %d instances for app %s (free: %v)", dynamicPlan[appCfg.Name], appCfg.Name, freeSize)
					continue
				}

				// Dynamic scheduling without resource requirements would otherwise loop forever.
				delete(needScheApps, appCfg.Name)
				progress = true
				logs.WarnContextf(d.ctx, "skip dynamic scheduling for app %s because replica.require.gpu_memory is not set", appCfg.Name)
			}

			if !progress {
				break
			}
		}
	}

	for _, appCfg := range d.cfg.Apps {
		replicas, ok := dynamicPlan[appCfg.Name]
		if !ok {
			continue
		}
		if replicas <= 0 {
			logs.WarnContextf(d.ctx, "not enough resources to schedule app %s, need %s", appCfg.Name, appCfg.ReplicaPolicy.Require)
			continue
		}

		logs.InfoContextf(d.ctx, "Creating %d dynamic replicas for app: %s", replicas, appCfg.Name)
		app, err := NewAppReplicaController(d.ctx, appCfg, replicas)
		if err != nil {
			logs.ErrorContextf(d.ctx, "create app replica controller for app %s failed, %s", appCfg.Name, err)
			return err
		}
		app.SetDaemon(d)
		apps[appCfg.Name] = app
	}

	logs.InfoContextf(d.ctx, "Total apps scheduled: %d", len(apps))
	d.apps = apps

	logs.InfoContextf(d.ctx, "Starting all applications...")
	for _, app := range d.apps {
		app.Start()
	}
	logs.InfoContextf(d.ctx, "All applications started successfully")

	return nil
}

type DaemonStatus struct {
	Apps []*AppStatus `json:"apps"`
}

type AppStatus struct {
	Name      string
	Version   string
	StartedAt string

	Main CmdStatus
}

type CmdStatus struct {
	Path           string
	StartedAt      string
	FirstStartedAt string
	RetryTimes     int
	Pid            int
	Version        string
	ReadyToExitAt  string
}

func (d *Daemon) Status() *DaemonStatus {
	sts := &DaemonStatus{Apps: []*AppStatus{}}
	for _, app := range d.apps {
		sts.Apps = append(sts.Apps, &AppStatus{
			Name:      app.appCfg.Name,
			StartedAt: app.startAt.String(),
		})
	}
	return sts
}

// Schedule 调度
func (d *Daemon) Schedule() error {
	return d.schedule()
}

func (d *Daemon) buildGatewayBackends() map[string][]*gateway.Backend {
	backends := map[string][]*gateway.Backend{}

	for appName, controller := range d.apps {
		if controller == nil || len(controller.appCfg.GatewayBackends) == 0 {
			continue
		}

		activeInstanceIndices := controller.ActiveInstanceIndices()
		if len(activeInstanceIndices) == 0 {
			continue
		}

		for _, rule := range controller.appCfg.GatewayBackends {
			pathPrefix := strings.TrimSpace(rule.PathPrefix)
			backendTpl := strings.TrimSpace(rule.Backend)
			if pathPrefix == "" || backendTpl == "" {
				logs.WarnContextf(d.ctx, "skip invalid gateway backend rule for app %s, path_prefix=%q, backend=%q", appName, rule.PathPrefix, rule.Backend)
				continue
			}

			groupKey := fmt.Sprintf("%s|%s", appName, pathPrefix)
			for _, replicaIdx := range activeInstanceIndices {
				backendAddr := applyGatewayBackendTemplate(backendTpl, replicaIdx)
				backends[groupKey] = append(backends[groupKey], &gateway.Backend{
					URL:        normalizeGatewayBackendURL(backendAddr),
					AppName:    appName,
					ReplicaIdx: replicaIdx,
					PathPrefix: pathPrefix,
				})
			}
		}
	}

	return backends
}

func (d *Daemon) updateGatewayBackendsWithDelay() {
	time.AfterFunc(2*time.Second, func() {
		d.updateGatewayBackends()
	})
}

func (d *Daemon) updateGatewayBackends() {
	if d.Gateway != nil {
		d.Gateway.UpdateBackends(d.buildGatewayBackends())
	}
}

func (d *Daemon) HandleInstanceUnavailable(appName string, replicaIdx int) {
	if d.Gateway == nil {
		return
	}
	d.Gateway.RemoveBackend(appName, replicaIdx)
}

func applyGatewayBackendTemplate(backend string, idx int) string {
	return strings.ReplaceAll(backend, "{{index}}", fmt.Sprintf("%d", idx))
}

func normalizeGatewayBackendURL(backend string) string {
	trimmed := strings.TrimSpace(backend)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	return "http://" + trimmed
}
