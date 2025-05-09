package eventbus

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/dpup/prefab/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBus_BasicPubSub(t *testing.T) {
	bus := NewBus(logging.EnsureLogger(t.Context()))

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		assert.Equal(t, "hello", data)
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
	bus := NewBus(logging.EnsureLogger(t.Context()))

	var called []int
	var mu sync.Mutex
	for i := range 10 {
		bus.Subscribe("topic", func(ctx context.Context, data any) error {
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, "hello", data)
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
	bus := NewBus(logging.EnsureLogger(t.Context()))

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		assert.Equal(t, "hello", data)
		time.Sleep(time.Millisecond * 50)
		called = true
		return nil
	})

	bus.Publish("topic", "hello")

	require.NoError(t, bus.Wait(logging.EnsureLogger(t.Context())))
	assert.True(t, called, "subscriber should have been called")
}

func TestBus_WaitTimeout(t *testing.T) {
	bus := NewBus(logging.EnsureLogger(t.Context()))

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		assert.Equal(t, "hello", data)
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
	bus := NewBus(ctx)

	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		return errors.New("subscriber error")
	})

	bus.Publish("topic", "hello")
	assert.NoError(t, bus.Wait(ctx))

	// TODO: Check for error in logs.
}

func TestBus_SubscriberPanic(t *testing.T) {
	ctx := logging.With(t.Context(), logging.NewDevLogger())
	bus := NewBus(ctx)

	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		panic("subscriber panic")
	})

	bus.Publish("topic", "hello")
	assert.NoError(t, bus.Wait(ctx))

	// TODO: Check for error in logs.
}
