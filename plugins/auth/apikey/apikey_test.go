package apikey

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestPlugin(t *testing.T) {
	tests := []struct {
		name           string
		opts           []APIOption
		expectedPrefix string
	}{
		{
			name:           "default configuration",
			opts:           nil,
			expectedPrefix: "pak",
		},
		{
			name: "custom key prefix",
			opts: []APIOption{
				WithKeyPrefix("custom"),
			},
			expectedPrefix: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin(tt.opts...)
			assert.NotNil(t, p)
			assert.Equal(t, PluginName, p.Name())
			assert.Equal(t, tt.expectedPrefix, p.keyPrefix)
		})
	}
}

func TestAPIPlugin_Name(t *testing.T) {
	p := Plugin()
	assert.Equal(t, PluginName, p.Name())
}

func TestAPIPlugin_Deps(t *testing.T) {
	p := Plugin()
	deps := p.Deps()
	assert.Len(t, deps, 1)
	assert.Contains(t, deps, auth.PluginName)
}

func TestAPIPlugin_Init(t *testing.T) {
	ctx := t.Context()

	// Create a mock auth plugin
	authPlugin := auth.Plugin()

	// Create a mock registry
	registry := &prefab.Registry{}
	registry.Register(authPlugin)

	// Create apikey plugin with a key function
	keyFunc := func(ctx context.Context, key string) (*KeyOwner, error) {
		return &KeyOwner{
			UserID:        "user123",
			Email:         "test@example.com",
			EmailVerified: true,
			Name:          "Test User",
			KeyCreatedAt:  time.Now(),
		}, nil
	}

	apikeyPlugin := Plugin(WithKeyFunc(keyFunc))

	// Initialize the apikey plugin and just check no errors
	err := apikeyPlugin.Init(ctx, registry)
	require.NoError(t, err)
}

func TestAPIPlugin_NewKey(t *testing.T) {
	tests := []struct {
		name      string
		keyPrefix string
	}{
		{
			name:      "default prefix",
			keyPrefix: "pak",
		},
		{
			name:      "custom prefix",
			keyPrefix: "myapi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin(WithKeyPrefix(tt.keyPrefix))
			key := p.NewKey()

			assert.NotEmpty(t, key)
			assert.Contains(t, key, tt.keyPrefix+"_")

			// Key should be prefix + _ + 64 hex characters (32 bytes)
			expectedLen := len(tt.keyPrefix) + 1 + 64
			assert.Len(t, key, expectedLen)
		})
	}
}

func TestAPIPlugin_fetchIdentity(t *testing.T) {
	keyCreatedAt := time.Now().Add(-24 * time.Hour)

	tests := []struct {
		name          string
		setupContext  func() context.Context
		keyFunc       KeyFunc
		keyPrefix     string
		expectedError bool
		expectedCode  codes.Code
		validateID    func(*testing.T, auth.Identity)
	}{
		{
			name: "no authorization header",
			setupContext: func() context.Context {
				return t.Context()
			},
			keyFunc: func(ctx context.Context, key string) (*KeyOwner, error) {
				t.Fatal("keyFunc should not be called")
				return nil, errors.New("should not be called")
			},
			expectedError: true,
			expectedCode:  codes.Unauthenticated,
		},
		{
			name: "empty authorization header",
			setupContext: func() context.Context {
				md := metadata.New(map[string]string{"authorization": ""})
				return metadata.NewIncomingContext(t.Context(), md)
			},
			keyFunc: func(ctx context.Context, key string) (*KeyOwner, error) {
				t.Fatal("keyFunc should not be called")
				return nil, errors.New("should not be called")
			},
			expectedError: true,
			expectedCode:  codes.Unauthenticated,
		},
		{
			name: "wrong prefix - bearer token",
			setupContext: func() context.Context {
				md := metadata.New(map[string]string{"authorization": "Bearer some-jwt-token"})
				return metadata.NewIncomingContext(t.Context(), md)
			},
			keyFunc: func(ctx context.Context, key string) (*KeyOwner, error) {
				t.Fatal("keyFunc should not be called")
				return nil, errors.New("should not be called")
			},
			keyPrefix:     "pak",
			expectedError: true,
			expectedCode:  codes.Unauthenticated,
		},
		{
			name: "wrong prefix - different api key",
			setupContext: func() context.Context {
				md := metadata.New(map[string]string{"authorization": "other_abc123"})
				return metadata.NewIncomingContext(t.Context(), md)
			},
			keyFunc: func(ctx context.Context, key string) (*KeyOwner, error) {
				t.Fatal("keyFunc should not be called")
				return nil, errors.New("should not be called")
			},
			keyPrefix:     "pak",
			expectedError: true,
			expectedCode:  codes.Unauthenticated,
		},
		{
			name: "valid key - successful lookup",
			setupContext: func() context.Context {
				md := metadata.New(map[string]string{"authorization": "pak_validkey123"})
				return metadata.NewIncomingContext(t.Context(), md)
			},
			keyFunc: func(ctx context.Context, key string) (*KeyOwner, error) {
				assert.Equal(t, "validkey123", key)
				return &KeyOwner{
					UserID:        "user123",
					Email:         "test@example.com",
					EmailVerified: true,
					Name:          "Test User",
					KeyCreatedAt:  keyCreatedAt,
				}, nil
			},
			keyPrefix:     "pak",
			expectedError: false,
			validateID: func(t *testing.T, id auth.Identity) {
				assert.Equal(t, "user123", id.Subject)
				assert.Equal(t, "test@example.com", id.Email)
				assert.Equal(t, "Test User", id.Name)
				assert.True(t, id.EmailVerified)
				assert.Equal(t, ProviderName, id.Provider)
				assert.Equal(t, keyCreatedAt, id.AuthTime)
			},
		},
		{
			name: "valid key - lookup returns not found",
			setupContext: func() context.Context {
				md := metadata.New(map[string]string{"authorization": "pak_invalidkey"})
				return metadata.NewIncomingContext(t.Context(), md)
			},
			keyFunc: func(ctx context.Context, key string) (*KeyOwner, error) {
				assert.Equal(t, "invalidkey", key)
				return nil, status.Error(codes.NotFound, "key not found")
			},
			keyPrefix:     "pak",
			expectedError: true,
			expectedCode:  codes.Unauthenticated,
		},
		{
			name: "valid key - lookup returns internal error",
			setupContext: func() context.Context {
				md := metadata.New(map[string]string{"authorization": "pak_errorkey"})
				return metadata.NewIncomingContext(t.Context(), md)
			},
			keyFunc: func(ctx context.Context, key string) (*KeyOwner, error) {
				assert.Equal(t, "errorkey", key)
				return nil, status.Error(codes.Internal, "database error")
			},
			keyPrefix:     "pak",
			expectedError: true,
			expectedCode:  codes.Unauthenticated,
		},
		{
			name: "custom prefix",
			setupContext: func() context.Context {
				md := metadata.New(map[string]string{"authorization": "myapi_customkey"})
				return metadata.NewIncomingContext(t.Context(), md)
			},
			keyFunc: func(ctx context.Context, key string) (*KeyOwner, error) {
				assert.Equal(t, "customkey", key)
				return &KeyOwner{
					UserID:        "custom-user",
					Email:         "custom@example.com",
					EmailVerified: false,
					Name:          "Custom User",
					KeyCreatedAt:  keyCreatedAt,
				}, nil
			},
			keyPrefix:     "myapi",
			expectedError: false,
			validateID: func(t *testing.T, id auth.Identity) {
				assert.Equal(t, "custom-user", id.Subject)
				assert.Equal(t, "custom@example.com", id.Email)
				assert.Equal(t, "Custom User", id.Name)
				assert.False(t, id.EmailVerified)
				assert.Equal(t, ProviderName, id.Provider)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []APIOption{WithKeyFunc(tt.keyFunc)}
			if tt.keyPrefix != "" {
				opts = append(opts, WithKeyPrefix(tt.keyPrefix))
			}

			p := Plugin(opts...)
			ctx := tt.setupContext()

			identity, err := p.fetchIdentity(ctx)

			if tt.expectedError {
				require.Error(t, err)
				if tt.expectedCode != 0 {
					st, ok := status.FromError(err)
					require.True(t, ok, "error should be a gRPC status error")
					assert.Equal(t, tt.expectedCode, st.Code())
				}
			} else {
				require.NoError(t, err)
				if tt.validateID != nil {
					tt.validateID(t, identity)
				}
			}
		})
	}
}

func TestWithKeyFunc(t *testing.T) {
	called := false
	keyFunc := func(ctx context.Context, key string) (*KeyOwner, error) {
		called = true
		return nil, errors.New("should not be called")
	}

	p := Plugin(WithKeyFunc(keyFunc))
	assert.NotNil(t, p.keyOwnerFunc)

	// Verify the function was set by calling it
	_, _ = p.keyOwnerFunc(t.Context(), "test")
	assert.True(t, called)
}

func TestWithKeyPrefix(t *testing.T) {
	p := Plugin(WithKeyPrefix("custom"))
	assert.Equal(t, "custom", p.keyPrefix)
}
