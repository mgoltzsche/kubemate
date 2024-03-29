package wifi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mgoltzsche/kubemate/pkg/cliutils"
	"github.com/sirupsen/logrus"
)

const iwIndent = "	"

// scanWifiNetworksIw uses the iw CLI to scan wifi networks.
// The result specifies the country per wifi network.
func scanWifiNetworksIw(iface string, logger *logrus.Entry) ([]WifiNetwork, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	out, err := cliutils.Run(ctx, "iw", "dev", iface, "scan", "ap-force")
	if err != nil {
		return nil, fmt.Errorf("wifi network scan: %w", err)
	}
	return parseIwNetworkScanResult(out, logger), nil
}

func parseIwNetworkScanResult(iwOutput string, logger *logrus.Entry) []WifiNetwork {
	lines := strings.Split(strings.TrimSpace(iwOutput), "\n")
	networks := make([]WifiNetwork, 0, 5)
	i := -1
	entry := false
	for _, line := range lines {
		if strings.HasPrefix(line, "BSS ") && len(line) >= 4+17 {
			// start new network entry
			networks = append(networks, WifiNetwork{MAC: line[4 : 4+17]})
			i++
			entry = true
			continue
		}
		if !strings.HasPrefix(line, iwIndent) {
			entry = false
			logger.Warnf("parse network scan result: unexpected line: %s", line)
			continue
		}
		if !entry {
			continue
		}
		line = line[len(iwIndent):]
		colonPos := strings.Index(line, ":")
		if colonPos < 1 || colonPos >= len(line)-3 {
			continue
		}
		// Add attribute to previously found entry
		key := line[:colonPos]
		value := line[colonPos+2:]
		switch key {
		case "SSID":
			networks[i].SSID = value
		case "Country":
			if len(value) >= 2 {
				networks[i].Country = value[:2]
			}
		}
	}
	filtered := make([]WifiNetwork, 0, len(networks))
	for _, n := range networks {
		if n.SSID != "" {
			filtered = append(filtered, n)
		}
	}
	return filtered
}
