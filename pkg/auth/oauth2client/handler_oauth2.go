package oauth2client

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// HomeHandler renders the welcome page.
func HomeHandler(c oauth2.Config) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		/*if req.URL.Path != "/" {
			// The "/" pattern matches everything, so we need to check that
			// we're at the root here.
			return
		}*/

		// rotate PKCE secrets
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
				document.cookie = '`+cookiePKCE+`=true';
			}
			
			(function(){
				// clear existing isPKCE cookie if returning to the home page.
				document.cookie = '`+cookiePKCE+`=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
			})();
		</script>`,
			c.AuthCodeURL("some-random-state-foobar")+"&nonce=some-random-nonce",
			c.AuthCodeURL("some-random-state-foobar")+"&nonce=some-random-nonce&code_challenge="+pkceCodeChallenge+"&code_challenge_method=S256",
			"http://localhost:3846/oauth2/auth?client_id=my-client&redirect_uri=http%3A%2F%2Flocalhost%3A3846%2Fcallback&response_type=token%20id_token&scope=fosite%20openid&state=some-random-state-foobar&nonce=some-random-nonce",
			c.AuthCodeURL("some-random-state-foobar")+"&nonce=some-random-nonce",
			"/oauth2/auth?client_id=my-client&scope=fosite&response_type=123&redirect_uri=https://localhost/callback",
		)))
	}
}

// ClientEndpoint implements the endpoint to handle a client credentials flow.
func ClientEndpoint(c clientcredentials.Config) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("<h1>Client Credentials Grant</h1>"))
		token, err := c.Token(context.Background())
		if err != nil {
			rw.Write([]byte(fmt.Sprintf(`<p>I tried to get a token but received an error: %s</p>`, err.Error())))
			return
		}
		rw.Write([]byte(fmt.Sprintf(`<p>Awesome, you just received an access token!<br><br>%s<br><br><strong>more info:</strong><br><br>%s</p>`, token.AccessToken, token)))
		rw.Write([]byte(`<p><a href="/">Go back</a></p>`))
	}
}

// OwnerHandler implements the endpoint to handle a resource owner password credentials flow.
func OwnerHandler(c oauth2.Config) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("<h1>Resource Owner Password Credentials Grant</h1>"))
		req.ParseForm()
		if req.Form.Get("username") == "" || req.Form.Get("password") == "" {
			rw.Write([]byte(`<form method="post">
			<ul>
				<li>
					<input type="text" name="username" placeholder="username"/> <small>try "peter"</small>
				</li>
				<li>
					<input type="password" name="password" placeholder="password"/> <small>try "secret"</small><br>
				</li>
				<li>
					<input type="submit" />
				</li>
			</ul>
		</form>`))
			rw.Write([]byte(`<p><a href="/">Go back</a></p>`))
			return
		}

		token, err := c.PasswordCredentialsToken(context.Background(), req.Form.Get("username"), req.Form.Get("password"))
		if err != nil {
			rw.Write([]byte(fmt.Sprintf(`<p>I tried to get a token but received an error: %s</p>`, err.Error())))
			rw.Write([]byte(`<p><a href="/">Go back</a></p>`))
			return
		}
		rw.Write([]byte(fmt.Sprintf(`<p>Awesome, you just received an access token!<br><br>%s<br><br><strong>more info:</strong><br><br>%s</p>`, token.AccessToken, token)))
		rw.Write([]byte(`<p><a href="/">Go back</a></p>`))
	}
}
