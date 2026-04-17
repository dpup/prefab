package oauth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/storage"
	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/server"
	"google.golang.org/grpc/metadata"
)

// OAuthPlugin provides OAuth2 authorization server functionality using go-oauth2.
type OAuthPlugin struct {
	manager     *manage.Manager
	server      *server.Server
	clientStore *clientStoreAdapter
	tokenStore  *tokenStoreAdapter

	accessTokenExpiry  time.Duration
	refreshTokenExpiry time.Duration
	authCodeExpiry     time.Duration
	issuer             string
	enforcePKCE        *bool // nil means use config, non-nil means use this value

	staticClients   []Client
	userClientStore ClientStore
	userTokenStore  TokenStore
}

// Builder provides a fluent interface for configuring the OAuth plugin.
type Builder struct {
	plugin *OAuthPlugin
}

// NewBuilder creates a new OAuth plugin builder with sensible defaults.
func NewBuilder() *Builder {
	return &Builder{
		plugin: &OAuthPlugin{
			accessTokenExpiry:  time.Hour,
			refreshTokenExpiry: 14 * 24 * time.Hour, // 2 weeks
			authCodeExpiry:     10 * time.Minute,
			staticClients:      []Client{},
		},
	}
}

// WithClient adds a static OAuth client.
func (b *Builder) WithClient(client Client) *Builder {
	if client.CreatedAt.IsZero() {
		client.CreatedAt = time.Now()
	}
	b.plugin.staticClients = append(b.plugin.staticClients, client)
	return b
}

// WithAccessTokenExpiry sets the access token expiration duration.
func (b *Builder) WithAccessTokenExpiry(d time.Duration) *Builder {
	b.plugin.accessTokenExpiry = d
	return b
}

// WithRefreshTokenExpiry sets the refresh token expiration duration.
func (b *Builder) WithRefreshTokenExpiry(d time.Duration) *Builder {
	b.plugin.refreshTokenExpiry = d
	return b
}

// WithAuthCodeExpiry sets the authorization code expiration duration.
func (b *Builder) WithAuthCodeExpiry(d time.Duration) *Builder {
	b.plugin.authCodeExpiry = d
	return b
}

// WithIssuer sets the token issuer.
func (b *Builder) WithIssuer(issuer string) *Builder {
	b.plugin.issuer = issuer
	return b
}

// WithClientStore sets a custom client store for persistent/dynamic client management.
// Use this when you need to store clients in a database or allow users to create clients.
func (b *Builder) WithClientStore(store ClientStore) *Builder {
	b.plugin.userClientStore = store
	return b
}

// WithTokenStore sets a custom token store for persistent token storage.
// Use this when you need to persist tokens across server restarts or in a distributed environment.
func (b *Builder) WithTokenStore(store TokenStore) *Builder {
	b.plugin.userTokenStore = store
	return b
}

// WithEnforcePKCE sets whether PKCE is required for public clients.
// When true, public clients must provide a code_challenge in authorization requests.
// If not set, the value is read from config key "oauth.enforcePkce".
func (b *Builder) WithEnforcePKCE(enforce bool) *Builder {
	b.plugin.enforcePKCE = &enforce
	return b
}

// Build returns the configured OAuth plugin.
func (b *Builder) Build() *OAuthPlugin {
	p := b.plugin

	// Use user-provided client store or default in-memory store
	var clientStore ClientStore
	if p.userClientStore != nil {
		clientStore = p.userClientStore
	} else {
		clientStore = newMemoryClientStore()
	}

	// Use user-provided token store or default in-memory store
	var tokenStore TokenStore
	if p.userTokenStore != nil {
		tokenStore = p.userTokenStore
	} else {
		tokenStore = NewMemoryTokenStore()
	}

	// Create adapters that wrap the stores
	p.clientStore = newClientStoreAdapter(clientStore)
	p.tokenStore = newTokenStoreAdapter(tokenStore)

	// Register static clients (log errors but don't fail - allows re-registration on restart)
	for _, client := range p.staticClients {
		if err := clientStore.CreateClient(context.Background(), &client); err != nil {
			// Try update in case client already exists
			_ = clientStore.UpdateClient(context.Background(), &client)
		}
	}

	// Create manager with configuration
	p.manager = manage.NewDefaultManager()

	// Configure token expiration
	p.manager.SetAuthorizeCodeTokenCfg(&manage.Config{
		AccessTokenExp:    p.accessTokenExpiry,
		RefreshTokenExp:   p.refreshTokenExpiry,
		IsGenerateRefresh: true,
	})
	p.manager.SetRefreshTokenCfg(&manage.RefreshingConfig{
		AccessTokenExp:     p.accessTokenExpiry,
		RefreshTokenExp:    p.refreshTokenExpiry,
		IsGenerateRefresh:  true,
		IsRemoveAccess:     true,
		IsRemoveRefreshing: true,
	})
	p.manager.SetClientTokenCfg(&manage.Config{
		AccessTokenExp: p.accessTokenExpiry,
	})

	// Set authorization code expiration
	p.manager.SetAuthorizeCodeExp(p.authCodeExpiry)

	// Map storage
	p.manager.MapClientStorage(p.clientStore)
	p.manager.MapTokenStorage(p.tokenStore)

	// Set custom redirect URI validation to support multiple redirect URIs
	// baseURI contains all redirect URIs joined by newline (from GetDomain())
	p.manager.SetValidateURIHandler(func(baseURI, redirectURI string) error {
		allowedURIs := strings.Split(baseURI, "\n")
		for _, allowed := range allowedURIs {
			if allowed == redirectURI {
				return nil
			}
		}
		return ErrAccessDenied
	})

	// Create server with sensible defaults
	p.server = server.NewDefaultServer(p.manager)
	p.server.SetAllowGetAccessRequest(false)

	// Allow both form and basic auth for client credentials
	p.server.SetClientInfoHandler(func(r *http.Request) (string, string, error) {
		clientID, clientSecret, ok := r.BasicAuth()
		if ok {
			return clientID, clientSecret, nil
		}
		return r.FormValue("client_id"), r.FormValue("client_secret"), nil
	})

	// Configure allowed grant types and response types
	p.server.SetAllowedGrantType(oauth2.AuthorizationCode, oauth2.Refreshing, oauth2.ClientCredentials)
	p.server.SetAllowedResponseType(oauth2.Code)

	// Set scope validation handler for the authorization_code flow.
	p.server.SetAuthorizeScopeHandler(func(w http.ResponseWriter, r *http.Request) (string, error) {
		scope := r.FormValue("scope")
		clientID := r.FormValue("client_id")
		return p.validateScopes(r.Context(), clientID, scope)
	})

	// Enforce the client's configured scope allowlist on client_credentials and
	// password grants. Without this, go-oauth2 passes the requested scope
	// through unchecked, letting any client mint tokens with arbitrary scopes.
	p.server.SetClientScopeHandler(func(tgr *oauth2.TokenGenerateRequest) (bool, error) {
		if _, err := p.validateScopes(tgr.Request.Context(), tgr.ClientID, tgr.Scope); err != nil {
			return false, nil
		}
		return true, nil
	})

	// On refresh, the new scope must be a subset of the originally-granted
	// scope. This blocks scope escalation via the refresh_token grant.
	p.server.SetRefreshingScopeHandler(func(tgr *oauth2.TokenGenerateRequest, oldScope string) (bool, error) {
		if tgr.Scope == "" {
			return true, nil
		}
		return isScopeSubset(tgr.Scope, oldScope), nil
	})

	return p
}

// isScopeSubset returns true if every scope in requested is present in granted.
// Scopes are space-separated per RFC 6749 §3.3.
func isScopeSubset(requested, granted string) bool {
	allowed := make(map[string]bool)
	for _, s := range strings.Fields(granted) {
		allowed[s] = true
	}
	for _, s := range strings.Fields(requested) {
		if !allowed[s] {
			return false
		}
	}
	return true
}

// Name returns the plugin name.
func (p *OAuthPlugin) Name() string {
	return PluginName
}

// Deps returns the plugin dependencies.
func (p *OAuthPlugin) Deps() []string {
	return []string{auth.PluginName}
}

// OptDeps returns optional dependencies.
func (p *OAuthPlugin) OptDeps() []string {
	return []string{storage.PluginName}
}

// Init initializes the OAuth plugin.
func (p *OAuthPlugin) Init(ctx context.Context, r *prefab.Registry) error {
	// Get auth plugin to register identity extractor
	authPlugin, ok := r.Get(auth.PluginName).(*auth.AuthPlugin)
	if !ok {
		return errors.New("failed to get auth plugin")
	}

	// Register OAuth token identity extractor
	authPlugin.AddIdentityExtractor(p.extractIdentityFromOAuthToken)

	// Set issuer from config if not set
	if p.issuer == "" {
		p.issuer = prefab.Config.String("oauth.issuer")
		if p.issuer == "" {
			p.issuer = prefab.Config.String("server.address")
		}
	}

	// Configure user authorization handler
	p.server.SetUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (string, error) {
		ctx := r.Context()
		identity, err := auth.IdentityFromContext(ctx)
		if err != nil {
			// Redirect to login
			return "", err
		}
		return identity.Subject, nil
	})

	return nil
}

// shouldEnforcePKCE returns whether PKCE should be enforced for public clients.
func (p *OAuthPlugin) shouldEnforcePKCE() bool {
	if p.enforcePKCE != nil {
		return *p.enforcePKCE
	}
	return prefab.Config.Bool("oauth.enforcePkce")
}

// ServerOptions returns the server options for the OAuth plugin.
func (p *OAuthPlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithHTTPHandler("/oauth/authorize", p.authorizeHandler()),
		prefab.WithHTTPHandler("/oauth/token", p.tokenHandler()),
		prefab.WithHTTPHandler("/oauth/revoke", p.revokeHandler()),
		prefab.WithHTTPHandler("/oauth/introspect", p.introspectHandler()),
		prefab.WithHTTPHandler("/.well-known/oauth-authorization-server", p.metadataHandler()),
		prefab.WithRequestConfig(p.injectOAuthContext),
	}
}

// GetClientStore returns the client store for external management.
func (p *OAuthPlugin) GetClientStore() ClientStore {
	return p.clientStore.store
}

// GetTokenStore returns the token store for external management.
func (p *OAuthPlugin) GetTokenStore() TokenStore {
	return p.tokenStore.store
}

// AddClient adds a client dynamically at runtime.
func (p *OAuthPlugin) AddClient(client Client) {
	p.clientStore.store.CreateClient(context.Background(), &client)
}

// validateScopes validates that the requested scopes are allowed for the client.
// Returns the validated scope string or an error if any scope is not allowed.
func (p *OAuthPlugin) validateScopes(ctx context.Context, clientID, requestedScope string) (string, error) {
	if requestedScope == "" {
		return "", nil
	}

	client, err := p.clientStore.store.GetClient(ctx, clientID)
	if err != nil {
		return "", err
	}

	// If client has no scope restrictions, allow all requested scopes
	if len(client.Scopes) == 0 {
		return requestedScope, nil
	}

	// Build set of allowed scopes for O(1) lookup
	allowedScopes := make(map[string]bool)
	for _, s := range client.Scopes {
		allowedScopes[s] = true
	}

	// Validate each requested scope
	requested := strings.Fields(requestedScope)
	var validated []string
	for _, s := range requested {
		if !allowedScopes[s] {
			return "", ErrInvalidScope
		}
		validated = append(validated, s)
	}

	return strings.Join(validated, " "), nil
}

// injectOAuthContext injects OAuth-specific values into the request context.
func (p *OAuthPlugin) injectOAuthContext(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	tokenString := extractBearerToken(md)
	if tokenString == "" {
		return ctx
	}

	// Get token info from store
	ti, err := p.tokenStore.GetByAccess(ctx, tokenString)
	if err != nil || ti == nil {
		return ctx
	}

	// Inject OAuth-specific values
	scope := ti.GetScope()
	scopes := strings.Fields(scope)
	ctx = WithOAuthScopes(ctx, scopes)
	ctx = WithOAuthClientID(ctx, ti.GetClientID())

	return ctx
}

// extractIdentityFromOAuthToken extracts identity from an OAuth access token.
func (p *OAuthPlugin) extractIdentityFromOAuthToken(ctx context.Context) (auth.Identity, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	tokenString := extractBearerToken(md)
	if tokenString == "" {
		return auth.Identity{}, errors.Mark(auth.ErrNotFound, 0)
	}

	// Get token info from store
	ti, err := p.tokenStore.GetByAccess(ctx, tokenString)
	if err != nil || ti == nil {
		return auth.Identity{}, errors.Mark(auth.ErrNotFound, 0)
	}

	// Check expiration
	if ti.GetAccessCreateAt().Add(ti.GetAccessExpiresIn()).Before(time.Now()) {
		return auth.Identity{}, errors.Mark(auth.ErrNotFound, 0)
	}

	// Build identity
	accessToken := ti.GetAccess()
	sessionID := accessToken
	if len(accessToken) > 16 {
		sessionID = accessToken[:16]
	}

	identity := auth.Identity{
		SessionID: sessionID,
		Subject:   ti.GetUserID(),
		Provider:  "oauth:" + ti.GetClientID(),
		AuthTime:  ti.GetAccessCreateAt(),
	}

	return identity, nil
}

// extractBearerToken extracts a bearer token from metadata.
func extractBearerToken(md metadata.MD) string {
	authHeader, ok := md["authorization"]
	if !ok || len(authHeader) == 0 || authHeader[0] == "" {
		return ""
	}

	parts := strings.SplitN(authHeader[0], " ", 2)
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}
	return authHeader[0]
}

// Context keys for OAuth-specific values.
type oauthScopesKey struct{}
type oauthClientIDKey struct{}

// WithOAuthScopes adds OAuth scopes to the context.
func WithOAuthScopes(ctx context.Context, scopes []string) context.Context {
	return context.WithValue(ctx, oauthScopesKey{}, scopes)
}

// OAuthScopesFromContext retrieves OAuth scopes from the context.
func OAuthScopesFromContext(ctx context.Context) []string {
	if scopes, ok := ctx.Value(oauthScopesKey{}).([]string); ok {
		return scopes
	}
	return nil
}

// WithOAuthClientID adds the OAuth client ID to the context.
func WithOAuthClientID(ctx context.Context, clientID string) context.Context {
	return context.WithValue(ctx, oauthClientIDKey{}, clientID)
}

// OAuthClientIDFromContext retrieves the OAuth client ID from the context.
func OAuthClientIDFromContext(ctx context.Context) string {
	if clientID, ok := ctx.Value(oauthClientIDKey{}).(string); ok {
		return clientID
	}
	return ""
}

// HasScope checks if the current context has the specified OAuth scope.
func HasScope(ctx context.Context, scope string) bool {
	scopes := OAuthScopesFromContext(ctx)
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope checks if the current context has any of the specified scopes.
func HasAnyScope(ctx context.Context, scopes ...string) bool {
	for _, scope := range scopes {
		if HasScope(ctx, scope) {
			return true
		}
	}
	return false
}

// HasAllScopes checks if the current context has all of the specified scopes.
func HasAllScopes(ctx context.Context, scopes ...string) bool {
	for _, scope := range scopes {
		if !HasScope(ctx, scope) {
			return false
		}
	}
	return true
}

// IsOAuthRequest returns true if the current request is authenticated via OAuth.
func IsOAuthRequest(ctx context.Context) bool {
	return OAuthClientIDFromContext(ctx) != ""
}

// ParseScopes parses a space-separated scope string into a slice.
func ParseScopes(scopeStr string) []string {
	if scopeStr == "" {
		return nil
	}
	return strings.Fields(scopeStr)
}

// FormatScopes formats a slice of scopes into a space-separated string.
func FormatScopes(scopes []string) string {
	return strings.Join(scopes, " ")
}
