package runner

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type ProcessState string

const (
	ProcessStateRunning    ProcessState = "running" // TODO: add ready check to set this state
	ProcessStateFailed     ProcessState = "failed"
	ProcessStateTerminated ProcessState = "terminated"
)

type Command struct {
	Spec   CommandSpec
	Status CommandStatus
}

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

type CommandStatus struct {
	State   ProcessState
	Message string
}

type Runner struct {
	mutex sync.Mutex
	spec  chan CommandSpec
	done  sync.WaitGroup
}

func NewRunner() *Runner {
	return &Runner{
		spec: make(chan CommandSpec),
	}
}

func (l *Runner) SetCommand(p CommandSpec) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.spec != nil {
		l.spec <- p
	}
}

func (l *Runner) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	close(l.spec)
	l.done.Wait()
	l.spec = nil
	return nil
}

func (l *Runner) Start() <-chan Command {
	ch := make(chan Command)
	l.done.Add(1)
	go l.run(ch)
	return ch
}

func (l *Runner) run(ch chan<- Command) {
	defer l.done.Done()
	var p *Process
	for c := range l.spec {
		if p == nil || c.String() != p.spec.String() {
			if p != nil {
				p.Stop()
			}
			p = startProcess(c, ch)
		}
	}
	if p != nil {
		p.Stop()
	}
	close(ch)
}

type Process struct {
	spec    CommandSpec
	proc    *os.Process
	running *sync.WaitGroup
}

func (p *Process) Stop() {
	err := p.proc.Signal(os.Interrupt)
	if err != nil && err != os.ErrProcessDone {
		logrus.Warnf("interrupting process: %s", err)
	}
	p.Wait()
}

func (p *Process) Wait() {
	p.running.Wait()
}

func startProcess(cmd CommandSpec, ch chan<- Command) *Process {
	c := exec.Command(cmd.Command, cmd.Args...)
	c.Env = os.Environ()
	stdout, err := c.StdoutPipe()
	if err != nil {
		ch <- Command{
			Spec: cmd,
			Status: CommandStatus{
				State:   ProcessStateFailed,
				Message: fmt.Sprintf("failed to start process: %s", err),
			},
		}
		return nil
	}
	stderr, err := c.StderrPipe()
	if err != nil {
		stdout.Close()
		ch <- Command{
			Spec: cmd,
			Status: CommandStatus{
				State:   ProcessStateFailed,
				Message: fmt.Sprintf("failed to start process: %s", err),
			},
		}
		return nil
	}
	err = c.Start()
	if err != nil {
		ch <- Command{
			Spec: cmd,
			Status: CommandStatus{
				State:   ProcessStateFailed,
				Message: fmt.Sprintf("failed to start process: %s", err),
			},
		}
		return nil
	}
	go func() {
		defer stdout.Close()
		_, _ = io.Copy(os.Stdout, stdout)
	}()
	go func() {
		defer stderr.Close()
		_, _ = io.Copy(os.Stderr, stderr)
	}()
	ch <- Command{
		Spec: cmd,
		Status: CommandStatus{
			State: ProcessStateRunning,
		},
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := c.Wait()
		s := CommandStatus{}
		s.State = ProcessStateTerminated
		if err != nil {
			s.State = ProcessStateFailed
			s.Message = fmt.Sprintf("process failed: %s", err)
		}
		ch <- Command{
			Spec:   cmd,
			Status: s,
		}
	}()
	return &Process{
		spec:    cmd,
		proc:    c.Process,
		running: wg,
	}
}
