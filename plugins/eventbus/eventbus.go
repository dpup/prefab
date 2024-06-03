// Package eventbus provides a simple publish/subscribe event bus. Plugins and
// components can optionally use this to communicate with each other.
package eventbus

import (
	"context"
	"time"
)

// Constant name for identifying the eventbus plugin.
const PluginName = "eventbus"

// Function type for event subscribers.
type Subscriber func(context.Context, any) error

// Plugin registers an eventbus with a Prefab server for use by other plugins
// to use.
func Plugin(eb EventBus) *EventBusPlugin {
	p := &EventBusPlugin{
		EventBus: eb,
	}
	return p
}

// EventBusPlugin provides access to an event bus for plugins and components to
// communicate with each other.
type EventBusPlugin struct {
	EventBus
}

// From prefab.Plugin
func (p *EventBusPlugin) Name() string {
	return PluginName
}

// EventBus provides a simple publish/subscribe interface for publishing and
// subscribing to events.
type EventBus interface {
	// Subscribe to an event. The handler will be called when the event is
	// published. Depending on the implementation errors may be logged or retried.
	// Subscribers should assume that they may be called multiple times
	// concurrently.
	Subscribe(event string, subscriber Subscriber)

	// Publish an event. The event will be sent to all subscribers.
	Publish(ctx context.Context, event string, data any)

	// Wait for the event bus to finish processing all events. You should ensure
	// that publishers are also stopped as the event bus won't reject new events.
	Wait(ctx context.Context, timeout time.Duration) error
}
