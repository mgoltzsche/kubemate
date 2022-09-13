package wifi

import (
	"context"
	"fmt"

	"github.com/mgoltzsche/kubemate/pkg/runner"
)

// See https://wiki.archlinux.org/title/iwd and https://iwd.wiki.kernel.org/ap_mode

func (w *Wifi) StartService() error {
	ctx := context.Background()
	return w.station.Start(ctx, runner.Cmd("/usr/libexec/iwd"))
}

func (w *Wifi) StartStation(ssid, password string) error {
	/*iwdDir := "/var/lib/iwd"
	confFile := filepath.Join(iwdDir, fmt.Sprintf("%s.psk", ssid))
	conf := fmt.Sprintf("[Security]\nPassphrase=%s", password)
	err := os.WriteFile(confFile, []byte(conf), 0600)
	if err != nil {
		return err
	}*/
	err := runCmds([][]string{
		{"iwctl", "device", w.WifiIface, "set-property", "Mode", "station"},
		{"iwctl", "--passphrase", password, "station", w.WifiIface, "connect", ssid},
	})
	if err != nil {
		return fmt.Errorf("start wifi station: %w", err)
	}
	return nil
}

func (w *Wifi) StopStation() error {
	err := runCmd("iwctl", "station", w.WifiIface, "disconnect")
	if err != nil {
		return fmt.Errorf("disconnect wifi station: %w", err)
	}
	return nil
}
