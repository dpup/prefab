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
	"github.com/dpup/prefab/errors"
	"google.golang.org/grpc/codes"
)

// PluginName is the identifier for the OAuth plugin.
const PluginName = "oauth"

// Standard OAuth2 errors.
var (
	ErrInvalidClient = errors.NewC("invalid_client", codes.Unauthenticated)
	ErrInvalidGrant  = errors.NewC("invalid_grant", codes.InvalidArgument)
	ErrInvalidScope  = errors.NewC("invalid_scope", codes.InvalidArgument)
	ErrAccessDenied  = errors.NewC("access_denied", codes.PermissionDenied)
)
