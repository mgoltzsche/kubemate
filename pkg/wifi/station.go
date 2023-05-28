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
		w.backupResolvConf()
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
	_, err = w.dhcpcd.Start(runner.Cmd("dhcpcd", "-B", "--metric=204", w.WifiIface))
	if err != nil {
		return err
	}
	return w.writeHostResolvConf()
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
	w.dhcpcd.Stop()
	w.restoreResolvConf()
	err := w.writeHostResolvConf()
	if err != nil {
		w.logger.Error(err)
	}
}

func (w *Wifi) writeHostResolvConf() error {
	if w.WriteHostResolvConf {
		err := copyFile("/etc/resolv.conf", "/host/etc/resolv.conf")
		if err != nil {
			return fmt.Errorf("copy resolv.conf to host file system: %w", err)
		}
	}
	return nil
}

func (w *Wifi) backupResolvConf() {
	backupFile := resolvConfBackupFile()
	if _, err := os.Stat(backupFile); os.IsNotExist(err) { // resolv.conf backup does not exist
		w.logger.Debugf("backing up /etc/resolv.conf to %s", backupFile)
		err := copyFile("/etc/resolv.conf", backupFile)
		if err != nil {
			w.logger.Errorf("wifi station: backup resolv.conf: %s", err)
		}
	}
}

func (w *Wifi) restoreResolvConf() {
	backupFile := resolvConfBackupFile()
	if _, err := os.Stat(backupFile); err == nil { // resolv.conf backup exists
		w.logger.Debugf("restoring /etc/resolv.conf from %s", backupFile)
		err := copyFile(backupFile, "/etc/resolv.conf")
		if err != nil {
			w.logger.Errorf("wifi station: restore resolv.conf: %s", err)
		}
	}
}

func resolvConfBackupFile() string {
	return filepath.Join(os.TempDir(), "kubemate-resolv-conf-backup")
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
