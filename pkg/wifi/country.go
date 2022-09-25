package wifi

import (
	"context"
	"fmt"
)

// detectCountry derives the wifi country based on near wifi networks.
func detectCountry(iface string) (string, error) {
	networks, err := scanWifiNetworks(context.Background(), iface)
	if err != nil {
		return "", fmt.Errorf("detect wifi country: %w", err)
	}
	for _, n := range networks {
		if n.Country != "" {
			return n.Country, nil
		}
	}
	return "", fmt.Errorf("detect wifi country: no wifi network found to derive country from")
}
