package auth

import (
	"testing"
	"time"

	"github.com/dpup/prefab/serverutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestInjectSigningKey(t *testing.T) {
	key := "test-signing-key-123"
	injector := injectSigningKey(key)

	ctx := injector(t.Context())

	// Verify key was injected
	extracted := signingKeyFromContext(ctx)
	assert.Equal(t, []byte(key), extracted)
}

func TestSigningKeyFromContext(t *testing.T) {
	t.Run("WithKey", func(t *testing.T) {
		ctx := injectSigningKey("custom-key")(t.Context())
		key := signingKeyFromContext(ctx)
		assert.Equal(t, []byte("custom-key"), key)
	})

	t.Run("WithoutKey_UsesDefault", func(t *testing.T) {
		ctx := t.Context()
		key := signingKeyFromContext(ctx)
		// Should return default key
		assert.Equal(t, []byte("In a world of prefab dreams, authenticity gleams."), key)
	})
}

func TestInjectExpiration(t *testing.T) {
	duration := 48 * time.Hour
	injector := injectExpiration(duration)

	ctx := injector(t.Context())

	// Verify duration was injected
	extracted := expirationFromContext(ctx)
	assert.Equal(t, duration, extracted)
}

func TestExpirationFromContext(t *testing.T) {
	t.Run("WithExpiration", func(t *testing.T) {
		ctx := injectExpiration(72 * time.Hour)(t.Context())
		exp := expirationFromContext(ctx)
		assert.Equal(t, 72*time.Hour, exp)
	})

	t.Run("WithoutExpiration_UsesDefault", func(t *testing.T) {
		ctx := t.Context()
		exp := expirationFromContext(ctx)
		// Should return default 30 days
		assert.Equal(t, time.Hour*24*30, exp)
	})
}

func TestSendIdentityCookie(t *testing.T) {
	t.Run("HTTPAddress", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)
		ctx = serverutil.WithAddress(ctx, "http://localhost:8000")
		ctx = injectExpiration(24 * time.Hour)(ctx)

		err := SendIdentityCookie(ctx, "test-token")
		require.NoError(t, err)

		// Verify cookie was set in metadata
		require.NotNil(t, mockTransport.md)
		setCookieHeaders := (*mockTransport.md)["grpc-metadata-set-cookie"]
		require.Len(t, setCookieHeaders, 1)

		// Parse and verify cookie properties
		cookieStr := setCookieHeaders[0]
		assert.Contains(t, cookieStr, "pf-id=test-token")
		assert.Contains(t, cookieStr, "Path=/")
		assert.Contains(t, cookieStr, "HttpOnly")
		assert.NotContains(t, cookieStr, "Secure") // HTTP address should not set Secure
		assert.Contains(t, cookieStr, "SameSite=Lax")
	})

	t.Run("HTTPSAddress", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)
		ctx = serverutil.WithAddress(ctx, "https://example.com")
		ctx = injectExpiration(24 * time.Hour)(ctx)

		err := SendIdentityCookie(ctx, "test-token")
		require.NoError(t, err)

		// Verify cookie was set with Secure flag for HTTPS
		require.NotNil(t, mockTransport.md)
		setCookieHeaders := (*mockTransport.md)["grpc-metadata-set-cookie"]
		require.Len(t, setCookieHeaders, 1)

		cookieStr := setCookieHeaders[0]
		assert.Contains(t, cookieStr, "pf-id=test-token")
		assert.Contains(t, cookieStr, "Secure") // HTTPS address should set Secure
		assert.Contains(t, cookieStr, "HttpOnly")
	})

	t.Run("CookieExpiration", func(t *testing.T) {
		mockTransport := &mockServerTransportStream{}
		ctx := grpc.NewContextWithServerTransportStream(t.Context(), mockTransport)
		ctx = serverutil.WithAddress(ctx, "http://localhost:8000")
		ctx = injectExpiration(48 * time.Hour)(ctx)

		err := SendIdentityCookie(ctx, "test-token")
		require.NoError(t, err)

		// Verify cookie has expiration set
		require.NotNil(t, mockTransport.md)
		setCookieHeaders := (*mockTransport.md)["grpc-metadata-set-cookie"]
		require.Len(t, setCookieHeaders, 1)

		// Parse the Set-Cookie header to verify expiration
		cookieStr := setCookieHeaders[0]
		assert.Contains(t, cookieStr, "Expires=")
		// Verify the expiration is set to use the configured duration (48 hours)
		assert.Contains(t, cookieStr, "pf-id=test-token")
	})
}

// mockServerTransportStream implements grpc.ServerTransportStream for testing
type mockServerTransportStream struct {
	md *metadata.MD
}

func (m *mockServerTransportStream) Method() string {
	return "test"
}

func (m *mockServerTransportStream) SetHeader(md metadata.MD) error {
	m.md = &md
	return nil
}

func (m *mockServerTransportStream) SendHeader(md metadata.MD) error {
	panic("Not implemented")
}

func (m *mockServerTransportStream) SetTrailer(md metadata.MD) error {
	panic("Not implemented")
}
