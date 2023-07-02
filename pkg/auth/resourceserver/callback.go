package resourceserver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

func CallbackHandler(c Config) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != c.CallbackPath {
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write([]byte("404 Not Found"))
			return
		}
		codeVerifier := resetPKCE(rw)
		if req.URL.Query().Get("error") != "" {
			printCallbackHeader(rw, http.StatusBadRequest)
			rw.Write([]byte(fmt.Sprintf(`<h1>Error!</h1>
			Error: %s<br>
			Error Hint: %s<br>
			Description: %s<br>
			<br>`,
				req.URL.Query().Get("error"),
				req.URL.Query().Get("error_hint"),
				req.URL.Query().Get("error_description"),
			)))
			return
		}

		client := newOAuth2Client(c.Client, c.ClientID, c.ClientSecret)
		if req.URL.Query().Get("revoke") != "" {
			revokeURL := strings.Replace(c.Endpoint.TokenURL, "token", "revoke", 1)
			payload := url.Values{
				"token_type_hint": {"refresh_token"},
				"token":           {req.URL.Query().Get("revoke")},
			}
			resp, body, err := client.Post(revokeURL, payload)
			if err != nil {
				printCallbackHeader(rw, http.StatusInternalServerError)
				rw.Write([]byte(fmt.Sprintf(`<p>Could not revoke token %s</p>`, err)))
				return
			}

			printCallbackHeader(rw, resp.StatusCode)
			rw.Write([]byte(fmt.Sprintf(`<p>Received status code from the revoke endpoint:<br><code>%d</code></p>`, resp.StatusCode)))
			if body != "" {
				rw.Write([]byte(fmt.Sprintf(`<p>Got a response from the revoke endpoint:<br><code>%s</code></p>`, body)))
			}

			rw.Write([]byte(fmt.Sprintf(`<p>These tokens have been revoked, try to use the refresh token by <br><a href="%s">by clicking here</a></p>`, "?refresh="+url.QueryEscape(req.URL.Query().Get("revoke")))))
			rw.Write([]byte(fmt.Sprintf(`<p>Try to use the access token by <br><a href="%s">by clicking here</a></p>`, "/protected?token="+url.QueryEscape(req.URL.Query().Get("access_token")))))

			return
		}

		if req.URL.Query().Get("refresh") != "" {
			payload := url.Values{
				"grant_type":    {"refresh_token"},
				"refresh_token": {req.URL.Query().Get("refresh")},
				"scope":         {"fosite"},
			}
			_, body, err := client.Post(c.Endpoint.TokenURL, payload)
			if err != nil {
				printCallbackHeader(rw, http.StatusInternalServerError)
				rw.Write([]byte(fmt.Sprintf(`<p>Could not refresh token %s</p>`, err)))
				return
			}
			printCallbackHeader(rw, http.StatusOK)
			rw.Write([]byte(fmt.Sprintf(`<p>Got a response from the refresh grant:<br><code>%s</code></p>`, body)))
			return
		}

		if req.URL.Query().Get("code") == "" {
			printCallbackHeader(rw, http.StatusOK)
			rw.Write([]byte(fmt.Sprintln(`<p>Could not find the authorize code. If you've used the implicit grant, check the
			browser location bar for the
			access token <small><a href="http://en.wikipedia.org/wiki/Fragment_identifier#Basics">(the server side does not have access to url fragments)</a></small>
			</p>`,
			)))
			return
		}

		// We'll check whether we sent a code+PKCE request, and if so, send the code_verifier along when requesting the access token.
		var opts []oauth2.AuthCodeOption
		if isPKCE(req) {
			opts = append(opts, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
		}

		ctx := req.Context()
		ctx = context.WithValue(ctx, oauth2.HTTPClient, c.Client)
		token, err := c.Exchange(ctx, req.URL.Query().Get("code"), opts...)
		if err != nil {
			printCallbackHeader(rw, http.StatusInternalServerError)
			rw.Write([]byte(fmt.Sprintf(`<p>Cannot exchange the authorize code for an access token: %s</p>`, err.Error())))
			return
		}

		http.SetCookie(rw, &http.Cookie{
			Name:     "TOKEN",
			Value:    token.AccessToken,
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Now().Add(20 * time.Minute),
		})

		printCallbackHeader(rw, http.StatusOK)
		/*redirectURI := req.URL.Query().Get("app_request_uri")
		if redirectURI == "" {
			printCallbackHeader(rw, http.StatusOK)
		} else {
			if redirectURI[0] != '/' {
				printCallbackHeader(rw, http.StatusBadRequest)
				rw.Write([]byte(`<p><i>app_redirect_uri</i> query parameter must start with a slash.</p>`))
				return
			}
			rw.Header().Set("Location", redirectURI)
			printCallbackHeader(rw, http.StatusTemporaryRedirect)
		}*/
		rw.Write([]byte(fmt.Sprintf(`<p>Cool! You are now a proud token owner.<br>
		<ul>
			<li>
				Access token (click to make <a href="%s">authorized call</a>):<br>
				<code>%s</code>
			</li>
			<li>
				Refresh token (click <a href="%s">here to use it</a>) (click <a href="%s">here to revoke it</a>):<br>
				<code>%s</code>
			</li>
			<li>
				Extra info: <br>
				<code>%s</code>
			</li>
		</ul>`,
			"/protected?token="+token.AccessToken,
			token.AccessToken,
			"?refresh="+url.QueryEscape(token.RefreshToken),
			"?revoke="+url.QueryEscape(token.RefreshToken)+"&access_token="+url.QueryEscape(token.AccessToken),
			token.RefreshToken,
			token,
		)))
	}
}

func printCallbackHeader(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(`<h1>OAuth2 Callback site</h1><a href="/">Go back</a>`))
}
