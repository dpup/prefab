// Package membus provides an in-memory implementation of eventbus.EventBus.
package membus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/eventbus"
)

type queueState struct {
	handlers []eventbus.Handler
	counter  atomic.Uint64
}

// Option configures the bus.
type Option func(*Bus)

// WithWorkerPool sets the number of worker goroutines for processing events.
// Default is 100 workers. Set to 0 to use unbounded goroutines.
func WithWorkerPool(size int) Option {
	return func(b *Bus) {
		b.workers = size
	}
}

// New returns a new in-memory EventBus.
func New(ctx context.Context, opts ...Option) eventbus.EventBus {
	b := &Bus{
		subscriberCtx: logging.With(ctx, logging.FromContext(ctx).Named("eventbus")),
		workers:       100,
		jobs:          make(chan job, 500),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

type job struct {
	ctx     context.Context
	handler eventbus.Handler
	msg     *eventbus.Message
}

// Bus is an in-memory implementation of EventBus.
type Bus struct {
	subscribers      map[string][]eventbus.Handler
	queueSubscribers map[string]*queueState
	subscriberCtx    context.Context

	mu sync.Mutex
	wg sync.WaitGroup

	jobs    chan job
	workers int
	started bool
}

// Subscribe registers a handler for broadcast messages.
func (b *Bus) Subscribe(topic string, handler eventbus.Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subscribers == nil {
		b.subscribers = make(map[string][]eventbus.Handler)
	}
	b.subscribers[topic] = append(b.subscribers[topic], handler)
}

// Publish sends a message to all subscribers.
func (b *Bus) Publish(topic string, data any) {
	b.mu.Lock()

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

	for _, handler := range handlers {
		msg := eventbus.NewMessage(generateMessageID(), topic, data)

		b.wg.Add(1)
		if b.workers == 0 {
			go b.execute(ctx, handler, msg)
		} else {
			b.jobs <- job{ctx: ctx, handler: handler, msg: msg}
		}
	}
	b.mu.Unlock()
}

// SubscribeQueue registers a handler for queue messages.
func (b *Bus) SubscribeQueue(topic string, handler eventbus.Handler) {
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

	idx := qs.counter.Add(1) - 1
	handler := qs.handlers[idx%uint64(len(qs.handlers))]

	msg := eventbus.NewMessage(generateMessageID(), topic, data)

	b.wg.Add(1)
	if b.workers == 0 {
		go b.execute(ctx, handler, msg)
	} else {
		b.jobs <- job{ctx: ctx, handler: handler, msg: msg}
	}
	b.mu.Unlock()
}

func generateMessageID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (b *Bus) startWorkers() {
	if b.workers == 0 {
		return
	}
	for range b.workers {
		go b.worker()
	}
}

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

func (b *Bus) execute(ctx context.Context, handler eventbus.Handler, msg *eventbus.Message) {
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
