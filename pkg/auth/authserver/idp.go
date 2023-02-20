package authserver

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/storage"
	"github.com/ory/fosite/token/jwt"
	"github.com/sirupsen/logrus"
)

// See https://github.com/ory/fosite-example/tree/master/authorizationserver

func GenerateKey() *rsa.PrivateKey {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic("unable to create private key")
	}
	return privateKey
}

func GenerateSecret() []byte {
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		panic(err)
	}
	return secret
}

type identityProvider struct {
	oauth2 fosite.OAuth2Provider
}

func NewIdentityProvider() *identityProvider {
	var privateKey = GenerateKey()
	var secret = GenerateSecret()

	// check the api docs of fosite.Config for further configuration options
	var config = &fosite.Config{
		AccessTokenLifespan: time.Minute * 30,
		// This secret is being used to sign access and refresh tokens as well as
		// authorization codes. It must be exactly 32 bytes long.
		GlobalSecret: secret,
		// ...
	}
	var store = storage.NewExampleStore() // TODO: implement persistent store
	var oauth2 = compose.ComposeAllEnabled(config, store, privateKey)
	return &identityProvider{
		oauth2: oauth2,
	}
}

func (p *identityProvider) RegisterHTTPRoutes(router *http.ServeMux, logger *logrus.Entry) {
	router.Handle("/oauth2/auth", authorizer(p.oauth2))
	router.Handle("/oauth2/token", tokenHandler(p.oauth2))
	router.Handle("/oauth2/revoke", revokeHandler(p.oauth2))
	router.Handle("/oauth2/introspect", introspectHandler(p.oauth2, logger))
}

func authorizer(oauth2 fosite.OAuth2Provider) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		// Let's create an AuthorizeRequest object!
		// It will analyze the request and extract important information like scopes, response type and others.
		ar, err := oauth2.NewAuthorizeRequest(ctx, req)
		if err != nil {
			oauth2.WriteAuthorizeError(ctx, rw, ar, err)
			return
		}

		// Normally, this would be the place where you would check if the user is logged in and gives his consent.
		// We're simplifying things and just checking if the request includes a valid username and password
		if req.Form.Get("username") != "peter" {
			rw.Header().Set("Content-Type", "text/html;charset=UTF-8")
			rw.Write([]byte(`<h1>Login page</h1>`))
			rw.Write([]byte(`
			<p>Howdy! This is the log in page. For this example, it is enough to supply the username.</p>
			<form method="post">
				<input type="text" name="username" /> <small>try peter</small><br>
				<input type="submit">
			</form>
		`))
			return
		}

		// Now that the user is authorized, we set up a session. When validating / looking up tokens, we additionally get
		// the session. You can store anything you want in it.

		// The session will be persisted by the store and made available when e.g. validating tokens or handling token endpoint requests.
		// The default OAuth2 and OpenID Connect handlers require the session to implement a few methods. Apart from that, the
		// session struct can be anything you want it to be.
		mySessionData := &fosite.DefaultSession{
			Username: req.Form.Get("username"),
		}

		// It's also wise to check the requested scopes, e.g.:
		// if authorizeRequest.GetScopes().Has("admin") {
		//     http.Error(rw, "you're not allowed to do that", http.StatusForbidden)
		//     return
		// }

		// Now we need to get a response. This is the place where the AuthorizeEndpointHandlers kick in and start processing the request.
		// NewAuthorizeResponse is capable of running multiple response type handlers which in turn enables this library
		// to support open id connect.
		response, err := oauth2.NewAuthorizeResponse(ctx, ar, mySessionData)
		if err != nil {
			oauth2.WriteAuthorizeError(ctx, rw, ar, err)
			return
		}

		// Awesome, now we redirect back to the client redirect uri and pass along an authorize code
		oauth2.WriteAuthorizeResponse(ctx, rw, ar, response)
	})
}

// The token endpoint is usually at "https://mydomain.com/oauth2/token"
func tokenHandler(oauth2 fosite.OAuth2Provider) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		// Create an empty session object that will be passed to storage implementation to populate (unmarshal) the session into.
		// By passing an empty session object as a "prototype" to the store, the store can use the underlying type to unmarshal the value into it.
		// For an example of storage implementation that takes advantage of that, see SQL Store (fosite_store_sql.go) from ory/Hydra project.
		mySessionData := new(fosite.DefaultSession)

		// This will create an access request object and iterate through the registered TokenEndpointHandlers to validate the request.
		accessRequest, err := oauth2.NewAccessRequest(ctx, req, mySessionData)
		if err != nil {
			oauth2.WriteAccessError(ctx, rw, accessRequest, err)
			return
		}

		if mySessionData.Username == "super-admin-guy" {
			// do something...
		}

		// Next we create a response for the access request. Again, we iterate through the TokenEndpointHandlers
		// and aggregate the result in response.
		response, err := oauth2.NewAccessResponse(ctx, accessRequest)
		if err != nil {
			oauth2.WriteAccessError(ctx, rw, accessRequest, err)
			return
		}
		oauth2.WriteAccessResponse(ctx, rw, accessRequest, response)
		// The client has a valid access token now
	})
}

func revokeHandler(oauth2 fosite.OAuth2Provider) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		// This will accept the token revocation request and validate various parameters.
		err := oauth2.NewRevocationRequest(ctx, req)
		// All done, send the response.
		oauth2.WriteRevocationResponse(ctx, rw, err)
	})
}

func introspectHandler(oauth2 fosite.OAuth2Provider, logger *logrus.Entry) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		mySessionData := newSession("")
		ir, err := oauth2.NewIntrospectionRequest(ctx, req, mySessionData)
		if err != nil {
			logger.Error(err, "oauth2 introspection request failed")
			oauth2.WriteIntrospectionError(ctx, rw, err)
			return
		}
		oauth2.WriteIntrospectionResponse(ctx, rw, ir)
	})
}

// A session is passed from the `/auth` to the `/token` endpoint. You probably want to store data like: "Who made the request",
// "What organization does that person belong to" and so on.
// For our use case, the session will meet the requirements imposed by JWT access tokens, HMAC access tokens and OpenID Connect
// ID Tokens plus a custom field

// newSession is a helper function for creating a new session. This may look like a lot of code but since we are
// setting up multiple strategies it is a bit longer.
// Usually, you could do:
//
//	session = new(fosite.DefaultSession)
func newSession(user string) *openid.DefaultSession {
	return &openid.DefaultSession{
		Claims: &jwt.IDTokenClaims{
			Issuer:      "https://fosite.my-application.com",
			Subject:     user,
			Audience:    []string{"https://my-client.my-application.com"},
			ExpiresAt:   time.Now().Add(time.Hour * 6),
			IssuedAt:    time.Now(),
			RequestedAt: time.Now(),
			AuthTime:    time.Now(),
		},
		Headers: &jwt.Headers{
			Extra: make(map[string]interface{}),
		},
	}
}
