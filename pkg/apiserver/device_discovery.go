package apiserver

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/mdns"
	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	mdnsZone           = "_kubemate._tcp"
	mdnsDiscoveryLabel = "kubemate.mgoltzsche.github.com/mdns-discovery"
)

type DeviceDiscovery struct {
	deviceName string
	port       int
	srv        *mdns.Server
}

func NewDeviceDiscovery(deviceName string, port int) *DeviceDiscovery {
	return &DeviceDiscovery{
		deviceName: deviceName,
		port:       port,
	}
}

func (d *DeviceDiscovery) Advertise() error {
	info := []string{"kubemate"}
	ips, err := publicIPs()
	if err != nil {
		if len(ips) == 0 {
			return fmt.Errorf("detect public IPs: %w", err)
		} else {
			logrus.WithError(err).Warn("error when detecting devices")
		}
	}
	svc, err := mdns.NewMDNSService(d.deviceName, mdnsZone, "", "", d.port, ips, info)
	if err != nil {
		return err
	}
	srv, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		return err
	}
	d.srv = srv
	return nil
}

func publicIPs() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(ifaces)-1)
	for _, iface := range ifaces {
		addrs, e := iface.Addrs()
		if e != nil {
			if err == nil {
				err = e
			}
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipnet.IP.To4()
			if v4 == nil || v4.IsLoopback() {
				continue
			}
			ips = append(ips, v4)
		}
	}
	return ips, err
}

func (d *DeviceDiscovery) Discover(store storage.Interface) error {
	return populateDevicesFromMDNS(d.deviceName, store)
}

func (d *DeviceDiscovery) Close() error {
	if s := d.srv; s != nil {
		err := s.Shutdown()
		d.srv = nil
		return err
	}
	return nil
}

func populateDevicesFromMDNS(deviceName string, devices storage.Interface) error {
	foundDevices := map[string]struct{}{
		deviceName: struct{}{},
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ch := make(chan *mdns.ServiceEntry, 4)
	go func() {
		for entry := range ch {
			d := &deviceapi.Device{}
			d.Name = strings.TrimRight(entry.Host, ".")
			foundDevices[d.Name] = struct{}{}
			if d.Name == deviceName {
				continue
			}
			err := devices.Get(d.Name, d)
			if errors.IsNotFound(err) {
				logrus.Infof("discovered new device %s (%s) via mdns", d.Name, entry.Name)
				d.Labels = map[string]string{mdnsDiscoveryLabel: "true"}
				err = devices.Create(d.Name, d)
			}
			if err != nil && !errors.IsAlreadyExists(err) {
				logrus.WithError(err).Error("failed to register device")
			}
		}
		wg.Done()
	}()
	p := mdns.DefaultParams(mdnsZone)
	p.DisableIPv6 = true // fails within docker network otherwise
	p.Entries = ch
	err := mdns.Query(p)
	close(ch)
	wg.Wait()
	if err != nil {
		return fmt.Errorf("mdns lookup: %w", err)
	}

	// Remove old devices
	l := &deviceapi.DeviceList{}
	err = devices.List(l)
	if err != nil {
		return fmt.Errorf("scan for devices: %w", err)
	}
	for _, d := range l.Items {
		if hasLabel(&d, mdnsDiscoveryLabel) {
			if _, found := foundDevices[d.Name]; !found {
				logrus.Infof("device %s appears to be offline", d.Name)
				if e := devices.Delete(d.Name, &d, func() error { return nil }); e != nil && err == nil && !errors.IsNotFound(err) {
					err = e
				}
			}
		}
	}
	return err
}

func hasLabel(o *deviceapi.Device, label string) bool {
	if o.Labels == nil {
		return false
	}
	_, ok := o.Labels[label]
	return ok
}
