package device

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	deviceapi "github.com/mgoltzsche/kubemate/pkg/apis/devices/v1"
	"github.com/mgoltzsche/kubemate/pkg/cliutils"
	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/mgoltzsche/kubemate/pkg/storage"
	"github.com/sirupsen/logrus"
)

type deviceDnsServerReconciler struct {
	deviceName string
	dir        string
	ifaces     storage.Interface
	bind       *runner.Runner
}

func newDeviceDnsServerReconciler(dir, deviceName string, deviceStore, ifaces storage.Interface, logger *logrus.Entry) *deviceDnsServerReconciler {
	bind := runner.New(logger.WithField("proc", "named"))
	bind.Reporter = func(c runner.Command) {
		// Update device resource's status
		if c.Status.State == runner.ProcessStateFailed {
			logrus.Warnf("bind %s: %s", c.Status.State, c.Status.Message)
		} else {
			logrus.Infof("bind %s: %s", c.Status.State, c.Status.Message)
		}
		d := &deviceapi.Device{}
		err := deviceStore.Update(deviceName, d, func() error {
			if d.Status.State != deviceapi.DeviceStateTerminating {
				d.Status.DNSServer.Running = c.Status.State == runner.ProcessStateRunning
				if !d.Status.DNSServer.Running {
					d.Status.DNSServer.RestartCount++
				}
			}
			return nil
		})
		if err != nil {
			logger.WithError(err).Error("failed to update device status")
		}
	}
	return &deviceDnsServerReconciler{
		deviceName: deviceName,
		dir:        dir,
		ifaces:     ifaces,
		bind:       bind,
	}
}

func (r *deviceDnsServerReconciler) Reconcile(ctx context.Context, d *deviceapi.Device) error {
	isAP, err := isAccessPoint(r.ifaces)
	if err != nil {
		return err
	}
	dnsServerEnabled := d.Spec.Mode == deviceapi.DeviceModeServer || isAP
	if !dnsServerEnabled {
		return r.bind.Stop()
	}

	conf, err := generateBindConfig(ctx, r.deviceName, r.dir)
	if err != nil {
		return err
	}
	return r.bind.Start(runner.Cmd("named", "-gc", conf))
}

func isAccessPoint(ifaces storage.Interface) (bool, error) {
	l := deviceapi.NetworkInterfaceList{}
	err := ifaces.List(&l)
	if err != nil {
		return false, fmt.Errorf("check access point mode: %w", err)
	}
	for _, r := range l.Items {
		if r.Spec.Wifi.Mode == deviceapi.WifiModeAccessPoint {
			return true, nil
		}
	}
	return false, nil
}

func generateBindConfig(ctx context.Context, deviceName, dir string) (string, error) {
	// See https://www.talk-about-it.ca/setup-bind9-with-isc-dhcp-server-dynamic-host-registration/
	dataDir := filepath.Join(dir, "data")
	zoneDir := filepath.Join(dir, "zones")
	keyFile := filepath.Join(dir, "zone.key")
	zone := "kube.m8"
	reverseZone := "0.0.11.in-addr.arpa"
	zoneFile, err := createZoneFileIfNotExist(zone, deviceName, zoneDir)
	if err != nil {
		return "", err
	}
	reverseZoneFile, err := createReverseZoneFileIfNotExist(reverseZone, deviceName, zoneDir)
	if err != nil {
		return "", err
	}
	err = os.MkdirAll(dataDir, 0755)
	if err != nil {
		return "", fmt.Errorf("create dns server data dir: %w", err)
	}
	keyName := "kubemate"
	err = generateTsigKeyIfNotExist(ctx, keyFile, keyName)
	if err != nil {
		return "", fmt.Errorf("generate dns server key: %w", err)
	}
	file, _, err := cliutils.WriteTempConfigFile("bind", `
acl goodclients {
	localhost;
	localnets;
};

options {
	directory "{dataDir}";
	pid-file "/var/run/named/named.pid";

	listen-on port 53 { any; };
	listen-on-v6 { none; };

	recursion yes;
	allow-query { goodclients; };

	forwarders {
		1.1.1.1;
		8.8.8.8;
	};
	forward only;

	dnssec-enable yes;
	dnssec-validation yes;
	auth-nxdomain no;
	notify no;

	allow-transfer {
		none; // Don't allow zone transfers by default, zone below allows
	};
};

// TSIG key used for the dynamic update
include "{keyFile}";

zone "{zone}" {
	type master;
	file "{zoneFile}";
	allow-update { key "{keyName}"; };
};
zone "{reverseZone}" {
	type master;
	notify no;
	file "{reverseZoneFile}";
	allow-update { key "{keyName}"; };
};
`, "{dataDir}", dataDir, "{zone}", zone, "{reverseZone}", reverseZone, "{zoneFile}", zoneFile, "{reverseZoneFile}", reverseZoneFile, "{keyName}", keyName, "{keyFile}", keyFile)
	if err != nil {
		return "", err
	}
	return file, nil
}

func createZoneFileIfNotExist(zone, deviceName, dir string) (string, error) {
	zoneFile := filepath.Join(dir, zone)
	_, err := os.Stat(zoneFile)
	if err == nil {
		return zoneFile, nil // zone file already exists
	}
	conf := strings.NewReplacer("{zone}", zone, "{deviceName}", deviceName).
		Replace(`$ORIGIN .
$TTL 1200 ; 10 minutes
{zone}                  IN SOA  {zone}. root.{zone}. (
                                16         ; serial
                                60         ; refresh (1 minute)
                                60         ; retry (1 minute)
                                60         ; expire (1 minute)
                                60         ; minimum (1 minute)
                                )
                        NS      {zone}.
                        NS      localhost.
                        A       11.0.0.1
$ORIGIN {zone}.
$TTL 60 ; 1 minute
{deviceName}            A       11.0.0.1
`)
	//zoneFile, _, err := cliutils.WriteTempConfigFile("dns-zone", conf)
	err = writeFile(zoneFile, conf)
	if err != nil {
		return "", fmt.Errorf("create dns zone file: %w", err)
	}
	return zoneFile, err
}

func createReverseZoneFileIfNotExist(zone, deviceName, dir string) (string, error) {
	zoneFile := filepath.Join(dir, "0.0.11.in-addr.arpa")
	_, err := os.Stat(zoneFile)
	if err == nil {
		return zoneFile, nil // zone file already exists
	}
	conf := strings.NewReplacer("{zone}", zone, "{deviceName}", deviceName).
		Replace(`$ORIGIN .
$TTL 1200 ; 10 minutes
0.0.11.in-addr.arpa    IN SOA  {zone}. root.{zone}. (
                                16         ; serial
                                60         ; refresh (1 minute)
                                60         ; retry (1 minute)
                                60         ; expire (1 minute)
                                60         ; minimum (1 minute)
                                )
                        NS      dhcpdns.
                        A       11.0.0.1
$ORIGIN 0.0.11.in-addr.arpa.
$TTL 60 ; 1 minute
1                       PTR     {deviceName}.{zone}.
`)
	//zoneFile, _, err := cliutils.WriteTempConfigFile("reverse-dns-zone", conf)
	err = writeFile(zoneFile, conf)
	if err != nil {
		return "", fmt.Errorf("create dns zone file: %w", err)
	}
	return zoneFile, err
}

func writeFile(file, content string) (err error) {
	err = os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(file), ".tmp-")
	defer func() {
		if err == nil {
			err = os.Rename(f.Name(), file)
		}
	}()
	defer func() {
		e := f.Close()
		if e != nil && err == nil {
			err = e
		}
		if err == nil {
			err = os.Chmod(f.Name(), 0644)
		}
	}()
	_, err = f.Write([]byte(content))
	if err != nil {
		return err
	}
	return err
}

func generateTsigKeyIfNotExist(ctx context.Context, file, keyName string) (err error) {
	if _, err := os.Stat(file); err == nil {
		return nil // already exists
	}
	dir := filepath.Dir(file)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	key, err := cliutils.Run(ctx, "tsig-keygen", "-a", "hmac-sha256", keyName)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "kubemate-tsig-key-")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(f.Name())
		}
	}()
	defer func() {
		if err == nil {
			err = os.Rename(f.Name(), file)
		}
	}()
	defer func() {
		if err == nil {
			err = f.Sync()
		}
		e := f.Close()
		if e != nil && err == nil {
			err = e
		}
	}()
	err = os.Chmod(f.Name(), 0600)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(f, key)
	if err != nil {
		return err
	}
	return nil
}
