package eventbus

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
)

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
	subscribers   map[string][]Subscriber
	subscriberCtx context.Context

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
