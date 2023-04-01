package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

func ForceHTTPSHost(host string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Host != host && !strings.HasPrefix(req.RemoteAddr, "127.0.0.1") {
			u := fmt.Sprintf("https://%s/", host)
			logrus.WithField("host", req.Host).WithField("from", req.URL.String()).WithField("to", u).WithField("client", req.RemoteAddr).Debug("redirecting request")
			http.Redirect(w, req, u, http.StatusFound)
			return
		}
		h.ServeHTTP(w, req)
	})
}
