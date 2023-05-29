package apiserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

func newAPIServerProxy(host, tlsDir string, enabled *bool) *apiServerProxy {
	return &apiServerProxy{
		targetURL: &url.URL{
			Scheme: "https",
			Host:   host,
		},
		tlsDir:  tlsDir,
		enabled: enabled,
	}
}

type apiServerProxy struct {
	targetURL *url.URL
	tlsDir    string
	enabled   *bool
}

func (s *apiServerProxy) DelegationTarget() genericapiserver.DelegationTarget {
	t := &apiServerDelegationTarget{}
	t.DelegationTarget = genericapiserver.NewEmptyDelegateWithCustomHandler(t)
	t.config = s
	return t
}

func (s *apiServerProxy) APIGroupListCompletionFilter(delegate http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if path.Clean(req.URL.Path) == "/apis" && *s.enabled {
			backendAGL := metav1.APIGroupList{}
			err := s.request(req.Context(), req.RequestURI, &backendAGL)
			if err != nil {
				logrus.WithError(err).Warn("failed to complete APIGroupList")
				delegate.ServeHTTP(w, req)
				return
			}
			resp := responseRecorder{
				header: w.Header(),
			}
			delegate.ServeHTTP(&resp, req)
			w.WriteHeader(resp.status)
			if resp.status != http.StatusOK {
				w.Write(resp.body.Bytes())
				return
			}
			agl := &metav1.APIGroupList{}
			err = json.Unmarshal(resp.body.Bytes(), agl)
			if err != nil {
				logrus.WithError(err).Warn("failed to unmarshal APIGroupList from delegate response")
				w.Write(resp.body.Bytes())
				return
			}
			agl.Groups = append(agl.Groups, backendAGL.Groups...)
			b, err := json.MarshalIndent(agl, "", "  ")
			if err != nil {
				logrus.WithError(err).Warn("failed to marshal merged APIGroupList")
				w.Write(resp.body.Bytes())
				return
			}
			w.Write(b)
			return
		}
		delegate.ServeHTTP(w, req)
	})
}

func (s *apiServerProxy) request(ctx context.Context, path string, responseBody interface{}) error {
	tls, err := s.tlsTransport()
	if err != nil {
		return err
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s%s", s.targetURL.Host, path), nil)
	client := &http.Client{}
	client.Transport = tls
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, responseBody)
	if err != nil {
		return err
	}
	return nil
}

func (s *apiServerProxy) tlsTransport() (*http.Transport, error) {
	clientCertFile := filepath.Join(s.tlsDir, "client-admin.crt")
	clientKeyFile := filepath.Join(s.tlsDir, "client-admin.key")
	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load apiserver client cert: %w", err)
	}
	caCert, err := ioutil.ReadFile(filepath.Join(s.tlsDir, "server-ca.crt"))
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
	}
	tlsConfig.BuildNameToCertificate()
	return &http.Transport{TLSClientConfig: tlsConfig}, nil
}

type apiServerDelegationTarget struct {
	genericapiserver.DelegationTarget
	config *apiServerProxy
}

func (s *apiServerDelegationTarget) ListedPaths() []string {
	paths, err := s.listedPaths()
	if err != nil {
		logrus.Warnf("failed to get target apiserver paths: %s", err)
		return []string{}
	}
	return paths
}

func (s *apiServerDelegationTarget) listedPaths() ([]string, error) {
	p := paths{}
	err := s.config.request(context.TODO(), "", &p)
	return p.Paths, err
}

type paths struct {
	Paths []string `json:"paths"`
}

func (s *apiServerDelegationTarget) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tls, err := s.config.tlsTransport()
	if err != nil {
		logrus.WithError(err).Warn("failed to load proxy target TLS config")
		if r.URL.Path == "/api" {
			writeEmptyAPIVersions(w, r)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"failed to load target apiserver's TLS config"}`))
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(s.config.targetURL)
	proxy.Transport = tls
	r.URL.Host = s.config.targetURL.Host
	r.URL.Scheme = s.config.targetURL.Scheme
	r.Host = s.config.targetURL.Host
	usr, found := genericapirequest.UserFrom(r.Context())
	if !found {
		usr = &user.DefaultInfo{
			Name: user.Anonymous,
		}
	}
	// Impersonate user if not in admin group, see https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation
	var isAdmin bool
	for _, g := range usr.GetGroups() {
		if g == adminGroup {
			isAdmin = true
			break
		}
	}
	if !isAdmin {
		for _, g := range usr.GetGroups() {
			r.Header.Add("Impersonate-Group", g)
		}
		r.Header.Set("Impersonate-User", usr.GetName())
		r.Header.Set("Impersonate-Uid", usr.GetUID())
	}

	if !*s.config.enabled {
		if r.URL.Path == "/api" {
			writeEmptyAPIVersions(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"message":"server is disabled on this device"}`))
		return
	}

	proxy.ServeHTTP(w, r)
}

func writeEmptyAPIVersions(w http.ResponseWriter, r *http.Request) {
	// TODO: fallback to empty response on non-200/401/402 request to make this more resilient.
	//       Currently when proxying is enabled and k3s is not available the kubemate controller cannot be restarted.
	// Return empty result when k3s is unavailable.
	// This is because kubectl and controller-runtime fail otherwise while trying to discover resource groups using this path.
	logrus.Debug("Falling back to returning empty /api result")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"kind":"APIVersions"}`))
}

type responseRecorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *responseRecorder) Header() http.Header {
	return w.header
}

func (w *responseRecorder) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *responseRecorder) WriteHeader(statusCode int) {
	w.status = statusCode
}
