package membus

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/eventbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBus_BasicPubSub(t *testing.T) {
	bus := New(logging.EnsureLogger(t.Context()))

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
		assert.Equal(t, "hello", msg.Data)
		called = true
		return nil
	})

	bus.Publish("topic", "hello")

	assert.Eventually(t, func() bool { return called },
		time.Millisecond*10,
		time.Millisecond,
		"subscriber should have been called")
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := New(logging.EnsureLogger(t.Context()))

	var called []int
	var mu sync.Mutex
	for i := range 10 {
		bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, "hello", msg.Data)
			called = append(called, i)
			return nil
		})
	}

	bus.Publish("topic", "hello")

	assert.Eventually(t, func() bool {
		slices.Sort(called) // Execution order isn't gauranteed.
		assert.Equal(t, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, called)
		return len(called) == 10
	},
		time.Millisecond*10,
		time.Millisecond,
		"subscribers should have been called")
}

func TestBus_Wait(t *testing.T) {
	bus := New(logging.EnsureLogger(t.Context()))

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
		assert.Equal(t, "hello", msg.Data)
		time.Sleep(time.Millisecond * 50)
		called = true
		return nil
	})

	bus.Publish("topic", "hello")

	require.NoError(t, bus.Wait(logging.EnsureLogger(t.Context())))
	assert.True(t, called, "subscriber should have been called")
}

func TestBus_WaitTimeout(t *testing.T) {
	bus := New(logging.EnsureLogger(t.Context()))

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
		assert.Equal(t, "hello", msg.Data)
		time.Sleep(time.Millisecond * 50)
		called = true
		return nil
	})

	bus.Publish("topic", "hello")

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()

	require.Error(t, bus.Wait(ctx))
	assert.False(t, called, "subscriber should not have been called yet")
}

func TestBus_SubscriberError(t *testing.T) {
	ctx := logging.With(t.Context(), logging.NewDevLogger())
	bus := New(ctx)

	bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
		return errors.New("subscriber error")
	})

	bus.Publish("topic", "hello")
	assert.NoError(t, bus.Wait(ctx))

	// TODO: Check for error in logs.
}

func TestBus_SubscriberPanic(t *testing.T) {
	ctx := logging.With(t.Context(), logging.NewDevLogger())
	bus := New(ctx)

	bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
		panic("subscriber panic")
	})

	bus.Publish("topic", "hello")
	assert.NoError(t, bus.Wait(ctx))

	// TODO: Check for error in logs.
}

func TestBus_WorkerPoolConcurrency(t *testing.T) {
	bus := New(logging.EnsureLogger(t.Context()))

	var mu sync.Mutex
	var concurrent int
	var maxConcurrent int

	for range 200 {
		bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
			mu.Lock()
			concurrent++
			if concurrent > maxConcurrent {
				maxConcurrent = concurrent
			}
			mu.Unlock()

			time.Sleep(time.Millisecond) // Simulate work

			mu.Lock()
			concurrent--
			mu.Unlock()
			return nil
		})
	}

	bus.Publish("topic", "hello")
	require.NoError(t, bus.Wait(logging.EnsureLogger(t.Context())))

	// With 200 subscribers and 100 workers, max concurrent should be ~100
	t.Logf("Max concurrent subscribers: %d", maxConcurrent)
	assert.LessOrEqual(t, maxConcurrent, 100, "should not exceed worker pool size")
}

func TestBus_WorkerPoolLimit(t *testing.T) {
	ctx := logging.EnsureLogger(t.Context())
	bus := New(ctx, WithWorkerPool(10))

	var called int
	var mu sync.Mutex

	// Add many subscribers
	for range 100 {
		bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
			mu.Lock()
			called++
			mu.Unlock()
			time.Sleep(time.Millisecond * 10)
			return nil
		})
	}

	// Publish event
	bus.Publish("topic", "hello")

	// Wait for completion
	require.NoError(t, bus.Wait(ctx))

	// All 100 subscribers should have been called
	assert.Equal(t, 100, called, "all subscribers should be processed by worker pool")
}

func TestBus_HighLoad(t *testing.T) {
	ctx := logging.EnsureLogger(t.Context())
	bus := New(ctx, WithWorkerPool(50))

	var processed sync.WaitGroup
	processed.Add(1000)

	// Add 1000 subscribers
	for range 1000 {
		bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
			processed.Done()
			return nil
		})
	}

	// Publish event
	bus.Publish("topic", "hello")

	// Wait for all to complete
	done := make(chan struct{})
	go func() {
		processed.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second * 5):
		t.Fatal("timeout waiting for high load processing")
	}

	assert.NoError(t, bus.Wait(ctx))
}

func TestBus_GracefulShutdown(t *testing.T) {
	ctx := logging.EnsureLogger(t.Context())
	bus := New(ctx, WithWorkerPool(10)).(*Bus)

	var completed int
	var mu sync.Mutex

	// Add subscribers that take time
	for range 50 {
		bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
			time.Sleep(time.Millisecond * 10)
			mu.Lock()
			completed++
			mu.Unlock()
			return nil
		})
	}

	// Publish event
	bus.Publish("topic", "hello")

	// Give workers time to start processing
	time.Sleep(time.Millisecond * 5)

	// Shutdown should wait for all jobs to complete
	require.NoError(t, bus.Shutdown(ctx))

	mu.Lock()
	final := completed
	mu.Unlock()

	assert.Equal(t, 50, final, "all subscribers should complete")
}

func TestBus_LegacyMode(t *testing.T) {
	// Test with workers=0 (unbounded goroutines, legacy behavior)
	ctx := logging.EnsureLogger(t.Context())
	bus := New(ctx, WithWorkerPool(0))

	var called int
	var mu sync.Mutex

	for range 10 {
		bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
			mu.Lock()
			called++
			mu.Unlock()
			return nil
		})
	}

	bus.Publish("topic", "hello")
	require.NoError(t, bus.Wait(ctx))

	assert.Equal(t, 10, called)
}

func TestBus_CustomWorkerPoolSize(t *testing.T) {
	ctx := logging.EnsureLogger(t.Context())
	bus := New(ctx, WithWorkerPool(5))

	var called int
	var mu sync.Mutex

	for range 20 {
		bus.Subscribe("topic", func(ctx context.Context, msg *eventbus.Message) error {
			mu.Lock()
			called++
			mu.Unlock()
			return nil
		})
	}

	bus.Publish("topic", "hello")
	require.NoError(t, bus.Wait(ctx))

	assert.Equal(t, 20, called, "all subscribers should be called")
}
func TestBus_MessageMetadata(t *testing.T) {
	bus := New(logging.EnsureLogger(t.Context()))

	var msg *eventbus.Message
	bus.Subscribe("topic", func(ctx context.Context, m *eventbus.Message) error {
		msg = m
		return nil
	})

	bus.Publish("topic", "hello")

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*100)
	defer cancel()
	require.NoError(t, bus.Wait(ctx))

	require.NotNil(t, msg)
	assert.NotEmpty(t, msg.ID)
	assert.Equal(t, "topic", msg.Topic)
	assert.Equal(t, "hello", msg.Data)
	assert.Equal(t, 1, msg.Attempt)
}
