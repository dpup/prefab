// Package eventbus provides a simple publish/subscribe event bus. Plugins and
// components can optionally use this to communicate with each other.
package eventbus

import (
	"context"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/logging"
)

const (
	// PluginName identifies this plugin.
	PluginName = "eventbus"
)

// Subscriber is a function type for event subscribers.
// Deprecated: Use Handler instead.
type Subscriber func(context.Context, any) error

// Handler processes messages from the event bus.
type Handler func(context.Context, *Message) error

// Message wraps event data with metadata.
type Message struct {
	ID      string // Unique identifier
	Topic   string // Topic name
	Data    any    // Payload
	Attempt int    // Delivery attempt (1-based)

	ack  func() // Called on successful processing
	nack func() // Called on processing failure
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

// NewMessage creates a message with default no-op ack/nack functions.
func NewMessage(id, topic string, data any) *Message {
	return &Message{
		ID:      id,
		Topic:   topic,
		Data:    data,
		Attempt: 1,
		ack:     func() {},
		nack:    func() {},
	}
}

// NewMessageWithCallbacks creates a message with custom ack/nack callbacks.
func NewMessageWithCallbacks(id, topic string, data any, attempt int, ack, nack func()) *Message {
	return &Message{
		ID:      id,
		Topic:   topic,
		Data:    data,
		Attempt: attempt,
		ack:     ack,
		nack:    nack,
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

	// Wait blocks until locally-initiated operations complete. For in-memory
	// implementations, this means all handlers have finished. For distributed
	// implementations, this means messages have been sent to the remote system.
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

// Shutdownable is implemented by EventBus implementations that need graceful shutdown.
type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

// From prefab.ShutdownPlugin.
func (p *EventBusPlugin) Shutdown(ctx context.Context) error {
	// If the bus implements Shutdownable, use that for graceful shutdown
	if bus, ok := p.EventBus.(Shutdownable); ok {
		err := bus.Shutdown(ctx)
		if err == nil {
			logging.Info(ctx, "Event bus drained")
		}
		return err
	}

	// Otherwise, just wait for completion
	err := p.Wait(ctx)
	if err == nil {
		logging.Info(ctx, "Event bus drained")
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
