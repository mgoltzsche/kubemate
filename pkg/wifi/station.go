package wifi

import (
	"bytes"
	"context"
	"fmt"
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
	err = w.StartWifiInterface()
	if err != nil {
		return err
	}
	if confChanged {
		err = w.restartWifiInterface()
		if err != nil {
			return err
		}
		err = triggerWifiNetworkScan(w.WifiIface)
		if err != nil {
			return fmt.Errorf("trigger wifi network scan: %w", err)
		}
	}
	ctx := context.Background()
	return w.station.Start(ctx, runner.Cmd("wpa_supplicant", "-i", w.WifiIface, "-c", confFile))
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
