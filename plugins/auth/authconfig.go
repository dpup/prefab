package auth

import (
	"context"
	"time"

	"github.com/dpup/prefab"
)

func init() {
	prefab.RegisterConfigKeys(
		prefab.ConfigKeyInfo{
			Key:         "auth.signingKey",
			Description: "JWT signing key for identity tokens",
			Type:        "string",
		},
		prefab.ConfigKeyInfo{
			Key:         "auth.expiration",
			Description: "How long identity tokens should be valid for",
			Type:        "duration",
			Default:     "24h",
		},
		prefab.ConfigKeyInfo{
			Key:         "auth.delegation.enabled",
			Description: "Enable identity delegation (admin assume user)",
			Type:        "bool",
			Default:     "false",
		},
		prefab.ConfigKeyInfo{
			Key:         "auth.delegation.requireReason",
			Description: "Require reason field for identity delegation",
			Type:        "bool",
			Default:     "true",
		},
		prefab.ConfigKeyInfo{
			Key:         "auth.delegation.expiration",
			Description: "Maximum duration for delegated identity tokens (defaults to auth.expiration if not set)",
			Type:        "duration",
			Default:     "",
		},
	)
}

const defaultTokenExpiration = time.Hour * 24 * 30

type signingKey struct{}

type tokenExpiration struct{}

func injectSigningKey(b string) prefab.ConfigInjector {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, signingKey{}, b)
	}
}

func injectExpiration(d time.Duration) prefab.ConfigInjector {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, tokenExpiration{}, d)
	}
}

func signingKeyFromContext(ctx context.Context) []byte {
	if v, ok := ctx.Value(signingKey{}).(string); ok {
		return []byte(v)
	}
	return []byte("In a world of prefab dreams, authenticity gleams.")
}

// SigningKeyFromContext returns the JWT signing key from context.
// This is exported for use by plugins that need to create their own tokens.
func SigningKeyFromContext(ctx context.Context) []byte {
	return signingKeyFromContext(ctx)
}

func expirationFromContext(ctx context.Context) time.Duration {
	if v, ok := ctx.Value(tokenExpiration{}).(time.Duration); ok {
		return v
	}
	return defaultTokenExpiration
}
