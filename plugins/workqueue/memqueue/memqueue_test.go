package memqueue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/workqueue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueue_Basic(t *testing.T) {
	queue := New(logging.EnsureLogger(t.Context()))

	var received *workqueue.Task
	queue.Subscribe("tasks", func(ctx context.Context, task *workqueue.Task) error {
		received = task
		return nil
	})

	queue.Enqueue("tasks", "hello")

	assert.Eventually(t, func() bool { return received != nil },
		time.Millisecond*10,
		time.Millisecond,
		"subscriber should have received task")

	assert.Equal(t, "hello", received.Data)
	assert.Equal(t, "tasks", received.Queue)
	assert.Equal(t, 1, received.Attempt)
	assert.NotEmpty(t, received.ID)
}

func TestQueue_SingleConsumer(t *testing.T) {
	queue := New(logging.EnsureLogger(t.Context()))

	var callCount int
	var mu sync.Mutex

	// Add 3 subscribers
	for range 3 {
		queue.Subscribe("tasks", func(ctx context.Context, task *workqueue.Task) error {
			mu.Lock()
			callCount++
			mu.Unlock()
			return nil
		})
	}

	// Enqueue one task
	queue.Enqueue("tasks", "hello")

	// Wait for processing
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*100)
	defer cancel()
	require.NoError(t, queue.Wait(ctx))

	// Only one subscriber should have received the task
	assert.Equal(t, 1, callCount, "only one subscriber should receive task")
}

func TestQueue_RoundRobin(t *testing.T) {
	queue := New(logging.EnsureLogger(t.Context()))

	callCounts := make([]int, 3)
	var mu sync.Mutex

	// Add 3 subscribers
	for i := range 3 {
		idx := i // Capture loop variable
		queue.Subscribe("tasks", func(ctx context.Context, task *workqueue.Task) error {
			mu.Lock()
			callCounts[idx]++
			mu.Unlock()
			return nil
		})
	}

	// Enqueue 6 tasks - should distribute 2 to each subscriber
	for range 6 {
		queue.Enqueue("tasks", "hello")
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*100)
	defer cancel()
	require.NoError(t, queue.Wait(ctx))

	// Each subscriber should have received 2 tasks
	for i, count := range callCounts {
		assert.Equal(t, 2, count, "subscriber %d should receive 2 tasks", i)
	}
}

func TestQueue_AckNack(t *testing.T) {
	queue := New(logging.EnsureLogger(t.Context()))

	var ackCalled, nackCalled bool

	queue.Subscribe("queue1", func(ctx context.Context, task *workqueue.Task) error {
		task.Ack()
		ackCalled = true
		return nil
	})

	queue.Subscribe("queue2", func(ctx context.Context, task *workqueue.Task) error {
		task.Nack()
		nackCalled = true
		return nil
	})

	queue.Enqueue("queue1", "hello")
	queue.Enqueue("queue2", "hello")

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*100)
	defer cancel()
	require.NoError(t, queue.Wait(ctx))

	// In-memory implementation, ack/nack are no-ops but should not panic
	assert.True(t, ackCalled, "ack should have been called")
	assert.True(t, nackCalled, "nack should have been called")
}

func TestQueue_NoSubscribers(t *testing.T) {
	queue := New(logging.EnsureLogger(t.Context()))

	// Enqueue without subscribers should not panic
	queue.Enqueue("tasks", "hello")

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*10)
	defer cancel()
	require.NoError(t, queue.Wait(ctx))
}

func TestQueue_LegacyMode(t *testing.T) {
	// Test queue with workers=0 (unbounded goroutines)
	ctx := logging.EnsureLogger(t.Context())
	queue := New(ctx, WithWorkerPool(0))

	var called int
	var mu sync.Mutex

	for range 3 {
		queue.Subscribe("tasks", func(ctx context.Context, task *workqueue.Task) error {
			mu.Lock()
			called++
			mu.Unlock()
			return nil
		})
	}

	queue.Enqueue("tasks", "hello")

	tctx, cancel := context.WithTimeout(ctx, time.Millisecond*100)
	defer cancel()
	require.NoError(t, queue.Wait(tctx))

	assert.Equal(t, 1, called, "only one subscriber should receive task")
}

func TestQueue_Error(t *testing.T) {
	ctx := logging.With(t.Context(), logging.NewDevLogger())
	queue := New(ctx)

	queue.Subscribe("tasks", func(ctx context.Context, task *workqueue.Task) error {
		return errors.New("handler error")
	})

	queue.Enqueue("tasks", "hello")
	assert.NoError(t, queue.Wait(ctx))
}

func TestQueue_Panic(t *testing.T) {
	ctx := logging.With(t.Context(), logging.NewDevLogger())
	queue := New(ctx)

	queue.Subscribe("tasks", func(ctx context.Context, task *workqueue.Task) error {
		panic("handler panic")
	})

	queue.Enqueue("tasks", "hello")
	assert.NoError(t, queue.Wait(ctx))
}

func TestQueue_WorkerPoolLimit(t *testing.T) {
	ctx := logging.EnsureLogger(t.Context())
	queue := New(ctx, WithWorkerPool(10))

	var called int
	var mu sync.Mutex

	// Add many subscribers
	for range 100 {
		queue.Subscribe("tasks", func(ctx context.Context, task *workqueue.Task) error {
			mu.Lock()
			called++
			mu.Unlock()
			time.Sleep(time.Millisecond * 10)
			return nil
		})
	}

	// Enqueue task
	queue.Enqueue("tasks", "hello")

	// Wait for completion
	require.NoError(t, queue.Wait(ctx))

	// One subscriber should have been called
	assert.Equal(t, 1, called, "one subscriber should be processed by worker pool")
}

func TestQueue_TaskMetadata(t *testing.T) {
	queue := New(logging.EnsureLogger(t.Context()))

	var task *workqueue.Task
	queue.Subscribe("tasks", func(ctx context.Context, t *workqueue.Task) error {
		task = t
		return nil
	})

	queue.Enqueue("tasks", "hello")

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*100)
	defer cancel()
	require.NoError(t, queue.Wait(ctx))

	require.NotNil(t, task)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "tasks", task.Queue)
	assert.Equal(t, "hello", task.Data)
	assert.Equal(t, 1, task.Attempt)
}
