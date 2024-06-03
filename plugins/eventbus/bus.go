package eventbus

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dpup/prefab/logging"
)

// NewBus returns a new EventBus.
func NewBus() EventBus {
	return &Bus{}
}

// Implementation of EventBus which uses a simple map to store subscribers.
type Bus struct {
	subscribers map[string][]Subscriber

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
func (b *Bus) Publish(ctx context.Context, event string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if subs, ok := b.subscribers[event]; ok {
		for _, sub := range subs {
			b.wg.Add(1)
			go b.execute(ctx, sub, data)
		}
	}
}

// Wait for the event bus to finish processing all events.
func (b *Bus) Wait(ctx context.Context, timeout time.Duration) error {
	c := make(chan struct{})
	go func() {
		defer close(c)
		b.wg.Wait()
	}()
	select {
	case <-c:
		return nil
	case <-time.After(timeout):
		return errors.New("eventbus: timeout waiting for subscribers to finish")
	}
}

func (b *Bus) execute(ctx context.Context, sub Subscriber, data any) {
	defer func() {
		if r := recover(); r != nil {
			logging.Errorf(ctx, "eventbus: recovered from panic: %v", r)
		}
		b.wg.Done()
	}()
	if err := sub(context.Background(), data); err != nil {
		logging.Errorf(ctx, "eventbus: subscriber error: %w", err)
	}
}
