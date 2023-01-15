package wifi

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// detectCountry derives the wifi country based on near wifi networks.
func (w *Wifi) detectCountry(iface string, logger *logrus.Entry) (string, error) {
	for _, n := range w.Networks() {
		if n.Country != "" {
			return n.Country, nil
		}
	}
	return "", fmt.Errorf("detect wifi country: no wifi network found to derive country from")
}
