package eventbus

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
)

// NewBus returns a new EventBus. ctx is passed to subscribers when they are
// executed.
func NewBus(ctx context.Context) EventBus {
	return &Bus{
		subscriberCtx: ctx,
	}
}

// Implementation of EventBus which uses a simple map to store subscribers.
type Bus struct {
	subscribers   map[string][]Subscriber
	subscriberCtx context.Context

	mu sync.Mutex     // Protects subscribers.
	wg sync.WaitGroup // Waits for active subscribers to complete.
}

// Subscribe to an event.
func (b *Bus) Subscribe(event string, handler Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subscribers == nil {
		b.subscribers = make(map[string][]Subscriber)
	}
	b.subscribers[event] = append(b.subscribers[event], handler)
}

// Publish an event.
func (b *Bus) Publish(event string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if subs, ok := b.subscribers[event]; ok {
		for _, sub := range subs {
			b.wg.Add(1)
			go b.execute(sub, data)
		}
	}
}

// Wait for the event bus to finish processing all events.
func (b *Bus) Wait(ctx context.Context) error {
	c := make(chan struct{})
	go func() {
		defer close(c)
		b.wg.Wait()
	}()
	select {
	case <-c:
		return nil
	case <-ctx.Done():
		return errors.New("eventbus: timeout waiting for subscribers to finish")
	}
}

func (b *Bus) execute(sub Subscriber, data any) {
	defer func() {
		if r := recover(); r != nil {
			err, _ := errors.ParseStack(debug.Stack())
			skipFrames := 3
			logging.Errorw(b.subscriberCtx, "eventbus: recovered from panic",
				"error", r, "error.stack", err.MinimalStack(skipFrames))
		}
		b.wg.Done()
	}()
	if err := sub(b.subscriberCtx, data); err != nil {
		logging.Errorw(b.subscriberCtx, "eventbus: subscriber error", "error", err)
	}
}
