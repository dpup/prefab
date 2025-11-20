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

// queueState tracks queue subscribers with round-robin counter.
type queueState struct {
	handlers []Handler
	counter  atomic.Uint64
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
	ctx     context.Context
	handler Handler
	msg     *Message
}

// Bus is an in-memory implementation of EventBus.
type Bus struct {
	subscribers      map[string][]Handler
	queueSubscribers map[string]*queueState
	subscriberCtx    context.Context

	mu sync.Mutex     // Protects subscribers.
	wg sync.WaitGroup // Waits for active subscribers to complete.

	// Worker pool for bounded concurrency
	jobs    chan job // Job queue
	workers int      // Number of workers
	started bool     // Track if workers have been started
}

// Subscribe registers a handler for broadcast messages.
func (b *Bus) Subscribe(topic string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subscribers == nil {
		b.subscribers = make(map[string][]Handler)
	}
	b.subscribers[topic] = append(b.subscribers[topic], handler)
}

// Publish sends a message to all subscribers.
func (b *Bus) Publish(topic string, data any) {
	b.mu.Lock()

	// Start workers on first publish (lazy initialization)
	if !b.started {
		b.startWorkers()
		b.started = true
	}

	handlers, ok := b.subscribers[topic]
	if !ok || len(handlers) == 0 {
		b.mu.Unlock()
		return
	}

	ctx := logging.With(b.subscriberCtx, logging.FromContext(b.subscriberCtx).Named(topic))
	logging.Infow(ctx, "publishing message", "data", data)

	for _, handler := range handlers {
		msg := &Message{
			ID:      generateMessageID(),
			Topic:   topic,
			Data:    data,
			Attempt: 1,
			ack:     func() {},
			nack:    func() {},
		}

		b.wg.Add(1)
		if b.workers == 0 {
			// Legacy mode: unbounded goroutines
			go b.execute(ctx, handler, msg)
		} else {
			// Worker pool mode: send jobs to channel
			b.jobs <- job{ctx: ctx, handler: handler, msg: msg}
		}
	}
	b.mu.Unlock()
}

// SubscribeQueue registers a handler for queue messages.
func (b *Bus) SubscribeQueue(topic string, handler Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.queueSubscribers == nil {
		b.queueSubscribers = make(map[string]*queueState)
	}
	if b.queueSubscribers[topic] == nil {
		b.queueSubscribers[topic] = &queueState{}
	}
	b.queueSubscribers[topic].handlers = append(b.queueSubscribers[topic].handlers, handler)
}

// Enqueue sends a message to one queue subscriber.
func (b *Bus) Enqueue(topic string, data any) {
	b.mu.Lock()

	// Start workers on first use (lazy initialization)
	if !b.started {
		b.startWorkers()
		b.started = true
	}

	qs, ok := b.queueSubscribers[topic]
	if !ok || len(qs.handlers) == 0 {
		b.mu.Unlock()
		return
	}

	ctx := logging.With(b.subscriberCtx, logging.FromContext(b.subscriberCtx).Named(topic))
	logging.Infow(ctx, "enqueueing message", "data", data)

	// Round-robin selection
	idx := qs.counter.Add(1) - 1
	handler := qs.handlers[idx%uint64(len(qs.handlers))]

	msg := &Message{
		ID:      generateMessageID(),
		Topic:   topic,
		Data:    data,
		Attempt: 1,
		ack:     func() {},
		nack:    func() {},
	}

	b.wg.Add(1)
	if b.workers == 0 {
		go b.execute(ctx, handler, msg)
	} else {
		b.jobs <- job{ctx: ctx, handler: handler, msg: msg}
	}
	b.mu.Unlock()
}

// generateMessageID creates a random message ID.
func generateMessageID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
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
		b.execute(job.ctx, job.handler, job.msg)
	}
}

// Shutdown closes the job channel and waits for all workers to finish.
func (b *Bus) Shutdown(ctx context.Context) error {
	b.mu.Lock()
	if b.started && b.workers > 0 {
		close(b.jobs)
	}
	b.mu.Unlock()

	return b.Wait(ctx)
}

// Wait blocks until all pending messages are processed.
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
		return errors.New("eventbus: timeout waiting for handlers to finish")
	}
}

func (b *Bus) execute(ctx context.Context, handler Handler, msg *Message) {
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
	if err := handler(ctx, msg); err != nil {
		logging.Errorw(b.subscriberCtx, "eventbus: handler error", "error", err, "message_id", msg.ID)
	}
}
