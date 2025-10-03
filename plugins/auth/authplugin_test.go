package auth

import (
	"context"
	"testing"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/storage"
	"github.com/dpup/prefab/plugins/storage/memstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlugin(t *testing.T) {
	p := Plugin(
		WithSigningKey("test-signing-key"),
		WithExpiration(24*time.Hour),
	)
	require.NotNil(t, p)
	assert.Equal(t, PluginName, p.Name())
	assert.Equal(t, "test-signing-key", p.jwtSigningKey)
	assert.Equal(t, 24*time.Hour, p.jwtExpiration)
	assert.Len(t, p.identityExtractors, 2) // default extractors
}

func TestWithSigningKey(t *testing.T) {
	p := Plugin(
		WithSigningKey("custom-key"),
		WithExpiration(24*time.Hour),
	)
	assert.Equal(t, "custom-key", p.jwtSigningKey)
}

func TestWithExpiration(t *testing.T) {
	p := Plugin(
		WithExpiration(48 * time.Hour),
	)
	assert.Equal(t, 48*time.Hour, p.jwtExpiration)
}

func TestWithBlocklist(t *testing.T) {
	bl := NewBlocklist(memstore.New())
	p := Plugin(
		WithBlocklist(bl),
		WithExpiration(24*time.Hour),
	)
	assert.Equal(t, bl, p.blocklist)
}

func TestRandomSigningKey(t *testing.T) {
	key1 := randomSigningKey()
	key2 := randomSigningKey()

	// Keys should be hex-encoded (32 bytes = 64 hex chars)
	assert.Len(t, key1, 64)
	assert.Len(t, key2, 64)

	// Keys should be different (random)
	assert.NotEqual(t, key1, key2)
}

func TestAuthPluginName(t *testing.T) {
	p := Plugin(WithExpiration(24 * time.Hour))
	assert.Equal(t, "auth", p.Name())
}

func TestAuthPluginOptDeps(t *testing.T) {
	p := Plugin(WithExpiration(24 * time.Hour))
	deps := p.OptDeps()
	assert.Contains(t, deps, storage.PluginName)
}

func TestAuthPluginInit(t *testing.T) {
	t.Run("WithoutStorage", func(t *testing.T) {
		p := Plugin(WithExpiration(24 * time.Hour))
		registry := &prefab.Registry{}

		err := p.Init(t.Context(), registry)
		require.NoError(t, err)
		assert.Nil(t, p.blocklist) // No blocklist without storage
	})

	t.Run("WithStorage", func(t *testing.T) {
		p := Plugin(WithExpiration(24 * time.Hour))
		registry := &prefab.Registry{}

		// Register storage plugin
		store := memstore.New()
		storagePlugin := storage.Plugin(store)
		registry.Register(storagePlugin)

		// Need logger in context for Init
		ctx := logging.With(t.Context(), logging.NewDevLogger())
		err := p.Init(ctx, registry)
		require.NoError(t, err)
		assert.NotNil(t, p.blocklist) // Blocklist should be auto-created
	})

	t.Run("WithExistingBlocklist", func(t *testing.T) {
		bl := NewBlocklist(memstore.New())
		p := Plugin(
			WithBlocklist(bl),
			WithExpiration(24*time.Hour),
		)
		registry := &prefab.Registry{}

		// Even with storage, existing blocklist should be preserved
		store := memstore.New()
		storagePlugin := storage.Plugin(store)
		registry.Register(storagePlugin)

		ctx := logging.With(t.Context(), logging.NewDevLogger())
		err := p.Init(ctx, registry)
		require.NoError(t, err)
		assert.Equal(t, bl, p.blocklist) // Should keep existing blocklist
	})
}

func TestAuthPluginServerOptions(t *testing.T) {
	p := Plugin(
		WithSigningKey("test-key"),
		WithExpiration(24*time.Hour),
	)
	opts := p.ServerOptions()

	// Should return multiple server options
	assert.NotEmpty(t, opts)
	// Verify we have gRPC service, gateway, and config injectors
	assert.GreaterOrEqual(t, len(opts), 5)
}

func TestAddLoginHandler(t *testing.T) {
	p := Plugin(WithExpiration(24 * time.Hour))

	handler := func(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
		return &LoginResponse{}, nil
	}

	p.AddLoginHandler("test-provider", handler)
	assert.NotNil(t, p.authService.handlers)
	assert.Contains(t, p.authService.handlers, "test-provider")
}

func TestAddIdentityExtractor(t *testing.T) {
	p := Plugin(WithExpiration(24 * time.Hour))
	initialCount := len(p.identityExtractors)

	extractor := func(ctx context.Context) (Identity, error) {
		return Identity{}, ErrNotFound
	}

	p.AddIdentityExtractor(extractor)
	assert.Len(t, p.identityExtractors, initialCount+1)
}

func TestInjectBlocklist(t *testing.T) {
	t.Run("WithBlocklist", func(t *testing.T) {
		bl := NewBlocklist(memstore.New())
		p := Plugin(
			WithBlocklist(bl),
			WithExpiration(24*time.Hour),
		)

		ctx := p.injectBlocklist(t.Context())

		// Verify blocklist is in context
		extracted, ok := ctx.Value(blocklistKey{}).(Blocklist)
		require.True(t, ok)
		assert.Equal(t, bl, extracted)
	})

	t.Run("WithoutBlocklist", func(t *testing.T) {
		p := Plugin(WithExpiration(24 * time.Hour))
		ctx := p.injectBlocklist(t.Context())

		// Should return context unchanged
		_, ok := ctx.Value(blocklistKey{}).(Blocklist)
		assert.False(t, ok)
	})
}

func TestInjectIdentityExtractors(t *testing.T) {
	p := Plugin(WithExpiration(24 * time.Hour))
	ctx := p.injectIdentityExtractors(t.Context())

	// Verify extractors are in context
	extractors, ok := ctx.Value(identityExtractorsKey{}).([]IdentityExtractor)
	require.True(t, ok)
	assert.Len(t, extractors, 2) // default extractors
}
