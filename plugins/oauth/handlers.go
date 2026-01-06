package oauth

import (
	"encoding/json"
	"net/http"

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
			"response_types_supported":              []string{"code"},
			"grant_types_supported":                 []string{"authorization_code", "refresh_token", "client_credentials"},
			"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
			"code_challenge_methods_supported":      []string{"plain", "S256"},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metadata); err != nil {
			http.Error(w, "failed to encode metadata", http.StatusInternalServerError)
		}
	})
}
