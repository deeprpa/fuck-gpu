package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/ygpkg/yg-go/logs"
)

type AppReplicaController struct {
	appCfg config.AppConfig
	ctx    context.Context

	daemon  *Daemon
	startAt time.Time
	cmds    []*RuntimeInstance
}

func NewAppReplicaController(ictx context.Context, cfg config.AppConfig, replica int) (*AppReplicaController, error) {
	app := &AppReplicaController{
		appCfg:  cfg,
		ctx:     logs.WithContextFields(ictx, "app", cfg.Name),
		startAt: time.Now(),
	}

	logs.InfoContextf(app.ctx, "Creating %d replicas for application: %s", replica, cfg.Name)

	for i := 0; i < replica; i++ {
		cmd, err := app.NewCommand(app.ctx, cfg, i)
		if err != nil {
			logs.ErrorContextf(app.ctx, "new command %v failed, %s", cfg.Command, err)
			return nil, err
		}
		app.cmds = append(app.cmds, cmd)
	}

	logs.InfoContextf(app.ctx, "Successfully created %d commands for application: %s", len(app.cmds), cfg.Name)

	return app, nil
}

func (a *AppReplicaController) NewCommand(ictx context.Context, cfg config.AppConfig, idx int) (*RuntimeInstance, error) {
	c := &RuntimeInstance{
		AppName:       cfg.Name,
		cfg:           cfg,
		Index:         idx,
		ctx:           logs.WithContextFields(a.ctx, "idx", fmt.Sprintf("%d", idx)),
		chExitRoutine: make(chan struct{}),
		errExit:       make(chan error),
		retryTimes:    0,
		onPermanentExit: func(inst *RuntimeInstance) {
			a.onInstancePermanentlyExited(inst)
		},
	}

	cmd, err := c.getCommand(cfg.Command)
	if err != nil {
		return nil, err
	}
	c.cmd = cmd

	return c, nil
}

func (a *AppReplicaController) Start() {
	for _, cmd := range a.cmds {
		go cmd.Start()
	}
}

func (a *AppReplicaController) Stop() {
	for _, cmd := range a.cmds {
		cmd.Exit()
	}
}

func (a *AppReplicaController) Restart() error {
	a.Stop()

	// 重新创建命令
	newCmds := []*RuntimeInstance{}
	for i := range a.cmds {
		newCmd, err := a.NewCommand(a.ctx, a.appCfg, i)
		if err != nil {
			logs.ErrorContextf(a.ctx, "restart command %v failed, %s", a.appCfg.Command, err)
			return err
		}
		newCmds = append(newCmds, newCmd)
	}
	a.cmds = newCmds

	// 启动新命令
	a.Start()
	return nil
}

// CountActiveInstances counts instances that are still running
func (a *AppReplicaController) CountActiveInstances() int {
	count := 0
	for _, cmd := range a.cmds {
		if !isInstanceActive(cmd) {
			continue
		}
		count++
	}
	return count
}

// ActiveInstanceIndices returns currently running replica indices.
func (a *AppReplicaController) ActiveInstanceIndices() []int {
	indices := make([]int, 0, len(a.cmds))
	for _, cmd := range a.cmds {
		if !isInstanceActive(cmd) {
			continue
		}
		indices = append(indices, cmd.Index)
	}
	return indices
}

func isInstanceActive(inst *RuntimeInstance) bool {
	if inst == nil || inst.cmd == nil {
		return false
	}
	if inst.cmd.Process == nil {
		return false
	}
	if inst.cmd.ProcessState != nil {
		return false
	}
	return true
}

func (a *AppReplicaController) onInstancePermanentlyExited(inst *RuntimeInstance) {
	if a.daemon == nil || inst == nil {
		return
	}
	a.daemon.HandleInstanceUnavailable(a.appCfg.Name, inst.Index)
}

// SetDaemon keeps daemon reference for future controller callbacks.
func (a *AppReplicaController) SetDaemon(daemon *Daemon) {
	a.daemon = daemon
}
