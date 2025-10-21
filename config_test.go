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

	// Test specific defaults
	tests := []struct {
		key      string
		expected interface{}
	}{
		{"name", "Prefab Server"},
		{"server.host", defaultHost},
		{"server.port", defaultPort},
		{"auth.expiration", "24h"},
		{"upload.path", "/upload"},
		{"upload.downloadPrefix", "/download"},
		{"upload.maxFiles", 10},
		{"upload.maxMemory", 4 << 20},
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

	// Test that validTypes slice is populated
	if validTypes, ok := defaults["upload.validTypes"].([]string); ok {
		if len(validTypes) == 0 {
			t.Error("upload.validTypes should not be empty")
		}
	} else {
		t.Error("upload.validTypes should be a []string")
	}
}

func TestConfigLoadedDefaults(t *testing.T) {
	// Test that defaults are actually loaded into Config
	// Note: Some values may be overridden by prefab.yaml, so we test
	// values that are less likely to be in the YAML file

	if ConfigString("name") == "" {
		t.Error("Config name should not be empty (should have default)")
	}

	if ConfigString("server.port") == "" {
		t.Error("Config server.port should not be empty (should have default)")
	}

	// These are less likely to be overridden by prefab.yaml
	if ConfigString("upload.path") != "/upload" {
		t.Errorf("Config upload.path = %q, want %q", ConfigString("upload.path"), "/upload")
	}

	if ConfigString("upload.downloadPrefix") != "/download" {
		t.Errorf("Config upload.downloadPrefix = %q, want %q", ConfigString("upload.downloadPrefix"), "/download")
	}

	if ConfigInt("upload.maxFiles") != 10 {
		t.Errorf("Config upload.maxFiles = %d, want %d", ConfigInt("upload.maxFiles"), 10)
	}
}
