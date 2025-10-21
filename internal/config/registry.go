package config

import (
	"sort"
	"strings"
	"sync"

	"github.com/agnivade/levenshtein"
)

// ConfigKeyInfo contains metadata about a known configuration key.
type ConfigKeyInfo struct {
	Key         string      // The full config key path (e.g., "server.port")
	Description string      // Human-readable description of what this config does
	Type        string      // Type hint: "string", "int", "bool", "duration", "[]string", etc.
	Default     interface{} // Optional default value
	Deprecated  bool        // If true, this key is deprecated
	ReplacedBy  string      // If deprecated, the new key to use instead
}

// registry holds all known configuration keys
var (
	registry   = make(map[string]ConfigKeyInfo)
	registryMu sync.RWMutex
)

// RegisterConfigKey registers a known configuration key with metadata.
// This should be called by core code and plugins to document expected config keys.
func RegisterConfigKey(info ConfigKeyInfo) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[info.Key] = info
}

// RegisterConfigKeys registers multiple configuration keys at once.
func RegisterConfigKeys(infos ...ConfigKeyInfo) {
	registryMu.Lock()
	defer registryMu.Unlock()
	for _, info := range infos {
		registry[info.Key] = info
	}
}

// RegisterDeprecatedKey registers a deprecated configuration key and its replacement.
func RegisterDeprecatedKey(oldKey, newKey string) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[oldKey] = ConfigKeyInfo{
		Key:        oldKey,
		Deprecated: true,
		ReplacedBy: newKey,
	}
}

// IsRegisteredKey checks if a config key is known in the registry.
func IsRegisteredKey(key string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, exists := registry[key]
	return exists
}

// LookupConfigKey returns metadata for a registered config key.
func LookupConfigKey(key string) (ConfigKeyInfo, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	info, exists := registry[key]
	return info, exists
}

// AllRegisteredKeys returns all registered config keys sorted alphabetically.
func AllRegisteredKeys() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// DefaultConfigs returns a map of all registered config keys with their default values.
// Only keys that have a non-nil Default value are included.
func DefaultConfigs() map[string]interface{} {
	registryMu.RLock()
	defer registryMu.RUnlock()

	defaults := make(map[string]interface{})
	for key, info := range registry {
		if info.Default != nil {
			defaults[key] = info.Default
		}
	}
	return defaults
}

// FindSimilarKeys finds registered keys that are similar to the given key.
// Returns up to maxResults keys sorted by similarity (most similar first).
//
// Uses a combination of:
// - Levenshtein distance for typo detection
// - Prefix matching for hierarchical keys
//
// With moderate thresholds:
// - Edit distance ≤ 3 for general matching
// - Edit distance ≤ 2 for keys with matching prefixes
func FindSimilarKeys(key string, maxResults int) []string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	type scored struct {
		key   string
		score int // Lower is better
	}

	var candidates []scored
	keyPrefix := getPrefix(key)

	for registeredKey := range registry {
		score := calculateSimilarity(key, registeredKey, keyPrefix)

		// Moderate threshold: Include if edit distance ≤ 3
		// OR if prefix matches and edit distance ≤ 2
		if score <= 3 {
			candidates = append(candidates, scored{registeredKey, score})
		}
	}

	// Sort by score (lower is more similar)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	// Return top N results
	result := make([]string, 0, maxResults)
	for i := 0; i < len(candidates) && i < maxResults; i++ {
		result = append(result, candidates[i].key)
	}

	return result
}

// calculateSimilarity returns a similarity score between two keys.
// Lower scores are more similar. Uses Levenshtein distance with prefix bonus.
func calculateSimilarity(key1, key2, key1Prefix string) int {
	distance := levenshtein.ComputeDistance(key1, key2)

	// Give a bonus (reduce score) if keys share the same prefix
	// This helps suggest keys in the same namespace
	key2Prefix := getPrefix(key2)
	if key1Prefix != "" && key1Prefix == key2Prefix {
		// Reduce distance for keys in same namespace
		if distance > 0 {
			distance--
		}
	}

	return distance
}

// getPrefix extracts the prefix of a hierarchical key.
// For "server.security.corsOrigins", returns "server.security"
func getPrefix(key string) string {
	lastDot := strings.LastIndex(key, ".")
	if lastDot == -1 {
		return ""
	}
	return key[:lastDot]
}

// HasRegisteredPrefix checks if any registered key starts with the given prefix.
// Used to allow unknown keys under registered namespaces (e.g., "myapp.*").
func HasRegisteredPrefix(key string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()

	// Check each level of the key hierarchy
	parts := strings.Split(key, ".")
	for i := 1; i < len(parts); i++ {
		prefix := strings.Join(parts[:i], ".")
		if _, exists := registry[prefix]; exists {
			return true
		}
	}
	return false
}
