// Package plugin defines the plugin interface and the plugin registry.
package plugin

import "context"

// The base plugin interface.
type Plugin interface {
	// Name of the plugin, used for querying and dependency resolution.
	Name() string
}

// Implemented if plugin depends on other plugins.
type DependentPlugin interface {
	// Deps returns the names for plugins which this plugin depends on.
	Deps() []string
}

// Implemented if plugin has optional dependencies, which should be initialized
// before the plugin, but are not required.
type OptionalDependentPlugin interface {
	// OptDeps returns the names for plugins which this plugin optionally depends on.
	OptDeps() []string
}

// Implemented if the plugin needs to be initialized outside construction.
type InitializablePlugin interface {
	// Init the plugin. Will be called in dependency order.
	Init(ctx context.Context, r *Registry) error
}
