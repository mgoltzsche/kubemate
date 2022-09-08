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

// Derived from https://fwhibbit.es/en/automatic-access-point-with-docker-and-raspberry-pi-zero-w
// Also see https://wiki.archlinux.org/title/software_access_point

type Wifi struct {
	dhcpd         *runner.Runner
	ap            *runner.Runner
	EthIface      string
	WifiIface     string
	ApSSID        string
	ApPassword    string
	DHCPLeaseFile string
	CountryCode   string
}

func New(logger *logrus.Entry) *Wifi {
	ap := runner.New(logger.WithField("proc", "hostapd"))
	dhcpd := runner.New(logger.WithField("proc", "dhcpd"))
	return &Wifi{
		ap:            ap,
		dhcpd:         dhcpd,
		CountryCode:   "DE",
		EthIface:      detectIface("eth", "enp"),
		WifiIface:     detectIface("wlan", "wlp"),
		ApSSID:        "kubespot",
		DHCPLeaseFile: "/var/lib/dhcp/dhcpd.leases",
	}
}

func (w *Wifi) Close() (err error) {
	w.StopAccessPoint()
	err1 := w.ap.Stop()
	err2 := w.dhcpd.Stop()
	if err1 != nil {
		err = err1
	}
	if err2 != nil {
		err = err2
	}
	return err
}

func (w *Wifi) StartAccessPoint() error {
	if w.ApPassword == "" {
		return fmt.Errorf("start accesspoint: no wifi password configured")
	}
	err := w.generateNetworkInterfacesConfIfNotExist()
	if err != nil {
		return err
	}
	hostapdConf, err := w.generateHostapdConf()
	if err != nil {
		return err
	}
	dhcpdConf, err := w.generateDhcpdConf()
	if err != nil {
		return err
	}
	err = w.createDHCPLeaseFileIfNotExist()
	if err != nil {
		return err
	}
	w.installIPRoutes()
	err = w.restartWifiInterface()
	if err != nil {
		return err
	}
	ctx := context.Background()
	err = w.dhcpd.Start(ctx, runner.Cmd("dhcpd", "-4", "-f", "-d", w.WifiIface, "-cf", dhcpdConf, "-lf", w.DHCPLeaseFile))
	if err != nil {
		return err
	}
	err = w.ap.Start(ctx, runner.Cmd("hostapd", hostapdConf))
	if err != nil {
		return err
	}
	return nil
}

func (w *Wifi) StopAccessPoint() {
	w.uninstallIPRoutes()
	w.ap.Stop()
	w.dhcpd.Stop()
	w.restartWifiInterfaceOrWarn()
}

func (w *Wifi) generateNetworkInterfacesConfIfNotExist() error {
	file := "/etc/network/interfaces"
	if _, err := os.Stat(file); os.IsNotExist(err) {
		conf := `auto %[1]s
iface %[1]s inet static
        address 11.0.0.1
        netmask 255.255.255.0

auto %[2]s
iface %[2]s inet dhcp
`
		conf = fmt.Sprintf(conf, w.WifiIface, w.EthIface)
		err := os.WriteFile(file, []byte(conf), 0644)
		if err != nil {
			return fmt.Errorf("write network interface config: %w", err)
		}
	}
	return nil
}

func (w *Wifi) generateDhcpdConf() (string, error) {
	return writeConf("dhcpd", `authoritative;
subnet 11.0.0.0 netmask 255.255.255.0 {
        range 11.0.0.10 11.0.0.20;
        option broadcast-address 11.0.0.255;
        option routers 11.0.0.1;
        default-lease-time 600;
        max-lease-time 7200;
        option domain-name "local";
        option domain-name-servers 1.1.1.1;
}
`)
}

func (w *Wifi) generateHostapdConf() (string, error) {
	return writeConf("hostapd", `interface=%q
driver=nl80211
ssid=%q
hw_mode=g
ieee80211n=1
channel=6
auth_algs=1
ignore_broadcast_ssid=0
wpa=2
country_code=%s
macaddr_acl=0

wpa_passphrase=%q
wpa_key_mgmt=WPA-PSK
wpa_pairwise=CCMP
rsn_pairwise=CCMP
`, w.WifiIface, w.ApSSID, w.CountryCode, w.ApPassword)
}

func (w *Wifi) createDHCPLeaseFileIfNotExist() error {
	f, err := os.OpenFile(w.DHCPLeaseFile, os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("create dhcp lease file: %w", err)
	}
	_ = f.Close()
	return nil
}

func writeConf(name, confTpl string, args ...interface{}) (string, error) {
	conf := fmt.Sprintf(confTpl, args...)
	h := sha256.New()
	_, _ = h.Write([]byte(conf))
	confHash := hex.EncodeToString(h.Sum(nil))
	file := filepath.Join(os.TempDir(), fmt.Sprintf("kubemate-%s-%s.conf", name, confHash[:12]))
	err := os.WriteFile(file, []byte(conf), 0600)
	if err != nil {
		return "", fmt.Errorf("write %s config: %w", name, err)
	}
	return file, nil
}

func (w *Wifi) restartWifiInterfaceOrWarn() {
	err := w.restartWifiInterface()
	if err != nil {
		logrus.Warn(err)
	}
}

func (w *Wifi) restartWifiInterface() error {
	logrus.WithField("iface", w.WifiIface).Debug("restarting wifi network interface")
	for _, c := range [][]string{
		{"ifdown", w.WifiIface},
		{"ip", "addr", "flush", "dev", w.WifiIface},
		{"ifup", w.WifiIface},
		{"ip", "addr", "add", "11.0.0.1/24", "dev", w.WifiIface},
	} {
		err := runCmd(c[0], c[1:]...)
		if err != nil {
			return fmt.Errorf("restart wifi interface %s: %w", w.WifiIface, err)
		}
	}
	return nil
}

func (w *Wifi) installIPRoutes() {
	logrus.WithField("iface", w.WifiIface).Debug("installing wifi iptables rules")
	w.configureIPRoutes(addIPTablesRule)
}

func (w *Wifi) uninstallIPRoutes() {
	logrus.WithField("iface", w.WifiIface).Debug("uninstalling wifi iptables rules")
	w.configureIPRoutes(delIPTablesRule)
}

func (w *Wifi) configureIPRoutes(apply func(table, chain, inIface, outIface, jump, state string)) {
	apply("nat", "POSTROUTING", "", w.WifiIface, "MASQUERADE", "")
	apply("filter", "FORWARD", w.EthIface, w.WifiIface, "ACCEPT", "RELATED,ESTABLISHED")
	apply("filter", "FORWARD", w.WifiIface, w.EthIface, "ACCEPT", "")
}

func addIPTablesRule(table, chain, inIface, outIface, jump, state string) {
	err := modifyIPTables("-C", table, chain, inIface, outIface, jump, state)
	if err != nil {
		err = modifyIPTables("-A", table, chain, inIface, outIface, jump, state)
		if err != nil {
			logrus.Warn(fmt.Errorf("failed to add iptables rule %s:%s %s->%s %s %s: %w", table, chain, inIface, outIface, jump, state, err))
		}
	}
}

func delIPTablesRule(table, chain, inIface, outIface, jump, state string) {
	err := modifyIPTables("-C", table, chain, inIface, outIface, jump, state)
	if err != nil {
		return // iptables rule does not exist
	}
	err = modifyIPTables("-D", table, chain, inIface, outIface, jump, state)
	if err != nil {
		logrus.Warn(fmt.Errorf("failed to del iptables rule %s:%s %s->%s %s %s: %w", table, chain, inIface, outIface, jump, state, err))
	}
}

func modifyIPTables(op, table, chain, inIface, outIface, jump, state string) error {
	args := []string{"-t", table, op, chain, "-o", outIface, "-j", jump}
	if len(inIface) > 0 {
		args = append(args, "-i", inIface)
	}
	if len(state) > 0 {
		args = append(args, "-m", "state", "--state", state)
	}
	return runCmd("iptables", args...)
}

func runCmd(cmd string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, cmd, args...)
	var buf bytes.Buffer
	c.Stderr = &buf
	err := c.Run()
	if err != nil && buf.Len() > 0 {
		return fmt.Errorf("%s: %w: %s", cmd, err, strings.TrimSpace(buf.String()))
	}
	return err
}

func detectIface(prefixes ...string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		logrus.Error(fmt.Errorf("detect %s interface: %w", prefixes[0], err))
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
