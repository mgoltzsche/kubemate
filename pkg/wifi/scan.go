package wifi

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// TODO: consider using iw for wifi scan.
// Use it particularly to detect the country based on existing networks when the device is started for the first time.
//   iw dev wlp6s0 scan

var wifiScanResultLineRegex = regexp.MustCompile(`^([^\s]+)\t([0-9]+)\t([0-9-]+)\t([^\s]+)\t(.+)$`)

type WifiNetwork struct {
	SSID string
}

func ScanForWifiNetworks(ctx context.Context, iface string) ([]WifiNetwork, error) {
	err := triggerWifiNetworkScan(iface)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "wpa_cli", "-i", iface, "scan_results")
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err = c.Run()
	if err != nil {
		return nil, fmt.Errorf("wpa_cli: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	return parseWPACLIScanResult(lines), nil
}

func triggerWifiNetworkScan(iface string) error {
	err := runCmd("wpa_cli", "-i", iface, "scan")
	if err != nil {
		return fmt.Errorf("trigger wifi network scan: %w", err)
	}
	return nil
}

func parseWPACLIScanResult(lines []string) []WifiNetwork {
	networks := make([]WifiNetwork, 0, len(lines))
	for _, line := range lines {
		found := wifiScanResultLineRegex.FindStringSubmatch(line)
		if len(found) == 6 {
			networks = append(networks, WifiNetwork{
				SSID: found[5],
			})
		}
	}
	return networks
}
