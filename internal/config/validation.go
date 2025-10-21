package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/v2"
)

// ValidationWarning represents a configuration warning for unknown or potentially misspelled keys.
type ValidationWarning struct {
	Key         string
	Suggestions []string
}

func (w ValidationWarning) String() string {
	msg := fmt.Sprintf("'%s' is not a known config key", w.Key)
	if len(w.Suggestions) > 0 {
		if len(w.Suggestions) == 1 {
			msg += fmt.Sprintf(". Did you mean '%s'?", w.Suggestions[0])
		} else {
			msg += ". Did you mean one of these?\n"
			for _, suggestion := range w.Suggestions {
				msg += fmt.Sprintf("    - %s\n", suggestion)
			}
		}
	}
	return msg
}

// ValidateConfigKeys checks all loaded configuration keys against the registry
// and returns warnings for unknown keys with suggestions for similar keys.
//
// This validation uses Config.Keys() to enumerate all loaded keys from all sources
// (YAML files, environment variables, defaults, etc.) and compares them against
// the registered known keys.
func ValidateConfigKeys(config *koanf.Koanf) []ValidationWarning {
	loadedKeys := config.Keys()
	var warnings []ValidationWarning

	for _, key := range loadedKeys {
		// Check if this is a registered key
		if info, exists := LookupConfigKey(key); exists {
			// If it's deprecated, warn about it
			if info.Deprecated {
				warnings = append(warnings, ValidationWarning{
					Key:         key,
					Suggestions: []string{info.ReplacedBy},
				})
			}
			continue
		}

		// Check if this key starts with a registered prefix
		// This allows namespace flexibility (e.g., "myapp.*" keys won't warn if "myapp" is registered)
		if hasRegisteredPrefix(key) {
			continue
		}

		// Find similar keys to suggest
		suggestions := FindSimilarKeys(key, 3)

		warnings = append(warnings, ValidationWarning{
			Key:         key,
			Suggestions: suggestions,
		})
	}

	return warnings
}

// hasRegisteredPrefix checks if any registered key is a prefix of the given key.
// This allows applications to register namespace prefixes (like "myapp") without
// needing to register every possible sub-key.
func hasRegisteredPrefix(key string) bool {
	parts := strings.Split(key, ".")

	// Check progressively shorter prefixes
	// For "myapp.feature.setting", check "myapp.feature", then "myapp"
	for i := len(parts) - 1; i > 0; i-- {
		prefix := strings.Join(parts[:i], ".")
		if info, exists := LookupConfigKey(prefix); exists {
			// If the registered key has a wildcard or is explicitly a namespace prefix
			// we consider it valid. For now, we just check if it exists.
			// Future enhancement: add a "IsNamespace" flag to ConfigKeyInfo
			_ = info
			return true
		}
	}

	return false
}

// FormatValidationWarnings formats a slice of validation warnings into a readable message.
func FormatValidationWarnings(warnings []ValidationWarning) string {
	if len(warnings) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("⚠️  Configuration warnings detected:\n")
	for _, warning := range warnings {
		// Indent multi-line warnings
		lines := strings.Split(warning.String(), "\n")
		for i, line := range lines {
			if line == "" {
				continue
			}
			if i == 0 {
				sb.WriteString(fmt.Sprintf("  - %s\n", line))
			} else {
				sb.WriteString(fmt.Sprintf("    %s\n", line))
			}
		}
	}
	sb.WriteString("\nThese warnings indicate potential typos or unknown config keys.\n")
	sb.WriteString("To suppress warnings for custom application configs, register them with RegisterConfigKey().\n")
	return sb.String()
}
