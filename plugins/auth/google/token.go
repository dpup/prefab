package google

import (
	"context"
	"time"

	"github.com/dpup/prefab/plugins/auth"
)

// OAuthToken contains the OAuth2 token data received from Google after a
// successful authentication. Applications can use this token to access Google
// APIs on behalf of the user.
type OAuthToken struct {
	// AccessToken is the token used to authenticate API requests.
	AccessToken string

	// RefreshToken is used to obtain new access tokens after expiry.
	// Only present when offline access is enabled via WithOfflineAccess().
	// Google only returns a refresh token on the first authorization, or when
	// the user re-consents. Store this securely.
	RefreshToken string

	// TokenType is the type of token, typically "Bearer".
	TokenType string

	// Expiry is the time at which the access token expires.
	// A zero value means the token does not expire.
	Expiry time.Time
}

// HasRefreshToken returns true if the token includes a refresh token.
func (t OAuthToken) HasRefreshToken() bool {
	return t.RefreshToken != ""
}

// IsExpired returns true if the access token has expired.
// Returns false if the token has no expiry time set.
func (t OAuthToken) IsExpired() bool {
	if t.Expiry.IsZero() {
		return false
	}
	return time.Now().After(t.Expiry)
}

// TokenHandler is called after successful OAuth authentication with Google.
// The handler receives the authenticated identity and the OAuth tokens.
//
// Applications should use this handler to store tokens for later use, such as
// accessing Google APIs (Gmail, Calendar, Drive, etc.) on behalf of the user.
//
// The identity parameter contains the authenticated user's information
// (subject, email, name) that can be used to associate the token with a user
// in your system.
//
// Returning an error from the handler will abort the login flow and return
// the error to the user. Return nil to allow the login to proceed normally.
//
// Example:
//
//	google.WithTokenHandler(func(ctx context.Context, identity auth.Identity, token google.OAuthToken) error {
//	    return userService.StoreGoogleToken(ctx, identity.Subject, token)
//	})
type TokenHandler func(ctx context.Context, identity auth.Identity, token OAuthToken) error
