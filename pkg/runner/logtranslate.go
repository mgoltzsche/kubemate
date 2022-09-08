package runner

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/sirupsen/logrus"
)

var (
	goLogFormat    = regexp.MustCompile(`^time="[^"]+" level=([^ ]+) msg="`)
	shortLogFormat = regexp.MustCompile(`^(I|E|W|D|T|F)[0-9]+ [^ ]+ +[0-9]+ `)
)

func parseAndLogProcessLogLine(line string, logger *logrus.Entry) bool {
	found := shortLogFormat.FindString(line)
	if found != "" {
		msg := line[len(found):]
		switch found[0] {
		case 'I':
			logger.Info(msg)
		case 'F':
			fallthrough
		case 'E':
			logger.Error(msg)
		case 'W':
			logger.Warn(msg)
		case 'D':
			logger.Debug(msg)
		case 'T':
			logger.Trace(msg)
		default:
			logger.Warn(msg)
		}
		return true
	}
	m := goLogFormat.FindStringSubmatch(line)
	if len(m) == 2 {
		msg := line[len(m[0]):]
		if msg[len(msg)-1] == '"' {
			msg = msg[:len(msg)-1]
		}
		msgu, err := strconv.Unquote(fmt.Sprintf(`"%s"`, msg))
		if err == nil {
			msg = msgu
		}
		switch m[1] {
		case "info":
			logger.Info(msg)
		case "fatal":
			fallthrough
		case "error":
			logger.Error(msg)
		case "warn":
			logger.Warn(msg)
		case "debug":
			logger.Debug(msg)
		case "trace":
			logger.Trace(msg)
		default:
			logger.Warn(msg)
		}
		return true
	}
	return false
}
