package auth

import (
	"time"

	"github.com/dpup/prefab"
)

// Constant name for identifying the core auth plugin
const PluginName = "auth"

// AuthOptions allow configuration of the AuthPlugin.
type AuthOption func(*AuthPlugin)

// WithSigningKey sets the signing key to use when signing JWT tokens.
func WithSigningKey(signingKey string) AuthOption {
	return func(p *AuthPlugin) {
		p.jwtSigningKey = signingKey
	}
}

// WithExpiration sets the expiration to use when signing JWT tokens.
func WithExpiration(expiration time.Duration) AuthOption {
	return func(p *AuthPlugin) {
		p.jwtExpiration = expiration
	}
}

// WithBlockist configures the blocklist to use for token revocation. Tokens
// can be revoked by application code and will be revoked during Logout. The
// blocklist is checked during token validation.
func WithBlocklist(bl Blocklist) AuthOption {
	return func(p *AuthPlugin) {
		p.blocklist = bl
		p.authService.blocklist = bl
	}
}

// Plugin returns a new AuthPlugin.
func Plugin(opts ...AuthOption) *AuthPlugin {
	ap := &AuthPlugin{
		authService:   &impl{},
		jwtSigningKey: prefab.Config.String("auth.signingKey"),
		jwtExpiration: prefab.Config.MustDuration("auth.expiration"),
	}
	for _, opt := range opts {
		opt(ap)
	}
	return ap
}

// AuthPlugin exposes plugin interfaces that register and manage the AuthService
// and related functionality.
type AuthPlugin struct {
	authService *impl

	jwtSigningKey string
	jwtExpiration time.Duration
	blocklist     Blocklist
}

// From plugin.Plugin
func (ap *AuthPlugin) Name() string {
	return PluginName
}

// From prefab.OptionProvider
func (ap *AuthPlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithGRPCService(&AuthService_ServiceDesc, ap.authService),
		prefab.WithGRPCGateway(RegisterAuthServiceHandlerFromEndpoint),
		prefab.WithRequestConfig(injectSigningKey(ap.jwtSigningKey)),
		prefab.WithRequestConfig(injectExpiration(ap.jwtExpiration)),
		prefab.WithRequestConfig(injectBlocklist(ap.blocklist)),
	}
}

// AddLoginHandler can be called by other plugins to register login handlers.
func (ap *AuthPlugin) AddLoginHandler(provider string, h LoginHandler) {
	ap.authService.AddLoginHandler(provider, h)
}
