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

var wifiScanResultLineRegex = regexp.MustCompile(`^([^\s]+)\t([0-9]+)\t([0-9-]+)\t([^\s]+)\t(.+)$`)

type WifiNetwork struct {
	SSID string
}

func scanForWifiNetworks(ctx context.Context, iface string) ([]WifiNetwork, error) {
	err := triggerWifiNetworkScan(iface)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := runCmdOut(ctx, "wpa_cli", "-i", iface, "scan_results")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return parseWPACLIScanResult(lines), nil
}

func runCmdOut(ctx context.Context, cmd string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", cmd, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
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
