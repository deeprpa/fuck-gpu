package daemon

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver"
	"github.com/deeprpa/fuck-gpu/config"
	"github.com/ygpkg/yg-go/logs"
)

type RuntimeInstance struct {
	AppName string
	Index   int

	ctx context.Context
	cfg config.AppConfig

	cmd     *exec.Cmd
	errExit chan error

	firstStartedAt *time.Time
	startedAt      *time.Time
	readyExitAt    *time.Time

	localVer *semver.Version

	chExitRoutine chan struct{}
	retryTimes    time.Duration
	isRestarting  bool
	isStarted     bool
	logOutput     *prefixedLineWriter
}

type prefixedLineWriter struct {
	prefix string
	output io.Writer

	mu     sync.Mutex
	buffer bytes.Buffer
}

func newPrefixedLineWriter(output io.Writer, prefix string) *prefixedLineWriter {
	return &prefixedLineWriter{output: output, prefix: prefix}
}

func (w *prefixedLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.buffer.Write(p); err != nil {
		return 0, err
	}

	for {
		data := w.buffer.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}

		line := string(data[:idx])
		if _, err := fmt.Fprintf(w.output, "%s | %s\n", w.prefix, line); err != nil {
			return 0, err
		}

		w.buffer.Next(idx + 1)
	}

	return len(p), nil
}

func (w *prefixedLineWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.buffer.Len() == 0 {
		return nil
	}

	line := w.buffer.String()
	w.buffer.Reset()
	_, err := fmt.Fprintf(w.output, "%s | %s\n", w.prefix, line)
	return err
}

func (c *RuntimeInstance) checkProcessStatus() {
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

func (c *RuntimeInstance) Start() error {
	// 防止重复启动
	if c.isStarted {
		logs.DebugContextf(c.ctx, "already started, skipping")
		return nil
	}
	c.isStarted = true

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

func (c *RuntimeInstance) restart() error {
	// 防止重复重启
	if c.isRestarting {
		logs.DebugContextf(c.ctx, "already restarting, skipping")
		return nil
	}
	c.isRestarting = true
	defer func() { c.isRestarting = false }()

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

func (c *RuntimeInstance) waitProcessExit() {
	defer func() {
		if c.logOutput != nil {
			_ = c.logOutput.Flush()
		}
	}()

	err := c.cmd.Wait()
	if err != nil && c.errExit != nil {
		c.errExit <- err
		logs.ErrorContextf(c.ctx, "pcocess exit. %s", err)
		return
	}
}

func (c *RuntimeInstance) ReadyToExit() error {
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

func (c *RuntimeInstance) Exit() error {
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

func (c *RuntimeInstance) getCommand(cmdCfg config.CommandConfig) (*exec.Cmd, error) {
	// Get base port from web app config
	basePort := 0

	// Process command with template support for instance index
	cmdStr := applyTemplate(cmdCfg.Command, c.Index, basePort)

	// Process args with template support for instance index
	args := make([]string, len(cmdCfg.Args))
	for i, arg := range cmdCfg.Args {
		args[i] = applyTemplate(arg, c.Index, basePort)
	}

	logs.DebugContextf(c.ctx, "Creating command with: %s %v", cmdStr, args)

	cmd := exec.Command(cmdStr, args...)
	logPrefix := fmt.Sprintf("%s-%d", c.AppName, c.Index)
	lineWriter := newPrefixedLineWriter(os.Stdout, logPrefix)
	c.logOutput = lineWriter
	if cmdCfg.WorkDir != "" {
		cmd.Dir = cmdCfg.WorkDir
	}
	if len(cmdCfg.Envs) > 0 {
		if cmd.Env == nil {
			cmd.Env = make([]string, 0, len(cmdCfg.Envs))
		}
		for _, env := range cmdCfg.Envs {
			// Apply template to environment variable values
			envValue := applyTemplate(env.Value, c.Index, basePort)
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", env.Key, envValue))
		}
	}
	cmd.Stderr = lineWriter
	cmd.Stdout = lineWriter
	return cmd, nil
}

// applyTemplate applies template variables to a string
// idx: instance index (0, 1, 2, ...)
// basePort: base port from web_app config (default 0 if not a web app)
func applyTemplate(s string, idx int, basePort int) string {
	// Replace {{index}} with instance index
	s = strings.ReplaceAll(s, "{{index}}", fmt.Sprintf("%d", idx))
	// Replace {{port}} with instance port (basePort + index)
	if basePort > 0 {
		s = strings.ReplaceAll(s, "{{port}}", fmt.Sprintf("%d", basePort+idx))
	}
	return s
}

func (c *RuntimeInstance) waitProcess(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	st, err := p.Wait()
	if err != nil {
		logs.ErrorContextf(c.ctx, "wait exit failed, %s, %v", err, st)
	}
	logs.ErrorContextf(c.ctx, "exit %v", st)
}
