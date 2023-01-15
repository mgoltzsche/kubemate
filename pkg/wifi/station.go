package wifi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
		err = w.station.Stop()
		if err != nil {
			return err
		}
		w.mode = WifiModeStation
	}
	err = w.station.Start(runner.Cmd("wpa_supplicant", "-i", w.WifiIface, "-c", confFile))
	if err != nil {
		return err
	}
	err = w.runDHClient()
	if err != nil {
		return err
	}
	return nil
}

func (w *Wifi) StopStation() error {
	return w.station.Stop()
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

func (w *Wifi) runDHClient() error {
	err := os.MkdirAll(filepath.Dir(w.DHClientLeaseFile), 0755)
	if err != nil {
		return err
	}
	cf, err := w.generateDHClientConfig()
	if err != nil {
		return err
	}
	logger := w.logger.WithField("proc", "dhclient")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "dhclient", "-cf", cf, "-sf", "/etc/dhclient-script", "-lf", w.DHClientLeaseFile, w.WifiIface)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err = c.Run()
	if err != nil {
		return fmt.Errorf("dhclient: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if line != "" {
			logger.Debug(line)
		}
	}
	for _, line := range strings.Split(stderr.String(), "\n") {
		if line != "" {
			logger.Error(line)
		}
	}
	return err
}

func (w *Wifi) generateDHClientConfig() (string, error) {
	confTpl := `
backoff-cutoff 2;
initial-interval 1;
link-timeout 10;
reboot 10;
retry 10;
select-timeout 5;
timeout 30;

interface %q {
  prepend domain-name-servers 127.0.0.1;
  request subnet-mask,
          broadcast-address,
          routers,
          domain-name,
          domain-name-servers,
          host-name;
  require routers,
          subnet-mask,
          domain-name-servers;
 }`
	file, _, err := writeConf("dhclient", confTpl, w.WifiIface)
	return file, err
}
