package wifi

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/mgoltzsche/kubemate/pkg/cliutils"
)

var wifiScanResultLineRegex = regexp.MustCompile(`^([^\s]+)\t([0-9]+)\t([0-9-]+)\t([^\s]+)\t(.+)$`)

// scanWifiNetworksWPACLI uses wpa_supplicant to scan wifi networks.
// The result does not specify the wifi country per network.
func scanWifiNetworksWPACLI(iface string) ([]WifiNetwork, error) {
	// Scanning interrupts the current wifi connection:
	/*err := runCmd("wpa_cli", "-i", iface, "scan")
	if err != nil {
		return nil, err
	}*/
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	out, err := cliutils.Run(ctx, "wpa_cli", "-i", iface, "scan_results")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return parseWPACLIScanResult(lines), nil
}

func parseWPACLIScanResult(lines []string) []WifiNetwork {
	networks := make([]WifiNetwork, 0, len(lines))
	for _, line := range lines {
		found := wifiScanResultLineRegex.FindStringSubmatch(line)
		if len(found) == 6 {
			networks = append(networks, WifiNetwork{
				SSID: found[5],
				MAC:  found[1],
			})
		}
	}
	return networks
}
