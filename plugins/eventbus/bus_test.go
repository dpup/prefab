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
)

func TestBus_BasicPubSub(t *testing.T) {
	bus := NewBus()

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		assert.Equal(t, "hello", data)
		called = true
		return nil
	})

	bus.Publish(context.Background(), "topic", "hello")

	assert.Eventually(t, func() bool { return called },
		time.Millisecond*10,
		time.Millisecond,
		"subscriber should have been called")
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus()

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

	bus.Publish(context.Background(), "topic", "hello")

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
	bus := NewBus()

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		assert.Equal(t, "hello", data)
		time.Sleep(time.Millisecond * 50)
		called = true
		return nil
	})

	bus.Publish(context.Background(), "topic", "hello")

	assert.NoError(t, bus.Wait(context.Background(), time.Second))
	assert.True(t, called, "subscriber should have been called")
}

func TestBus_WaitTimeout(t *testing.T) {
	bus := NewBus()

	var called bool
	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		assert.Equal(t, "hello", data)
		time.Sleep(time.Millisecond * 50)
		called = true
		return nil
	})

	bus.Publish(context.Background(), "topic", "hello")

	assert.Error(t, bus.Wait(context.Background(), time.Millisecond))
	assert.False(t, called, "subscriber should not have been called yet")
}

func TestBus_SubscriberError(t *testing.T) {
	bus := NewBus()

	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		return errors.New("subscriber error")
	})

	ctx := logging.With(context.Background(), logging.NewDevLogger())
	bus.Publish(ctx, "topic", "hello")
	assert.NoError(t, bus.Wait(ctx, time.Second))

	// TODO: Check for error in logs.
}

func TestBus_SubscriberPanic(t *testing.T) {
	bus := NewBus()

	bus.Subscribe("topic", func(ctx context.Context, data any) error {
		panic("subscriber panic")
	})

	ctx := logging.With(context.Background(), logging.NewDevLogger())
	bus.Publish(ctx, "topic", "hello")
	assert.NoError(t, bus.Wait(ctx, time.Second))

	// TODO: Check for error in logs.
}
