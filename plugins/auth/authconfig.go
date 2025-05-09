package auth

import (
	"context"
	"time"

	"github.com/dpup/prefab"
)

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

func expirationFromContext(ctx context.Context) time.Duration {
	if v, ok := ctx.Value(tokenExpiration{}).(time.Duration); ok {
		return v
	}
	return defaultTokenExpiration
}
