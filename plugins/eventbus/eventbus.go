// Package eventbus provides a simple publish/subscribe event bus. Plugins and
// components can optionally use this to communicate with each other.
package eventbus

import (
	"context"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/logging"
)

const (
	// Constant name for identifying the eventbus plugin.
	PluginName = "eventbus"
)

// Function type for event subscribers.
type Subscriber func(context.Context, any) error

// EventBus provides a simple publish/subscribe interface for publishing and
// subscribing to events.
type EventBus interface {
	// Subscribe to an event. The handler will be called when the event is
	// published. Depending on the implementation errors may be logged or retried.
	// Subscribers should assume that they may be called multiple times
	// concurrently.
	Subscribe(event string, subscriber Subscriber)

	// Publish an event. The event will be sent to all subscribers.
	Publish(event string, data any)

	// Wait for the event bus to finish processing all events. You should ensure
	// that publishers are also stopped as the event bus won't reject new events.
	Wait(ctx context.Context) error
}

// Plugin registers an eventbus with a Prefab server for use by other plugins
// to use. The bus can be retrieved from the request context using the
// FromContext function.
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

// From prefab.Plugin.
func (p *EventBusPlugin) Name() string {
	return PluginName
}

// From prefab.OptionProvider.
func (p *EventBusPlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithRequestConfig(p.inject),
	}
}

// From prefab.ShutdownPlugin.
func (p *EventBusPlugin) Shutdown(ctx context.Context) error {
	err := p.Wait(ctx)
	if err == nil {
		logging.Info(ctx, "👍 Event bus drained")
	}
	return err
}

func (p *EventBusPlugin) inject(ctx context.Context) context.Context {
	return context.WithValue(ctx, eventBusKey{}, p)
}

// FromContext retrieves the event bus from a context.
func FromContext(ctx context.Context) EventBus {
	if p, ok := ctx.Value(eventBusKey{}).(*EventBusPlugin); ok {
		return p.EventBus
	}
	return nil
}

type eventBusKey struct{}
