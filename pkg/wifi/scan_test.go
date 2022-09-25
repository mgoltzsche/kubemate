package wifi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const exampleIwScanResult = `BSS 3c:37:12:04:6c:62(on wlp6s0)
	TSF: 8136435404857 usec (94d, 04:07:15)
	freq: 5620
	beacon interval: 100 TUs
	capability: ESS Privacy SpectrumMgmt ShortSlotTime RadioMeasure (0x1511)
	signal: -90.00 dBm
	last seen: 1024 ms ago
	SSID: Some Network
	Supported rates: 6.0* 9.0 12.0* 18.0 24.0* 36.0 48.0 54.0 
	DS Parameter set: channel 124
	TIM: DTIM Count 0 DTIM Period 1 Bitmap Control 0x0 Bitmap[0] 0x0
	Country: DE	Environment: Indoor/Outdoor
		Channels [36 - 36] @ 23 dBm
		Channels [40 - 40] @ 23 dBm
		Channels [44 - 44] @ 23 dBm
		Channels [48 - 48] @ 23 dBm
		Channels [52 - 52] @ 23 dBm
		Channels [56 - 56] @ 23 dBm
		Channels [60 - 60] @ 23 dBm
		Channels [64 - 64] @ 23 dBm
		Channels [100 - 100] @ 30 dBm
		Channels [104 - 104] @ 30 dBm
		Channels [108 - 108] @ 30 dBm
		Channels [112 - 112] @ 30 dBm
		Channels [116 - 116] @ 30 dBm
		Channels [120 - 120] @ 30 dBm
		Channels [124 - 124] @ 30 dBm
		Channels [128 - 128] @ 30 dBm
		Channels [132 - 132] @ 30 dBm
		Channels [136 - 136] @ 30 dBm
		Channels [140 - 140] @ 30 dBm
	Power constraint: 3 dB
	BSS Load:
		 * station count: 3
		 * channel utilisation: 1/255
		 * available admission capacity: 0 [*32us]
	HT capabilities:
		Capabilities: 0x9ef
			RX LDPC
			HT20/HT40
			SM Power Save disabled
			RX HT20 SGI
			RX HT40 SGI
			TX STBC
			RX STBC 1-stream
			Max AMSDU length: 7935 bytes
			No DSSS/CCK HT40
		Maximum RX AMPDU length 65535 bytes (exponent: 0x003)
		Minimum RX AMPDU time spacing: No restriction (0x00)
		HT TX/RX MCS rate indexes supported: 0-15
	HT operation:
		 * primary channel: 124
		 * secondary channel offset: above
		 * STA channel width: any
		 * RIFS: 0
		 * HT protection: no
		 * non-GF present: 1
		 * OBSS non-GF present: 0
		 * dual beacon: 0
		 * dual CTS protection: 0
		 * STBC beacon: 0
		 * L-SIG TXOP Prot: 0
		 * PCO active: 0
		 * PCO phase: 0
	Overlapping BSS scan params:
		 * passive dwell: 20 TUs
		 * active dwell: 10 TUs
		 * channel width trigger scan interval: 300 s
		 * scan passive total per channel: 200 TUs
		 * scan active total per channel: 20 TUs
		 * BSS width channel transition delay factor: 5
		 * OBSS Scan Activity Threshold: 0.25 %
	Extended capabilities:
		 * HT Information Exchange Supported
		 * Extended Channel Switching
		 * TFS
		 * WNM-Sleep Mode
		 * TIM Broadcast
		 * BSS Transition
		 * Operating Mode Notification
		 * Max Number Of MSDUs In A-MSDU is unlimited
	VHT capabilities:
		VHT Capabilities (0x338959b2):
			Max MPDU length: 11454
			Supported Channel Width: neither 160 nor 80+80
			RX LDPC
			short GI (80 MHz)
			TX STBC
			SU Beamformer
			SU Beamformee
			MU Beamformer
			RX antenna pattern consistency
			TX antenna pattern consistency
		VHT RX MCS set:
			1 streams: MCS 0-9
			2 streams: MCS 0-9
			3 streams: not supported
			4 streams: not supported
			5 streams: not supported
			6 streams: not supported
			7 streams: not supported
			8 streams: not supported
		VHT RX highest supported: 0 Mbps
		VHT TX MCS set:
			1 streams: MCS 0-9
			2 streams: MCS 0-9
			3 streams: not supported
			4 streams: not supported
			5 streams: not supported
			6 streams: not supported
			7 streams: not supported
			8 streams: not supported
		VHT TX highest supported: 0 Mbps
	VHT operation:
		 * channel width: 1 (80 MHz)
		 * center freq segment 1: 122
		 * center freq segment 2: 0
		 * VHT basic MCS set: 0xfffc
	WMM:	 * Parameter version 1
		 * BE: CW 15-1023, AIFSN 3
		 * BK: CW 15-1023, AIFSN 7
		 * VI: CW 7-15, AIFSN 2, TXOP 3008 usec
		 * VO: CW 3-7, AIFSN 2, TXOP 1504 usec
	WPS:	 * Version: 1.0
		 * Wi-Fi Protected Setup State: 2 (Configured)
		 * RF Bands: 0x3
		 * Unknown TLV (0x1049, 6 bytes): 00 37 2a 00 01 20
	RSN:	 * Version: 1
		 * Group cipher: CCMP
		 * Pairwise ciphers: CCMP
		 * Authentication suites: PSK
		 * Capabilities: 1-PTKSA-RC 1-GTKSA-RC (0x0000)
BSS d0:05:2a:71:b8:75(on wlp6s0)
	TSF: 260181913660 usec (3d, 00:16:21)
	freq: 5500
	beacon interval: 100 TUs
	capability: ESS Privacy SpectrumMgmt RadioMeasure (0x1111)
	signal: -87.00 dBm
	last seen: 1620 ms ago
	SSID: WLAN-Kabel
	Supported rates: 6.0* 9.0 12.0* 18.0 24.0* 36.0 48.0 54.0 
	TIM: DTIM Count 0 DTIM Period 3 Bitmap Control 0x0 Bitmap[0] 0x0
	Country: DE	Environment: Indoor/Outdoor
		Channels [36 - 36] @ 20 dBm
		Channels [40 - 40] @ 20 dBm
		Channels [44 - 44] @ 20 dBm
		Channels [48 - 48] @ 20 dBm
		Channels [52 - 52] @ 20 dBm
		Channels [56 - 56] @ 20 dBm
		Channels [60 - 60] @ 20 dBm
		Channels [64 - 64] @ 20 dBm
		Channels [100 - 100] @ 20 dBm
		Channels [104 - 104] @ 20 dBm
		Channels [108 - 108] @ 20 dBm
		Channels [112 - 112] @ 20 dBm
		Channels [116 - 116] @ 20 dBm
		Channels [132 - 132] @ 20 dBm
		Channels [136 - 136] @ 20 dBm
		Channels [140 - 140] @ 20 dBm
	Power constraint: 0 dB
	TPC report: TX power: 17 dBm
	RSN:	 * Version: 1
		 * Group cipher: CCMP
		 * Pairwise ciphers: CCMP
		 * Authentication suites: PSK
		 * Capabilities: 16-PTKSA-RC 1-GTKSA-RC (0x000c)
	HT capabilities:
		Capabilities: 0x9ef
			RX LDPC
			HT20/HT40
			SM Power Save disabled
			RX HT20 SGI
			RX HT40 SGI
			TX STBC
			RX STBC 1-stream
			Max AMSDU length: 7935 bytes
			No DSSS/CCK HT40
		Maximum RX AMPDU length 65535 bytes (exponent: 0x003)
		Minimum RX AMPDU time spacing: 4 usec (0x05)
		HT RX MCS rate indexes supported: 0-23
		HT TX MCS rate indexes are undefined
	HT operation:
		 * primary channel: 100
		 * secondary channel offset: above
		 * STA channel width: any
		 * RIFS: 1
		 * HT protection: no
		 * non-GF present: 0
		 * OBSS non-GF present: 0
		 * dual beacon: 0
		 * dual CTS protection: 0
		 * STBC beacon: 0
		 * L-SIG TXOP Prot: 0
		 * PCO active: 0
		 * PCO phase: 0
	Extended capabilities:
		 * Extended Channel Switching
		 * Operating Mode Notification
		 * Max Number Of MSDUs In A-MSDU is unlimited
	VHT capabilities:
		VHT Capabilities (0x0f8259b2):
			Max MPDU length: 11454
			Supported Channel Width: neither 160 nor 80+80
			RX LDPC
			short GI (80 MHz)
			TX STBC
			SU Beamformer
			SU Beamformee
		VHT RX MCS set:
			1 streams: MCS 0-9
			2 streams: MCS 0-9
			3 streams: MCS 0-9
			4 streams: not supported
			5 streams: not supported
			6 streams: not supported
			7 streams: not supported
			8 streams: not supported
		VHT RX highest supported: 0 Mbps
		VHT TX MCS set:
			1 streams: MCS 0-9
			2 streams: MCS 0-9
			3 streams: MCS 0-9
			4 streams: not supported
			5 streams: not supported
			6 streams: not supported
			7 streams: not supported
			8 streams: not supported
		VHT TX highest supported: 0 Mbps
	VHT operation:
		 * channel width: 1 (80 MHz)
		 * center freq segment 1: 106
		 * center freq segment 2: 0
		 * VHT basic MCS set: 0x0000
	WPS:	 * Version: 1.0
		 * Wi-Fi Protected Setup State: 2 (Configured)
		 * UUID: 86a3f591-24b3-ecfe-d636-0b97183bd5fb
		 * RF Bands: 0x3
		 * Unknown TLV (0x1049, 6 bytes): 00 37 2a 00 01 20
	WMM:	 * Parameter version 1
		 * u-APSD
		 * BE: CW 15-1023, AIFSN 3
		 * BK: CW 15-1023, AIFSN 7
		 * VI: CW 7-15, AIFSN 2, TXOP 3008 usec
		 * VO: CW 3-7, AIFSN 2, TXOP 1504 usec
`

func TestParseNetworkScanResult(t *testing.T) {
	for _, c := range []struct {
		name   string
		input  string
		expect []WifiNetwork
	}{
		{
			name:  "complete example",
			input: exampleIwScanResult,
			expect: []WifiNetwork{
				{
					MAC:     "3c:37:12:04:6c:62",
					SSID:    "Some Network",
					Country: "DE",
				},
				{
					MAC:     "d0:05:2a:71:b8:75",
					SSID:    "WLAN-Kabel",
					Country: "DE",
				},
			},
		},
		{
			name:  "minimal valid entry",
			input: "BSS d0:05:2a:71:b8:75\n	SSID: fake-ssid\n	Country: DE",
			expect: []WifiNetwork{{
				MAC:     "d0:05:2a:71:b8:75",
				SSID:    "fake-ssid",
				Country: "DE",
			}},
		},
		{
			name:   "invalid: mac address too short",
			input:  "BSS d0:05:2a:71:b8:7\n	SSID: fake-ssid",
			expect: []WifiNetwork{},
		},
		{
			name:  "invalid: country too short",
			input: "BSS d0:05:2a:71:b8:75\n	SSID: fake-ssid\n	Country: D",
			expect: []WifiNetwork{
				{
					MAC:     "d0:05:2a:71:b8:75",
					SSID:    "fake-ssid",
					Country: "",
				},
			},
		},
		{
			name:   "invalid: ssid missing",
			input:  "BSS d0:05:2a:71:b8:75\n	Country: DE",
			expect: []WifiNetwork{},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			networks := parseNetworkScanResult(c.input)
			require.Equal(t, c.expect, networks)
		})
	}
}
