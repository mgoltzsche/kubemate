package discovery

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1alpha1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
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
	store           storage.Interface
	logger          *logrus.Entry
}

func NewDeviceDiscovery(deviceName string, port int, advertiseIfaces []string, store storage.Interface, logger *logrus.Entry) *DeviceDiscovery {
	d := &DeviceDiscovery{
		deviceName:      deviceName,
		port:            port,
		advertiseIfaces: advertiseIfaces,
		logger:          logger.WithField("comp", "device-discovery"),
	}
	d.store = storage.RefreshPeriodically(store, 10*time.Second, func(store storage.Interface) {
		err := d.Discover()
		if err != nil {
			d.logger.Error("failed to discover devices via mdns")
		}
	})
	return d
}

func (d *DeviceDiscovery) Store() storage.Interface {
	return d.store
}

func (d *DeviceDiscovery) Advertise(device *deviceapi.DeviceDiscovery, ip net.IP) error {
	if device.Name != d.deviceName {
		return fmt.Errorf("refusing to advertise a different device than this one via mdns")
	}
	info := []string{
		"kubemate",
		fmt.Sprintf("%s=%s", mdnsFieldDeviceMode, device.Spec.Mode),
	}
	if device.Spec.Server != "" {
		info = append(info, fmt.Sprintf("%s=%s", mdnsFieldServer, device.Spec.Server))
	}
	logrus.
		WithField("ip", ip.String()).
		WithField("device", d.deviceName).
		Info("advertise device via mdns")
	hostname := fmt.Sprintf("%s.", d.deviceName)
	ips := []net.IP{ip}
	svc, err := mdns.NewMDNSService(d.deviceName, mdnsZone, "", hostname, d.port, ips, info)
	if err != nil {
		return fmt.Errorf("advertise mdns name: %s", err)
	}
	// Terminate previous mdns server if exists
	if d.srv != nil {
		err = d.srv.Shutdown()
		if err != nil {
			return fmt.Errorf("advertise mdns name: %s", err)
		}
	}
	// (re)start mdns server with new service
	srv, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		return fmt.Errorf("advertise mdns name: %s", err)
	}
	d.srv = srv
	dev := &deviceapi.DeviceDiscovery{}
	dev.Name = d.deviceName
	err = d.store.Update(d.deviceName, dev, func() error {
		dev.Spec = device.Spec
		return nil
	})
	if err != nil {
		dev.Spec = device.Spec
		e := d.store.Create(d.deviceName, dev)
		if e != nil {
			return fmt.Errorf("advertise mdns name: %s. %s", err, e)
		}
		return nil
	}
	return nil
}

func (d *DeviceDiscovery) Discover() error {
	d.logger.Debug("scanning for devices via mdns")
	return populateDevicesFromMDNS(d.deviceName, d.store, d.logger)
}

// TODO: remove this in favour of the NetworkInterface resource, each exposing an IP within its status.
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
			if ip, ok := ifaceIPMap[iface.Name]; ok {
				ips = append(ips, ip)
			}
		}
	}
	if err != nil {
		if len(ips) == 0 {
			return nil, fmt.Errorf("detect external IPs: %w", err)
		}
		d.logger.WithError(err).Warn("error while detecting external IPs")
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

func (d *DeviceDiscovery) Close() error {
	if s := d.srv; s != nil {
		err := s.Shutdown()
		d.srv = nil
		return err
	}
	return nil
}

func populateDevicesFromMDNS(deviceName string, devices storage.Interface, logger *logrus.Entry) error {
	foundDevices := map[string]struct{}{
		deviceName: struct{}{},
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	ch := make(chan *mdns.ServiceEntry, 4)
	go func() {
		for entry := range ch {
			d := &deviceapi.DeviceDiscovery{}
			d.Name = strings.TrimRight(entry.Host, ".")
			foundDevices[d.Name] = struct{}{}
			if d.Name == deviceName {
				continue
			}
			modify := func() {
				d.Labels = map[string]string{mdnsDiscoveryLabel: "true"}
				addrs := fmt.Sprintf("https://%s", strings.TrimRight(entry.Host, "."))
				if entry.Port != 443 {
					addrs = fmt.Sprintf("%s:%d", addrs, entry.Port)
				}
				d.Spec.Address = addrs
				d.Spec.Mode = deviceapi.DeviceMode(getMDNSEntryField(entry, mdnsFieldDeviceMode))
				d.Spec.Server = getMDNSEntryField(entry, mdnsFieldServer)
			}
			err := devices.Get(d.Name, d)
			if errors.IsNotFound(err) {
				modify()
				err = devices.Create(d.Name, d)
			} else if err == nil {
				existingDevice := d.DeepCopy()
				modify()
				if equality.Semantic.DeepEqual(&existingDevice.Spec, &d.Spec) {
					continue
				}
				err = devices.Update(d.Name, d, func() error {
					modify()
					return nil
				})
			}
			logger.
				WithField("mode", d.Spec.Mode).
				WithField("address", d.Spec.Address).
				WithField("device", entry.Name).
				Info("discovered new device via mdns")
			if err != nil && !errors.IsAlreadyExists(err) {
				logrus.WithError(err).
					WithField("address", d.Spec.Address).
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
	l := &deviceapi.DeviceDiscoveryList{}
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

func hasLabel(o *deviceapi.DeviceDiscovery, label string) bool {
	if o.Labels == nil {
		return false
	}
	_, ok := o.Labels[label]
	return ok
}
