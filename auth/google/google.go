// Package Google provides authentication via Google SSO.
package google

import (
	"context"
	"net/url"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/plugin"
	"github.com/dpup/prefab/server/serverutil"

	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		clientID:     viper.GetString("auth.google.id"),
		clientSecret: viper.GetString("auth.google.secret"),
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

// From plugin.Plugin
func (p *GooglePlugin) Name() string {
	return PluginName
}

// From plugin.DependentPlugin
func (p *GooglePlugin) Deps() []string {
	return []string{auth.PluginName}
}

// From plugin.Plugin
func (p *GooglePlugin) Init(ctx context.Context, r *plugin.Registry) error {
	if p.clientID == "" {
		return status.Error(codes.InvalidArgument, "google: config missing client id")
	}
	if p.clientSecret == "" {
		return status.Error(codes.InvalidArgument, "google: config missing client secret")
	}

	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)
	ap.AddLoginHandler(ProviderName, p.handleLogin)
	return nil
}

func (p *GooglePlugin) handleLogin(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Provider != ProviderName {
		return nil, status.Error(codes.InvalidArgument, "google login handler called for wrong provider")
	}
	if len(req.Creds) == 0 {
		return p.redirectToGoogle(ctx)
	}
	if req.Creds["code"] != "" {
		return p.handleAuthorizationCode(ctx, req.Creds["code"])
	}
	if req.Creds["idtoken"] != "" {
		return p.handleIDToken(ctx, req.Creds["idtoken"])
	}
	return nil, status.Error(codes.InvalidArgument, "invalid credentials, google login requires a `code` or an `idtoken`")
}

// Trigger a redirect to google login. This will result in an authorization code
// being sent back to this endpoint, which will then set a cookie and redirect
// the user to the page specified in the original `redirectUri` query param.
//
// https://developers.google.com/identity/protocols/oauth2/web-server#httprest
func (p *GooglePlugin) redirectToGoogle(ctx context.Context) (*auth.LoginResponse, error) {

	// TODO: Forward incoming redirectUri to google inside state param. Possibly
	//   wrap state and then unwrap before returning to the real redirectUri. Or
	//   maybe the client doesn't need state at all for this code path since it's
	//   all server side.
	//
	// TODO: Create HTTP handler to handle the oauth callback and forward on to the
	// right login URL.

	address := serverutil.AddressFromContext(ctx)

	q := url.Values{}
	q.Add("client_id", p.clientID)
	q.Add("scope", "openid email profile")
	q.Add("response_type", "code")
	q.Add("access_type", "online") // Refresh token not needed as token is ephemeral.
	q.Add("prompt", "select_account")
	q.Add("redirect_uri", address+"/api/auth/google/callback")
	q.Add("state", "some state") // TODO: propagate state.

	u := url.URL{
		Scheme:   "https",
		Host:     "accounts.google.com",
		Path:     "/o/oauth2/v2/auth",
		RawQuery: q.Encode(),
	}

	return &auth.LoginResponse{
		Issued:      false,
		RedirectUri: u.String(),
	}, nil
}

// Handle an OAuth2 authorization code retrieved from Google.
//
// This endpoint can either be called by a client who received the `code`
// directly from Google, or it can be called via the HTTP callback handler
// following the auth flow being triggered by `redirectToGoogle`.
func (p *GooglePlugin) handleAuthorizationCode(ctx context.Context, code string) (*auth.LoginResponse, error) {
	// 1. Exchange code for token.
	// 2. Use token to fetch user's OAuth2 profile.
	// 3. Create an identity token.
	// 4. Return the identity token or set cookie.
	return nil, status.Error(codes.Unimplemented, "google login not implemented")
}

// Handle an ID Token retrieved via a clientside login. See:
// https://developers.google.com/identity/sign-in/web/backend-auth
func (p *GooglePlugin) handleIDToken(ctx context.Context, idToken string) (*auth.LoginResponse, error) {
	// 1. Verify the ID token.
	// 2. Create an identity token.
	// 3. Return the identity token or set cookie.
	return nil, status.Error(codes.Unimplemented, "google login not implemented")
}
