package prefab

import (
	"testing"
	"time"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigMustString(t *testing.T) {
	t.Run("returns value when key exists and is non-empty", func(t *testing.T) {
		// Save original config
		originalConfig := Config
		defer func() { Config = originalConfig }()

		// Create test config
		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"test.key": "test-value",
		}, "."), nil)

		value := ConfigMustString("test.key", "help message")
		assert.Equal(t, "test-value", value)
	})

	t.Run("panics when key doesn't exist", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")

		assert.Panics(t, func() {
			ConfigMustString("missing.key", "set the key")
		})
	})

	t.Run("panics when value is empty", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"test.key": "",
		}, "."), nil)

		assert.Panics(t, func() {
			ConfigMustString("test.key", "help message")
		})
	})
}

func TestConfigMustInt(t *testing.T) {
	t.Run("returns value when in range", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"test.port": 8080,
		}, "."), nil)

		value := ConfigMustInt("test.port", 1, 65535)
		assert.Equal(t, 8080, value)
	})

	t.Run("panics when value below min", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"test.port": 0,
		}, "."), nil)

		assert.Panics(t, func() {
			ConfigMustInt("test.port", 1, 65535)
		})
	})

	t.Run("panics when value above max", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"test.port": 70000,
		}, "."), nil)

		assert.Panics(t, func() {
			ConfigMustInt("test.port", 1, 65535)
		})
	})

	t.Run("panics when key doesn't exist", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")

		assert.Panics(t, func() {
			ConfigMustInt("missing.key", 1, 100)
		})
	})
}

func TestConfigMustDurationRange(t *testing.T) {
	t.Run("returns value when in range", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"test.timeout": "30s",
		}, "."), nil)

		value := ConfigMustDurationRange("test.timeout", time.Second, time.Minute)
		assert.Equal(t, 30*time.Second, value)
	})

	t.Run("panics when value below min", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"test.timeout": "500ms",
		}, "."), nil)

		assert.Panics(t, func() {
			ConfigMustDurationRange("test.timeout", time.Second, time.Minute)
		})
	})

	t.Run("panics when value above max", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"test.timeout": "2m",
		}, "."), nil)

		assert.Panics(t, func() {
			ConfigMustDurationRange("test.timeout", time.Second, time.Minute)
		})
	})
}

func TestValidateIntRange(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		min     int
		max     int
		wantErr bool
	}{
		{"value at min", 1, 1, 100, false},
		{"value at max", 100, 1, 100, false},
		{"value in middle", 50, 1, 100, false},
		{"value below min", 0, 1, 100, true},
		{"value above max", 101, 1, 100, true},
		{"negative value", -5, 1, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIntRange(tt.value, tt.min, tt.max)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"port 1", 1, false},
		{"port 80", 80, false},
		{"port 8080", 8080, false},
		{"port 65535", 65535, false},
		{"port 0", 0, true},
		{"negative port", -1, true},
		{"port 65536", 65536, true},
		{"port 100000", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePositiveInt(t *testing.T) {
	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{"positive value", 10, false},
		{"value 1", 1, false},
		{"zero", 0, true},
		{"negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositiveInt(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePositiveDuration(t *testing.T) {
	tests := []struct {
		name    string
		value   time.Duration
		wantErr bool
	}{
		{"positive duration", time.Second, false},
		{"large duration", 24 * time.Hour, false},
		{"zero", 0, true},
		{"negative", -time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePositiveDuration(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNonNegativeDuration(t *testing.T) {
	tests := []struct {
		name    string
		value   time.Duration
		wantErr bool
	}{
		{"positive duration", time.Second, false},
		{"zero duration", 0, false},
		{"negative duration", -time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonNegativeDuration(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http URL", "http://example.com", false},
		{"valid https URL", "https://example.com/path", false},
		{"valid URL with port", "https://example.com:8080", false},
		{"empty string", "", true},
		{"no scheme", "example.com", true},
		{"no host", "http://", true},
		{"invalid URL", "ht!tp://example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNonEmpty(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"non-empty string", "value", false},
		{"whitespace", "   ", false}, // Only checks emptiness, not whitespace
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNonEmpty(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	t.Run("returns no errors for valid config", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"server.port":                    8080,
			"server.host":                    "localhost",
			"server.maxMsgSizeBytes":         1024,
			"server.security.hstsExpiration": "30d",
			"server.security.corsMaxAge":     "1h",
			"auth.expiration":                "24h",
		}, "."), nil)

		errors := ValidateConfig()
		assert.Empty(t, errors)
	})

	t.Run("returns errors for invalid port", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"server.port": 70000,
		}, "."), nil)

		errors := ValidateConfig()
		require.Len(t, errors, 1)
		assert.Equal(t, "server.port", errors[0].Key)
		assert.Contains(t, errors[0].Message, "must be between 1 and 65535")
	})

	t.Run("returns errors for empty host", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"server.host": "",
		}, "."), nil)

		errors := ValidateConfig()
		require.Len(t, errors, 1)
		assert.Equal(t, "server.host", errors[0].Key)
	})

	t.Run("returns errors for invalid maxMsgSizeBytes", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"server.maxMsgSizeBytes": -100,
		}, "."), nil)

		errors := ValidateConfig()
		require.Len(t, errors, 1)
		assert.Equal(t, "server.maxMsgSizeBytes", errors[0].Key)
	})

	t.Run("returns errors for invalid auth expiration", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"auth.expiration": "-1h",
		}, "."), nil)

		errors := ValidateConfig()
		require.Len(t, errors, 1)
		assert.Equal(t, "auth.expiration", errors[0].Key)
	})

	t.Run("aggregates multiple errors", func(t *testing.T) {
		originalConfig := Config
		defer func() { Config = originalConfig }()

		Config = koanf.New(".")
		Config.Load(confmap.Provider(map[string]interface{}{
			"server.port":     0,
			"server.host":     "",
			"auth.expiration": "-1h",
		}, "."), nil)

		errors := ValidateConfig()
		assert.Len(t, errors, 3)
	})
}

func TestFormatValidationErrors(t *testing.T) {
	t.Run("returns empty string for no errors", func(t *testing.T) {
		result := FormatValidationErrors(nil)
		assert.Empty(t, result)

		result = FormatValidationErrors([]ValidationError{})
		assert.Empty(t, result)
	})

	t.Run("formats single error", func(t *testing.T) {
		errors := []ValidationError{
			{Key: "server.port", Message: "must be between 1 and 65535, got: 70000"},
		}

		result := FormatValidationErrors(errors)
		assert.Contains(t, result, "Configuration validation failed")
		assert.Contains(t, result, "server.port: must be between 1 and 65535, got: 70000")
		assert.Contains(t, result, "Fix these errors")
	})

	t.Run("formats multiple errors", func(t *testing.T) {
		errors := []ValidationError{
			{Key: "server.port", Message: "invalid"},
			{Key: "server.host", Message: "cannot be empty"},
		}

		result := FormatValidationErrors(errors)
		assert.Contains(t, result, "server.port")
		assert.Contains(t, result, "server.host")
	})
}
