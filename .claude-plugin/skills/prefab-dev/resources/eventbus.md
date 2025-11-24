# Event Bus

The eventbus plugin provides a simple publish/subscribe system for inter-plugin communication.

## Setup

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/eventbus"
)

func main() {
    ctx := context.Background()
    bus := eventbus.NewBus(ctx)

    s := prefab.New(
        prefab.WithPlugin(eventbus.Plugin(bus)),
    )
}
```

## Configuration Options

```go
// Configure worker pool size (default: 100)
bus := eventbus.NewBus(ctx, eventbus.WithWorkerPool(50))

// Unbounded goroutines (legacy behavior)
bus := eventbus.NewBus(ctx, eventbus.WithWorkerPool(0))
```

## Publishing Events

```go
func (s *Server) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*Order, error) {
    order := &Order{ID: uuid.New().String(), ...}

    // Get event bus from context
    bus := eventbus.FromContext(ctx)
    if bus != nil {
        bus.Publish("order.created", order)
    }

    return order, nil
}
```

## Subscribing to Events

Subscribe during plugin initialization:

```go
type notificationPlugin struct {
    email *email.EmailPlugin
}

func (p *notificationPlugin) Init(ctx context.Context, r *prefab.Registry) error {
    // Get event bus
    ebp := r.Get(eventbus.PluginName).(*eventbus.EventBusPlugin)

    // Subscribe to events
    ebp.Subscribe("order.created", p.onOrderCreated)
    ebp.Subscribe("user.registered", p.onUserRegistered)

    return nil
}

func (p *notificationPlugin) onOrderCreated(ctx context.Context, data any) error {
    order := data.(*Order)
    // Send notification email
    return p.sendOrderEmail(ctx, order)
}
```

## Event Bus Interface

```go
type EventBus interface {
    Subscribe(event string, subscriber Subscriber)
    Publish(event string, data any)
    Wait(ctx context.Context) error
}

type Subscriber func(context.Context, any) error
```

## Common Events

Auth plugin events (subscribe in your plugins):

```go
const (
    EventLogin  = "auth.login"   // User logged in
    EventLogout = "auth.logout"  // User logged out
)

func (p *myPlugin) Init(ctx context.Context, r *prefab.Registry) error {
    bus := r.Get(eventbus.PluginName).(*eventbus.EventBusPlugin)

    bus.Subscribe(auth.EventLogin, func(ctx context.Context, data any) error {
        identity := data.(auth.Identity)
        // Track login event
        return nil
    })

    return nil
}
```

## Best Practices

1. **Non-blocking**: Subscribers run asynchronously; don't depend on immediate completion
2. **Idempotent handlers**: Subscribers may be called multiple times concurrently
3. **Error handling**: Errors are logged but don't affect the publisher
4. **Graceful shutdown**: The server waits for the event bus to drain before exiting

## Graceful Shutdown

The plugin automatically drains the event bus during server shutdown:

```go
// From prefab.ShutdownPlugin
func (p *EventBusPlugin) Shutdown(ctx context.Context) error {
    return p.Wait(ctx)
}
```

## Example: Audit Logging

```go
type auditPlugin struct{}

func (p *auditPlugin) Init(ctx context.Context, r *prefab.Registry) error {
    bus := r.Get(eventbus.PluginName).(*eventbus.EventBusPlugin)

    bus.Subscribe("order.created", p.logEvent("order_created"))
    bus.Subscribe("order.updated", p.logEvent("order_updated"))
    bus.Subscribe("user.registered", p.logEvent("user_registered"))

    return nil
}

func (p *auditPlugin) logEvent(eventType string) eventbus.Subscriber {
    return func(ctx context.Context, data any) error {
        logging.Infow(ctx, "audit event", "type", eventType, "data", data)
        return nil
    }
}
```
