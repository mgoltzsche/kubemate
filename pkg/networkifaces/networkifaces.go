package networkifaces

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/mgoltzsche/kubemate/pkg/wifi"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/api/errors"
)

// NetworkIfaceSync implements the NetworkInterface resource synchronization.
type NetworkIfaceSync struct {
	Interfaces    []string
	DefaultAPSSID string
	Store         storage.Interface
	mutex         sync.Mutex
	cancel        context.CancelFunc
}

// Start starts the network status synchronization asynchronously.
func (s *NetworkIfaceSync) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.cancel != nil {
		return nil // already started
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	err := startNetworkLinkStatusSync(ctx, s.DefaultAPSSID, s.Interfaces, s.Store)
	if err != nil {
		return fmt.Errorf("start network link status sync: %w", err)
	}
	return nil
}

// Stop stops the network status synchronization.
func (s *NetworkIfaceSync) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func startNetworkLinkStatusSync(ctx context.Context, apSSID string, ifaces []string, store storage.Interface) error {
	// TODO: remove old iface resources. unify this with the mdns device synchronization.
	ch := make(chan netlink.LinkUpdate)
	err := netlink.LinkSubscribe(ch, ctx.Done())
	if err != nil {
		return err
	}
	ifaceSet := map[string]struct{}{}
	for _, iface := range ifaces {
		ifaceSet[iface] = struct{}{}
		link, err := netlink.LinkByName(iface)
		if err != nil {
			return err
		}
		o := deviceapi.NetworkInterface{}
		o.Name = iface
		o.Spec.Wifi = deviceapi.WifiSpec{
			Mode: deviceapi.WifiModeDisabled,
			AccessPoint: deviceapi.WifiAccessPointSpec{
				SSID: apSSID,
			},
		}
		updateNetworkInterfaceStatus(link.Attrs(), &o)
		err = store.Create(iface, &o)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			err = store.Update(o.Name, &o, func() error {
				updateNetworkInterfaceStatus(link.Attrs(), &o)
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	go func() {
		for evt := range ch {
			name := evt.Attrs().Name
			_, ok := ifaceSet[name]
			if ok {
				logrus.WithField("netlink", name).WithField("flags", evt.Link.Attrs().Flags.String()).Debug("observed network link status update")
				iface := &deviceapi.NetworkInterface{}
				err := store.Update(name, iface, func() error {
					updateNetworkInterfaceStatus(evt.Link.Attrs(), iface)
					return nil
				})
				if err != nil {
					logrus.Error(fmt.Errorf("update networkinterface status: %w", err))
					continue
				}
				time.Sleep(time.Second)
			}
		}
	}()
	return nil
}

func updateNetworkInterfaceStatus(a *netlink.LinkAttrs, o *deviceapi.NetworkInterface) {
	l := &o.Status.Link
	l.Index = a.Index
	l.Type = ifaceType(a)
	l.Up = a.OperState == netlink.OperUp
	l.MAC = a.HardwareAddr.String()
	l.IP4 = ""
	l.Error = ""
	if l.Up {
		ipv4, err := ifaceIPv4Addr(a.Name)
		if err != nil {
			logrus.Warnf("failed to get IPv4 address for network interface %s: %s", o.Name, err)
			l.Error = err.Error()
			return
		}
		if ipv4 != nil {
			l.IP4 = ipv4.String()
		}
	}
}

func ifaceType(a *netlink.LinkAttrs) deviceapi.NetworkInterfaceType {
	t := deviceapi.NetworkInterfaceType(a.EncapType)
	if startsWith(a.Name, wifi.WifiInterfaceNamePrefixes) {
		t = deviceapi.NetworkInterfaceTypeWifi
	}
	return t
}

func startsWith(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func ifaceIPv4Addr(name string) (net.IP, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("get network interface %s addrs: %w", name, err)
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
		return v4, nil
	}
	return nil, nil
}

func toBroadcastIP(ip *net.IPNet) net.IP {
	brd := make(net.IP, len(ip.IP.To4()))
	binary.BigEndian.PutUint32(brd, binary.BigEndian.Uint32(ip.IP.To4())|^binary.BigEndian.Uint32(net.IP(ip.Mask).To4()))
	return brd
}
