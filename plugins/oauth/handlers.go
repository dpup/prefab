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

		err := p.server.HandleAuthorizeRequest(w, r)
		if err != nil {
			logger.Error("authorization error", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
	})
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
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
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
