package auth

import (
	"context"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/storage"
)

// Constant name for identifying the core auth plugin.
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

// WithBlockist configures a custom blocklist to use for token revocation.
// Tokens can be revoked by application code and will be revoked during Logout.
// The blocklist is checked during token validation.
func WithBlocklist(bl Blocklist) AuthOption {
	return func(p *AuthPlugin) {
		p.blocklist = bl
	}
}

// Plugin returns a new AuthPlugin.
func Plugin(opts ...AuthOption) *AuthPlugin {
	ap := &AuthPlugin{
		authService:   &impl{},
		jwtSigningKey: prefab.Config.String("auth.signingKey"),
		jwtExpiration: prefab.Config.MustDuration("auth.expiration"),
		identityExtractors: []IdentityExtractor{
			identityFromAuthHeader,
			identityFromCookie,
		},
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

	jwtSigningKey      string
	jwtExpiration      time.Duration
	blocklist          Blocklist
	identityExtractors []IdentityExtractor
}

// From prefab.Plugin.
func (ap *AuthPlugin) Name() string {
	return PluginName
}

// From prefab.OptionalDependentPlugin.
func (ap *AuthPlugin) OptDeps() []string {
	return []string{
		storage.PluginName,
	}
}

// From prefab.InitializablePlugin.
func (ap *AuthPlugin) Init(ctx context.Context, r *prefab.Registry) error {
	// If a blocklist hasn't been configured, and a storage plugin is registered,
	// then create a default blocklist for revoked tokens.
	if ap.blocklist == nil {
		store, ok := r.Get(storage.PluginName).(*storage.StoragePlugin)
		if store != nil && ok {
			logging.Info(ctx, "auth: initializing blocklist")
			if err := store.InitModel(&BlockedToken{}); err != nil {
				return errors.Errorf("auth: failed to initialize blocklist model: %w", err)
			}
			ap.blocklist = NewBlocklist(store)
		}
	}
	return nil
}

// From prefab.OptionProvider.
func (ap *AuthPlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithGRPCService(&AuthService_ServiceDesc, ap.authService),
		prefab.WithGRPCGateway(RegisterAuthServiceHandlerFromEndpoint),
		prefab.WithRequestConfig(injectSigningKey(ap.jwtSigningKey)),
		prefab.WithRequestConfig(injectExpiration(ap.jwtExpiration)),
		prefab.WithRequestConfig(ap.injectBlocklist),
		prefab.WithRequestConfig(ap.injectIdentityExtractors),
	}
}

// AddLoginHandler can be called by other plugins to register login handlers.
func (ap *AuthPlugin) AddLoginHandler(provider string, h LoginHandler) {
	ap.authService.AddLoginHandler(provider, h)
}

// AddIdentityExtractor can be called by other plugins to register identity
// extractors which will be used to authenticate requests.
//
// The AuthPlugin assumes that any identity returned by an extractor has been
// verified, and will not perform any additional verification. Extractors should
// return ErrNotFound if no identity is observed.
func (ap *AuthPlugin) AddIdentityExtractor(provider IdentityExtractor) {
	ap.identityExtractors = append(ap.identityExtractors, provider)
}

func (ap *AuthPlugin) injectBlocklist(ctx context.Context) context.Context {
	if ap.blocklist == nil {
		return ctx
	}
	return WithBlockist(ctx, ap.blocklist)
}

func (ap *AuthPlugin) injectIdentityExtractors(ctx context.Context) context.Context {
	return WithIdentityExtractors(ctx, ap.identityExtractors...)
}
