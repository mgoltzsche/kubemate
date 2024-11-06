package apiserver

import (
	"net/http"
	"strings"
)

func rootPathHandler(uiPath string, fallbackHandler, apiHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			if strings.HasPrefix(req.Header.Get("Accept"), "application/json") {
				apiHandler.ServeHTTP(w, req)
				return
			}

			http.Redirect(w, req, "/ui/", http.StatusFound)
			return
		}

		fallbackHandler.ServeHTTP(w, req)
	})
}
