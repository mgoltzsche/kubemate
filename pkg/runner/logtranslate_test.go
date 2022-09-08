package runner

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestParseAndLogProcessLogLine(t *testing.T) {
	for _, c := range []struct {
		name   string
		input  string
		match  bool
		level  string
		output string
	}{
		{
			name:  "invalid",
			input: "hello world",
			match: false,
		},
		{
			name:   "simple info",
			input:  "I0903 21:06:43.913400  589029 kube.go:128] success",
			match:  true,
			level:  "info",
			output: "kube.go:128] success",
		},
		{
			name:   "simple error",
			input:  "E0903 21:06:43.913400  589029 kube.go:128] fake error",
			match:  true,
			level:  "error",
			output: "fake error",
		},
		{
			name:   "simple fatal",
			input:  "F0903 21:06:43.913400  589029 kube.go:128] fake fatal",
			match:  true,
			level:  "error",
			output: "fake fatal",
		},
		{
			name:   "simple warning",
			input:  "W0903 21:06:43.913400  589029 kube.go:128] caution!",
			match:  true,
			level:  "warning",
			output: "caution!",
		},
		{
			name:   "simple debug",
			input:  "D0903 21:06:43.913400  589029 kube.go:128] fake debug",
			match:  true,
			level:  "debug",
			output: "fake debug",
		},
		{
			name:   "simple trace",
			input:  "T0903 21:06:43.913400  589029 kube.go:128] fake trace",
			match:  true,
			level:  "trace",
			output: "fake trace",
		},
		{
			name:   "go info",
			input:  `time="2022-09-03T21:56:04Z" level=info msg="operation \"x\" succeeded"`,
			match:  true,
			level:  "info",
			output: "operation \\\"x\\\" succeeded",
		},
		{
			name:   "go error",
			input:  `time="2022-09-03T21:56:04Z" level=error msg="some failure"`,
			match:  true,
			level:  "error",
			output: "some failure",
		},
		{
			name:   "go fatal",
			input:  `time="2022-09-03T21:56:04Z" level=fatal msg="some failure"`,
			match:  true,
			level:  "error",
			output: "some failure",
		},
		{
			name:   "go warning",
			input:  `time="2022-09-03T21:56:04Z" level=warning msg="caution"`,
			match:  true,
			level:  "warning",
			output: "caution",
		},
		{
			name:   "go debug",
			input:  `time="2022-09-03T21:56:04Z" level=debug msg="debug"`,
			match:  true,
			level:  "debug",
			output: "debug",
		},
		{
			name:   "go trace",
			input:  `time="2022-09-03T21:56:04Z" level=trace msg="trace"`,
			match:  true,
			level:  "trace",
			output: "trace",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			log := logrus.New()
			log.SetOutput(buf)
			log.SetLevel(logrus.TraceLevel)
			logger := logrus.NewEntry(log)
			m := parseAndLogProcessLogLine(c.input, logger)
			require.Equal(t, c.match, m, "match")
			require.Contains(t, buf.String(), c.output, "output")
			if c.match {
				require.Containsf(t, buf.String(), fmt.Sprintf(" level=%s ", c.level), "level")
			}
		})
	}
}
