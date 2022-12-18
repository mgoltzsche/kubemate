package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

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
	proc    *os.Process
	cmd     CommandSpec
	running sync.WaitGroup
	err     error
	logger  *logrus.Entry
}

func StartProcess(logger *logrus.Entry, cmd CommandSpec) (*Proc, error) {
	c := exec.Command(cmd.Command, cmd.Args...)
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
			if strings.Contains(lower, "fail") || strings.Contains(lower, "error") || strings.Contains(lower, "invalid") {
				logger.Warn(line)
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
	p := &Proc{
		proc:   c.Process,
		cmd:    cmd,
		logger: logger,
	}
	p.running.Add(1)
	go func() {
		defer p.running.Done()
		err := c.Wait()
		log := logger
		if err != nil {
			log = log.WithError(err)
		}
		log.Debugf("%s process exited", p.cmd.Command)
		p.err = err
	}()
	return p, nil
}

func (p *Proc) Wait() error {
	p.running.Wait()
	return p.err
}

func (p *Proc) Stop() error {
	p.logger.Debug("stopping process")
	err := p.proc.Signal(os.Interrupt)
	if err != nil && err != os.ErrProcessDone {
		return fmt.Errorf("stop %s process: %w", p.cmd.Command, err)
	}
	p.Wait()
	return nil
}
