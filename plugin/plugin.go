// Package plugin defines the plugin interface and the plugin registry.
package plugin

type Plugin interface {
	// Init the plugin. Will be called in dependency order.
	Init(r *Registry) error
}
