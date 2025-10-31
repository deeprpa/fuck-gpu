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

	startAt time.Time
	cmds    []*Command
}

func NewAppReplicaController(ictx context.Context, cfg config.AppConfig, replica int) (*AppReplicaController, error) {
	app := &AppReplicaController{
		appCfg:  cfg,
		ctx:     logs.WithContextFields(ictx, "app", cfg.Name),
		startAt: time.Now(),
	}

	for i := 0; i < replica; i++ {
		cmd, err := app.NewCommand(app.ctx, cfg, i)
		if err != nil {
			logs.ErrorContextf(app.ctx, "new command %v failed, %s", cfg.Command, err)
			return nil, err
		}
		app.cmds = append(app.cmds, cmd)
	}

	return app, nil
}

func (a *AppReplicaController) NewCommand(ictx context.Context, cfg config.AppConfig, idx int) (*Command, error) {
	c := &Command{
		appName:       cfg.Name,
		cfg:           cfg,
		idx:           idx,
		ctx:           logs.WithContextFields(a.ctx, "idx", fmt.Sprintf("%d", idx+1)),
		chExitRoutine: make(chan struct{}),
		errExit:       make(chan error),
		retryTimes:    0,
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

func (a *AppReplicaController) Restart() error {
	// a.cmd.Exit()

	// ncmd, err := a.NewCommand(a.cfg)
	// if err != nil {
	// 	return err
	// }
	// ncmd.restart()
	// a.cmd = ncmd
	return nil
}
