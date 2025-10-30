package daemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/Masterminds/semver"
	"github.com/deeprpa/fuck-gpu/config"
	"github.com/ygpkg/yg-go/logs"
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
	cmd := exec.Command(cmdCfg.Command, cmdCfg.Args...)
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
