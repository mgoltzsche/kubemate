package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcess(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	// does not work with SIGINT
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGQUIT} {
		t.Run(fmt.Sprintf("signal %d", int(sig)), func(t *testing.T) {
			pidFile, err := os.CreateTemp("", "process-test-child-pid-")
			require.NoError(t, err)
			pidFile.Close()
			defer os.Remove(pidFile.Name())
			p, err := StartProcess(logrus.NewEntry(logger), sig, time.Second, Cmd("sh", "-c", fmt.Sprintf(`
				sleep 20 &
				echo $! > %s
				echo exit signal is %[2]d. sleeping...
				exitGracefully() {
					echo received signal %[2]d
					exit 0
				}
				fakeReload() {
					echo received reload signal
				}
				trap fakeReload 1
				trap exitGracefully %[2]d
				wait
				echo first wait terminated by SIGHUP
				wait
		`, pidFile.Name(), int(sig))))
			require.NoError(t, err, "StartProcess")
			ch := make(chan error, 1)
			go func() {
				ch <- p.Wait()
				close(ch)
			}()
			select {
			case err := <-ch:
				t.Errorf("Wait() did not wait but returned immediately with error: %s", err)
				t.FailNow()
			case <-time.After(100 * time.Millisecond):
			}
			p.Signal(syscall.SIGHUP)
			select {
			case err := <-ch:
				t.Errorf("Wait() returned on SIGHUP: %s", err)
				t.FailNow()
			case <-time.After(100 * time.Millisecond):
			}
			b, err := os.ReadFile(pidFile.Name())
			require.NoError(t, err, "read pid file")
			pid := strings.TrimSpace(string(b))
			err = exec.Command("kill", "-0", pid).Run()
			require.NoErrorf(t, err, "child pid %s should exist", string(pid))
			require.NoError(t, err, "parse pid")
			done := make(chan struct{}, 1)
			go func() {
				p.Stop()
				err = <-ch
				assert.NoError(t, err, "Wait()")
				p.Stop()
				done <- struct{}{}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(7 * time.Second):
				t.Errorf("timed out waiting for stop")
				t.FailNow()
			}
			err = exec.Command("kill", "-0", pid).Run()
			if err == nil {
				time.Sleep(100 * time.Millisecond)
				err = exec.Command("kill", "-0", pid).Run()
			}
			require.Error(t, err, "should have killed child pid %s", pid)
		})
	}
}

func TestProcessDisallowSIGINT(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	_, err := StartProcess(logrus.NewEntry(logger), syscall.SIGINT, time.Second, Cmd("true"))
	require.Error(t, err)
}
