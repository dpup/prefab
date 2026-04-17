package oauth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dpup/prefab/logging"
)

// Form field and token_type_hint values used by the OAuth endpoints.
const (
	grantTypeRefreshToken     = "refresh_token"
	tokenTypeHintRefreshToken = "refresh_token"
	tokenTypeHintAccessToken  = "access_token"
)

// authorizeHandler handles the OAuth2 authorization endpoint.
func (p *OAuthPlugin) authorizeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)
		if logger == nil {
			logger = logging.NewDevLogger()
		}

		// Enforce PKCE for public clients if configured
		if p.shouldEnforcePKCE() {
			if err := p.validatePKCERequired(r); err != nil {
				logger.Warn("PKCE validation failed", "error", err)
				writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
		}

		err := p.server.HandleAuthorizeRequest(w, r)
		if err != nil {
			logger.Error("authorization error", "error", err)
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "The request is invalid")
		}
	})
}

// validatePKCERequired checks if PKCE is required for the client and validates accordingly.
// When PKCE is enforced, only the S256 method is accepted. The `plain` method
// provides no protection against the authorization-code interception attack
// PKCE is designed to defeat (code_challenge == code_verifier), so we refuse to
// accept it — including when the method parameter is omitted, which the
// underlying library would default to plain.
func (p *OAuthPlugin) validatePKCERequired(r *http.Request) error {
	clientID := r.FormValue("client_id")
	if clientID == "" {
		return nil // Let the main handler deal with missing client_id
	}

	client, err := p.clientStore.store.GetClient(r.Context(), clientID)
	if err != nil {
		// Client not found - defer validation to the OAuth library which provides
		// proper error responses. We only validate PKCE for known public clients.
		return nil //nolint:nilerr // intentionally defer client validation to OAuth library
	}

	// Only enforce PKCE for public clients
	if !client.Public {
		return nil
	}

	codeChallenge := r.FormValue("code_challenge")
	if codeChallenge == "" {
		return ErrPKCERequired
	}

	// When PKCE is enforced, require S256. Missing method defaults to plain in
	// the underlying library, so we reject that too.
	method := r.FormValue("code_challenge_method")
	if method != "S256" {
		return ErrPKCEMethodRequired
	}

	return nil
}

// tokenHandler handles the OAuth2 token endpoint.
func (p *OAuthPlugin) tokenHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)
		if logger == nil {
			logger = logging.NewDevLogger()
		}

		// Pre-authenticate the client on the refresh_token grant. go-oauth2's
		// RefreshAccessToken does not verify request credentials against the
		// token's owner — without this check, anyone who possesses a refresh
		// token can exchange it for a new access token, bypassing client
		// authentication entirely. ParseForm errors are intentionally ignored:
		// partial parses still populate r.Form with the values we need, and
		// gating on err==nil would let a malformed field like `x=%ZZ` skip
		// the entire auth check.
		_ = r.ParseForm()
		if r.FormValue("grant_type") == grantTypeRefreshToken {
			if err := p.authenticateRefreshGrant(r); err != nil {
				logger.Warn("refresh token client authentication failed", "error", err)
				writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
				return
			}
		}

		err := p.server.HandleTokenRequest(w, r)
		if err != nil {
			logger.Error("token error", "error", err)
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "The token request is invalid")
		}
	})
}

// authenticateRefreshGrant verifies that the request is authenticated as the
// same client that owns the supplied refresh token.
func (p *OAuthPlugin) authenticateRefreshGrant(r *http.Request) error {
	refresh := r.FormValue("refresh_token")
	if refresh == "" {
		return ErrInvalidGrant
	}

	info, err := p.tokenStore.store.GetByRefresh(r.Context(), refresh)
	if err != nil {
		return ErrInvalidGrant
	}

	client, err := p.authenticateClient(r)
	if err != nil {
		return err
	}

	if client.ID != info.ClientID {
		return ErrInvalidClient
	}
	return nil
}

// requestBaseURL derives the externally-visible base URL from the request,
// honoring X-Forwarded-Proto and X-Forwarded-Host so that servers behind
// TLS-terminating proxies advertise https:// instead of http://.
func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host
}

// writeOAuthError writes a standard OAuth2 error response.
func writeOAuthError(w http.ResponseWriter, status int, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error":             errorCode,
		"error_description": description,
	}); err != nil {
		// Logging would require context; error is already written to response
		_ = err
	}
}

// metadataHandler returns OAuth2 authorization server metadata.
//
// Operators should configure oauth.issuer (or the global address config) so
// the advertised issuer is stable and does not depend on request-level data.
// When the issuer is not configured the handler derives it from the request,
// honoring X-Forwarded-Proto/Host so that TLS-terminating proxies do not
// cause the endpoint to advertise http:// URLs. This derived form is
// convenient for development but is vulnerable to Host-header poisoning: set
// oauth.issuer in production.
func (p *OAuthPlugin) metadataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issuer := p.issuer
		if issuer == "" {
			issuer = requestBaseURL(r)
		}

		// Advertise only S256 when PKCE is enforced — the plain method
		// offers no protection against authorization-code interception.
		pkceMethods := []string{"S256"}
		if !p.shouldEnforcePKCE() {
			pkceMethods = []string{"plain", "S256"}
		}

		metadata := map[string]interface{}{
			"issuer":                                        issuer,
			"authorization_endpoint":                        issuer + "/oauth/authorize",
			"token_endpoint":                                issuer + "/oauth/token",
			"revocation_endpoint":                           issuer + "/oauth/revoke",
			"introspection_endpoint":                        issuer + "/oauth/introspect",
			"response_types_supported":                      []string{"code"},
			"grant_types_supported":                         []string{"authorization_code", "refresh_token", "client_credentials"},
			"token_endpoint_auth_methods_supported":         []string{"client_secret_basic", "client_secret_post"},
			"revocation_endpoint_auth_methods_supported":    []string{"client_secret_basic", "client_secret_post"},
			"introspection_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
			"code_challenge_methods_supported":              pkceMethods,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metadata); err != nil {
			http.Error(w, "failed to encode metadata", http.StatusInternalServerError)
		}
	})
}

// revokeHandler handles token revocation requests per RFC 7009.
// Clients can only revoke tokens that were issued to them.
func (p *OAuthPlugin) revokeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)
		if logger == nil {
			logger = logging.NewDevLogger()
		}

		if r.Method != http.MethodPost {
			writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "Method not allowed")
			return
		}

		// Authenticate the client
		client, err := p.authenticateClient(r)
		if err != nil {
			writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
			return
		}

		token := r.FormValue("token")
		if token == "" {
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Missing token parameter")
			return
		}

		tokenTypeHint := r.FormValue("token_type_hint")

		// Try to find and revoke the token, verifying ownership
		revoked := p.revokeTokenWithOwnershipCheck(ctx, token, tokenTypeHint, client.ID)

		if revoked {
			logger.Info("token revoked", "client_id", client.ID)
		}

		// RFC 7009: Always return 200 OK, even if token was not found
		w.WriteHeader(http.StatusOK)
	})
}

// revokeTokenWithOwnershipCheck attempts to revoke a token after verifying
// it belongs to the requesting client. Returns true if a token was revoked.
func (p *OAuthPlugin) revokeTokenWithOwnershipCheck(ctx context.Context, token, tokenTypeHint, clientID string) bool {
	switch tokenTypeHint {
	case tokenTypeHintRefreshToken:
		return p.tryRevokeRefreshToken(ctx, token, clientID)
	case tokenTypeHintAccessToken:
		return p.tryRevokeAccessToken(ctx, token, clientID)
	default:
		// No hint: try access token first, then refresh token
		if p.tryRevokeAccessToken(ctx, token, clientID) {
			return true
		}
		return p.tryRevokeRefreshToken(ctx, token, clientID)
	}
}

// tryRevokeAccessToken attempts to revoke an access token if it belongs to
// the client. Also removes the paired refresh token (RFC 7009 §2.1
// recommends revoking the full grant).
func (p *OAuthPlugin) tryRevokeAccessToken(ctx context.Context, token, clientID string) bool {
	info, err := p.tokenStore.store.GetByAccess(ctx, token)
	if err != nil || info.ClientID != clientID {
		return false
	}
	if err := p.tokenStore.store.RemoveByAccess(ctx, token); err != nil {
		return false
	}
	if info.Refresh != "" {
		_ = p.tokenStore.store.RemoveByRefresh(ctx, info.Refresh)
	}
	return true
}

// tryRevokeRefreshToken attempts to revoke a refresh token if it belongs to
// the client. Per RFC 7009 §2.1, revoking a refresh token also invalidates
// the access tokens issued alongside it.
func (p *OAuthPlugin) tryRevokeRefreshToken(ctx context.Context, token, clientID string) bool {
	info, err := p.tokenStore.store.GetByRefresh(ctx, token)
	if err != nil || info.ClientID != clientID {
		return false
	}
	if err := p.tokenStore.store.RemoveByRefresh(ctx, token); err != nil {
		return false
	}
	if info.Access != "" {
		_ = p.tokenStore.store.RemoveByAccess(ctx, info.Access)
	}
	return true
}

// introspectHandler handles token introspection requests per RFC 7662.
// Clients can only introspect tokens that were issued to them.
func (p *OAuthPlugin) introspectHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx)
		if logger == nil {
			logger = logging.NewDevLogger()
		}

		if r.Method != http.MethodPost {
			writeOAuthError(w, http.StatusMethodNotAllowed, "invalid_request", "Method not allowed")
			return
		}

		// Authenticate the client
		client, err := p.authenticateClient(r)
		if err != nil {
			writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
			return
		}

		token := r.FormValue("token")
		if token == "" {
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Missing token parameter")
			return
		}

		tokenTypeHint := r.FormValue("token_type_hint")

		// Try to find the token
		tokenInfo, found := p.findToken(ctx, token, tokenTypeHint)

		if !found {
			// Token not found - return inactive
			writeIntrospectionResponse(w, logger, map[string]interface{}{"active": false})
			return
		}

		// Verify token ownership - clients can only introspect their own tokens
		if tokenInfo.ClientID != client.ID {
			writeIntrospectionResponse(w, logger, map[string]interface{}{"active": false})
			return
		}

		// Check if token is expired
		expiresAt, isAccessToken := p.getTokenExpiry(tokenInfo, token)

		if time.Now().After(expiresAt) {
			writeIntrospectionResponse(w, logger, map[string]interface{}{"active": false})
			return
		}

		// Build introspection response
		response := p.buildIntrospectionResponse(tokenInfo, expiresAt, isAccessToken)

		logger.Debug("token introspected", "client_id", client.ID, "active", true)
		writeIntrospectionResponse(w, logger, response)
	})
}

// findToken looks up a token by access or refresh token based on the hint.
func (p *OAuthPlugin) findToken(ctx context.Context, token, tokenTypeHint string) (TokenInfo, bool) {
	switch tokenTypeHint {
	case tokenTypeHintRefreshToken:
		if info, err := p.tokenStore.store.GetByRefresh(ctx, token); err == nil {
			return info, true
		}
	case tokenTypeHintAccessToken:
		if info, err := p.tokenStore.store.GetByAccess(ctx, token); err == nil {
			return info, true
		}
	default:
		// No hint: try access token first, then refresh token
		if info, err := p.tokenStore.store.GetByAccess(ctx, token); err == nil {
			return info, true
		}
		if info, err := p.tokenStore.store.GetByRefresh(ctx, token); err == nil {
			return info, true
		}
	}
	return TokenInfo{}, false
}

// getTokenExpiry returns the expiration time and whether it's an access token.
func (p *OAuthPlugin) getTokenExpiry(tokenInfo TokenInfo, token string) (time.Time, bool) {
	if tokenInfo.Access == token {
		return tokenInfo.AccessCreateAt.Add(tokenInfo.AccessExpiresIn), true
	}
	return tokenInfo.RefreshCreateAt.Add(tokenInfo.RefreshExpiresIn), false
}

// buildIntrospectionResponse builds the introspection response map.
func (p *OAuthPlugin) buildIntrospectionResponse(tokenInfo TokenInfo, expiresAt time.Time, isAccessToken bool) map[string]interface{} {
	response := map[string]interface{}{
		"active":    true,
		"client_id": tokenInfo.ClientID,
		"exp":       expiresAt.Unix(),
	}

	if tokenInfo.Scope != "" {
		response["scope"] = tokenInfo.Scope
	}

	if tokenInfo.UserID != "" {
		response["sub"] = tokenInfo.UserID
	}

	if isAccessToken {
		response["token_type"] = "Bearer"
		response["iat"] = tokenInfo.AccessCreateAt.Unix()
	} else {
		response["iat"] = tokenInfo.RefreshCreateAt.Unix()
	}

	if p.issuer != "" {
		response["iss"] = p.issuer
	}

	return response
}

// getClientCredentials extracts client credentials from the request.
// Supports both HTTP Basic authentication and form-encoded credentials.
func (p *OAuthPlugin) getClientCredentials(r *http.Request) (clientID, clientSecret string, err error) {
	// Try Basic auth first
	clientID, clientSecret, ok := r.BasicAuth()
	if ok && clientID != "" {
		return clientID, clientSecret, nil
	}

	// Fall back to form-encoded credentials
	if err := r.ParseForm(); err != nil {
		return "", "", err
	}

	clientID = r.FormValue("client_id")
	clientSecret = r.FormValue("client_secret")

	if clientID == "" {
		return "", "", ErrInvalidClient
	}

	return clientID, clientSecret, nil
}

// authenticateClient authenticates a client from request credentials.
// Uses constant-time comparison to prevent timing attacks on client secrets.
func (p *OAuthPlugin) authenticateClient(r *http.Request) (*Client, error) {
	clientID, clientSecret, err := p.getClientCredentials(r)
	if err != nil {
		return nil, ErrInvalidClient
	}

	client, err := p.clientStore.store.GetClient(r.Context(), clientID)
	if err != nil {
		return nil, ErrInvalidClient
	}

	// Public clients don't require secret validation
	if client.Public {
		return client, nil
	}

	// Refuse to authenticate a confidential client with an empty stored secret;
	// otherwise subtle.ConstantTimeCompare("", "") returns 1 and any caller
	// authenticates as the misconfigured client.
	if client.Secret == "" {
		return nil, ErrInvalidClient
	}

	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(client.Secret), []byte(clientSecret)) != 1 {
		return nil, ErrInvalidClient
	}

	return client, nil
}

// writeIntrospectionResponse writes a JSON introspection response.
func writeIntrospectionResponse(w http.ResponseWriter, logger logging.Logger, response map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		if logger != nil {
			logger.Error("failed to encode introspection response", "error", err)
		}
	}
}
