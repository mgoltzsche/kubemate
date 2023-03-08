package middleware

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

// See https://wiki.ding.net/index.php?title=Detecting_captive_portals
var detectionHosts = map[string]struct{}{
	// Firefox
	"detectportal.firefox.com": struct{}{},
	// Android 4.4
	"clients3.google.com": struct{}{},
	// Android 6+
	"connectivitycheck.gstatic.com": struct{}{},
	// Windows 10, 11
	"www.msftconnecttest.com:80": struct{}{},
	// MacOS & iOS
	"captive.apple.com": struct{}{},
	// Xiaomi MIUI 11+
	"connect.rom.miui.com": struct{}{},
}

func WithCaptivePortalRedirects(url string, delegate http.Handler) http.Handler {
	return &captivePortalRedirectHandler{
		url:      url,
		delegate: delegate,
	}
}

type captivePortalRedirectHandler struct {
	url      string
	delegate http.Handler
}

func (h *captivePortalRedirectHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	_, isDetectionRequest := detectionHosts[req.Host]
	if isDetectionRequest {
		logrus.WithField("host", req.Host).WithField("client", req.RemoteAddr).Debugf("redirecting captive portal test request to %s", h.url)
		http.Redirect(w, req, h.url, http.StatusFound)
		return
	}
	h.delegate.ServeHTTP(w, req)
}
