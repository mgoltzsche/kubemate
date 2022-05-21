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

	"github.com/sirupsen/logrus"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

func newReverseProxy(host string) genericapiserver.DelegationTarget {
	/*proxyHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		proxyRequest(targetURL, rw, req)
	})*/
	r := &apiServerProxy{
		targetURL: &url.URL{
			Scheme: "https",
			Host:   host,
		},
	}
	r.DelegationTarget = genericapiserver.NewEmptyDelegateWithCustomHandler(r)
	return r
}

type apiServerProxy struct {
	genericapiserver.DelegationTarget
	targetURL *url.URL
}

func (s *apiServerProxy) ListedPaths() []string {
	paths, err := s.listedPaths()
	if err != nil {
		logrus.Warnf("failed to get target apiserver paths: %s", err)
		return []string{}
	}
	logrus.Printf("## paths: %+v", paths)
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
	logrus.Println("## raw paths:", string(b))
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

func (s *apiServerProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	tls, err := s.tlsTransport()
	if err != nil {
		logrus.WithError(err).Warn("failed to load proxy target TLS config")
		rw.WriteHeader(http.StatusServiceUnavailable)
		_, _ = rw.Write([]byte(`{"message":"failed to load target apiserver's TLS config"}`))
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(s.targetURL)
	proxy.Transport = tls
	req.URL.Host = s.targetURL.Host
	req.URL.Scheme = s.targetURL.Scheme
	req.Host = s.targetURL.Host
	// TODO: impersonate user, see https://kubernetes.io/docs/reference/access-authn-authz/authentication/#user-impersonation
	proxy.ServeHTTP(rw, req)
}

func (s *apiServerProxy) tlsTransport() (*http.Transport, error) {
	clientCertFile := "/var/lib/rancher/k3s/server/tls/client-admin.crt"
	clientKeyFile := "/var/lib/rancher/k3s/server/tls/client-admin.key"
	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load apiserver client cert: %w", err)
	}
	caCert, err := ioutil.ReadFile("/var/lib/rancher/k3s/server/tls/server-ca.crt")
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
