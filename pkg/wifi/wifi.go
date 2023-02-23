package wifi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgoltzsche/kubemate/pkg/runner"
	"github.com/sirupsen/logrus"
)

// Derived from
// * https://fwhibbit.es/en/automatic-access-point-with-docker-and-raspberry-pi-zero-w
// * https://nims11.wordpress.com/2012/04/27/hostapd-the-linux-way-to-create-virtual-wifi-access-point/
// Also see https://wiki.archlinux.org/title/software_access_point

// TODO: consider using iwlist and iwd tools instead of wpa_supplicant and hostadp, e.g. `iwlist wlp6s0 scan`
// Those tools allow for more granular control when to scan.
// See https://news.ycombinator.com/item?id=21733666
// See https://iwd.wiki.kernel.org/ap_mode

type WifiMode string

const (
	WifiModeDisabled    WifiMode = "disabled"
	WifiModeStation     WifiMode = "station"
	WifiModeAccessPoint WifiMode = "accesspoint"
)

var WifiInterfaceNamePrefixes = []string{"wlan", "wlp"}

type Wifi struct {
	dhcpd               *runner.Runner
	ap                  *runner.Runner
	station             *runner.Runner
	wifiIfaceStarted    bool
	mode                WifiMode
	logger              *logrus.Entry
	EthIface            string
	WifiIface           string
	DHCPDLeaseFile      string
	DHCPCDLeaseFile     string
	DNSKeyFile          string
	CaptivePortalURL    string
	CountryCode         string
	WriteHostResolvConf bool
	networks            []WifiNetwork
}

type WifiNetwork struct {
	MAC     string
	SSID    string
	Country string
}

func New(logger *logrus.Entry, dataDir string, onProcessTermination runner.StatusReportFunc) *Wifi {
	logger = logger.WithField("comp", "wifi")
	ap := runner.New(logger.WithField("proc", "hostapd"))
	ap.Reporter = onProcessTermination
	dhcpd := runner.New(logger.WithField("proc", "dhcpd"))
	dhcpd.Reporter = onProcessTermination
	station := runner.New(logger.WithField("proc", "wpa_supplicant"))
	station.Reporter = onProcessTermination
	return &Wifi{
		ap:               ap,
		dhcpd:            dhcpd,
		station:          station,
		logger:           logger,
		CountryCode:      "DE",
		EthIface:         detectIface(logger, []string{"eth", "enp"}),
		WifiIface:        detectIface(logger, WifiInterfaceNamePrefixes),
		DHCPDLeaseFile:   filepath.Join(dataDir, "dhcp", "dhcpd.leases"),
		DHCPCDLeaseFile:  filepath.Join(dataDir, "dhcp", "dhcpcd.leases"),
		DNSKeyFile:       "/etc/bind/rndc.key",
		CaptivePortalURL: "localhost",
	}
}

func (w *Wifi) Close() (err error) {
	w.StopAccessPoint()
	err1 := w.ap.Stop()
	err2 := w.dhcpd.Stop()
	err3 := w.StopStation()
	err4 := w.StopWifiInterface()
	if err1 != nil {
		err = err1
	}
	if err2 != nil {
		err = err2
	}
	if err3 != nil {
		err = err3
	}
	if err4 != nil {
		err = err4
	}
	return err
}

func (w *Wifi) Mode() WifiMode {
	return w.mode
}

// DetectCountry derives the wifi country based on near wifi networks.
func (w *Wifi) DetectCountry() error {
	if !w.wifiIfaceStarted {
		return fmt.Errorf("cannot detect wifi country while wifi interface is down")
	}
	if w.CountryCode == "" {
		if !w.wifiIfaceStarted {
			return fmt.Errorf("cannot detect country when wifi interface is down")
		}
		country, err := w.detectCountry(w.WifiIface, w.logger)
		if err != nil {
			return err
		}
		w.CountryCode = country
	}
	return nil
}

// Networks returns the list of available wifi networks.
func (w *Wifi) Networks() []WifiNetwork {
	return w.networks
}

func (w *Wifi) scan() error {
	if !w.wifiIfaceStarted {
		return fmt.Errorf("cannot scan wifi networks while network interface %s is down", w.WifiIface)
	}
	w.logger.Debug("scanning wifi networks")
	l, err := scanWifiNetworksIw(w.WifiIface, w.logger)
	if err != nil {
		return err
	}
	w.networks = l
	return nil
}

func (w *Wifi) restartWifiInterface() error {
	w.logger.WithField("iface", w.WifiIface).Debug("restarting wifi network interface")
	err := runCmds([][]string{
		{"ip", "link", "set", w.WifiIface, "down"},
		{"ip", "addr", "flush", "dev", w.WifiIface},
		{"ip", "link", "set", w.WifiIface, "up"},
	})
	if err != nil {
		return fmt.Errorf("restart wifi network interface %s: %w", w.WifiIface, err)
	}
	w.wifiIfaceStarted = true
	err = w.scan()
	if err != nil {
		return err
	}
	return nil
}

func (w *Wifi) StartWifiInterface() error {
	if !w.wifiIfaceStarted {
		return w.restartWifiInterface()
	}
	return nil
}

func (w *Wifi) StopWifiInterface() error {
	if w.wifiIfaceStarted {
		w.logger.WithField("iface", w.WifiIface).Debug("stopping wifi network interface")
		err := runCmds([][]string{
			{"ip", "link", "set", w.WifiIface, "down"},
		})
		if err != nil {
			return fmt.Errorf("stop wifi interface %s: %w", w.WifiIface, err)
		}
		w.wifiIfaceStarted = false
	}
	return nil
}

func createLeaseFileIfNotExist(file string) error {
	err := os.MkdirAll(filepath.Dir(file), 0755)
	if err != nil {
		return fmt.Errorf("create dhcpd lease file: %w", err)
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("create dhcpd lease file: %w", err)
	}
	_ = f.Close()
	return nil
}

func writeConf(name, confTpl string, args ...interface{}) (string, bool, error) {
	conf := fmt.Sprintf(confTpl, args...)
	h := sha256.New()
	_, _ = h.Write([]byte(conf))
	confHash := hex.EncodeToString(h.Sum(nil))
	file := filepath.Join(os.TempDir(), fmt.Sprintf("kubemate_%s_%s.conf", name, confHash[:12]))
	if _, err := os.Stat(file); os.IsNotExist(err) {
		err := os.WriteFile(file, []byte(conf), 0600)
		if err != nil {
			return "", false, fmt.Errorf("write %s config: %w", name, err)
		}
		return file, true, nil
	}
	return file, false, nil
}

func runCmd(cmd string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, cmd, args...)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	err := c.Run()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", cmd, err, strings.TrimSpace(stderr.String()))
	}
	return err
}

func runCmds(cmds [][]string) error {
	for _, c := range cmds {
		err := runCmd(c[0], c[1:]...)
		if err != nil {
			return err
		}
	}
	return nil
}

func detectIface(logger *logrus.Entry, prefixes []string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		logger.Error(fmt.Errorf("detect %s interface: %w", prefixes[0], err))
		return ""
	}
	for _, iface := range ifaces {
		name := iface.Name
		for _, p := range prefixes {
			if strings.HasPrefix(name, p) {
				return name
			}
		}
	}
	return ""
}
