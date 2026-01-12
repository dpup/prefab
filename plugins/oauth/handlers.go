package oauth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dpup/prefab/logging"
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
func (p *OAuthPlugin) validatePKCERequired(r *http.Request) error {
	clientID := r.FormValue("client_id")
	if clientID == "" {
		return nil // Let the main handler deal with missing client_id
	}

	client, err := p.clientStore.store.GetClient(r.Context(), clientID)
	if err != nil {
		return nil // Let the main handler deal with invalid client
	}

	// Only enforce PKCE for public clients
	if !client.Public {
		return nil
	}

	codeChallenge := r.FormValue("code_challenge")
	if codeChallenge == "" {
		return ErrPKCERequired
	}

	// Validate code_challenge_method if provided
	method := r.FormValue("code_challenge_method")
	if method != "" && method != "plain" && method != "S256" {
		return ErrInvalidGrant
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

		err := p.server.HandleTokenRequest(w, r)
		if err != nil {
			logger.Error("token error", "error", err)
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "The token request is invalid")
		}
	})
}

// writeOAuthError writes a standard OAuth2 error response.
func writeOAuthError(w http.ResponseWriter, status int, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             errorCode,
		"error_description": description,
	})
}

// metadataHandler returns OAuth2 authorization server metadata.
func (p *OAuthPlugin) metadataHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issuer := p.issuer
		if issuer == "" {
			scheme := "https"
			if r.TLS == nil {
				scheme = "http"
			}
			issuer = scheme + "://" + r.Host
		}

		metadata := map[string]interface{}{
			"issuer":                                issuer,
			"authorization_endpoint":                issuer + "/oauth/authorize",
			"token_endpoint":                        issuer + "/oauth/token",
			"revocation_endpoint":                   issuer + "/oauth/revoke",
			"introspection_endpoint":                issuer + "/oauth/introspect",
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token", "client_credentials"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
			"revocation_endpoint_auth_methods_supported":    []string{"client_secret_basic", "client_secret_post"},
			"introspection_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
			"code_challenge_methods_supported":              []string{"plain", "S256"},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metadata); err != nil {
			http.Error(w, "failed to encode metadata", http.StatusInternalServerError)
		}
	})
}

// revokeHandler handles token revocation requests per RFC 7009.
// Clients can revoke access tokens or refresh tokens they have issued.
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
		clientID, clientSecret, err := p.getClientCredentials(r)
		if err != nil {
			writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
			return
		}

		client, err := p.clientStore.store.GetClient(ctx, clientID)
		if err != nil || (!client.Public && client.Secret != clientSecret) {
			writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
			return
		}

		token := r.FormValue("token")
		if token == "" {
			writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Missing token parameter")
			return
		}

		tokenTypeHint := r.FormValue("token_type_hint")

		// Try to revoke based on hint, or try both if no hint provided
		var revoked bool
		switch tokenTypeHint {
		case "refresh_token":
			if err := p.tokenStore.store.RemoveByRefresh(ctx, token); err == nil {
				revoked = true
			}
		case "access_token", "":
			// Try access token first
			if err := p.tokenStore.store.RemoveByAccess(ctx, token); err == nil {
				revoked = true
			} else if tokenTypeHint == "" {
				// If no hint and access token removal failed, try refresh token
				if err := p.tokenStore.store.RemoveByRefresh(ctx, token); err == nil {
					revoked = true
				}
			}
		}

		if revoked {
			logger.Info("token revoked", "client_id", clientID)
		}

		// RFC 7009: Always return 200 OK, even if token was not found
		w.WriteHeader(http.StatusOK)
	})
}

// introspectHandler handles token introspection requests per RFC 7662.
// Resource servers can validate tokens and retrieve their metadata.
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
		clientID, clientSecret, err := p.getClientCredentials(r)
		if err != nil {
			writeOAuthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed")
			return
		}

		client, err := p.clientStore.store.GetClient(ctx, clientID)
		if err != nil || (!client.Public && client.Secret != clientSecret) {
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
		var tokenInfo TokenInfo
		var found bool

		switch tokenTypeHint {
		case "refresh_token":
			if info, err := p.tokenStore.store.GetByRefresh(ctx, token); err == nil {
				tokenInfo = info
				found = true
			}
		case "access_token", "":
			// Try access token first
			if info, err := p.tokenStore.store.GetByAccess(ctx, token); err == nil {
				tokenInfo = info
				found = true
			} else if tokenTypeHint == "" {
				// If no hint and access token lookup failed, try refresh token
				if info, err := p.tokenStore.store.GetByRefresh(ctx, token); err == nil {
					tokenInfo = info
					found = true
				}
			}
		}

		if !found {
			// Token not found - return inactive
			writeIntrospectionResponse(w, map[string]interface{}{"active": false})
			return
		}

		// Check if token is expired
		var expiresAt time.Time
		var isAccessToken bool
		if tokenInfo.Access == token {
			expiresAt = tokenInfo.AccessCreateAt.Add(tokenInfo.AccessExpiresIn)
			isAccessToken = true
		} else {
			expiresAt = tokenInfo.RefreshCreateAt.Add(tokenInfo.RefreshExpiresIn)
		}

		if time.Now().After(expiresAt) {
			writeIntrospectionResponse(w, map[string]interface{}{"active": false})
			return
		}

		// Build introspection response
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

		logger.Debug("token introspected", "client_id", clientID, "active", true)
		writeIntrospectionResponse(w, response)
	})
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

// writeIntrospectionResponse writes a JSON introspection response.
func writeIntrospectionResponse(w http.ResponseWriter, response map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	json.NewEncoder(w).Encode(response)
}
