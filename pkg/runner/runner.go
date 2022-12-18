package runner

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
)

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

type Runner struct {
	proc     *Proc
	mutex    sync.Mutex
	Reporter StatusReportFunc
	logger   *logrus.Entry
}

func New(logger *logrus.Entry) *Runner {
	return &Runner{
		Reporter: noopStatusReporter,
		logger:   logger,
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
	p, err := StartProcess(m.logger, cmd)
	if err != nil {
		m.report(cmd, CommandStatus{
			State:   ProcessStateFailed,
			Message: fmt.Sprintf("failed to start process: %s", err),
		})
		return err
	}
	m.proc = p
	go func() {
		m.report(cmd, CommandStatus{
			State: ProcessStateRunning,
		})
		err := p.Wait()
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
