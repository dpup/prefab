package google

import (
	"testing"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlugin(t *testing.T) {
	tests := []struct {
		name     string
		opts     []GoogleOption
		validate func(*testing.T, *GooglePlugin)
	}{
		{
			name: "default configuration from config file",
			opts: nil,
			validate: func(t *testing.T, p *GooglePlugin) {
				assert.NotNil(t, p)
				assert.Equal(t, PluginName, p.Name())
			},
		},
		{
			name: "with custom client",
			opts: []GoogleOption{
				WithClient("custom-id", "custom-secret"),
			},
			validate: func(t *testing.T, p *GooglePlugin) {
				assert.NotNil(t, p)
				assert.Equal(t, "custom-id", p.clientID)
				assert.Equal(t, "custom-secret", p.clientSecret)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin(tt.opts...)
			if tt.validate != nil {
				tt.validate(t, p)
			}
		})
	}
}

func TestGooglePlugin_Name(t *testing.T) {
	p := Plugin()
	assert.Equal(t, PluginName, p.Name())
}

func TestGooglePlugin_Deps(t *testing.T) {
	p := Plugin()
	deps := p.Deps()
	assert.Len(t, deps, 1)
	assert.Contains(t, deps, auth.PluginName)
}

func TestGooglePlugin_ServerOptions(t *testing.T) {
	p := Plugin(WithClient("test-id", "test-secret"))
	opts := p.ServerOptions()

	assert.NotEmpty(t, opts)
	// Should register callback handler and client config
	assert.GreaterOrEqual(t, len(opts), 2)
}

func TestGooglePlugin_Init(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name          string
		setupPlugin   func() *GooglePlugin
		expectedError string
	}{
		{
			name: "missing client id",
			setupPlugin: func() *GooglePlugin {
				return Plugin(WithClient("", "secret"))
			},
			expectedError: "google: config missing client id",
		},
		{
			name: "missing client secret",
			setupPlugin: func() *GooglePlugin {
				return Plugin(WithClient("id", ""))
			},
			expectedError: "google: config missing client secret",
		},
		{
			name: "successful initialization",
			setupPlugin: func() *GooglePlugin {
				return Plugin(WithClient("test-id", "test-secret"))
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authPlugin := auth.Plugin()
			registry := &prefab.Registry{}
			registry.Register(authPlugin)

			p := tt.setupPlugin()
			err := p.Init(ctx, registry)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWithClient(t *testing.T) {
	p := Plugin(WithClient("my-id", "my-secret"))
	assert.Equal(t, "my-id", p.clientID)
	assert.Equal(t, "my-secret", p.clientSecret)
}
