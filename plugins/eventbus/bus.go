package eventbus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
)

// consumerGroup tracks subscribers in a queue consumer group.
type consumerGroup struct {
	subscribers []QueueSubscriber
	counter     atomic.Uint64 // For round-robin delivery
}

// BusOption configures the event bus.
type BusOption func(*Bus)

// WithWorkerPool sets the number of worker goroutines for processing events.
// Default is 100 workers. Set to 0 to use unbounded goroutines (legacy behavior).
func WithWorkerPool(size int) BusOption {
	return func(b *Bus) {
		b.workers = size
	}
}

// NewBus returns a new EventBus. ctx is passed to subscribers when they are
// executed.
func NewBus(ctx context.Context, opts ...BusOption) EventBus {
	b := &Bus{
		subscriberCtx: logging.With(ctx, logging.FromContext(ctx).Named("eventbus")),
		workers:       100,                 // Default: 100 workers
		jobs:          make(chan job, 500), // Buffer 500 jobs
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// job represents work to be processed by the worker pool.
type job struct {
	ctx  context.Context
	sub  Subscriber
	data any
}

// Implementation of EventBus which uses a simple map to store subscribers.
type Bus struct {
	subscribers      map[string][]Subscriber
	queueSubscribers map[string]map[string]*consumerGroup // topic -> group -> consumers
	subscriberCtx    context.Context

	mu sync.Mutex     // Protects subscribers.
	wg sync.WaitGroup // Waits for active subscribers to complete.

	// Worker pool for bounded concurrency
	jobs    chan job // Job queue
	workers int      // Number of workers
	started bool     // Track if workers have been started
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

	// Start workers on first publish (lazy initialization)
	if !b.started {
		b.startWorkers()
		b.started = true
	}

	if subs, ok := b.subscribers[event]; ok {
		ctx := logging.With(b.subscriberCtx, logging.FromContext(b.subscriberCtx).Named(event))
		logging.Infow(ctx, "publishing event", "data", data)

		if b.workers == 0 {
			// Legacy mode: unbounded goroutines
			for _, sub := range subs {
				b.wg.Add(1)
				go b.execute(ctx, sub, data)
			}
		} else {
			// Worker pool mode: send jobs to channel
			for _, sub := range subs {
				b.wg.Add(1)
				b.jobs <- job{ctx: ctx, sub: sub, data: data}
			}
		}
	}
	b.mu.Unlock()
}

// SubscribeQueue subscribes to a queue with consumer group semantics.
func (b *Bus) SubscribeQueue(topic string, group string, subscriber QueueSubscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.queueSubscribers == nil {
		b.queueSubscribers = make(map[string]map[string]*consumerGroup)
	}
	if b.queueSubscribers[topic] == nil {
		b.queueSubscribers[topic] = make(map[string]*consumerGroup)
	}
	if b.queueSubscribers[topic][group] == nil {
		b.queueSubscribers[topic][group] = &consumerGroup{}
	}
	b.queueSubscribers[topic][group].subscribers = append(
		b.queueSubscribers[topic][group].subscribers,
		subscriber,
	)
}

// Enqueue adds a message to a queue for single-consumer processing.
func (b *Bus) Enqueue(topic string, data any) {
	b.mu.Lock()

	// Start workers on first use (lazy initialization)
	if !b.started {
		b.startWorkers()
		b.started = true
	}

	groups, ok := b.queueSubscribers[topic]
	if !ok || len(groups) == 0 {
		b.mu.Unlock()
		return
	}

	ctx := logging.With(b.subscriberCtx, logging.FromContext(b.subscriberCtx).Named(topic))
	logging.Infow(ctx, "enqueueing message", "data", data)

	// Deliver to one subscriber per consumer group
	for _, cg := range groups {
		if len(cg.subscribers) == 0 {
			continue
		}

		// Round-robin selection within the group
		idx := cg.counter.Add(1) - 1
		sub := cg.subscribers[idx%uint64(len(cg.subscribers))]

		msg := &Message{
			ID:      generateMessageID(),
			Topic:   topic,
			Data:    data,
			Attempt: 1,
			// In-memory implementation: ack/nack are no-ops
			// Distributed implementations would handle redelivery
			ack:  func() {},
			nack: func() {},
		}

		b.wg.Add(1)
		if b.workers == 0 {
			go b.executeQueue(ctx, sub, msg)
		} else {
			// Wrap queue subscriber in job
			b.jobs <- job{
				ctx:  ctx,
				sub:  func(ctx context.Context, _ any) error { return sub(ctx, msg) },
				data: nil,
			}
		}
	}
	b.mu.Unlock()
}

// generateMessageID creates a random message ID.
func generateMessageID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// executeQueue runs a queue subscriber with panic recovery.
func (b *Bus) executeQueue(ctx context.Context, sub QueueSubscriber, msg *Message) {
	defer func() {
		if r := recover(); r != nil {
			err, _ := errors.ParseStack(debug.Stack())
			skipFrames := 3
			numFrames := 5
			logging.Errorw(ctx, "eventbus: recovered from panic",
				"error", r, "error.stack_trace", err.MinimalStack(skipFrames, numFrames))
		}
		b.wg.Done()
	}()
	if err := sub(ctx, msg); err != nil {
		logging.Errorw(b.subscriberCtx, "eventbus: queue subscriber error", "error", err, "message_id", msg.ID)
	}
}

// startWorkers starts the worker pool goroutines.
func (b *Bus) startWorkers() {
	if b.workers == 0 {
		return // No worker pool
	}
	for range b.workers {
		go b.worker()
	}
}

// worker processes jobs from the job queue.
func (b *Bus) worker() {
	for job := range b.jobs {
		b.execute(job.ctx, job.sub, job.data)
	}
}

// Shutdown closes the job channel and waits for all workers to finish.
// This should be called before Wait() to ensure graceful shutdown.
func (b *Bus) Shutdown(ctx context.Context) error {
	b.mu.Lock()
	if b.started && b.workers > 0 {
		close(b.jobs) // Signal workers to exit after draining
	}
	b.mu.Unlock()

	return b.Wait(ctx)
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

func (b *Bus) execute(ctx context.Context, sub Subscriber, data any) {
	defer func() {
		if r := recover(); r != nil {
			err, _ := errors.ParseStack(debug.Stack())
			skipFrames := 3
			numFrames := 5
			logging.Errorw(ctx, "eventbus: recovered from panic",
				"error", r, "error.stack_trace", err.MinimalStack(skipFrames, numFrames))
		}
		b.wg.Done()
	}()
	if err := sub(ctx, data); err != nil {
		logging.Errorw(b.subscriberCtx, "eventbus: subscriber error", "error", err)
	}
}
