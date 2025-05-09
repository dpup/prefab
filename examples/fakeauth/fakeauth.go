// Example of how to use the fake auth plugin for testing scenarios
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/plugins/auth"
	fake "github.com/dpup/prefab/plugins/auth/fakeauth"
	"github.com/dpup/prefab/plugins/storage"
	"github.com/dpup/prefab/plugins/storage/memstore"
)

func main() {
	// Create auth and fake auth plugins
	authPlugin := auth.Plugin(
		// Use a fixed signing key for testing
		auth.WithSigningKey("test-signing-key"),
		// Set short expiration for demonstration
		auth.WithExpiration(5*time.Minute),
	)

	fakeAuthPlugin := fake.Plugin(
		// Optionally customize the default identity
		fake.WithDefaultIdentity(auth.Identity{
			Provider:      fake.ProviderName,
			Subject:       "test-admin-123",
			Email:         "admin@example.com",
			Name:          "Test Admin",
			EmailVerified: true,
		}),
		// Optionally add validation logic
		fake.WithIdentityValidator(func(ctx context.Context, creds map[string]string) error {
			// Example: only allow certain test emails
			if email, ok := creds["email"]; ok && email != "" {
				if !isValidTestEmail(email) {
					return fmt.Errorf("invalid test email: %s", email)
				}
			}
			return nil
		}),
	)

	// Create a memory storage for the auth blocklist
	// This is needed for logout to work properly as it allows the auth plugin
	// to track and persist blocked/revoked tokens. Without storage, logout would
	// only clear the cookie but not invalidate the token, so it could still be used.
	memStore := memstore.New()

	// Create a new prefab server with plugins
	s := prefab.New(
		// Register storage plugin first (auth plugin depends on it for blocklist)
		prefab.WithPlugin(storage.Plugin(memStore)),
		prefab.WithPlugin(authPlugin),
		prefab.WithPlugin(fakeAuthPlugin),
		// Register static file serving for demo HTML
		prefab.WithStaticFiles("/", "./examples/fakeauth/static"),
	)

	// Start the server
	if err := s.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// Helper function to validate test emails
func isValidTestEmail(email string) bool {
	validDomains := []string{"example.com", "test.com", "fake.com"}

	for _, domain := range validDomains {
		if len(email) > len(domain)+1 && email[len(email)-len(domain)-1:] == "@"+domain {
			return true
		}
	}

	return false
}
