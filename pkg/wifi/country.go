package wifi

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// detectCountry derives the wifi country based on near wifi networks.
func detectCountry(iface string, logger *logrus.Entry) (string, error) {
	networks, err := scanWifiNetworksIw(iface, logger)
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
