package controller

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/discovery"
	"github.com/mgoltzsche/kubemate/pkg/ingress"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/utils"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/watch"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

func InstallDeviceController(genericServer *genericapiserver.GenericAPIServer, deviceName string, devices, clusterTokens storage.Interface, discovery *discovery.DeviceDiscovery, wifi *wifi.Wifi, wifiPasswords storage.Interface, dataDir, manifestDir string, docker bool, kubeletArgs []string, ingressCtrl *ingress.IngressController) {
	logger := logrus.NewEntry(logrus.StandardLogger())
	k3sRunner := runner.New(logger.WithField("proc", "k3s"))
	criDockerdRunner := runner.New(logger.WithField("proc", "cri-dockerd"))
	controllers := newControllerManager(logrus.WithField("comp", "controller-manager"))
	logger = logger.WithField("comp", "device-controller")
	genericServer.AddPostStartHookOrDie("kubemate", func(ctx genericapiserver.PostStartHookContext) error {
		// Add CRDs to k3s' manifest directory
		err := copyManifests(manifestDir, filepath.Join(dataDir, "server", "manifests"))
		if err != nil {
			return fmt.Errorf("copy default manifests into data dir: %w", err)
		}
		// Update device resource's status
		k3sRunner.Reporter = func(cmd runner.Command) {
			if cmd.Status.State == runner.ProcessStateFailed {
				logrus.Warnf("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
			} else {
				logrus.Infof("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
			}
			device := &deviceapi.Device{}
			err := devices.Update(deviceName, device, func() (resource.Resource, error) {
				device.Status.Generation = device.Generation
				if device.Status.State != deviceapi.DeviceStateTerminating {
					// TODO: map properly
					device.Status.State = deviceapi.DeviceState(cmd.Status.State)
				}
				device.Status.Message = cmd.Status.Message
				device.Status.Address = fmt.Sprintf("https://%s", genericServer.ExternalAddress)
				device.Status.Current = true
				return device, nil
			})
			if err != nil {
				logrus.WithError(err).Error("failed to update device status")
			}
		}
		goCtx, cancel := context.WithCancel(context.Background()) // TODO: fix termination
		go func() {
			<-ctx.StopCh
			cancel()
		}()
		if docker {
			// Launch the docker shim
			criDockerdRunner.Reporter = func(cmd runner.Command) {
				if cmd.Status.State == runner.ProcessStateFailed {
					logrus.Errorf("cri-dockerd %s: %s", cmd.Status.State, cmd.Status.Message)
				} else {
					logrus.Infof("cri-dockerd %s: %s", cmd.Status.State, cmd.Status.Message)
				}
			}
			// TODO: make this work with --network-plugin=cni (which is the new default)
			criDockerdRunner.Start(goCtx, runner.Cmd("cri-dockerd", "--network-plugin=kubenet"))
		}
		// Listen for device spec changes
		deviceWatch, err := devices.Watch(goCtx, "")
		if err != nil {
			return err
		}
		clusterTokenWatch, err := clusterTokens.Watch(goCtx, "")
		if err != nil {
			return err
		}
		err = reconcileCommand(devices, clusterTokens, wifiPasswords, deviceName, wifi, discovery, dataDir, docker, kubeletArgs, k3sRunner, controllers, ingressCtrl, logger)
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
		err = reconcileOnNetworkInterfaceLinkUpdate(goCtx, reconcileRequests)
		if err != nil {
			return err
		}
		go func() {
			for _ = range reconcileRequests {
				err = reconcileCommand(devices, clusterTokens, wifiPasswords, deviceName, wifi, discovery, dataDir, docker, kubeletArgs, k3sRunner, controllers, ingressCtrl, logger)
				if err != nil {
					logger.WithError(err).Error("failed to reconcile device command")
					time.Sleep(time.Second)
					go scheduleReconciliation(reconcileRequests)
					continue
				}
			}
			logger.Debug("device controller terminated")
		}()
		return nil
	})
	genericServer.AddPreShutdownHookOrDie("kubemate", func() error {
		d := &deviceapi.Device{}
		err := devices.Update(deviceName, d, func() (resource.Resource, error) {
			d.Status.State = deviceapi.DeviceStateTerminating
			return d, nil
		})
		if err != nil {
			logrus.Error("setting terminating device state: %w", err)
		}
		controllers.Stop()
		ingressCtrl.Stop()
		err = k3sRunner.Stop()
		if err != nil {
			logrus.Error(fmt.Errorf("terminate k3s: %w", err))
		}
		err = criDockerdRunner.Stop()
		if err != nil {
			logrus.Error(fmt.Errorf("terminate cri-dockerd: %w", err))
		}
		err = wifi.Close()
		if err != nil {
			logrus.Error(fmt.Errorf("terminate wifi services: %w", err))
		}
		return nil
	})
}

func scheduleReconciliation(ch chan<- struct{}) {
	defer recover()
	ch <- struct{}{}
}

func reconcileCommand(devices, clusterTokens, wifiPasswords storage.Interface, deviceName string, w *wifi.Wifi, discovery *discovery.DeviceDiscovery, dataDir string, docker bool, kubeletArgs []string, k3s *runner.Runner, controllers *controllerManager, ingressCtrl *ingress.IngressController, logger *logrus.Entry) error {
	logger.Debug("reconciling device")
	d := deviceapi.Device{}
	err := devices.Get(deviceName, &d)
	if err != nil {
		return err
	}
	// Reconcile wifi
	switch d.Spec.Wifi.Mode {
	case deviceapi.WifiModeAccessPoint:
		err = setWifiCountry(&d, devices, w, logger)
		if err != nil {
			return err
		}
		wifiPassword := deviceapi.WifiPassword{}
		err = wifiPasswords.Get(deviceapi.AccessPointPasswordKey, &wifiPassword)
		if err != nil {
			return err
		}
		err = w.StartAccessPoint(deviceName, wifiPassword.Data.Password)
		if err != nil {
			return err
		}
	case deviceapi.WifiModeStation:
		w.StopAccessPoint()
		err = setWifiCountry(&d, devices, w, logger)
		if err != nil {
			return err
		}
		err = w.StartWifiInterface()
		if err != nil {
			return err
		}
		var pw deviceapi.WifiPassword
		ssid := d.Spec.Wifi.Station.SSID
		if ssid == "" {
			logger.Warn("no ssid configured to connect to")
		} else {
			err = wifiPasswords.Get(ssidToResourceName(ssid), &pw)
			if err != nil {
				logger.WithError(err).WithField("ssid", ssid).Warn("no password configured for wifi network")
			}
		}
		err = w.StartStation(ssid, pw.Data.Password)
		if err != nil {
			return err
		}
	default:
		w.StopStation()
		w.StopAccessPoint()
		err = w.StopWifiInterface()
		if err != nil {
			return err
		}
	}

	// Reconcile k3s
	if m := d.Spec.Mode; m != deviceapi.DeviceModeServer && m != deviceapi.DeviceModeAgent {
		logger.Warnf("unsupported device mode %q specified", d.Spec.Mode)
		return nil
	}
	ips, err := discovery.ExternalIPs()
	if err != nil {
		return err
	}
	nodeIP := ips[0]
	var args []string
	fn := func() error {
		switch d.Spec.Mode {
		case deviceapi.DeviceModeServer:
			args = buildK3sServerArgs(&d, nodeIP, dataDir, docker, kubeletArgs, clusterTokens)
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
			args = buildK3sAgentArgs(&server, joinAddr, nodeIP, dataDir, docker, kubeletArgs, clusterTokens)
		}
		return nil
	}
	var statusMessage string
	if err = fn(); err != nil {
		logger.WithError(err).Warn("failed to reconcile device")
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
		err = discovery.Advertise(&d, ips)
		if err != nil {
			return err
		}
	}
	if len(args) > 0 {
		if d.Status.State == deviceapi.DeviceStateTerminating {
			k3s.Stop()
		} else {
			ctx := context.Background()
			k3s.Start(ctx, runner.Cmd("/proc/self/exe", args...))
		}
		if d.Spec.Mode == deviceapi.DeviceModeServer && d.Status.State != deviceapi.DeviceStateTerminating {
			controllers.Start()
			ingressCtrl.Start()
		} else {
			controllers.Stop()
			ingressCtrl.Stop()
		}
	}
	return nil
}

func ssidToResourceName(ssid string) string {
	ssid = fmt.Sprintf("ssid-%s", ssid)
	return utils.TruncateName(ssid, utils.MaxResourceNameLength)
}

// setWifiCountry detects the wifi country and stores it with the Device resource.
func setWifiCountry(d *deviceapi.Device, devices storage.Interface, w *wifi.Wifi, logger *logrus.Entry) error {
	w.CountryCode = d.Spec.Wifi.CountryCode
	if w.CountryCode == "" {
		err := w.StartWifiInterface()
		if err != nil {
			return err
		}
		err = w.DetectCountry()
		if err != nil {
			return err
		}
		err = devices.Update(d.Name, d, func() (resource.Resource, error) {
			d.Spec.Wifi.CountryCode = w.CountryCode
			return d, nil
		})
		if err != nil {
			return err
		}
		logger.Infof("detected wifi country %s", w.CountryCode)
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

func buildK3sServerArgs(d *deviceapi.Device, nodeIP net.IP, dataDir string, docker bool, kubeletArgs []string, clusterTokens storage.Interface) []string {
	args := []string{
		"server",
		fmt.Sprintf("--node-external-ip=%s", nodeIP.String()),
		"--disable-cloud-controller",
		"--disable-helm-controller",
		"--disable=servicelb,traefik,metrics-server",
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
		args = append(args,
			"--container-runtime-endpoint=unix:///var/run/cri-dockerd.sock",
		)
	}
	for _, a := range kubeletArgs {
		args = append(args, fmt.Sprintf("--kubelet-arg=%s", a))
	}
	return args
}

func buildK3sAgentArgs(server *deviceapi.Device, joinAddress string, nodeIP net.IP, dataDir string, docker bool, kubeletArgs []string, clusterTokens storage.Interface) []string {
	args := []string{
		"agent",
		fmt.Sprintf("--node-external-ip=%s", nodeIP.String()),
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
		args = append(args,
			"--container-runtime-endpoint=unix:///var/run/cri-dockerd.sock",
		)
	}
	for _, a := range kubeletArgs {
		args = append(args, fmt.Sprintf("--kubelet-arg=%s", a))
	}
	return args
}

// TODO: reconcile only when relevant external iface changed - not whenever a new iface for a container comes up.
func reconcileOnNetworkInterfaceLinkUpdate(ctx context.Context, ch chan<- struct{}) error {
	linkCh := make(chan netlink.LinkUpdate)
	err := netlink.LinkSubscribe(linkCh, ctx.Done())
	if err != nil {
		return err
	}
	go func() {
		for _ = range linkCh {
			logrus.Debug("received network link update")
			ch <- struct{}{}
			time.Sleep(time.Second)
		}
	}()
	return nil
}
