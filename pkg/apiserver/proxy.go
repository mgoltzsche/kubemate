package apiserver

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

func newReverseProxy(host, tlsDir string, enabled *bool) genericapiserver.DelegationTarget {
	r := &apiServerProxy{
		targetURL: &url.URL{
			Scheme: "https",
			Host:   host,
		},
		tlsDir:  tlsDir,
		enabled: enabled,
	}
	r.DelegationTarget = genericapiserver.NewEmptyDelegateWithCustomHandler(r)
	return r
}

type apiServerProxy struct {
	genericapiserver.DelegationTarget
	targetURL *url.URL
	tlsDir    string
	enabled   *bool
}

func (s *apiServerProxy) ListedPaths() []string {
	paths, err := s.listedPaths()
	if err != nil {
		logrus.Warnf("failed to get target apiserver paths: %s", err)
		return []string{}
	}
	return paths
}

func (s *apiServerProxy) listedPaths() ([]string, error) {
	tls, err := s.tlsTransport()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", fmt.Sprintf("https://%s", s.targetURL.Host), nil)
	client := &http.Client{}
	client.Transport = tls
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var p paths
	err = json.Unmarshal(b, &p)
	if err != nil {
		return nil, err
	}
	return p.Paths, nil
}

type paths struct {
	Paths []string `json:"paths"`
}

func (s *apiServerProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tls, err := s.tlsTransport()
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
	proxy := httputil.NewSingleHostReverseProxy(s.targetURL)
	proxy.Transport = tls
	r.URL.Host = s.targetURL.Host
	r.URL.Scheme = s.targetURL.Scheme
	r.Host = s.targetURL.Host
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

	if !*s.enabled {
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
