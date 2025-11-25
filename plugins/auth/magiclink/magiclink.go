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
//     through to the destination, the application can use the token to exchange
//     for an identity token by using the login endpoint with an `issue_token`
//     param.
//
// TODO: Provide a way to prevent replay of magic links.
// TODO: Provide a way to rate-limit and/or block login requests.
package magiclink

import (
	"context"
	"strings"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/email"
	"github.com/dpup/prefab/plugins/eventbus"
	"github.com/dpup/prefab/plugins/templates"
	"github.com/dpup/prefab/serverutil"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"gopkg.in/gomail.v2"
)

const (
	// Constant name for the Magic Link auth plugin.
	PluginName = "auth_magiclink"

	// Constant name used as the auth provider in API requests.
	ProviderName = "magiclink"

	jwtIssuer   = "prefab"
	jwtAudience = "magiclink"
	jwtLeeway   = 5 * time.Second
)

func init() {
	// Register magiclink plugin config keys
	prefab.RegisterConfigKeys(
		prefab.ConfigKeyInfo{
			Key:         "auth.magiclink.signingKey",
			Description: "JWT signing key for magic link tokens",
			Type:        "string",
		},
		prefab.ConfigKeyInfo{
			Key:         "auth.magiclink.expiration",
			Description: "How long magic link tokens should be valid for",
			Type:        "duration",
		},
	)
}

// MagicLinkOptions allow configuration of the MagicLinkPlugin.
type MagicLinkOption func(*MagicLinkPlugin)

// WithSigningKey sets the signing key to use when signing magic link tokens.
func WithSigningKey(signingKey []byte) MagicLinkOption {
	return func(p *MagicLinkPlugin) {
		p.signingKey = signingKey
	}
}

// WithExpiration sets the expiration to use when signing magic link tokens.
func WithExpiration(expiration time.Duration) MagicLinkOption {
	return func(p *MagicLinkPlugin) {
		p.tokenExpiration = expiration
	}
}

// Plugin for handling passwordless authentication via email.
func Plugin(opts ...MagicLinkOption) *MagicLinkPlugin {
	p := &MagicLinkPlugin{
		signingKey:      prefab.Config.Bytes("auth.magiclink.signingKey"),
		tokenExpiration: prefab.Config.Duration("auth.magiclink.expiration"),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Plugin for handling passwordless authentication via email.
type MagicLinkPlugin struct {
	emailer  *email.EmailPlugin
	renderer *templates.TemplatePlugin

	signingKey      []byte
	tokenExpiration time.Duration
}

// From prefab.Plugin.
func (p *MagicLinkPlugin) Name() string {
	return PluginName
}

// From prefab.DependentPlugin.
func (p *MagicLinkPlugin) Deps() []string {
	return []string{auth.PluginName, email.PluginName, templates.PluginName}
}

// From prefab.InitializablePlugin.
func (p *MagicLinkPlugin) Init(ctx context.Context, r *prefab.Registry) error {
	if len(p.signingKey) == 0 {
		return errors.New("magiclink: config missing signing key")
	}
	if p.tokenExpiration == 0 {
		return errors.New("magiclink: config missing token expiration")
	}

	p.emailer = r.Get(email.PluginName).(*email.EmailPlugin)
	p.renderer = r.Get(templates.PluginName).(*templates.TemplatePlugin)

	ap := r.Get(auth.PluginName).(*auth.AuthPlugin)
	ap.AddLoginHandler(ProviderName, p.handleLogin)
	return nil
}

func (p *MagicLinkPlugin) handleLogin(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Provider != ProviderName {
		return nil, errors.NewC("magiclink login handler called for wrong provider", codes.InvalidArgument)
	}
	if req.Creds["email"] != "" {
		return p.handleEmail(ctx, req.Creds["email"], req.RedirectUri)
	}
	if req.Creds["token"] != "" {
		return p.handleToken(ctx, req.Creds["token"], req.IssueToken, req.RedirectUri)
	}
	return nil, errors.NewC("missing credentials, magiclink login requires an `email` or `token`", codes.InvalidArgument)
}

func (p *MagicLinkPlugin) handleEmail(ctx context.Context, email string, redirectUri string) (*auth.LoginResponse, error) {
	token, err := p.generateToken(email)
	if err != nil {
		return nil, err
	}

	var url string
	switch {
	case strings.Contains(redirectUri, "?"):
		url = redirectUri + "&token=" + token
	case redirectUri != "":
		url = redirectUri + "?token=" + token
	default:
		address := serverutil.AddressFromContext(ctx)
		url = address + "/api/auth/login?provider=magiclink&creds[token]=" + token
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
		return nil, errors.Codef(codes.Internal, "magiclink: email sending failed: %v", err)
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

	if bus := eventbus.FromContext(ctx); bus != nil {
		bus.Publish(auth.LoginEvent, auth.NewAuthEvent(identity))
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
		jwt.WithLeeway(jwtLeeway),
		jwt.WithIssuedAt(),
	)
	if err != nil {
		return auth.Identity{}, errors.Wrap(err, 0).WithCode(codes.InvalidArgument)
	}
	if !token.Valid || token.Claims == nil {
		return auth.Identity{}, errors.NewC("invalid token", codes.InvalidArgument)
	}

	claims := token.Claims.(*Claims)
	return auth.Identity{
		Provider:      ProviderName,
		SessionID:     claims.ID,
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
