package config

import (
	"strings"
	"testing"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
)

func TestValidateConfigKeys_Integration(t *testing.T) {
	// Create a new config instance for this test
	testConfig := koanf.New(".")

	// Load test config with intentional typos
	err := testConfig.Load(confmap.Provider(map[string]interface{}{
		"server.host":                         "localhost",
		"server.port":                         8000,
		"server.security.corsAlowHeaders":     []string{"x-test"}, // Typo: missing one 'l'
		"server.security.corsAlowMethods":     []string{"GET"},    // Typo: missing one 'l'
		"auth.expiration":                     "24h",
		"auth.signngKey":                      "test",             // Typo: should be signingKey
		"unknownKey":                          "value",
		"myapp.customKey":                     "value",                               // Unknown app key
		"server.security.corsAllowHeaders_v2": []string{"x-custom"}, // Similar to known key
	}, "."), nil)

	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	// Ensure key registration has happened (init() should have run)
	if len(AllRegisteredKeys()) < 10 {
		t.Skip("Skipping test: not enough registered keys (init() may not have run)")
	}

	// Run validation
	warnings := ValidateConfigKeys(testConfig)

	// We should have warnings for the typos
	if len(warnings) == 0 {
		t.Fatal("Expected warnings but got none")
	}

	// Check that we got warnings for the typos
	foundCorsAlowHeaders := false
	foundCorsAlowMethods := false
	foundSignngKey := false

	for _, w := range warnings {
		t.Logf("Warning: %s", w.String())

		// Check for the typo warnings
		if w.Key == "server.security.corsAlowHeaders" {
			foundCorsAlowHeaders = true
			// Should suggest the correct key
			if len(w.Suggestions) == 0 {
				t.Error("Expected suggestions for corsAlowHeaders typo")
			}
			// Check if correct key is in suggestions
			hasSuggestion := false
			for _, s := range w.Suggestions {
				if s == "server.security.corsAllowHeaders" {
					hasSuggestion = true
					break
				}
			}
			if !hasSuggestion {
				t.Errorf("Expected corsAllowHeaders in suggestions, got %v", w.Suggestions)
			}
		}

		if w.Key == "server.security.corsAlowMethods" {
			foundCorsAlowMethods = true
		}

		if w.Key == "auth.signngKey" {
			foundSignngKey = true
			// Should suggest signingKey
			hasSuggestion := false
			for _, s := range w.Suggestions {
				if s == "auth.signingKey" {
					hasSuggestion = true
					break
				}
			}
			if !hasSuggestion {
				t.Errorf("Expected auth.signingKey in suggestions for signngKey typo, got %v", w.Suggestions)
			}
		}
	}

	if !foundCorsAlowHeaders {
		t.Error("Expected warning for server.security.corsAlowHeaders typo")
	}
	if !foundCorsAlowMethods {
		t.Error("Expected warning for server.security.corsAlowMethods typo")
	}
	if !foundSignngKey {
		t.Error("Expected warning for auth.signngKey typo")
	}

	// Test that known keys don't generate warnings
	testConfig2 := koanf.New(".")
	err = testConfig2.Load(confmap.Provider(map[string]interface{}{
		"server.host":                          "localhost",
		"server.port":                          8000,
		"server.security.corsAllowHeaders":     []string{"x-test"}, // Correct key
		"server.security.corsAllowMethods":     []string{"GET"},    // Correct key
		"server.security.corsOrigins":          []string{"*"},      // Correct key
		"auth.expiration":                      "24h",
		"server.security.corsAllowCredentials": true,
	}, "."), nil)

	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	warnings = ValidateConfigKeys(testConfig2)

	// Should have no warnings for correct keys
	if len(warnings) > 0 {
		t.Errorf("Expected no warnings for correct config keys, but got %d warnings:", len(warnings))
		for _, w := range warnings {
			t.Logf("  - %s", w.String())
		}
	}
}

func TestFormatValidationWarnings(t *testing.T) {
	warnings := []ValidationWarning{
		{
			Key:         "server.security.corsAllowHeaders",
			Suggestions: []string{"server.security.corsAllowedHeaders"},
		},
		{
			Key:         "unknownKey",
			Suggestions: []string{},
		},
	}

	result := FormatValidationWarnings(warnings)

	// Should contain the warning emoji
	if !strings.Contains(result, "⚠️") {
		t.Error("Expected warning emoji in formatted output")
	}

	// Should mention the keys
	if !strings.Contains(result, "server.security.corsAllowHeaders") {
		t.Error("Expected formatted output to mention corsAllowHeaders")
	}

	// Should have instructions
	if !strings.Contains(result, "RegisterConfigKey") {
		t.Error("Expected formatted output to mention RegisterConfigKey")
	}

	t.Logf("Formatted warnings:\n%s", result)
}
