package daemon

import (
	"fmt"
	"os"
	"time"

	"github.com/Masterminds/semver"
	"github.com/deeprpa/fuck-gpu/config"
	"github.com/sirupsen/logrus"
)

type RuningItem int8

const (
	RA RuningItem = 1
	RB RuningItem = -1
)

func (r RuningItem) String() string {
	if r == RA {
		return "A"
	}
	return "B"
}
func (r RuningItem) Adverse() RuningItem {
	return -1 * r
}

type App struct {
	cfg *config.AppConfig
	log *logrus.Entry

	startAt time.Time
	// runingItem A or B
	runingItem RuningItem
	cA         *Command
	cB         *Command
	argsAB     map[RuningItem][]string

	localVer  *semver.Version
	upgradeAt *time.Time

	upgCheckerStatus string
	upgCheckerCh     chan struct{}
}

func NewApp(cfg *config.AppConfig) (*App, error) {
	a := &App{
		cfg: cfg,
		log: logrus.WithFields(logrus.Fields{
			"module": "app",
			"app":    cfg.Name,
		}),
		startAt:    time.Now(),
		runingItem: RA,
		argsAB: map[RuningItem][]string{
			RA: cfg.Command.Args,
			RB: cfg.Command.Args,
		},
	}

	cmd, err := a.NewCommand(cfg.Command, a.runingItem)
	if err != nil {
		a.log.Errorf("new command %v failed, %s", cfg.Command, err)
		return nil, err
	}
	a.cA = cmd

	return a, nil
}

func (a *App) NewCommand(cfg config.CommandConfig, ab RuningItem) (*Command, error) {
	cfg.TmpDir = a.cfg.TmpDir
	err := os.MkdirAll(cfg.TmpDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("mkdir tmpdir %v failed, %s", cfg.TmpDir, err)
	}

	c := &Command{
		appName: a.cfg.Name,
		cfg:     cfg,
		log: logrus.WithFields(logrus.Fields{
			"module":  "command",
			"command": ab,
		}),
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

func (a *App) Start() {
	go a.cA.Start()
	if a.openUpgrade() {
		go a.routineCheckUpgrade()
	}
}

func (a *App) Upgrade() (err error) {
	defer func() {
		if err != nil {

		}
	}()

	ab := a.runingItem.Adverse()
	a.log.Debug("do Upgrade to ", ab)
	{
		if cmd := a.command(ab); cmd != nil {
			cmd.Exit()
		}
	}

	cmdCfg := a.cfg.Command
	cmdCfg.CommandFile, err = a.upg.PrepareCommand()
	if err != nil {
		a.log.Errorf("upg.Prepare failed, %s", err)
		return
	}
	cmdCfg.Args = a.argsAB[ab]

	var newCmd *Command
	newCmd, err = a.NewCommand(cmdCfg, ab)
	if err != nil {
		a.log.Errorf("upg.Prepare get command failed, %s", err)
		return
	}
	a.log.Debugf("got upgrade command successful, %s", newCmd)

	if v, err := newCmd.getCommandVersion(); err != nil {
		return fmt.Errorf("can not got command version, %s", err)
	} else {
		a.localVer = v
	}

	// a.log.Debugf("new command config %+v", cmdCfg)
	// a.log.Debugf("old command config %+v", c.cfg.Command)
	oldCmd := a.command(a.runingItem)
	if err := oldCmd.ReadyToExit(); err != nil {
		a.log.Errorf("old command ready to exit failed, %s", err)
	}
	if err = a.upg.Upgrade(oldCmd.cmd, newCmd.cmd); err != nil {
		a.log.Errorf("start new command failed, %s", err)
	}
	a.log.Debugf("upgrade precheck successful.")

	if a.runingItem == RA {
		if a.cB != nil {
			a.cB.Exit()
		}
		a.cB = newCmd
	} else {
		if a.cA != nil {
			a.cA.Exit()
		}
		a.cA = newCmd
	}
	now := time.Now()
	a.runingItem = a.runingItem.Adverse()
	a.upgradeAt = &now

	go newCmd.Start()

	return
}

func (c *App) routineCheckUpgrade() {
	if c.upg == nil {
		return
	}
	if c.upgCheckerStatus == "RUNNING" {
		return
	}

	c.upgCheckerStatus = "RUNNING"
	defer func() {
		c.upgCheckerStatus = "STOPED"
	}()
	c.upgCheckerCh = make(chan struct{})
	var (
		tc   = time.NewTimer(time.Second * 3)
		wait = time.Second * 60
	)
	defer tc.Stop()
	for {
		select {
		case <-tc.C:
			ok, newV, err := c.upg.NeedUpgrade(c.localVer)
			if err != nil {
				c.log.Warnf("upgrader check is latest failed, %s", err)
				tc.Reset(wait)
				continue
			}
			if ok {
				c.log.Infof("start to upgarde to %s", newV)
				c.Upgrade()
			}

			tc.Reset(wait)
		case <-c.upgCheckerCh:
			return
		}
	}
}

func (a *App) StopUpgrader() {
	if a.upgCheckerStatus != "RUNNING" {
		return
	}
	select {
	case <-a.upgCheckerCh:
	default:
		close(a.upgCheckerCh)
	}
}

func (a *App) StartUpgrader() {
	if a.upgCheckerStatus == "RUNNING" {
		return
	}
	go a.routineCheckUpgrade()
}

func (a *App) ExitSpare() {
	cmd := a.command(a.runingItem.Adverse())
	if cmd == nil {
		return
	}
	cmd.Exit()
}

func (a *App) Restart() error {
	a.ExitSpare()
	cmd := a.runningCommand()
	cmd.Exit()

	ncmd, err := a.NewCommand(a.cfg.Command, a.runingItem)
	if err != nil {
		return err
	}
	ncmd.restart()
	if a.runingItem == RA {
		a.cA = ncmd
	} else {
		a.cB = ncmd
	}
	return nil
}

func (a *App) openUpgrade() bool {
	return a.upg != nil
}

func (a *App) command(ab RuningItem) *Command {
	if ab == RB {
		return a.cB
	}
	return a.cA
}
func (a *App) runningCommand() *Command {
	return a.command(a.runingItem)
}
