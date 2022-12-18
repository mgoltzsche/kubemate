package apiserver

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	filePathRegex = regexp.MustCompile("/[^/]+\\.[^\\./]+$")
	rootURL, _    = url.Parse("/")
)

type webUIHandler struct {
	dir             string
	apiPaths        map[string]struct{}
	apiHandler      http.Handler
	fileHandler     http.Handler
	fallbackHandler http.Handler
}

func NewWebUIHandler(dir string, apiPaths []string, apiHandler, fallbackHandler http.Handler) http.Handler {
	m := make(map[string]struct{}, len(apiPaths))
	for _, p := range apiPaths {
		m[fmt.Sprintf("%s/", path.Clean(p))] = struct{}{}
	}
	fileHandler := http.FileServer(http.Dir(dir))
	return &webUIHandler{
		dir:             dir,
		apiHandler:      apiHandler,
		fileHandler:     fileHandler,
		fallbackHandler: fallbackHandler,
		apiPaths:        m,
	}
}

func (h *webUIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logrus.Trace(fmt.Sprintf("request %s %s", r.Method, r.URL.String()))
	if r.URL.Path == "/api" {
		// TODO: forward to k3s when available! Or remove workaround entirely but it currently required by kubectl and controller-runtime.
		logrus.Debug("Falling back to returning empty /api result")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"kind":"APIVersions"}`))
		return
	}
	serveStaticFile := r.URL.Path == "/" && strings.Contains(r.Header.Get("Accept"), "text/html")
	if r.URL.Path != "/" && !serveStaticFile {
		pathSegments := strings.Split(r.URL.Path, "/")
		if len(pathSegments) > 1 {
			_, isAPIPath := h.apiPaths[fmt.Sprintf("/%s/", pathSegments[1])]
			serveStaticFile = !isAPIPath
		}
	}
	if serveStaticFile {
		if _, err := os.Stat(filepath.Join(h.dir, filepath.FromSlash(r.URL.Path))); os.IsNotExist(err) {
			h.fallbackHandler.ServeHTTP(w, r)
			return
		}
		if !filePathRegex.Match([]byte(r.URL.Path)) {
			r.URL = rootURL
		}
		h.fileHandler.ServeHTTP(w, r)
		return
	}
	h.apiHandler.ServeHTTP(w, r)
}
