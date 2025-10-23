package daemon

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deeprpa/fuck-gpu/config"
	"github.com/ygpkg/yg-go/lifecycle"
)

type Daemon struct {
	cfg *config.GlobalConfig
	lc  *lifecycle.LifeCycle

	apps []*App
}

func NewDaemon(lc *lifecycle.LifeCycle, cfg *config.GlobalConfig) (*Daemon, error) {
	d := &Daemon{
		cfg:  cfg,
		lc:   lc,
		apps: []*App{},
	}
	err := os.MkdirAll(cfg.TmpDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("mkdir tmpdir %v failed, %s", cfg.TmpDir, err)
	}

	for _, appCfg := range d.cfg.Apps {
		if appCfg.TmpDir == "" {
			appCfg.TmpDir = filepath.Join(cfg.TmpDir, appCfg.Name)
		}
		app, err := NewApp(lc.Context(), appCfg)
		if err != nil {
			return nil, err
		}
		d.apps = append(d.apps, app)
	}
	return d, nil
}

func (d *Daemon) Run() error {
	for _, app := range d.apps {
		app.Start()
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

	Main  CmdStatus
	Spare *CmdStatus
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
		if cmd := app.cmd; cmd != nil {
			ast.Spare = &CmdStatus{}
			if cmd.startedAt != nil {
				ast.Spare.StartedAt = cmd.startedAt.String()
			}
			if cmd.firstStartedAt != nil {
				ast.Spare.FirstStartedAt = cmd.firstStartedAt.String()
			}
			if cmd.localVer != nil {
				ast.Spare.Version = cmd.localVer.String()
			}
			if cmd.readyExitAt != nil {
				ast.Spare.ReadyToExitAt = cmd.readyExitAt.String()
			}
			ast.Spare.RetryTimes = int(cmd.retryTimes)
			if cc := cmd.cmd; cc != nil {
				ast.Spare.Path = cc.Path
				if cc.Process != nil {
					ast.Spare.Pid = cc.Process.Pid
				}
			}
		}
		sts.Apps = append(sts.Apps, ast)
	}

	return sts
}

func (d *Daemon) App() *App {
	if len(d.apps) == 0 {
		return nil
	}
	return d.apps[0]
}
