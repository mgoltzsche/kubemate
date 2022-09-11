package wifi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseWPACLIScanResult(t *testing.T) {
	for _, c := range []struct {
		name   string
		lines  []string
		expect []WifiNetwork
	}{
		{
			name: "lines",
			lines: []string{
				"unexpected",
				"de:53:7c:de:7b:ee	5500	-85	[WPA-EAP-CCMP][WPA2-EAP-CCMP][ESS]	PYUR Community",
				"",
				"42:53:7c:de:7b:23	5500	-85	[WPA-EAP-CCMP]	Othernet",
			},
			expect: []WifiNetwork{
				{SSID: "PYUR Community"},
				{SSID: "Othernet"},
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			networks := parseWPACLIScanResult(c.lines)
			require.Equal(t, c.expect, networks)
		})
	}
}
