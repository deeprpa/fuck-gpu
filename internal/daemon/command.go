package daemon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/deeprpa/fuck-gpu/config"
	"github.com/deeprpa/fuck-gpu/pkgs/logs"
)

type Command struct {
	appName string
	ctx     context.Context
	cfg     config.CommandConfig

	cmd     *exec.Cmd
	errExit chan error

	firstStartedAt *time.Time
	startedAt      *time.Time
	readyExitAt    *time.Time

	localVer *semver.Version

	chExitRoutine chan struct{}
	retryTimes    time.Duration
}

func (c *Command) checkProcessStatus() {
	tickWait := time.Second * 3
	tc := time.NewTimer(tickWait)
	for {
		select {
		case <-tc.C:
			if c.cmd == nil || c.cmd.Process == nil {
				logs.DebugContextf(c.ctx, "command status NIL")
				tc.Reset(tickWait)
				continue
			}

			_, err := os.FindProcess(c.cmd.Process.Pid)
			if err != nil {
				logs.WarnContextf(c.ctx, "got process(%v) failed, %s", c.cmd.Process.Pid, err)
			}

			if c.cmd.ProcessState == nil {
				if c.cmd.Process != nil {
					if c.retryTimes > 1 {
						c.retryTimes = 1
					}
					logs.DebugContextf(c.ctx, "command(%v) status RUNING", c.cmd.Process.Pid)
				} else {
					logs.DebugContextf(c.ctx, "command status UNKNOWN")
				}
				tc.Reset(tickWait)
				continue
			}
			logs.DebugContextf(c.ctx, "command(%v) is exited: %v", c.cmd.Process.Pid, c.cmd.ProcessState.Exited())
			if c.cmd.ProcessState.Exited() && c.readyExitAt == nil {
				c.restart()
			}
			tc.Reset(tickWait)

		case exErr := <-c.errExit:

			if c.readyExitAt == nil {
				logs.InfoContextf(c.ctx, "command(%v) exited, %s", c.cmd.Process.Pid, exErr)
				c.restart()
			} else {
				logs.InfoContextf(c.ctx, "command(%v) exited, %s", c.cmd.Process.Pid, exErr)
			}
			tc.Reset(tickWait)

		case <-c.chExitRoutine:
			logs.DebugContextf(c.ctx, "return check process status.")
			return
		}
	}
}

func (c *Command) Start() error {
	now := time.Now()
	c.startedAt = &now
	if c.firstStartedAt == nil {
		c.firstStartedAt = c.startedAt
	}

	if err := c.cmd.Start(); err != nil {
		logs.ErrorContextf(c.ctx, "start %v failed, %s", c.cmd, err)
		return err
	}
	go c.waitProcessExit()
	go c.checkProcessStatus()
	logs.DebugContextf(c.ctx, "starting, ", c.cmd.Process.Pid)

	return nil
}

func (c *Command) restart() error {
	waitTime := time.Second * c.retryTimes * 2
	logs.InfoContextf(c.ctx, "restarting later %s", waitTime)
	time.Sleep(waitTime)
	if c.cmd.Process != nil {
		c.waitProcess(c.cmd.Process.Pid)
		c.cmd.Process = nil
		c.cmd.ProcessState = nil
	}
	if c.readyExitAt != nil {
		return nil
	}
	c.retryTimes++
	if err := c.cmd.Start(); err != nil {
		logs.ErrorContextf(c.ctx, "start %v failed, %s", c.cmd, err)
		return err
	}
	go c.waitProcessExit()
	logs.DebugContextf(c.ctx, "restarting, ", c.cmd.Process.Pid)
	return nil
}

func (c *Command) waitProcessExit() {
	err := c.cmd.Wait()
	if err != nil && c.errExit != nil {
		c.errExit <- err
		logs.ErrorContextf(c.ctx, "pcocess exit. %s", err)
		return
	}
}

func (c *Command) ReadyToExit() error {
	now := time.Now()
	c.readyExitAt = &now
	go func() {
		tc := time.NewTimer(30 * time.Minute)
		select {
		case <-tc.C:
			c.Exit()
		case <-c.chExitRoutine:
			return
		}
	}()
	return nil
}

func (c *Command) Exit() error {
	if c.cmd != nil && c.cmd.Process != nil {
		if err := c.cmd.Process.Kill(); err != nil {
			logs.ErrorContextf(c.ctx, "exited command failed, %s", err)
		}
		c.waitProcess(c.cmd.Process.Pid)
	}
	select {
	case <-c.chExitRoutine:
	default:
		close(c.chExitRoutine)
	}
	return nil
}

func (c *Command) getCommand(cmdCfg config.CommandConfig) (*exec.Cmd, error) {
	if strings.HasPrefix(cmdCfg.Path, "http") || strings.HasPrefix(cmdCfg.Path, "https") {
		cmdCfg.Mode = "http"
	}
	var cmd *exec.Cmd
	switch cmdCfg.Mode {
	case "", "local":
		cmd = exec.Command(cmdCfg.Path, cmdCfg.Args...)

	case "http", "https":
		execPath, err := c.downloadCommand(cmdCfg.Path)
		if err != nil {
			logs.ErrorContextf(c.ctx, "got remote exec failed, %s", err)
			return nil, err
		}
		cmd = exec.Command(execPath, cmdCfg.Args...)

	default:
		return nil, fmt.Errorf("not support command mode %s", cmdCfg.Mode)
	}

	if cmdCfg.WorkDir != "" {
		cmd.Dir = cmdCfg.WorkDir
	}
	if len(cmdCfg.Envs) > 0 {
		if cmd.Env == nil {
			cmd.Env = make([]string, 0, len(cmdCfg.Envs))
		}
		for k, v := range cmdCfg.Envs {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	cmd.Stderr = os.Stdout
	// cmd.Stdout = os.Stdout
	return cmd, nil
}

func (c *Command) downloadCommand(path string) (string, error) {
	logs.DebugContextf(c.ctx, "start download %s", path)

	u, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse exec url %v failed, %s", path, err)
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("download exec file %v failed, %s", u.String(), err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("downlaod file %v wrong status code %e", u.String(), resp.Status)
	}

	execName := fmt.Sprintf("%s-%s", c.appName, time.Now().Format("20060102T150405"))
	execPath := filepath.Join(c.cfg.TmpDir, "bin", execName)
	if err := c.checkOrCreateDir("bin"); err != nil {
		return "", err
	}
	f, err := os.Create(execPath)
	if err != nil {
		return execPath, fmt.Errorf("create tmp exec file %v failed, %s", execPath, err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return "", fmt.Errorf("write exec file failed, %s", err)
	}

	if err := os.Chmod(execPath, 0755); err != nil {
		return "", fmt.Errorf("chmod exec file failed, %s", err)
	}

	logs.InfoContextf(c.ctx, "downlaod exec successful, %s", execPath)
	return execPath, nil
}

func (c *Command) checkOrCreateDir(subPath string) error {
	dirPath := c.cfg.TmpDir
	if subPath != "" {
		dirPath = filepath.Join(c.cfg.TmpDir, subPath)
	}

	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return fmt.Errorf("mkdir %v failed, %s", dirPath, err)
	}
	return nil
}

func (c *Command) getCommandVersion() (*semver.Version, error) {
	if c.localVer != nil {
		return c.localVer, nil
	}
	cmd := exec.Command(c.cmd.Path, c.cfg.VerArgs...)
	bs, err := cmd.CombinedOutput()
	if err != nil {
		logs.ErrorContextf(c.ctx, "get command (%v) version failed, %s", cmd.Path, err)
		return nil, err
	}
	logs.DebugContextf(c.ctx, "get command (%v) version failed, %s", cmd.Path, bs)
	verStr := strings.TrimPrefix(strings.TrimSpace(string(bs)), "v")
	v, err := semver.NewVersion(verStr)
	if err != nil {
		logs.ErrorContextf(c.ctx, "got command version(%s) failed, %s", bs, err)
		return nil, err
	}
	c.localVer = v
	return v, nil
}

func (c *Command) waitProcess(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	st, err := p.Wait()
	if err != nil {
		logs.ErrorContextf(c.ctx, "wait exit failed, %s, %v", err, st)
	}
	logs.ErrorContextf(c.ctx, "exit %v", st)
	return
}
