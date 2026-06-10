package auth

import (
	"context"
	"crypto/rand"
	"log"
	"sync"
	"time"

	"github.com/dpup/prefab"
)

// fallbackSigningKey is an ephemeral, per-process key used only when a token
// operation runs on a context that never passed through the auth plugin's
// request-config injector. It is generated randomly at startup rather than
// being a hardcoded constant, so tokens produced on this path cannot be forged
// by anyone who has read the source. Reaching this path indicates a
// misconfiguration; the key is non-portable across processes by design.
var (
	fallbackSigningKey     = newFallbackSigningKey()
	fallbackSigningKeyOnce sync.Once
)

func newFallbackSigningKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("auth: failed to generate fallback signing key: " + err.Error())
	}
	return key
}

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
	fallbackSigningKeyOnce.Do(func() {
		log.Println("⚠️  WARNING: auth signing key not found in context; using an " +
			"ephemeral per-process key. This usually means a token operation ran on a " +
			"context that did not pass through the auth plugin's request config. Tokens " +
			"signed this way are not portable across processes or restarts.")
	})
	return fallbackSigningKey
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
