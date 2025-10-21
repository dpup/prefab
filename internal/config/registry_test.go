package config

import (
	"strings"
	"testing"

	"github.com/agnivade/levenshtein"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"hello", "hello", 0},
		{"", "hello", 5},
		{"hello", "", 5},
		{"corsAllowHeaders", "corsAllowedHeaders", 2}, // insert "e", "d"
		{"corsAllowMethods", "corsAllowedMethods", 2}, // insert "e", "d"
		{"test", "text", 1},                           // substitute 's' -> 'x'
		{"kitten", "sitting", 3},                      // classic example
	}

	for _, tt := range tests {
		result := levenshtein.ComputeDistance(tt.s1, tt.s2)
		if result != tt.expected {
			t.Errorf("levenshtein.ComputeDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, result, tt.expected)
		}
	}
}

func TestFindSimilarKeys(t *testing.T) {
	// Clear and populate registry for test
	registryMu.Lock()
	registry = make(map[string]ConfigKeyInfo)
	registry["server.security.corsAllowHeaders"] = ConfigKeyInfo{Key: "server.security.corsAllowHeaders"}
	registry["server.security.corsAllowMethods"] = ConfigKeyInfo{Key: "server.security.corsAllowMethods"}
	registry["server.security.corsOrigins"] = ConfigKeyInfo{Key: "server.security.corsOrigins"}
	registry["server.port"] = ConfigKeyInfo{Key: "server.port"}
	registry["auth.signingKey"] = ConfigKeyInfo{Key: "auth.signingKey"}
	registryMu.Unlock()

	tests := []struct {
		name           string
		key            string
		maxResults     int
		wantSuggestion string // Should include this suggestion
	}{
		{
			name:           "typo in corsAllowHeaders",
			key:            "server.security.corsAlowHeaders",
			maxResults:     3,
			wantSuggestion: "server.security.corsAllowHeaders",
		},
		{
			name:           "typo in corsAllowMethods",
			key:            "server.security.corsAlowMethods",
			maxResults:     3,
			wantSuggestion: "server.security.corsAllowMethods",
		},
		{
			name:           "completely different key",
			key:            "server.port",
			maxResults:     3,
			wantSuggestion: "", // Should not suggest anything for exact match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := FindSimilarKeys(tt.key, tt.maxResults)

			if tt.wantSuggestion == "" {
				// For exact matches, we might get some similar keys but we're not testing that
				return
			}

			found := false
			for _, result := range results {
				if result == tt.wantSuggestion {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("FindSimilarKeys(%q) = %v, want to include %q", tt.key, results, tt.wantSuggestion)
			}
		})
	}
}

func TestValidationWarningString(t *testing.T) {
	tests := []struct {
		name        string
		warning     ValidationWarning
		wantContain string
	}{
		{
			name: "single suggestion",
			warning: ValidationWarning{
				Key:         "server.security.corsAllowHeaders",
				Suggestions: []string{"server.security.corsAllowedHeaders"},
			},
			wantContain: "Did you mean 'server.security.corsAllowedHeaders'?",
		},
		{
			name: "multiple suggestions",
			warning: ValidationWarning{
				Key:         "server.prt",
				Suggestions: []string{"server.port", "server.host"},
			},
			wantContain: "Did you mean one of these?",
		},
		{
			name: "no suggestions",
			warning: ValidationWarning{
				Key:         "unknown.key",
				Suggestions: []string{},
			},
			wantContain: "'unknown.key' is not a known config key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.warning.String()
			if !strings.Contains(result, tt.wantContain) {
				t.Errorf("ValidationWarning.String() = %q, want to contain %q", result, tt.wantContain)
			}
		})
	}
}

func TestRegisterConfigKey(t *testing.T) {
	// Save and restore original registry
	registryMu.Lock()
	original := registry
	registry = make(map[string]ConfigKeyInfo)
	registryMu.Unlock()

	defer func() {
		registryMu.Lock()
		registry = original
		registryMu.Unlock()
	}()

	// Test registration
	info := ConfigKeyInfo{
		Key:         "test.key",
		Description: "Test key",
		Type:        "string",
	}
	RegisterConfigKey(info)

	// Verify it was registered
	if !IsRegisteredKey("test.key") {
		t.Error("RegisterConfigKey() failed to register key")
	}

	// Verify we can retrieve it
	retrieved, ok := LookupConfigKey("test.key")
	if !ok {
		t.Error("LookupConfigKey() failed to retrieve registered key")
	}
	if retrieved.Description != "Test key" {
		t.Errorf("LookupConfigKey() returned wrong info: got %q, want %q", retrieved.Description, "Test key")
	}
}

func TestGetPrefix(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"server.security.corsOrigins", "server.security"},
		{"server.port", "server"},
		{"simple", ""},
		{"one.two.three.four", "one.two.three"},
	}

	for _, tt := range tests {
		result := getPrefix(tt.key)
		if result != tt.expected {
			t.Errorf("getPrefix(%q) = %q, want %q", tt.key, result, tt.expected)
		}
	}
}
