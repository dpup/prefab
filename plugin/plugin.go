// Package plugin defines the plugin interface and the plugin registry.
package plugin

import "context"

type InitializablePlugin interface {
	// Init the plugin. Will be called in dependency order.
	Init(ctx context.Context, r *Registry) error
}
