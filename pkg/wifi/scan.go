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

// TODO: rewrite the scan: https://unix.stackexchange.com/questions/477488/connect-to-wifi-from-command-line-on-linux-systems-through-the-iwd-wireless-dae

var wifiScanResultLineRegex = regexp.MustCompile(`^([^\s]+)\t([0-9]+)\t([0-9-]+)\t([^\s]+)\t(.+)$`)

type WifiNetwork struct {
	SSID string
}

func ScanForWifiNetworks(ctx context.Context, iface string) ([]WifiNetwork, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "wpa_cli", "-i", iface, "scan_results")
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	if err != nil {
		return nil, fmt.Errorf("wpa_cli: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	return parseWPACLIScanResult(lines), nil
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
