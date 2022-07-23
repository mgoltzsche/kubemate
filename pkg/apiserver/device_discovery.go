package apiserver

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/mdns"
	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	mdnsZone            = "_kubemate._tcp"
	mdnsDiscoveryLabel  = "kubemate.mgoltzsche.github.com/mdns-discovery"
	mdnsFieldDeviceMode = "kubemate.mgoltzsche.github.com/device-mode"
	mdnsFieldServer     = "kubemate.mgoltzsche.github.com/server"
	mdnsFieldState      = "kubemate.mgoltzsche.github.com/state"
)

type DeviceDiscovery struct {
	deviceName      string
	port            int
	advertiseIfaces []string
	srv             *mdns.Server
}

func NewDeviceDiscovery(deviceName string, port int, advertiseIfaces []string) *DeviceDiscovery {
	return &DeviceDiscovery{
		deviceName:      deviceName,
		port:            port,
		advertiseIfaces: advertiseIfaces,
	}
}

func (d *DeviceDiscovery) Advertise(device *deviceapi.Device, ips []net.IP) error {
	if device.Name != d.deviceName {
		return fmt.Errorf("refusing to advertise a different device than this one via mdns")
	}
	if device.Generation != device.Status.Generation {
		return fmt.Errorf("mdns advertise: provided device status is not up-to-date")
	}
	info := []string{
		"kubemate",
		fmt.Sprintf("%s=%s", mdnsFieldDeviceMode, device.Spec.Mode),
		fmt.Sprintf("%s=%s", mdnsFieldState, device.Status.State),
	}
	if device.Spec.Server != "" {
		info = append(info, fmt.Sprintf("%s=%s", mdnsFieldServer, device.Spec.Server))
	}
	ipStrs := make([]string, len(ips))
	for i, ip := range ips {
		ipStrs[i] = ip.String()
	}
	logrus.
		WithField("ips", ips).
		WithField("device", d.deviceName).
		Info("advertise device via mdns")
	hostname := fmt.Sprintf("%s.", d.deviceName)
	svc, err := mdns.NewMDNSService(d.deviceName, mdnsZone, "", hostname, d.port, ips, info)
	if err != nil {
		return err
	}
	// Terminate previous mdns server if exists
	if d.srv != nil {
		err = d.srv.Shutdown()
		if err != nil {
			return err
		}
	}
	// (re)start mdns server with new service
	srv, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		return err
	}
	d.srv = srv
	return nil
}

func (d *DeviceDiscovery) ExternalIPs() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ifaceIPMap := make(map[string]net.IP, len(ifaces))
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
			if v4 == nil || v4.IsLoopback() || v4.IsUnspecified() || v4.IsMulticast() || v4.IsLinkLocalMulticast() || v4.IsInterfaceLocalMulticast() || v4.IsLinkLocalUnicast() {
				continue
			}
			brd := toBroadcastIP(ipnet)
			if brd.String() == v4.String() {
				continue
			}
			ifaceIPMap[iface.Name] = v4
		}
	}
	ips := make([]net.IP, 0, len(ifaceIPMap))
	if len(d.advertiseIfaces) > 0 {
		for _, ifaceName := range d.advertiseIfaces {
			ip, ok := ifaceIPMap[ifaceName]
			if ok {
				ips = append(ips, ip)
			}
		}
	} else {
		for _, iface := range ifaces {
			ips = append(ips, ifaceIPMap[iface.Name])
		}
	}
	if err != nil {
		if len(ips) == 0 {
			return nil, fmt.Errorf("detect external IPs: %w", err)
		}
		logrus.WithError(err).Warn("error while detecting external IPs")
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("detect external IPs: no external IP available")
	}
	return ips, nil
}

func toBroadcastIP(ip *net.IPNet) net.IP {
	brd := make(net.IP, len(ip.IP.To4()))
	binary.BigEndian.PutUint32(brd, binary.BigEndian.Uint32(ip.IP.To4())|^binary.BigEndian.Uint32(net.IP(ip.Mask).To4()))
	return brd
}

func detectIfaces() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, 2)
	for _, iface := range ifaces {
		name := iface.Name
		if strings.HasPrefix(name, "enp") || strings.HasPrefix(name, "wlp") || strings.HasPrefix(name, "eth") || strings.HasPrefix(name, "wlan") {
			names = append(names, name)
		}
	}
	return names, nil
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
			modify := func() {
				d.Labels = map[string]string{mdnsDiscoveryLabel: "true"}
				addr := entry.AddrV4.String()
				if addr == "" {
					addr = entry.AddrV6.String()
				}
				d.Status.Address = fmt.Sprintf("https://%s:%d", addr, entry.Port)
				d.Status.State = deviceapi.DeviceState(getMDNSEntryField(entry, mdnsFieldState))
				d.Spec.Mode = deviceapi.DeviceMode(getMDNSEntryField(entry, mdnsFieldDeviceMode))
				d.Spec.Server = getMDNSEntryField(entry, mdnsFieldServer)
			}
			err := devices.Get(d.Name, d)
			if errors.IsNotFound(err) {
				modify()
				err = devices.Create(d.Name, d)
			} else if err == nil {
				err = devices.Update(d.Name, d, func() (resource.Resource, error) {
					modify()
					return d, nil
				})
			}
			logrus.
				WithField("mode", d.Spec.Mode).
				WithField("state", d.Status.State).
				WithField("address", d.Status.Address).
				WithField("device", entry.Name).
				Info("discovered new device via mdns")
			if err != nil && !errors.IsAlreadyExists(err) {
				logrus.WithError(err).
					WithField("address", d.Status.Address).
					WithField("device", entry.Name).
					Error("failed to register device")
				continue
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

func getMDNSEntryField(entry *mdns.ServiceEntry, field string) string {
	prefix := fmt.Sprintf("%s=", field)
	for _, v := range entry.InfoFields {
		if strings.HasPrefix(v, prefix) {
			return v[len(prefix):]
		}
	}
	return ""
}

func hasLabel(o *deviceapi.Device, label string) bool {
	if o.Labels == nil {
		return false
	}
	_, ok := o.Labels[label]
	return ok
}
