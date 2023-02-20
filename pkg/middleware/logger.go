package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type accessLogMiddleware struct {
	delegate http.Handler
	logger   *logrus.Entry
}

func WithAccessLog(h http.Handler, l *logrus.Entry) http.Handler {
	return &accessLogMiddleware{
		delegate: h,
		logger:   l,
	}
}

func (h *accessLogMiddleware) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w = &accessLoggingWriter{
		ResponseWriter: w,
		startTime:      time.Now(),
		logger: h.logger.
			WithField("method", req.Method).
			WithField("host", req.Host).
			WithField("path", req.URL.Path),
	}
	h.delegate.ServeHTTP(w, req)
}

type accessLoggingWriter struct {
	http.ResponseWriter
	startTime time.Time
	logger    *logrus.Entry
}

func (w *accessLoggingWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.logger.
		WithField("status", statusCode).
		WithField("duration", time.Since(w.startTime)).
		Trace("served request")
}

func (w *accessLoggingWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer cannot be hijacked")
	}
	return h.Hijack()
}
