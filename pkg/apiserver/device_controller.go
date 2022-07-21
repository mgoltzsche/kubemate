package apiserver

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

func installDeviceController(genericServer *genericapiserver.GenericAPIServer, devices, clusterTokens storage.Interface, deviceName string, discovery *DeviceDiscovery, dataDir, manifestDir string, docker bool) {
	k3sRunner := runner.NewRunner()
	genericServer.AddPostStartHookOrDie("kubemate", func(ctx genericapiserver.PostStartHookContext) error {
		// Add CRDs to k3s' manifest directory
		err := copyManifests(manifestDir, filepath.Join(dataDir, "server", "manifests"))
		if err != nil {
			return fmt.Errorf("copy default manifests into data dir: %w", err)
		}
		// Update device resource's status
		k3sCh := k3sRunner.Start()
		go func() {
			for cmd := range k3sCh {
				if cmd.Status.State == runner.ProcessStateFailed {
					logrus.Warnf("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
				} else {
					logrus.Printf("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
				}
				device := &deviceapi.Device{}
				err := devices.Update(deviceName, device, func() (resource.Resource, error) {
					device.Status.Generation = device.Generation
					// TODO: map properly
					device.Status.State = deviceapi.DeviceState(cmd.Status.State)
					device.Status.Message = cmd.Status.Message
					device.Status.Address = fmt.Sprintf("https://%s", genericServer.ExternalAddress)
					device.Status.Current = true
					return device, nil
				})
				if err != nil {
					logrus.WithError(err).Error("failed to update device status")
					continue
				}
			}
		}()
		if docker {
			// Launch the docker shim
			criDockerdRunner := runner.NewRunner()
			criDockerdCh := criDockerdRunner.Start()
			criDockerdRunner.SetCommand(runner.CommandSpec{
				Command: "cri-dockerd",
				Args:    []string{},
			})
			go func() {
				for cmd := range criDockerdCh {
					if cmd.Status.State == runner.ProcessStateFailed {
						logrus.Errorf("cri-dockerd %s: %s", cmd.Status.State, cmd.Status.Message)
					} else {
						logrus.Printf("cri-dockerd %s: %s", cmd.Status.State, cmd.Status.Message)
					}
				}
			}()
		}
		// Listen for device spec changes
		goCtx, cancel := context.WithCancel(context.Background()) // TODO: fix termination
		go func() {
			<-ctx.StopCh
			cancel()
		}()
		deviceWatch, err := devices.Watch(goCtx, "")
		if err != nil {
			return err
		}
		clusterTokenWatch, err := clusterTokens.Watch(goCtx, "")
		if err != nil {
			return err
		}
		err = reconcileCommand(devices, clusterTokens, deviceName, discovery, dataDir, docker, k3sRunner)
		if err != nil {
			return err
		}
		reconcileRequests := make(chan struct{}, 10)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer deviceWatch.Stop()
			for evt := range deviceWatch.ResultChan() {
				if evt.Type == watch.Modified {
					d := evt.Object.(*deviceapi.Device)
					if d.Name == deviceName {
						reconcileRequests <- struct{}{}
					}
				}
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer clusterTokenWatch.Stop()
			for _ = range clusterTokenWatch.ResultChan() {
				reconcileRequests <- struct{}{}
			}
		}()
		go func() {
			wg.Wait()
			close(reconcileRequests)
		}()
		go func() {
			for _ = range reconcileRequests {
				err = reconcileCommand(devices, clusterTokens, deviceName, discovery, dataDir, docker, k3sRunner)
				if err != nil {
					logrus.WithError(err).Error("failed to reconcile device command")
					go scheduleRetry(reconcileRequests)
					continue
				}
			}
			logrus.Debug("device controller terminated")
		}()
		return nil
	})
	genericServer.AddPreShutdownHookOrDie("kubemate", func() error {
		return k3sRunner.Close()
	})
}

func scheduleRetry(ch chan<- struct{}) {
	defer recover()
	time.Sleep(time.Second)
	ch <- struct{}{}
}

func reconcileCommand(devices, clusterTokens storage.Interface, deviceName string, discovery *DeviceDiscovery, dataDir string, docker bool, k3s *runner.Runner) error {
	logrus.Info("reconcile device")
	d := deviceapi.Device{}
	err := devices.Get(deviceName, &d)
	if err != nil {
		return err
	}
	if m := d.Spec.Mode; m != deviceapi.DeviceModeServer && m != deviceapi.DeviceModeAgent {
		logrus.Warnf("unsupported device mode %q specified", d.Spec.Mode)
		return nil
	}
	var args []string
	fn := func() error {
		switch d.Spec.Mode {
		case deviceapi.DeviceModeServer:
			args = buildK3sServerArgs(&d, dataDir, docker, clusterTokens)
		case deviceapi.DeviceModeAgent:
			if d.Spec.Server == "" {
				return fmt.Errorf("no server specified to join")
			}
			if d.Spec.Server == d.Name {
				return fmt.Errorf("cannot join itself")
			}
			var server deviceapi.Device
			err := devices.Get(d.Spec.Server, &server)
			if err != nil {
				return err
			}
			if server.Spec.Mode != deviceapi.DeviceModeServer {
				return fmt.Errorf("cannot join device %q since it doesn't run in %s mode but in mode %q", d.Spec.Server, deviceapi.DeviceModeServer, d.Spec.Mode)
			}
			joinAddr, err := joinAddress(&server)
			if err != nil {
				return err
			}
			// TODO: provide token as env var
			args = buildK3sAgentArgs(&server, joinAddr, dataDir, docker, clusterTokens)
		}
		return nil
	}
	var statusMessage string
	if err = fn(); err != nil {
		logrus.WithError(err).Warn("failed to reconcile device")
		statusMessage = err.Error()
	}
	if d.Status.Message != statusMessage {
		// Update device status
		err = devices.Update(d.Name, &d, func() (resource.Resource, error) {
			d.Status.Message = statusMessage
			return &d, nil
		})
		if err != nil {
			return err
		}
	}
	if d.Generation == d.Status.Generation {
		// TODO: advertize only when status changed
		err = discovery.Advertise(&d)
		if err != nil {
			return err
		}
	}
	if len(args) > 0 {
		k3s.SetCommand(runner.CommandSpec{
			Command: "/proc/self/exe",
			Args:    args,
		})
	}
	return nil
}

func joinAddress(d *deviceapi.Device) (string, error) {
	a := ""
	if d.Spec.Mode == deviceapi.DeviceModeServer {
		u, err := url.Parse(d.Status.Address)
		if err != nil {
			return "", fmt.Errorf("status.address %q of device %q is not a valid address", d.Status.Address, d.Name)
		}
		a = fmt.Sprintf("https://%s:6443", u.Hostname())
	}
	return a, nil
}

func buildK3sServerArgs(d *deviceapi.Device, dataDir string, docker bool, clusterTokens storage.Interface) []string {
	args := []string{
		"server",
		"--disable-cloud-controller",
		"--disable-helm-controller",
		"--no-deploy=servicelb,traefik,metrics-server",
		fmt.Sprintf("--kube-apiserver-arg=--token-auth-file=%s", "/etc/kubemate/tokens"),
		fmt.Sprintf("--data-dir=%s", dataDir),
	}
	token := &deviceapi.DeviceToken{}
	err := clusterTokens.Get(d.Name, token)
	if err != nil {
		logrus.Error(err)
	} else {
		args = append(args, fmt.Sprintf("--token=%s", token.Data.Token))
	}
	if docker {
		args = append(args, "--container-runtime-endpoint=unix:///var/run/cri-dockerd.sock")
	}
	return args
}

func buildK3sAgentArgs(server *deviceapi.Device, joinAddress string, dataDir string, docker bool, clusterTokens storage.Interface) []string {
	args := []string{
		"agent",
		fmt.Sprintf("--data-dir=%s", dataDir),
	}
	token := &deviceapi.DeviceToken{}
	err := clusterTokens.Get(server.Name, token)
	if err != nil {
		logrus.Warn(fmt.Errorf("join server %s: %w", server.Name, err))
		return nil
	}
	args = append(args,
		fmt.Sprintf("--server=%s", joinAddress),
		fmt.Sprintf("--token=%s", token.Data.Token),
	)
	if docker {
		args = append(args, "--docker")
	}
	return args
}
