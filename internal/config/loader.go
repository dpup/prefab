package config

import (
	"sync"

	"github.com/knadh/koanf/v2"
)

var defaultsLoaded sync.Once

// EnsureDefaultsLoaded loads config defaults if not already loaded.
// Only sets default values for keys that don't already exist in the config.
//
// This should be called after all plugins have registered their config keys
// (typically in the server builder, after all init() functions have run).
// Thread-safe - uses sync.Once to ensure defaults are loaded exactly once.
func EnsureDefaultsLoaded(k *koanf.Koanf) {
	defaultsLoaded.Do(func() {
		defaults := DefaultConfigs()
		for key, val := range defaults {
			if !k.Exists(key) {
				k.Set(key, val)
			}
		}
	})
}
