package plugin

import (
	"fmt"
)

type entry struct {
	key    any
	plugin Plugin
	deps   []any
}

// Static registry used by actual plugins.
var globalRegistry = &Registry{}

// Init the global registry.
func Init() error {
	return globalRegistry.Init()
}

// Get a plugin registered globally.
func Get(key any) Plugin {
	return globalRegistry.Get(key)
}

// Register a global plugin.
func Register(key any, plugin Plugin, deps ...any) {
	globalRegistry.Register(key, plugin, deps)
}

// Registry manages plugins and their dependencies.
type Registry struct {
	plugins map[any]entry
}

// Get a plugin.
func (r *Registry) Get(key any) Plugin {
	if p, ok := r.plugins[key]; ok {
		return p.plugin
	}
	return nil
}

// Register a plugin.
func (r *Registry) Register(key any, plugin Plugin, deps ...any) {
	if r.plugins == nil {
		r.plugins = map[any]entry{}
	}
	r.plugins[key] = entry{key: key, plugin: plugin, deps: deps}
}

// Init all plugins in the registry, in dependency order.
func (r *Registry) Init() error {
	// TODO: Should this protect against being called twice?

	// Validate dependency graph first.
	visiting := make(map[any]bool)
	for key := range r.plugins {
		if err := r.validateDeps(key, visiting); err != nil {
			return err
		}
	}

	// Initialize plugins if graph is valid.
	initialized := make(map[any]bool)
	for key := range r.plugins {
		if err := r.initPlugin(key, initialized); err != nil {
			return err
		}
	}

	return nil
}

// Walks the plugin dependency graph and ensures deps are registered and that
// there are no cycles.
func (r *Registry) validateDeps(key any, visiting map[any]bool) error {
	if visiting[key] {
		return fmt.Errorf("plugin: dependency cycle detected involving %v", key)
	}

	visiting[key] = true
	entry, ok := r.plugins[key]
	if !ok {
		return fmt.Errorf("plugin: missing dependency, %v not registered", key)
	}

	for _, dep := range entry.deps {
		if err := r.validateDeps(dep, visiting); err != nil {
			return err
		}
	}

	delete(visiting, key)
	return nil
}

// Ensures plugins are initailized in dependency order.
func (r *Registry) initPlugin(key any, initialized map[any]bool) error {
	if initialized[key] {
		return nil
	}

	entry, ok := r.plugins[key]
	if !ok {
		return fmt.Errorf("plugin %v not registered", key)
	}

	for _, dep := range entry.deps {
		if err := r.initPlugin(dep, initialized); err != nil {
			return err
		}
	}

	if err := entry.plugin.Init(r); err != nil {
		return fmt.Errorf("plugin: failed to initialize %v: %w", key, err)
	}

	initialized[key] = true
	return nil
}
