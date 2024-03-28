package serverutil

import (
	"context"
)

const (
	defaultAddress = "http://localhost:8080"
)

// AddressFromContext returns the server's external address. This is the what
// links should reference, and likely points at a CDN or load balancer.
func AddressFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(addressKey{}).(string); ok {
		return v
	}
	return defaultAddress
}

// WithAddress adds the server's external address to the context.
func WithAddress(ctx context.Context, address string) context.Context {
	return context.WithValue(ctx, addressKey{}, address)
}

type addressKey struct{}
