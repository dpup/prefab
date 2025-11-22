package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/storage"
	"google.golang.org/grpc/codes"
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

// WithDelegationEnabled enables or disables identity delegation (admin assume user).
func WithDelegationEnabled(enabled bool) AuthOption {
	return func(p *AuthPlugin) {
		p.delegationEnabled = enabled
	}
}

// WithDelegationRequireReason sets whether a reason is required for delegation.
// Defaults to true for security and audit purposes.
func WithDelegationRequireReason(required bool) AuthOption {
	return func(p *AuthPlugin) {
		p.requireReason = required
	}
}

// WithDelegationExpiration sets a custom expiration duration for delegated tokens.
// If not set, delegated tokens use the same expiration as regular tokens (auth.expiration).
// It's recommended to use shorter durations for delegated tokens (e.g., 1h) for security.
func WithDelegationExpiration(expiration time.Duration) AuthOption {
	return func(p *AuthPlugin) {
		p.delegationExpiration = expiration
	}
}

// WithIdentityValidator configures a custom validation function that checks if a
// target identity exists and is valid before allowing delegation. This allows
// applications to prevent delegation to non-existent or suspended users.
func WithIdentityValidator(validator IdentityValidator) AuthOption {
	return func(p *AuthPlugin) {
		p.identityValidator = validator
	}
}

// WithAdminChecker configures a custom function to check if an identity has
// admin privileges for delegation. This is used as a fallback when the authz
// plugin is not available. If neither authz plugin nor admin checker is
// configured, all delegation requests will fail.
func WithAdminChecker(checker AdminChecker) AuthOption {
	return func(p *AuthPlugin) {
		p.adminChecker = checker
	}
}

// Plugin returns a new AuthPlugin.
func Plugin(opts ...AuthOption) *AuthPlugin {
	// Get signing key from config, or generate a random one with a warning
	signingKey := prefab.ConfigString("auth.signingKey")
	if signingKey == "" {
		signingKey = randomSigningKey()
		log.Println("⚠️  WARNING: Using randomly generated JWT signing key. " +
			"Tokens will be invalidated on server restart. " +
			"Set PF__AUTH__SIGNING_KEY environment variable or auth.signingKey in prefab.yaml for production.")
	}

	ap := &AuthPlugin{
		authService:   &impl{},
		jwtSigningKey: signingKey,
		jwtExpiration: prefab.ConfigMustDuration("auth.expiration"),
		identityExtractors: []IdentityExtractor{
			identityFromAuthHeader,
			identityFromCookie,
		},
		delegationEnabled: prefab.ConfigBool("auth.delegation.enabled"),
		requireReason:     true, // Default to true, can be overridden via config or WithDelegationRequireReason
	}

	// Override with config if set
	if prefab.Config.Exists("auth.delegation.requireReason") {
		ap.requireReason = prefab.ConfigBool("auth.delegation.requireReason")
	}

	// Load delegation expiration from config if set
	if prefab.Config.Exists("auth.delegation.expiration") {
		ap.delegationExpiration = prefab.ConfigMustDuration("auth.delegation.expiration")
	}

	for _, opt := range opts {
		opt(ap)
	}
	return ap
}

func randomSigningKey() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("failed to generate random signing key: " + err.Error())
	}
	return hex.EncodeToString(key)
}

// AuthPlugin exposes plugin interfaces that register and manage the AuthService
// and related functionality.
type AuthPlugin struct {
	authService *impl

	jwtSigningKey      string
	jwtExpiration      time.Duration
	blocklist          Blocklist
	identityExtractors []IdentityExtractor

	// Delegation configuration
	delegationEnabled    bool
	delegationExpiration time.Duration
	requireReason        bool
	adminChecker         AdminChecker
	identityValidator    IdentityValidator
	authorizer           Authorizer // Interface to avoid import cycle
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
	ap.initBlocklist(ctx, r)
	ap.initDelegation(ctx, r)

	// Inject delegation config into authService
	ap.authService.delegationEnabled = ap.delegationEnabled
	ap.authService.delegationExpiration = ap.delegationExpiration
	ap.authService.requireReason = ap.requireReason
	ap.authService.adminChecker = ap.adminChecker
	ap.authService.identityValidator = ap.identityValidator

	return nil
}

func (ap *AuthPlugin) initBlocklist(ctx context.Context, r *prefab.Registry) {
	// If a blocklist hasn't been configured, and a storage plugin is registered,
	// then create a default blocklist for revoked tokens.
	if ap.blocklist == nil {
		store, ok := r.Get(storage.PluginName).(*storage.StoragePlugin)
		if store != nil && ok {
			logging.Info(ctx, "auth: initializing blocklist")
			if err := store.InitModel(&BlockedToken{}); err != nil {
				logging.Errorw(ctx, "auth: failed to initialize blocklist model", "error", err)
				return
			}
			ap.blocklist = NewBlocklist(store)
		}
	}
}

func (ap *AuthPlugin) initDelegation(ctx context.Context, r *prefab.Registry) {
	if !ap.delegationEnabled {
		return
	}

	// Use string key to avoid import cycle - authz plugin registers as "authz"
	if authzPlugin := r.Get("authz"); authzPlugin != nil {
		// Type assert to Authorizer interface
		if authorizer, ok := authzPlugin.(Authorizer); ok {
			ap.authorizer = authorizer
			logging.Info(ctx, "auth: authz plugin available for delegation authorization")
		} else {
			logging.Warn(ctx, "auth: authz plugin does not implement Authorizer interface")
		}
	}

	// If no custom adminChecker was provided but we have an authorizer,
	// create an adminChecker that wraps the authorizer
	if ap.adminChecker == nil && ap.authorizer != nil {
		ap.adminChecker = ap.createAuthorizerWrapper()
	}
}

func (ap *AuthPlugin) createAuthorizerWrapper() AdminChecker {
	return func(ctx context.Context, identity Identity) (bool, error) {
		params := AuthorizeParams{
			ObjectKey:     DelegationResource,
			ObjectID:      nil,
			Scope:         "",
			Action:        DelegationAction,
			DefaultEffect: 0, // Deny
			Info:          "AssumeIdentity",
		}
		err := ap.authorizer.Authorize(ctx, params)
		if err != nil {
			// Check if it's a permission denied error
			if errors.Code(err) == codes.PermissionDenied {
				return false, nil
			}
			// Other errors are actual errors
			return false, err
		}
		return true, nil
	}
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
