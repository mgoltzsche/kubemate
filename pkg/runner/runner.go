package runner

import (
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const cooldown = time.Second

type ProcessState string

const (
	ProcessStateRunning ProcessState = "running"
	ProcessStateFailed  ProcessState = "failed"
	ProcessStateExited  ProcessState = "exited"
)

type Command struct {
	Spec   CommandSpec
	Status CommandStatus
}

type CommandStatus struct {
	State   ProcessState
	Message string
}

type StatusReportFunc func(cmd Command)

func noopStatusReporter(cmd Command) {}

type CooldownError struct {
	error
	Duration time.Duration
}

type Runner struct {
	proc              *Proc
	mutex             sync.Mutex
	Reporter          StatusReportFunc
	TerminationSignal syscall.Signal
	terminated        time.Time
	logger            *logrus.Entry
}

func New(logger *logrus.Entry) *Runner {
	return &Runner{
		Reporter:          noopStatusReporter,
		TerminationSignal: syscall.SIGINT,
		logger:            logger,
	}
}

func (m *Runner) Stop() (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.proc != nil {
		err = m.proc.Stop()
		m.proc = nil
	}
	return
}

func (m *Runner) SignalReload() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.proc == nil || m.proc.proc == nil {
		return fmt.Errorf("signal %s to reload: not running", m.proc.cmd.Command)
	}
	err := m.proc.proc.Signal(syscall.SIGHUP)
	if err != nil {
		return fmt.Errorf("signal %s to reload: %w", m.proc.cmd.Command, err)
	}
	return nil
}

func (m *Runner) Start(cmd CommandSpec) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.proc != nil {
		if m.proc.cmd.String() == cmd.String() {
			// Don't restart when corresponding process is already running
			return nil
		}
		err := m.proc.Stop()
		if err != nil {
			return err
		}
		m.proc = nil
	}
	now := time.Now()
	if r := m.terminated.Add(cooldown); now.Before(r) {
		d := r.Sub(now)
		return &CooldownError{
			error:    fmt.Errorf("refusing to restart %s during cooldown period", cmd.Command),
			Duration: d,
		}
	}
	p, err := StartProcess(m.logger, m.TerminationSignal, cmd)
	m.terminated = time.Now()
	if err != nil {
		m.report(cmd, CommandStatus{
			State:   ProcessStateFailed,
			Message: fmt.Sprintf("failed to start process: %s", err),
		})
		return err
	}
	m.proc = p
	// wait so that the process has enough time to register SIGHUP handler (which is called afterwards to reload config in case of dnsmasq)
	time.Sleep(50 * time.Millisecond)
	go func() {
		m.report(cmd, CommandStatus{
			State: ProcessStateRunning,
		})
		err := p.Wait()
		m.terminated = time.Now()
		s := CommandStatus{}
		s.State = ProcessStateExited
		if err != nil {
			s.State = ProcessStateFailed
			s.Message = fmt.Sprintf("process failed: %s", err)
		}
		m.mutex.Lock()
		m.proc = nil
		m.mutex.Unlock()
		m.report(cmd, s)
	}()
	return nil
}

func (m *Runner) report(c CommandSpec, s CommandStatus) {
	m.Reporter(Command{
		Spec:   c,
		Status: s,
	})
}
