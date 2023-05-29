package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

type CommandSpec struct {
	Command string
	Args    []string
}

func (c *CommandSpec) String() string {
	cmd := make([]string, 1, len(c.Args)+1)
	cmd[0] = c.Command
	cmd = append(cmd, c.Args...)
	for i, s := range cmd {
		if strings.Contains(s, " ") {
			cmd[i] = strconv.Quote(s)
		}
	}
	return strings.Join(cmd, " ")
}

func Cmd(cmd string, args ...string) CommandSpec {
	return CommandSpec{
		Command: cmd,
		Args:    args,
	}
}

type Proc struct {
	proc                   *os.Process
	pgid                   int
	cmd                    CommandSpec
	running                bool
	wg                     sync.WaitGroup
	terminationSignal      syscall.Signal
	terminationGracePeriod time.Duration
	err                    error
	logger                 *logrus.Entry
}

// StartProcess starts a process.
func StartProcess(logger *logrus.Entry, terminationSignal syscall.Signal, terminationGracePeriod time.Duration, cmd CommandSpec) (*Proc, error) {
	if terminationSignal == syscall.SIGINT {
		return nil, fmt.Errorf("process %s: termination signal SIGINT is not supported to terminate a process group", cmd.Command)
	}
	c := exec.Command(cmd.Command, cmd.Args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Env = os.Environ()
	stdout, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, err
	}
	logger.Debugf("starting process: %s %s", cmd.Command, strings.Join(cmd.Args, " "))
	err = c.Start()
	if err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start %s process: %w", cmd.Command, err)
	}
	go streamLines(stdout, logger, func(line string) {
		if !parseAndLogProcessLogLine(line, logger) {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "fail") || strings.Contains(lower, "error") || strings.Contains(lower, "warn") || strings.Contains(lower, "invalid") {
				logger.Error(line)
			} else {
				logger.Info(line)
			}
		}
	})
	go streamLines(stderr, logger, func(line string) {
		if !parseAndLogProcessLogLine(line, logger) {
			logger.Warn(line)
		}
	})
	pgid, err := syscall.Getpgid(c.Process.Pid)
	if err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("get %s process group: %w", cmd.Command, err)
	}
	p := &Proc{
		proc:                   c.Process,
		pgid:                   pgid,
		cmd:                    cmd,
		running:                true,
		terminationSignal:      terminationSignal,
		terminationGracePeriod: terminationGracePeriod,
		logger:                 logger.WithField("pid", c.Process.Pid),
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		err := c.Wait()
		_ = syscall.Kill(-p.pgid, syscall.SIGKILL)
		log := p.logger
		if err != nil {
			log = log.WithError(err)
		}
		log.Debugf("%s process exited", p.cmd.Command)
		p.running = false
		p.err = err
	}()
	return p, nil
}

// Pid returns the process ID.
func (p *Proc) Pid() int {
	return p.proc.Pid
}

// Running returns true if the process is still running.
func (p *Proc) Running() bool {
	return p.running
}

// Wait waits for the process to terminate.
func (p *Proc) Wait() error {
	p.wg.Wait()
	return p.err
}

// Signal sends the given signal to the process.
func (p *Proc) Signal(sig syscall.Signal) error {
	p.logger.Debugf("sending signal %d to %s process", sig, p.cmd.Command)
	return p.proc.Signal(sig)
}

// Stop stops the process.
func (p *Proc) Stop() {
	p.logger.Debugf("stopping %s process (signal %d)", p.cmd.Command, p.terminationSignal)
	err := p.proc.Signal(p.terminationSignal)
	if err != nil && err != os.ErrProcessDone {
		p.logger.Warnf("failed to send signal %d to %s process: %s", p.terminationSignal, p.cmd.Command, err)
	}
	ch := make(chan struct{})
	go func() {
		_ = p.Wait()
		ch <- struct{}{}
	}()
	select {
	case <-ch:
	case <-time.After(p.terminationGracePeriod):
		p.logger.Errorf("killing %s process since graceful termination timed out", p.cmd.Command)
		_ = syscall.Kill(-p.pgid, syscall.SIGKILL)
		_ = p.Wait()
	}
}
