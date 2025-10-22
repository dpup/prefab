package prefab

import (
	"testing"

	"github.com/dpup/prefab/internal/config"
)

func TestDefaultConfigs(t *testing.T) {
	defaults := config.DefaultConfigs()

	// Test that defaults are populated
	if len(defaults) == 0 {
		t.Error("DefaultConfigs() returned empty map")
	}

	// Test specific core defaults (not plugin defaults to avoid import cycles)
	tests := []struct {
		key      string
		expected interface{}
	}{
		{"name", "Prefab Server"},
		{"server.host", defaultHost},
		{"server.port", defaultPort},
	}

	for _, tt := range tests {
		value, exists := defaults[tt.key]
		if !exists {
			t.Errorf("Expected default for %q but it was not found", tt.key)
			continue
		}
		if value != tt.expected {
			t.Errorf("Default for %q = %v, want %v", tt.key, value, tt.expected)
		}
	}
}

func TestConfigLoadedDefaults(t *testing.T) {
	// Test that defaults are actually loaded into Config after EnsureDefaultsLoaded is called.
	// Note: Defaults are loaded lazily in builder.New(), so we call EnsureDefaultsLoaded directly.
	config.EnsureDefaultsLoaded(Config)

	// Some values may be overridden by prefab.yaml, so we test
	// values that are less likely to be in the YAML file

	if ConfigString("name") == "" {
		t.Error("Config name should not be empty (should have default)")
	}

	if ConfigString("server.port") == "" {
		t.Error("Config server.port should not be empty (should have default)")
	}
}
