package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

func ForceHTTPS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.TLS == nil && !strings.HasPrefix(req.RemoteAddr, "127.0.0.1") {
			u := fmt.Sprintf("https://%s%s", req.Host, req.URL.String())
			logrus.WithField("url", u).WithField("client", req.RemoteAddr).Debug("redirecting client to https")
			http.Redirect(w, req, u, http.StatusFound)
			return
		}
		h.ServeHTTP(w, req)
	})
}
