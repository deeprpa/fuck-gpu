package daemon

import (
	"context"
	"time"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/ygpkg/yg-go/logs"
)

type AppReplicaController struct {
	cfg *config.AppConfig
	ctx context.Context

	startAt time.Time
	cmd     *Command
	args    []string
}

func NewApp(ictx context.Context, cfg *config.AppConfig) (*AppReplicaController, error) {
	app := &AppReplicaController{
		cfg:     cfg,
		ctx:     logs.WithContextFields(ictx, "app", cfg.Name),
		startAt: time.Now(),
		args:    cfg.Command.Args,
	}

	cmd, err := app.NewCommand(cfg.Command)
	if err != nil {
		logs.ErrorContextf(app.ctx, "new command %v failed, %s", cfg.Command, err)
		return nil, err
	}
	app.cmd = cmd

	return app, nil
}

func (a *AppReplicaController) NewCommand(cfg config.CommandConfig) (*Command, error) {
	c := &Command{
		appName:       a.cfg.Name,
		cfg:           cfg,
		ctx:           logs.WithContextFields(a.ctx, "module", "command"),
		chExitRoutine: make(chan struct{}),
		errExit:       make(chan error),
		retryTimes:    1,
	}

	cmd, err := c.getCommand(cfg)
	if err != nil {
		return nil, err
	}
	c.cmd = cmd

	return c, nil
}

func (a *AppReplicaController) Start() {
	go a.cmd.Start()
}

func (a *AppReplicaController) ExitSpare() {
	if a.cmd == nil {
		return
	}
	a.cmd.Exit()
}

func (a *AppReplicaController) Restart() error {
	a.ExitSpare()

	a.cmd.Exit()

	ncmd, err := a.NewCommand(a.cfg.Command)
	if err != nil {
		return err
	}
	ncmd.restart()
	a.cmd = ncmd
	return nil
}
