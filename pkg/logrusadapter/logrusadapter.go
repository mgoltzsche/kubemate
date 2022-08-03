package logrusadapter

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
)

func New(delegate *logrus.Entry) logr.Logger {
	return logr.New(&logrusSink{logger: delegate})
}

type logrusSink struct {
	logger *logrus.Entry
	names  []string
}

// Init receives optional information about the logr library for LogSink
// implementations that need it.
func (l *logrusSink) Init(info logr.RuntimeInfo) {}

// Enabled tests whether this LogSink is enabled at the specified V-level.
// For example, commandline flags might be used to set the logging
// verbosity and disable some info logs.
func (l *logrusSink) Enabled(level int) bool {
	return l.logger.Level >= toLogrusLevel(level)
}

// Info logs a non-error message with the given key/value pairs as context.
// The level argument is provided for optional logging.  This method will
// only be called when Enabled(level) is true. See Logger.Info for more
// details.
func (l *logrusSink) Info(level int, msg string, keysAndValues ...interface{}) {
	l.withFields(keysAndValues).Log(toLogrusLevel(level), msg)
}

// Error logs an error, with the given message and key/value pairs as
// context.  See Logger.Error for more details.
func (l *logrusSink) Error(err error, msg string, keysAndValues ...interface{}) {
	// TODO: how to know the loglevel here?
	l.withFields(keysAndValues).WithError(err).Error(msg)
}

// WithValues returns a new LogSink with additional key/value pairs.  See
// Logger.WithValues for more details.
func (l *logrusSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return &logrusSink{logger: l.withFields(keysAndValues), names: l.names}
}

// WithName returns a new LogSink with the specified name appended.  See
// Logger.WithName for more details.
func (l *logrusSink) WithName(name string) logr.LogSink {
	names := append(l.names, name)
	return &logrusSink{logger: l.logger.WithField("component", strings.Join(names, ".")), names: names}
}

func (l *logrusSink) withFields(keysAndValues ...interface{}) *logrus.Entry {
	m := make(map[string]interface{}, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues)-1; i += 2 {
		m[fmt.Sprintf("%s", keysAndValues[i])] = keysAndValues[i+1]
	}
	return l.logger.WithFields(logrus.Fields(m))
}

func toLogrusLevel(level int) logrus.Level {
	level++
	if level < int(logrus.FatalLevel) {
		level = int(logrus.FatalLevel)
	}
	if level > int(logrus.TraceLevel) {
		level = int(logrus.TraceLevel)
	}
	return logrus.Level(level)
}
