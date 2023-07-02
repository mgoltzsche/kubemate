package resourceserver

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type session struct {
	User string
}

type Config struct {
	oauth2.Config
	IDPURL       string
	CallbackPath string
	Client       *http.Client
}

func ProtectedEndpoint(h http.Handler, c Config) (http.Handler, error) {
	home := homeHandler(c.Config)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		token := req.URL.Query().Get("token")
		if token == "" {
			if cookie, err := req.Cookie("TOKEN"); err == nil {
				token = cookie.Value
			}
		}

		conf := clientcredentials.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			Scopes:       c.Scopes,
			TokenURL:     c.Endpoint.TokenURL,
		}
		ctx := req.Context()
		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.Client)
		resp, err := conf.Client(ctx).PostForm(strings.Replace(c.Endpoint.TokenURL, "token", "introspect", -1), url.Values{"token": []string{token}, "scope": []string{req.URL.Query().Get("scope")}})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "<h1>An error occurred!</h1><p>Could not perform introspection request: %v</p>", err)
			return
		}
		defer resp.Body.Close()

		var introspection = struct {
			Active bool `json:"active"`
		}{}
		out, _ := ioutil.ReadAll(resp.Body)
		if err := json.Unmarshal(out, &introspection); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "<h1>An error occurred!</h1>%s\n%s", err.Error(), out)
			return
		}

		if !introspection.Active {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintln(w, `<h1>Request could not be authorized.</h1>`)

			home(w, req.WithContext(ctx))
			return
		}

		h.ServeHTTP(w, req)
	}), nil
}

// homeHandler renders the welcome page.
func homeHandler(c oauth2.Config) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		// rotate PKCE secrets
		// TODO: make it work with multiple concurrent clients
		pkceCodeVerifier = generateCodeVerifier(64)
		pkceCodeChallenge = generateCodeChallenge(pkceCodeVerifier)

		rw.Write([]byte(fmt.Sprintf(`
		<p>You can obtain an access token using various methods</p>
		<ul>
			<li>
				<a href="%s">Authorize code grant (with OpenID Connect)</a>
			</li>
			<li>
				<a href="%s" onclick="setPKCE()">Authorize code grant (with OpenID Connect) with PKCE</a>
			</li>
			<li>
				<a href="%s">Implicit grant (with OpenID Connect)</a>
			</li>
			<li>
				Client credentials grant <a href="/client">using primary secret</a> or <a href="/client-new">using rotateted secret</a>
			</li>
			<li>
				<a href="/owner">Resource owner password credentials grant</a>
			</li>
			<li>
				<a href="%s">Refresh grant</a>. <small>You will first see the login screen which is required to obtain a valid refresh token.</small>
			</li>
			<li>
				<a href="%s">Make an invalid request</a>
			</li>
		</ul>
		<script type="text/javascript">
			function setPKCE() {
				// push in a cookie that the user-agent can check to see if last request was a PKCE request.
				document.cookie = '`+cookiePKCE+`=true; path=/';
			}
			
			(function(){
				// clear existing isPKCE cookie if returning to the home page.
				document.cookie = '`+cookiePKCE+`=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
			})();
		</script>`,
			c.AuthCodeURL("some-random-state-foobar")+"&nonce=some-random-nonce",
			c.AuthCodeURL("some-random-state-foobar")+"&nonce=some-random-nonce&code_challenge="+pkceCodeChallenge+"&code_challenge_method=S256", // &app_redirect_uri="+req.RequestURI,
			"http://localhost:3846/oauth2/auth?client_id=my-client&redirect_uri=http%3A%2F%2Flocalhost%3A3846%2Fcallback&response_type=token%20id_token&scope=fosite%20openid&state=some-random-state-foobar&nonce=some-random-nonce",
			c.AuthCodeURL("some-random-state-foobar")+"&nonce=some-random-nonce",
			"/oauth2/auth?client_id=my-client&scope=fosite&response_type=123&redirect_uri=https://localhost/callback",
		)))
	}
}
