// Package magiclink provides passwordless authentication, allowing users to
// authenticate using a magic link that is sent to their email address.
//
// Configuration:
// |---------------------------|---------------------------|
// | Env                       | JSON                      |
// | --------------------------|---------------------------|
// | AUTH_MAGICLINK_SIGNINGKEY | auth.magiclink.signingkey |
// | AUTH_MAGICLINK_EXPIRATION | auth.magiclink.expiration |
// | --------------------------|---------------------------|
//
// TODO: Provide a way to prevent replay of magic links.
// TODO: Provide a way to rate-limit and/or block login requests.
package magiclink

import (
	"context"
	"time"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/email"
	"github.com/dpup/prefab/plugin"
	"github.com/dpup/prefab/templates"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/gomail.v2"
)

/*
Plan

- Login endpoint:
  - takes email
	- generates signed JWT with email and expiration
	- sends email with the magic-link
- Login endpoint:
	- takes token from magic-link
	- extracts email from token and creates auth token
	- sets cookie or returns an auth token
  - if cookie is set, needs a redirect
    - 302 with cookie should work, but won't on localhost
    - could use a meta-redirect if dest is localhost or different domain


Questions:
- should cookies only be set if there's a valid user?
- if so, what is the underlying source of user data?
- can it just be a pluggin?
- can this just say: this user has verified they have access to this email? And
  then ACL code should verify that the authenticated user is authorized to
  access the resource.
- can a browser have multiple authenticated identities
*/

const (
	// Constant name for the Magic Link auth plugin.
	PluginName = "auth_magiclink"

	// Constant name used as the auth provider in API requests.
	ProviderName = "magiclink"

	jwtIssuer   = "prefab"
	jwtAudience = "magiclink"
)

// Plugin for handling passwordless authentication via email.
func Plugin() *MagicLinkPlugin {
	return &MagicLinkPlugin{
		signingKey:      []byte(viper.GetString("auth.magiclink.signingkey")),
		tokenExpiration: viper.GetDuration("auth.magiclink.expiration"),
	}
}

// Plugin for handling passwordless authentication via email.
type MagicLinkPlugin struct {
	emailer         *email.EmailPlugin
	renderer        *templates.TemplatePlugin
	signingKey      []byte
	tokenExpiration time.Duration
}

// From plugin.Plugin
func (p *MagicLinkPlugin) Name() string {
	return PluginName
}

// From plugin.DependentPlugin
func (p *MagicLinkPlugin) Deps() []string {
	return []string{auth.PluginName, email.PluginName, templates.PluginName}
}

// From plugin.InitializablePlugin.
func (p *MagicLinkPlugin) Init(ctx context.Context, r *plugin.Registry) error {
	if len(p.signingKey) == 0 {
		return status.Error(codes.InvalidArgument, "magiclink: config missing signing key")
	}
	if p.tokenExpiration == 0 {
		return status.Error(codes.InvalidArgument, "magiclink: config missing token expiration")
	}

	p.emailer = r.Get(email.PluginName).(*email.EmailPlugin)
	p.renderer = r.Get(templates.PluginName).(*templates.TemplatePlugin)

	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)
	ap.AddLoginHandler(ProviderName, p.handleLogin)
	return nil
}

// LoginHandler processes magiclink login requests.
func (p *MagicLinkPlugin) handleLogin(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Provider != ProviderName {
		return nil, status.Error(codes.InvalidArgument, "magiclink login handler called for wrong provider")
	}
	if req.Creds["token"] != "" {
		return p.handleToken(ctx, req.Creds["token"])
	}
	if req.Creds["email"] != "" {
		return p.handleEmail(ctx, req.Creds["email"])
	}
	return nil, status.Error(codes.InvalidArgument, "missing credentials, magiclink login requires an `email` or `token`")
}

func (p *MagicLinkPlugin) handleEmail(ctx context.Context, email string) (*auth.LoginResponse, error) {
	token, err := p.generateToken(email)
	if err != nil {
		return nil, err
	}

	subject, err := p.renderer.Render(ctx, "auth_magiclink_subject", nil)
	if err != nil {
		return nil, err
	}
	body, err := p.renderer.Render(ctx, "auth_magiclink", map[string]interface{}{
		"Token":      token,
		"Expiration": p.tokenExpiration,
	})
	if err != nil {
		return nil, err
	}

	m := gomail.NewMessage()
	m.SetHeader("To", email)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)
	if err := p.emailer.Send(ctx, m); err != nil {
		return nil, err
	}

	return &auth.LoginResponse{
		Issued: false,
	}, nil
}

func (p *MagicLinkPlugin) handleToken(ctx context.Context, token string) (*auth.LoginResponse, error) {
	return nil, nil
}

func (p *MagicLinkPlugin) generateToken(email string) (string, error) {
	claims := &jwt.RegisteredClaims{
		ID:        uuid.NewString(),
		Audience:  jwt.ClaimStrings{jwtAudience},
		Issuer:    jwtIssuer,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(p.tokenExpiration)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.signingKey)
}
