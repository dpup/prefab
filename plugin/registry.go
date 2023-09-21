package plugin

import (
	"context"
	"fmt"
)

type entry struct {
	key    string
	plugin any
	deps   []string
}

// Registry manages plugins and their dependencies.
type Registry struct {
	plugins map[string]entry
}

// Get a plugin.
func (r *Registry) Get(key string) any {
	if p, ok := r.plugins[key]; ok {
		return p.plugin
	}
	return nil
}

// Register a plugin.
func (r *Registry) Register(key string, plugin any, deps ...string) {
	if r.plugins == nil {
		r.plugins = map[string]entry{}
	}
	r.plugins[key] = entry{key: key, plugin: plugin, deps: deps}
}

// Init all plugins in the registry, in dependency order.
func (r *Registry) Init(ctx context.Context) error {
	// TODO: Should this protect against being called twice?
	if r.plugins == nil {
		return nil
	}

	// Validate dependency graph first.
	visiting := make(map[string]bool)
	for key := range r.plugins {
		if err := r.validateDeps(key, visiting); err != nil {
			return err
		}
	}

	// Initialize plugins if graph is valid.
	initialized := make(map[any]bool)
	for key := range r.plugins {
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

// Ensures plugins are initialized in dependency order.
func (r *Registry) initPlugin(ctx context.Context, key string, initialized map[any]bool) error {
	if initialized[key] {
		return nil
	}

	entry, ok := r.plugins[key]
	if !ok {
		return fmt.Errorf("plugin %v not registered", key)
	}

	for _, dep := range entry.deps {
		if err := r.initPlugin(ctx, dep, initialized); err != nil {
			return err
		}
	}

	if p, ok := entry.plugin.(InitializablePlugin); ok {
		if err := p.Init(ctx, r); err != nil {
			return fmt.Errorf("plugin: failed to initialize %v: %w", key, err)
		}
	}

	initialized[key] = true
	return nil
}
