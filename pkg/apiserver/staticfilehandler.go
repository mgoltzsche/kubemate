package apiserver

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	filePathRegex = regexp.MustCompile("/[^/]+\\.[^\\./]+$")
	rootURL, _    = url.Parse("/")
)

type webUIHandler struct {
	dir         string
	delegate    http.Handler
	fileHandler http.Handler
	apiPaths    map[string]struct{}
}

func NewWebUIHandler(dir string, delegate http.Handler, apiPaths []string) http.Handler {
	m := make(map[string]struct{}, len(apiPaths))
	for _, p := range apiPaths {
		m[fmt.Sprintf("%s/", path.Clean(p))] = struct{}{}
	}
	fileHandler := http.FileServer(http.Dir(dir))
	return &webUIHandler{dir: dir, delegate: delegate, fileHandler: fileHandler, apiPaths: m}
}

func (h *webUIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serveStaticFile := r.URL.Path == "/" && strings.Contains(r.Header.Get("Accept"), "text/html")
	if r.URL.Path != "/" && !serveStaticFile {
		pathSegments := strings.Split(r.URL.Path, "/")
		if len(pathSegments) > 1 {
			_, isAPIPath := h.apiPaths[fmt.Sprintf("/%s/", pathSegments[1])]
			serveStaticFile = !isAPIPath
		}
	}
	if serveStaticFile {
		if !filePathRegex.Match([]byte(r.URL.Path)) {
			r.URL = rootURL
		}
		h.fileHandler.ServeHTTP(w, r)
		return
	}
	h.delegate.ServeHTTP(w, r)
}
