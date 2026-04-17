// Package oauth provides OAuth2 server functionality for Prefab applications.
//
// The OAuth plugin uses the go-oauth2/oauth2 library to provide a production-ready
// OAuth2 authorization server with minimal configuration.
//
// # Basic Usage
//
// Create an OAuth server with just a few lines:
//
//	oauthPlugin := oauth.NewBuilder().
//		WithClient(oauth.Client{
//			ID:           "my-app",
//			Secret:       "secret",
//			RedirectURIs: []string{"http://localhost:3000/callback"},
//			Scopes:       []string{"read", "write"},
//		}).
//		Build()
//
//	server := prefab.New(
//		prefab.WithPlugin(auth.Plugin()),
//		prefab.WithPlugin(oauthPlugin),
//	)
//
// # Scope-Based Authorization
//
// OAuth tokens carry scopes that can be used in authorization decisions:
//
//	if oauth.HasScope(ctx, "write") {
//		// Allow write operation
//	}
//
//	if oauth.IsOAuthRequest(ctx) {
//		clientID := oauth.OAuthClientIDFromContext(ctx)
//	}
package oauth

import (
	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"google.golang.org/grpc/codes"
)

func init() {
	prefab.RegisterConfigKeys(
		prefab.ConfigKeyInfo{
			Key:         "oauth.enforcePkce",
			Description: "Require PKCE (Proof Key for Code Exchange) for public OAuth clients",
			Type:        "bool",
			Default:     "false",
		},
		prefab.ConfigKeyInfo{
			Key:         "oauth.issuer",
			Description: "OAuth token issuer URL (defaults to the server's address config key)",
			Type:        "string",
		},
	)
}

// PluginName is the identifier for the OAuth plugin.
const PluginName = "oauth"

// Standard OAuth2 errors.
var (
	ErrInvalidClient      = errors.NewC("invalid_client", codes.Unauthenticated)
	ErrInvalidGrant       = errors.NewC("invalid_grant", codes.InvalidArgument)
	ErrInvalidScope       = errors.NewC("invalid_scope", codes.InvalidArgument)
	ErrAccessDenied       = errors.NewC("access_denied", codes.PermissionDenied)
	ErrPKCERequired       = errors.NewC("invalid_request: code_challenge required for public clients", codes.InvalidArgument)
	ErrPKCEMethodRequired = errors.NewC("invalid_request: code_challenge_method=S256 required for public clients", codes.InvalidArgument)
	ErrInvalidToken       = errors.NewC("invalid_token", codes.Unauthenticated)
	ErrTokenNotFound      = errors.NewC("token_not_found", codes.NotFound)
	ErrTokenRevoked       = errors.NewC("token_revoked", codes.Unauthenticated)
)
