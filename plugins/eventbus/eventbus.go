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

// Subscriber is a function type for event subscribers.
// Deprecated: Use Handler instead for new code.
type Subscriber func(context.Context, any) error

// Handler processes messages from the event bus.
type Handler func(context.Context, *Message) error

// Message wraps event data with metadata.
type Message struct {
	// ID uniquely identifies this message.
	ID string
	// Topic is the event/queue name.
	Topic string
	// Data is the message payload.
	Data any
	// Attempt is the delivery attempt number (1-based).
	Attempt int

	// ack is called to acknowledge successful processing.
	ack func()
	// nack is called to indicate processing failure.
	nack func()
}

// Ack acknowledges successful processing of the message.
func (m *Message) Ack() {
	if m.ack != nil {
		m.ack()
	}
}

// Nack indicates the message failed to process and should be redelivered.
func (m *Message) Nack() {
	if m.nack != nil {
		m.nack()
	}
}

// EventBus provides publish/subscribe and queue-based message delivery.
type EventBus interface {
	// Subscribe registers a handler that receives all published messages
	// on the topic (broadcast semantics).
	Subscribe(topic string, handler Handler)

	// Publish sends a message to all subscribers of the topic.
	Publish(topic string, data any)

	// SubscribeQueue registers a handler that competes with other queue
	// subscribers for messages (only one handler processes each message).
	SubscribeQueue(topic string, handler Handler)

	// Enqueue sends a message to exactly one queue subscriber.
	Enqueue(topic string, data any)

	// Wait blocks until all pending messages are processed.
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
	// If the underlying bus has a Shutdown method (like *Bus), call it
	// to close the worker pool gracefully
	if bus, ok := p.EventBus.(*Bus); ok {
		err := bus.Shutdown(ctx)
		if err == nil {
			logging.Info(ctx, "👍 Event bus drained")
		}
		return err
	}

	// Otherwise, just wait for completion (legacy behavior)
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
