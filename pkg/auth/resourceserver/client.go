package resourceserver

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

func HTTPClient(caFile string) (*http.Client, error) {
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	var tr *http.Transport
	if caFile != "" {
		certs, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read ca certificate: %w", err)
		}
		if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
			return nil, fmt.Errorf("append ca certificate to pool: %w", err)
		}
		tr = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		}
	}
	return &http.Client{Transport: tr}, nil
}

type oauth2Client struct {
	client       *http.Client
	clientID     string
	clientSecret string
}

func newOAuth2Client(c *http.Client, clientID, clientSecret string) *oauth2Client {
	return &oauth2Client{
		client:       c,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// Post sends a request to the given uri with a payload of url values.
func (c *oauth2Client) Post(uri string, payload url.Values) (res *http.Response, body string, err error) {
	req, err := http.NewRequest(http.MethodPost, uri, bytes.NewReader([]byte(payload.Encode())))
	if err != nil {
		return
	}

	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err = c.client.Do(req)
	if err != nil {
		return
	}

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	// reset body for re-reading
	res.Body = ioutil.NopCloser(bytes.NewReader(bodyBytes))

	return res, string(bodyBytes), err
}
