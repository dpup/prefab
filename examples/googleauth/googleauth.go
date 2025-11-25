// An example using google auth with offline token access.
//
// This example demonstrates how to:
//   - Configure Google OAuth for offline access (refresh tokens)
//   - Request additional scopes (e.g., Gmail API)
//   - Handle and store OAuth tokens for later API access
//
// Edit config.yaml or set AUTH_GOOGLE_ID and AUTH_GOOGLE_SECRET in your
// environment.
//
// $ go run examples/googleauth/googleauth.go
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/auth/google"
	"github.com/dpup/prefab/plugins/storage"
	"github.com/dpup/prefab/plugins/storage/sqlite"
)

func main() {
	// In-memory token store for demonstration. In production, use a persistent
	// store like the storage plugin with SQLite or Postgres.
	tokenStore := NewTokenStore()

	s := prefab.New(
		prefab.WithPlugin(auth.Plugin()),
		prefab.WithPlugin(google.Plugin(
			// Enable offline access to get a refresh token.
			google.WithOfflineAccess(),

			// Request additional scopes for Google APIs you want to access.
			// Uncomment the scopes you need:
			// google.WithScopes(
			// 	"https://www.googleapis.com/auth/gmail.readonly",
			// 	"https://www.googleapis.com/auth/calendar.readonly",
			// ),

			// Handle tokens after successful authentication.
			google.WithTokenHandler(tokenStore.HandleToken),
		)),
		// Register an SQLite store to persist revoked identity tokens.
		prefab.WithPlugin(storage.Plugin(sqlite.New("example_googleauth.s3db"))),
		prefab.WithStaticFiles("/", "./examples/googleauth/static/"),
	)

	fmt.Println("")
	fmt.Println("Visit http://localhost:8000/ in your browser")
	fmt.Println("")

	// Start the server.
	if err := s.Start(); err != nil {
		fmt.Println(err)
	}
}

// TokenStore demonstrates how to store OAuth tokens for later use.
// In production, you would persist these to a database.
type TokenStore struct {
	mu     sync.RWMutex
	tokens map[string]google.OAuthToken // keyed by user subject
}

// NewTokenStore creates a new in-memory token store.
func NewTokenStore() *TokenStore {
	return &TokenStore{
		tokens: make(map[string]google.OAuthToken),
	}
}

// HandleToken is called by the Google auth plugin after successful authentication.
// It receives the identity and OAuth token, allowing you to associate the token
// with your user model.
func (s *TokenStore) HandleToken(ctx context.Context, identity auth.Identity, token google.OAuthToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store the token keyed by the user's subject (Google user ID).
	// In production, you might key by your internal user ID instead.
	s.tokens[identity.Subject] = token

	fmt.Printf("Stored OAuth token for user %s (%s)\n", identity.Email, identity.Subject)
	if token.HasRefreshToken() {
		fmt.Println("  - Refresh token received (can access APIs offline)")
	} else {
		fmt.Println("  - No refresh token (user may have already authorized)")
	}

	return nil
}

// GetToken retrieves a stored token for the given user subject.
// Returns false if no token is stored for this user.
func (s *TokenStore) GetToken(subject string) (google.OAuthToken, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.tokens[subject]
	return token, ok
}
