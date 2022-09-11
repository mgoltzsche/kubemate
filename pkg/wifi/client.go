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

func (w *Wifi) StartClient(ssid, password string) error {
	confFile, confChanged, err := w.generateWpaSupplicantConf(ssid, password)
	if err != nil {
		return err
	}
	if confChanged {
		err = w.restartWifiInterface()
		if err != nil {
			return err
		}
	}
	ctx := context.Background()
	return w.client.Start(ctx, runner.Cmd("wpa_supplicant", "-i", w.WifiIface, "-c", confFile))
}

func (w *Wifi) StopClient() error {
	return w.client.Stop()
}

func (w *Wifi) generateWpaSupplicantConf(ssid, password string) (string, bool, error) {
	if w.CountryCode == "" {
		return "", false, fmt.Errorf("country code not specified")
	}
	network := "\n"
	if ssid != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "wpa_passphrase", ssid, password)
		var stderr, stdout bytes.Buffer
		cmd.Stderr = &stderr
		cmd.Stdout = &stdout
		err := cmd.Run()
		if err != nil {
			return "", false, fmt.Errorf("wpa_passphrase: %w: %s", err, strings.TrimSpace(stderr.String()))
		}
		network = stdout.String()
	}
	configTpl := `ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
country=%s
%s`
	return writeConf("wpa_supplicant", configTpl, w.CountryCode, network)
}