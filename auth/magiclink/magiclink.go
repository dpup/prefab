// Package magiclink provides passwordless authentication, allowing users to
// authenticate using a magic link that is sent to their email address.
//
// ### Configuration:
//
// |---------------------------|---------------------------|
// | Env                       | JSON                      |
// | --------------------------|---------------------------|
// | AUTH_MAGICLINK_SIGNINGKEY | auth.magiclink.signingkey |
// | AUTH_MAGICLINK_EXPIRATION | auth.magiclink.expiration |
// | --------------------------|---------------------------|
//
// ### Basic Flow
//
//  1. An initial request to the login endpoint is made with a user's email
//     address in the creds map.
//  2. A signed JWT is created and emailed to the user.
//  3. The user clicks the link, which makes a request back to the login endpoint
//     with the JWT in the URL
//  4. If the JWT is valid, a cookie is set with an identity token that can be
//     used to authenticate the user's identity.
//
// Variation:
//   - If the original login request has a `redirect_uri` parameter, then the
//     magic link is constructed using the redirect URI. Once the user clicks
//     through to the destination, the token can be exchanged for an identity
//     token by using the login endpoint with an `issue_token` param.
//
// TODO: validate redirect URI matches a configured set of prefixes.
// TODO: Provide a way to prevent replay of magic links.
// TODO: Provide a way to rate-limit and/or block login requests.
package magiclink

import (
	"context"
	"strings"
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
		address:         viper.GetString("address"),
		signingKey:      []byte(viper.GetString("auth.magiclink.signingkey")),
		tokenExpiration: viper.GetDuration("auth.magiclink.expiration"),
	}
}

// Plugin for handling passwordless authentication via email.
type MagicLinkPlugin struct {
	emailer  *email.EmailPlugin
	renderer *templates.TemplatePlugin

	address         string
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
	if p.address == "" {
		return status.Error(codes.InvalidArgument, "magiclink: config missing address")
	}
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
	if req.Creds["email"] != "" {
		return p.handleEmail(ctx, req.Creds["email"], req.RedirectUri)
	}
	if req.Creds["token"] != "" {
		return p.handleToken(ctx, req.Creds["token"], req.IssueToken, req.RedirectUri)
	}
	return nil, status.Error(codes.InvalidArgument, "missing credentials, magiclink login requires an `email` or `token`")
}

func (p *MagicLinkPlugin) handleEmail(ctx context.Context, email string, redirectUri string) (*auth.LoginResponse, error) {
	token, err := p.generateToken(email)
	if err != nil {
		return nil, err
	}

	url := p.address + "/v1/auth/login?provider=magiclink&creds[token]=" + token
	if redirectUri != "" {
		if strings.Contains(redirectUri, "?") {
			url = redirectUri + "&token=" + token
		} else {
			url = redirectUri + "?token=" + token
		}
	}

	subject, err := p.renderer.Render(ctx, "auth_magiclink_subject", nil)
	if err != nil {
		return nil, err
	}
	body, err := p.renderer.Render(ctx, "auth_magiclink", map[string]interface{}{
		"MagicLink":  url,
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

func (p *MagicLinkPlugin) handleToken(ctx context.Context, token string, issueToken bool, redirectUri string) (*auth.LoginResponse, error) {
	identity, err := p.parseToken(token)
	if err != nil {
		return nil, err
	}

	idt, err := auth.IdentityToken(ctx, identity)
	if err != nil {
		return nil, err
	}

	if issueToken {
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
		RedirectUri: redirectUri,
	}, nil
}

func (p *MagicLinkPlugin) generateToken(email string) (string, error) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Audience:  jwt.ClaimStrings{jwtAudience},
			Issuer:    jwtIssuer,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(p.tokenExpiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email: email,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.signingKey)
}

func (p *MagicLinkPlugin) parseToken(tokenString string) (auth.Identity, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return p.signingKey, nil
		},
		jwt.WithIssuer(jwtIssuer),
		jwt.WithAudience(jwtAudience),
		jwt.WithLeeway(5*time.Second),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return auth.Identity{}, err
	}
	if !token.Valid || token.Claims == nil {
		return auth.Identity{}, status.Error(codes.InvalidArgument, "invalid token")
	}

	claims := token.Claims.(*Claims)
	return auth.Identity{
		AuthTime:      time.Now(),
		Subject:       claims.Email,
		Email:         claims.Email,
		EmailVerified: true,
	}, nil
}

type Claims struct {
	jwt.RegisteredClaims
	Email       string `json:"email"`
	IssueToken  bool   `json:"it"`
	RedirectUri string `json:"ru"`
}
