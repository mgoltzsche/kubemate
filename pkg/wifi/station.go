package wifi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mgoltzsche/kubemate/pkg/runner"
)

const wpaSupplicantConfFile = "/tmp/kubemate-wpa-supplicant.conf"

func (w *Wifi) StartStation(ssid, password string) error {
	confFile, confChanged, err := w.generateWpaSupplicantConf(ssid, password)
	if err != nil {
		return err
	}
	if confChanged || w.mode != WifiModeStation {
		err = w.restartWifiInterface()
		if err != nil {
			return err
		}
		w.station.Stop()
		w.mode = WifiModeStation
	}
	_, err = w.station.Start(runner.Cmd("wpa_supplicant", "-i", w.WifiIface, "-c", confFile))
	if err != nil {
		return err
	}
	err = w.runDHCPCD()
	if err != nil {
		return err
	}
	if w.WriteHostResolvConf {
		err = copyFile("/etc/resolv.conf", "/host/etc/resolv.conf")
		if err != nil {
			return fmt.Errorf("copy resolv.conf to host file system: %w", err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	err = os.WriteFile(dst, b, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (w *Wifi) StopStation() {
	w.station.Stop()
}

func (w *Wifi) generateWpaSupplicantConf(ssid, password string) (string, bool, error) {
	if w.CountryCode == "" {
		return "", false, fmt.Errorf("country code not specified")
	}
	network := "\n"
	if ssid != "" {
		if password != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "wpa_passphrase", ssid, password)
			var stderr, stdout bytes.Buffer
			cmd.Stderr = &stderr
			cmd.Stdout = &stdout
			err := cmd.Run()
			if err != nil {
				msg := strings.TrimSpace(fmt.Sprintf("%s\n%s", stdout.String(), stderr.String()))
				if len(msg) == 0 {
					msg = err.Error()
				}
				return "", false, fmt.Errorf("wpa_passphrase: %s", msg)
			}
			network = stdout.String()
		}
	}
	configTpl := `ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
country=%s
%s`
	return writeConf("wpa_supplicant", configTpl, w.CountryCode, network)
}

func (w *Wifi) runDHCPCD() error {
	err := createLeaseFileIfNotExist(w.DHCPCDLeaseFile)
	if err != nil {
		return err
	}
	logger := w.logger.WithField("proc", "dhcpcd")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "dhcpcd", w.WifiIface)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err = c.Run()
	if err != nil {
		return fmt.Errorf("dhcpcd: %w: %s", err, strings.TrimSpace(out.String()))
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if line != "" {
			logger.Debug(line)
		}
	}
	// TODO: run after ethernet cable has been unplugged, to advertize hostname for the wifi IP: dhcpcd -n wlan0
	return err
}
