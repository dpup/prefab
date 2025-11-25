// Package memqueue provides an in-memory implementation of workqueue.WorkQueue.
package memqueue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/workqueue"
)

type queueState struct {
	handlers []workqueue.Handler
	counter  atomic.Uint64
}

// Option configures the queue.
type Option func(*Queue)

// WithWorkerPool sets the number of worker goroutines for processing tasks.
// Default is 100 workers. Set to 0 to use unbounded goroutines.
func WithWorkerPool(size int) Option {
	return func(q *Queue) {
		q.workers = size
	}
}

// New returns a new in-memory WorkQueue.
func New(ctx context.Context, opts ...Option) workqueue.WorkQueue {
	q := &Queue{
		subscriberCtx: logging.With(ctx, logging.FromContext(ctx).Named("workqueue")),
		workers:       100,
		jobs:          make(chan job, 500),
	}
	for _, opt := range opts {
		opt(q)
	}
	return q
}

type job struct {
	ctx     context.Context
	handler workqueue.Handler
	task    *workqueue.Task
}

// Queue is an in-memory implementation of WorkQueue.
type Queue struct {
	subscribers   map[string]*queueState
	subscriberCtx context.Context

	mu sync.Mutex
	wg sync.WaitGroup

	jobs    chan job
	workers int
	started bool
}

// Subscribe registers a handler for queue tasks.
func (q *Queue) Subscribe(queue string, handler workqueue.Handler) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.subscribers == nil {
		q.subscribers = make(map[string]*queueState)
	}
	if q.subscribers[queue] == nil {
		q.subscribers[queue] = &queueState{}
	}
	q.subscribers[queue].handlers = append(q.subscribers[queue].handlers, handler)
}

// Enqueue sends a task to one queue subscriber.
func (q *Queue) Enqueue(queue string, data any) {
	q.mu.Lock()

	if !q.started {
		q.startWorkers()
		q.started = true
	}

	qs, ok := q.subscribers[queue]
	if !ok || len(qs.handlers) == 0 {
		q.mu.Unlock()
		return
	}

	ctx := logging.With(q.subscriberCtx, logging.FromContext(q.subscriberCtx).Named(queue))

	idx := qs.counter.Add(1) - 1
	handler := qs.handlers[idx%uint64(len(qs.handlers))]

	task := workqueue.NewTask(generateTaskID(), queue, data)

	q.wg.Add(1)
	if q.workers == 0 {
		go q.execute(ctx, handler, task)
	} else {
		q.jobs <- job{ctx: ctx, handler: handler, task: task}
	}
	q.mu.Unlock()
}

func generateTaskID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (q *Queue) startWorkers() {
	if q.workers == 0 {
		return
	}
	for range q.workers {
		go q.worker()
	}
}

func (q *Queue) worker() {
	for job := range q.jobs {
		q.execute(job.ctx, job.handler, job.task)
	}
}

// Shutdown closes the job channel and waits for all workers to finish.
func (q *Queue) Shutdown(ctx context.Context) error {
	q.mu.Lock()
	if q.started && q.workers > 0 {
		close(q.jobs)
	}
	q.mu.Unlock()

	return q.Wait(ctx)
}

// Wait blocks until all pending tasks are processed.
func (q *Queue) Wait(ctx context.Context) error {
	c := make(chan struct{})
	go func() {
		defer close(c)
		q.wg.Wait()
	}()
	select {
	case <-c:
		return nil
	case <-ctx.Done():
		return errors.New("workqueue: timeout waiting for handlers to finish")
	}
}

func (q *Queue) execute(ctx context.Context, handler workqueue.Handler, task *workqueue.Task) {
	defer func() {
		if r := recover(); r != nil {
			err, _ := errors.ParseStack(debug.Stack())
			skipFrames := 3
			numFrames := 5
			logging.Errorw(ctx, "workqueue: recovered from panic",
				"error", r, "error.stack_trace", err.MinimalStack(skipFrames, numFrames))
		}
		q.wg.Done()
	}()
	if err := handler(ctx, task); err != nil {
		logging.Errorw(q.subscriberCtx, "workqueue: handler error", "error", err, "task_id", task.ID)
	}
}
