// Package Google provides authentication via Google SSO.
//
// Two methods of authentication are supported:
// - Client side via the Google SDK.
// - Server side via the OAuth2 flow.
//
// ## Client side authentication
//
// The client side flow uses the Google SDK to retrieve an identity token, which
// is then passed to the server for validation.
//
// 1. The user clicks on the "Sign in with Google" button.
// 2. The Google SDK opens a popup window.
// 3. The user logs in and grants access to the app.
// 4. The Google SDK returns an identity token to a javascript callback.
// 5. The client passes the identity token to the server's login endpoint.
// 6. The server validates the token and sets a cookie.
// 7. Subsequent API requests are authenticated via the cookie.
//
// A non-cookie option is to pass `issue_token` in the login endpoint, which
// will prompt the server to return an access token without setting cookies.
//
// ## Server side authentication
//
// https://developers.google.com/identity/protocols/oauth2/web-server#httprest
//
// To initiate a server side login, the client should make a POST request to the
// `/api/auth/login` endpoint with the following JSON body:
//
// ```json
//
//	{
//	  "provider": "google",
//	  "redirect_uri": "/dashboard"
//	}
//
// ```
//
// The server will respond with a JSON object containing a `redirect_uri` field,
// which the client should redirect the user to.
//
// A GET request may also be used, with the `provider` being passed as a
// query parameter. The response will be sent with a 301 status code, which can
// be used to redirect the user directly to Google if needed, short circuiting
// steps 1-3 below. When making a `fetch` request ensure that it is configured
// to not follow redirects.
//
// The full flow is as follows:
//
// 1. The client requests a login URL from the server.
// 2. The client redirects to the URL.
// 3. The user logs in and grants access to the app.
// 4. Google redirects the user back to the server with an authorization code.
// 5. The server exchanges the authorization code for an access token.
// 6. The server uses the access token to fetch the user's profile.
// 7. The server creates an identity token and sets a cookie.
// 8. The server redirects the user to the destination specified in the original request.
// 9. Subsequent API requests are authenticated via the cookie.
//
// ## Configuring Google OAuth App
//
// Follow the official steps here: https://support.google.com/cloud/answer/6158849
//
// For development the Authorized redirect URIs should be set to:
// http://localhost:8000/api/auth/google/callback
//
// In production switch out the protocol, host, and port with your domain.
package google

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/eventbus"
	"github.com/dpup/prefab/serverutil"
	"github.com/google/uuid"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
	"google.golang.org/grpc/codes"
)

const (
	// Constant name for the Google auth plugin.
	PluginName = "auth_google"

	// Constant name used as the auth provider in API requests.
	ProviderName = "google"
)

// GoogleOptions allow configuration of the GooglePlugin.
type GoogleOption func(*GooglePlugin)

// WithClient configures the GooglePlugin with the given client id and secret.
func WithClient(id, secret string) GoogleOption {
	return func(p *GooglePlugin) {
		p.clientID = id
		p.clientSecret = secret
	}
}

// Plugin for handling Google authentication.
func Plugin(opts ...GoogleOption) *GooglePlugin {
	p := &GooglePlugin{
		clientID:     prefab.Config.String("auth.google.id"),
		clientSecret: prefab.Config.String("auth.google.secret"),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// GooglePlugin for handling Google authentication.
type GooglePlugin struct {
	clientID     string
	clientSecret string
}

// From prefab.Plugin.
func (p *GooglePlugin) Name() string {
	return PluginName
}

// From prefab.DependentPlugin.
func (p *GooglePlugin) Deps() []string {
	return []string{auth.PluginName}
}

// From prefab.OptionProvider.
func (p *GooglePlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithHTTPHandlerFunc("/api/auth/google/callback", p.handleGoogleCallback),
		prefab.WithClientConfig("auth.google.clientId", p.clientID),
	}
}

// From prefab.Plugin.
func (p *GooglePlugin) Init(ctx context.Context, r *prefab.Registry) error {
	if p.clientID == "" {
		return errors.New("google: config missing client id")
	}
	if p.clientSecret == "" {
		return errors.New("google: config missing client secret")
	}

	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)
	ap.AddLoginHandler(ProviderName, p.handleLogin)

	return nil
}

func (p *GooglePlugin) handleLogin(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Provider != ProviderName {
		return nil, errors.NewC("google: login handler called for wrong provider", codes.InvalidArgument)
	}

	var userInfo *UserInfo
	var err error
	switch {
	case req.Creds["code"] != "":
		// Exchanges an authorization code for an access token, and sets up the
		// identity cookies.
		userInfo, err = p.handleAuthorizationCode(ctx, req.Creds["code"], req.Creds["state"])
	case req.Creds["idtoken"] != "":
		// Verifies the id token and uses the claims to set up the identity cookies.
		userInfo, err = p.handleIDToken(ctx, req.Creds["idtoken"])
	case len(req.Creds) == 0 || req.Creds["state"] != "":
		// Initiates a server side OAuth flow.
		return p.redirectToGoogle(ctx, req.RedirectUri, req.Creds["state"])
	default:
		return nil, errors.NewC("google: unexpected credentials, a `code` or an `idtoken` are required", codes.InvalidArgument)
	}

	if err != nil {
		return nil, err
	}
	return p.authenticateUserInfo(ctx, userInfo, req)
}

// Trigger a redirect to google login. This will result in an authorization code
// being sent back to the callback endpoint.
func (p *GooglePlugin) redirectToGoogle(ctx context.Context, dest string, state string) (*auth.LoginResponse, error) {
	wrappedState := p.newOauthState(dest, state)

	q := url.Values{}
	q.Add("client_id", p.clientID)
	q.Add("scope", "openid email profile")
	q.Add("response_type", "code")
	q.Add("access_type", "online") // Refresh token not needed as token is ephemeral.
	q.Add("prompt", "select_account")
	q.Add("redirect_uri", oauthCallback(ctx))
	q.Add("state", wrappedState.Encode())

	u := url.URL{
		Scheme:   "https",
		Host:     "accounts.google.com",
		Path:     "/o/oauth2/v2/auth",
		RawQuery: q.Encode(),
	}

	logging.Infof(ctx, "google: redirecting to: %s", u.String())

	return &auth.LoginResponse{
		Issued:      false,
		RedirectUri: u.String(),
	}, nil
}

// Since we can't control the structure of the callback, we use a standard HTTP
// handler to forward onto our standard GRPC-backed handler. This creates an
// extra hop, and could be handled internally, but for now this is simpler and
// keeps the code clean.
func (p *GooglePlugin) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.URL.Query().Get("code")
	rawState := r.URL.Query().Get("state")

	s, err := p.parseState(rawState)
	if err != nil {
		// TODO: Standardize pattern for HTTP handler error handler. Introduce a
		// handler interface which returns an error.
		logging.Errorf(ctx, "google: failed to parse state: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("google: invalid oauth state"))
		return
	}

	q := url.Values{}
	q.Add("provider", "google")
	q.Add("redirect_uri", s.RequestUri)
	q.Add("creds[code]", code)
	q.Add("creds[state]", rawState)

	u := url.URL{}
	u.Path = "/api/auth/login"
	u.RawQuery = q.Encode()

	logging.Info(ctx, "Google Login: forwarding callback to GRPC handler")
	w.Header().Add("location", u.String())
	w.WriteHeader(http.StatusFound)
}

// Handle an OAuth2 authorization code retrieved from Google.
//
// This endpoint can either be called by a client who received the `code`
// directly from Google, or it can be called via the HTTP callback handler
// following the auth flow being triggered by `redirectToGoogle`.
func (p *GooglePlugin) handleAuthorizationCode(ctx context.Context, code, rawState string) (*UserInfo, error) {
	_, err := p.parseState(rawState)
	if err != nil {
		return nil, errors.Codef(codes.InvalidArgument, "google: failed to parse state: %s", err)
	}

	var conf = &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  oauthCallback(ctx),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/userinfo.email",
		},
	}

	// Exchange authorization code for an access token.
	token, err := conf.Exchange(ctx, code)
	if err != nil {
		return nil, errors.Codef(codes.Internal, "google: token exchange failed: %s", err)
	}

	// Use the access token to fetch the user's profile.
	client := conf.Client(ctx, token)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, userInfoEndpoint, nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Codef(codes.Internal, "google: failed to fetch user profile: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Codef(codes.Internal, "google: failed to get user info, status: %d", resp.StatusCode)
	}
	return UserInfoFromJSON(resp.Body)
}

// Handle an ID Token retrieved via a clientside login. See:
// https://developers.google.com/identity/sign-in/web/backend-auth
func (p *GooglePlugin) handleIDToken(ctx context.Context, token string) (*UserInfo, error) {
	payload, err := idtoken.Validate(ctx, token, p.clientID)
	if err != nil {
		logging.Errorw(ctx, "google: failed to validate id token", "error", err)
		return nil, errors.Codef(codes.InvalidArgument, "google: failed to validate id token: %s", err)
	}
	return UserInfoFromClaims(payload.Claims)
}

// Maps the Google UserInfo to a prefab Identity. Is req.IssueToken is true,
// then the token is returned to the client. If not, then the token is set as a
// cookie.
func (p *GooglePlugin) authenticateUserInfo(ctx context.Context, userInfo *UserInfo, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	identity := auth.Identity{
		Provider:      ProviderName,
		SessionID:     uuid.NewString(),
		AuthTime:      time.Now(),
		Subject:       userInfo.ID,
		Name:          userInfo.Name,
		Email:         userInfo.Email,
		EmailVerified: userInfo.IsConfirmed(),
	}

	// Create an identity token based and return it to the client..
	idt, err := auth.IdentityToken(ctx, identity)
	if err != nil {
		return nil, err
	}

	logging.Infow(ctx, "google: user authenticated", "subject", identity.Subject, "email", identity.Email)

	if bus := eventbus.FromContext(ctx); bus != nil {
		bus.Publish(auth.LoginEvent, auth.AuthEvent{Identity: identity})
	}

	if req.IssueToken {
		return &auth.LoginResponse{
			Issued: true,
			Token:  idt,
		}, nil
	}

	if err := auth.SendIdentityCookie(ctx, idt); err != nil {
		return nil, err
	}

	return &auth.LoginResponse{
		Issued:      true,
		RedirectUri: req.RedirectUri,
	}, nil
}

func oauthCallback(ctx context.Context) string {
	return serverutil.AddressFromContext(ctx) + "/api/auth/google/callback"
}
