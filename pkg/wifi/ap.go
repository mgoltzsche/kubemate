package wifi

import (
	"fmt"
	"os"

	"github.com/mgoltzsche/kubemate/pkg/cliutils"
	"github.com/mgoltzsche/kubemate/pkg/runner"
)

func (w *Wifi) StartAccessPoint(ssid, password string) error {
	if password == "" {
		return fmt.Errorf("start accesspoint: no wifi password configured")
	}
	ifacesConfChanged, err := w.generateNetworkInterfacesConfIfNotExist()
	if err != nil {
		return err
	}
	hostapdConf, hostapdConfChanged, err := w.generateHostapdConf(ssid, password)
	if err != nil {
		return err
	}
	dhcpdConf, dhcpdConfChanged, err := w.generateDhcpdConf()
	if err != nil {
		return err
	}
	err = createLeaseFileIfNotExist(w.DHCPDLeaseFile)
	if err != nil {
		return err
	}
	if ifacesConfChanged || hostapdConfChanged || dhcpdConfChanged || w.mode != WifiModeAccessPoint {
		err = w.restartWifiInterface()
		if err != nil {
			return err
		}
		err = runCmd("ip", "addr", "add", "11.0.0.1/24", "dev", w.WifiIface)
		if err != nil {
			return err
		}
		w.mode = WifiModeAccessPoint
	}
	w.installAPRoutes()
	err = w.dhcpd.Start(runner.Cmd("dhcpd", "-4", "-f", "-d", w.WifiIface, "-cf", dhcpdConf, "-lf", w.DHCPDLeaseFile, "--no-pid"))
	if err != nil {
		return err
	}
	err = w.ap.Start(runner.Cmd("hostapd", hostapdConf))
	if err != nil {
		return err
	}
	return nil
}

func (w *Wifi) StopAccessPoint() {
	w.uninstallAPRoutes()
	w.ap.Stop()
	w.dhcpd.Stop()
	if w.mode == WifiModeAccessPoint {
		err := w.restartWifiInterface()
		if err != nil {
			w.logger.Error(fmt.Errorf("stop access point: %w", err))
		}
		w.mode = WifiModeDisabled
	}
}

func (w *Wifi) generateNetworkInterfacesConfIfNotExist() (bool, error) {
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
			return false, fmt.Errorf("write network interface config: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func (w *Wifi) generateDhcpdConf() (string, bool, error) {
	return cliutils.WriteTempConfigFile("dhcpd", `# DNS update configuration
ddns-update-style interim;
update-static-leases on; # update dns for static entries
allow client-updates;
include "{dnsKeyFile}";

# options for all networks
default-lease-time 600;
max-lease-time 7200;

authoritative;

zone kube.m8. {
  primary 127.0.0.1;
  key kubemate;
}

zone 0.0.11.in-addr.arpa. {
  primary 127.0.0.1;
  key kubemate;
}

ddns-domainname "kube.m8.";
ddns-rev-domainname "in-addr.arpa.";

option captive-portal-rfc7710 code 160 = string;

subnet 11.0.0.0 netmask 255.255.255.0 {
  range 11.0.0.10 11.0.0.20;
  option broadcast-address 11.0.0.255;
  option domain-name "kube.m8";
  option domain-name-servers 11.0.0.1;
  option routers 11.0.0.1;
  option captive-portal-rfc7710 "{captivePortal}";
}
`, "{dnsKeyFile}", w.DNSKeyFile, "{captivePortal}", w.CaptivePortalURL)
}

func (w *Wifi) generateHostapdConf(ssid, password string) (string, bool, error) {
	// See https://wiki.gentoo.org/wiki/Hostapd
	return cliutils.WriteTempConfigFile("hostapd", `interface={iface}
driver=nl80211
hw_mode=g
channel=6
ieee80211d=1
ignore_broadcast_ssid=0
country_code={countryCode}
ieee80211d=1

ssid={ssid}
wpa=2
wpa_passphrase={password}
wpa_key_mgmt=WPA-PSK
wpa_pairwise=TKIP
rsn_pairwise=CCMP
auth_algs=1
macaddr_acl=0
`, "{iface}", w.WifiIface, "{countryCode}", w.CountryCode, "{ssid}", ssid, "{password}", password)
}

func (w *Wifi) installAPRoutes() {
	w.logger.WithField("iface", w.WifiIface).Debug("adding access point ip routes")
	w.configureIPRoutes(w.addIPTablesRule)
}

func (w *Wifi) uninstallAPRoutes() {
	w.logger.WithField("iface", w.WifiIface).Debug("removing access point ip routes")
	w.configureIPRoutes(w.delIPTablesRule)
}

func (w *Wifi) configureIPRoutes(apply func(table, chain, inIface, outIface, jump, state string)) {
	apply("nat", "POSTROUTING", "", w.WifiIface, "MASQUERADE", "")
	apply("filter", "FORWARD", w.EthIface, w.WifiIface, "ACCEPT", "RELATED,ESTABLISHED")
	apply("filter", "FORWARD", w.WifiIface, w.EthIface, "ACCEPT", "")
}

func (w *Wifi) addIPTablesRule(table, chain, inIface, outIface, jump, state string) {
	err := modifyIPTables("-C", table, chain, inIface, outIface, jump, state)
	if err != nil {
		err = modifyIPTables("-A", table, chain, inIface, outIface, jump, state)
		if err != nil {
			w.logger.Warn(fmt.Errorf("failed to add iptables rule %s:%s %s->%s %s %s: %w", table, chain, inIface, outIface, jump, state, err))
		}
	}
}

func (w *Wifi) delIPTablesRule(table, chain, inIface, outIface, jump, state string) {
	err := modifyIPTables("-C", table, chain, inIface, outIface, jump, state)
	if err != nil {
		return // iptables rule does not exist
	}
	err = modifyIPTables("-D", table, chain, inIface, outIface, jump, state)
	if err != nil {
		w.logger.Warn(fmt.Errorf("failed to del iptables rule %s:%s %s->%s %s %s: %w", table, chain, inIface, outIface, jump, state, err))
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
