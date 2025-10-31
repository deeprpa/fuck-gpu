package daemon

import (
	"context"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/deeprpa/fuck-gpu/pkgs/gpucollect"
	"github.com/ygpkg/yg-go/lifecycle"
	"github.com/ygpkg/yg-go/logs"
)

// Daemon background daemon
type Daemon struct {
	ctx context.Context
	cfg *config.MainConfig
	lc  *lifecycle.LifeCycle

	apps []*AppReplicaController

	// InitStatus 初始化的状态
	InitStatus *EnvStatus

	// CurrentStatus 当前状态
	CurrentStatus *EnvStatus
}

// EnvStatus 环境状态
type EnvStatus struct {
	Resource config.Resource
}

// NewDaemon Create a new daemon instance
func NewDaemon(lc *lifecycle.LifeCycle, cfg *config.MainConfig) (*Daemon, error) {
	d := &Daemon{
		ctx:  logs.WithContextFields(lc.Context(), "daemon"),
		cfg:  cfg,
		lc:   lc,
		apps: []*AppReplicaController{},
	}

	for _, appCfg := range d.cfg.Apps {
		app, err := NewAppController(lc.Context(), appCfg)
		if err != nil {
			return nil, err
		}
		d.apps = append(d.apps, app)
	}
	return d, nil
}

func (d *Daemon) Run() error {
	if err := d.loadCurrentStatus(); err != nil {
		logs.ErrorContextf(d.ctx, "load current env status failed, %s", err)
		return err
	}

	for _, app := range d.apps {
		app.Start()
	}

	return nil
}

// loadCurrentStatus 加载当前环境状态
func (d *Daemon) loadCurrentStatus() error {
	globalCfg := d.cfg.Global
	if globalCfg.AllocatableResource != nil {
		d.InitStatus = &EnvStatus{
			Resource: *d.cfg.Global.AllocatableResource,
		}
		d.CurrentStatus = &EnvStatus{
			Resource: *d.cfg.Global.AllocatableResource,
		}
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
	d.InitStatus = &EnvStatus{
		Resource: config.Resource{
			GPUMemory: total.MemoryFree,
		},
	}
	d.CurrentStatus = &EnvStatus{
		Resource: config.Resource{
			GPUMemory: total.MemoryFree,
		},
	}

	return nil
}

// schedule 调度
func (d *Daemon) schedule() error {
	if d.InitStatus == nil {
		logs.WarnContextf(d.ctx, "")
		return nil
	}
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
	sts := &DaemonStatus{
		Apps: []*AppStatus{},
	}

	for _, app := range d.apps {
		ast := &AppStatus{
			Name:      app.cfg.Name,
			StartedAt: app.startAt.String(),
		}
		if cmd := app.cmd; cmd != nil {
			if cmd.startedAt != nil {
				ast.Main.StartedAt = cmd.startedAt.String()
			}
			if cmd.firstStartedAt != nil {
				ast.Main.FirstStartedAt = cmd.firstStartedAt.String()
			}
			if cmd.localVer != nil {
				ast.Main.Version = cmd.localVer.String()
			}
			ast.Main.RetryTimes = int(cmd.retryTimes)
			if cc := cmd.cmd; cc != nil {
				ast.Main.Path = cc.Path
				if cc.Process != nil {
					ast.Main.Pid = cc.Process.Pid
				}
			}
		}

		sts.Apps = append(sts.Apps, ast)
	}

	return sts
}

func (d *Daemon) App() *AppReplicaController {
	if len(d.apps) == 0 {
		return nil
	}
	return d.apps[0]
}

// Schedule 调度
func (d *Daemon) Schedule() error {
	return nil
}
