package plugin

import (
	"context"
	"fmt"
)

// Registry manages plugins and their dependencies.
type Registry struct {
	plugins map[string]Plugin
	keys    []string
}

// Get a plugin.
func (r *Registry) Get(key string) Plugin {
	if p, ok := r.plugins[key]; ok {
		return p
	}
	return nil
}

// Register a plugin.
func (r *Registry) Register(plugin Plugin) {
	if r.plugins == nil {
		r.plugins = map[string]Plugin{}
	}
	n := plugin.Name()
	r.plugins[n] = plugin
	r.keys = append(r.keys, n)
}

// Init all plugins in the registry. Plugins will be visited , in dependency order.
func (r *Registry) Init(ctx context.Context) error {
	// TODO: Should this protect against being called twice?

	if r.plugins == nil {
		return nil
	}

	// Validate dependency graph first.
	visiting := make(map[string]bool)
	for _, key := range r.keys {
		if err := r.validateDeps(key, visiting); err != nil {
			return err
		}
	}

	// Initialize plugins if graph is valid.
	initialized := make(map[string]bool)
	for _, key := range r.keys {
		if err := r.initPlugin(ctx, key, initialized); err != nil {
			return err
		}
	}

	return nil
}

// Walks the plugin dependency graph and ensures deps are registered and that
// there are no cycles.
func (r *Registry) validateDeps(key string, visiting map[string]bool) error {
	if visiting[key] {
		return fmt.Errorf("plugin: dependency cycle detected involving %v", key)
	}

	visiting[key] = true
	plugin, ok := r.plugins[key]
	if !ok {
		return fmt.Errorf("plugin: missing dependency, %v not registered", key)
	}

	if d, ok := plugin.(DependentPlugin); ok {
		for _, dep := range d.Deps() {
			if err := r.validateDeps(dep, visiting); err != nil {
				return err
			}
		}
	}

	delete(visiting, key)
	return nil
}

// Ensures plugins are initialized in dependency order.
func (r *Registry) initPlugin(ctx context.Context, key string, initialized map[string]bool) error {
	if initialized[key] {
		return nil
	}

	plugin, ok := r.plugins[key]
	if !ok {
		return fmt.Errorf("plugin %v not registered", key)
	}

	if d, ok := plugin.(DependentPlugin); ok {
		for _, dep := range d.Deps() {
			if err := r.initPlugin(ctx, dep, initialized); err != nil {
				return err
			}
		}
	}

	if p, ok := plugin.(InitializablePlugin); ok {
		if err := p.Init(ctx, r); err != nil {
			return fmt.Errorf("plugin: failed to initialize %v: %w", key, err)
		}
	}

	initialized[key] = true
	return nil
}
