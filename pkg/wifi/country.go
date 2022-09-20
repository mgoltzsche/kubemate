package wifi

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const countryPrefix = "	Country: "

// detectCountry derives the wifi country based on near wifi networks.
func detectCountry(iface string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := runCmdOut(ctx, "iw", "dev", iface, "scan")
	if err != nil {
		return "", fmt.Errorf("detect wifi country: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, countryPrefix) && len(l) > len(countryPrefix)+2 {
			return l[len(countryPrefix) : len(countryPrefix)+2], nil
		}
	}
	return "", fmt.Errorf("detect wifi country: no wifi network found to derive country from")
}
