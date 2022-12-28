package networkifaces

import (
	"context"
	"fmt"
	"sync"
	"time"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/resource"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NetworkIfaceSync struct {
	ExternalNetworkInterfaces []string
	NetworkInterfaceStore     storage.Interface
	mutex                     sync.Mutex
	cancel                    context.CancelFunc
}

func (s *NetworkIfaceSync) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.cancel != nil {
		return nil // already started
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	err := startNetworkLinkStatusSync(ctx, s.ExternalNetworkInterfaces, s.NetworkInterfaceStore)
	if err != nil {
		return fmt.Errorf("start network link status sync: %w", err)
	}
	return nil
}

func (s *NetworkIfaceSync) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func startNetworkLinkStatusSync(ctx context.Context, ifaces []string, store storage.Interface) error {
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
		err = store.Create(iface, &deviceapi.NetworkInterface{
			ObjectMeta: metav1.ObjectMeta{
				Name: iface,
			},
			Status: deviceapi.NetworkInterfaceStatus{
				Up: link.Attrs().OperState == netlink.OperUp,
			},
		})
		if err != nil {
			return err
		}
	}
	go func() {
		for evt := range ch {
			name := evt.Attrs().Name
			_, ok := ifaceSet[name]
			if ok {
				logrus.WithField("netlink", name).Debug("observed network link status update")
				iface := &deviceapi.NetworkInterface{}
				err := store.Update(name, iface, func() (resource.Resource, error) {
					iface.Status.Up = evt.Link.Attrs().OperState == netlink.OperUp
					return iface, nil
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
