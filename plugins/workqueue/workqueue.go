// Package workqueue provides a task queue with single-consumer semantics.
// Each task is processed by exactly one worker.
package workqueue

import (
	"context"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/logging"
)

const (
	// PluginName identifies this plugin.
	PluginName = "workqueue"
)

// Handler processes tasks from the work queue.
type Handler func(context.Context, *Task) error

// Task wraps task data with metadata.
type Task struct {
	ID      string // Unique identifier
	Queue   string // Queue name
	Data    any    // Payload
	Attempt int    // Processing attempt (1-based)

	ack  func() // Called on successful processing
	nack func() // Called on processing failure
}

// Ack acknowledges successful processing of the task.
func (t *Task) Ack() {
	if t.ack != nil {
		t.ack()
	}
}

// Nack indicates the task failed to process and should be redelivered.
func (t *Task) Nack() {
	if t.nack != nil {
		t.nack()
	}
}

// NewTask creates a task with default no-op ack/nack functions.
func NewTask(id, queue string, data any) *Task {
	return &Task{
		ID:      id,
		Queue:   queue,
		Data:    data,
		Attempt: 1,
		ack:     func() {},
		nack:    func() {},
	}
}

// NewTaskWithCallbacks creates a task with custom ack/nack callbacks.
func NewTaskWithCallbacks(id, queue string, data any, attempt int, ack, nack func()) *Task {
	return &Task{
		ID:      id,
		Queue:   queue,
		Data:    data,
		Attempt: attempt,
		ack:     ack,
		nack:    nack,
	}
}

// WorkQueue provides task queue with single-consumer semantics.
// Each enqueued task is processed by exactly one worker.
type WorkQueue interface {
	// Subscribe registers a handler that competes with other handlers.
	// Only one handler will process each task.
	Subscribe(queue string, handler Handler)

	// Enqueue adds a task to the queue for single-consumer processing.
	Enqueue(queue string, data any)

	// Wait blocks until locally-initiated operations complete. For in-memory
	// implementations, this means all handlers have finished. For distributed
	// implementations, this means tasks have been sent to the remote system.
	Wait(ctx context.Context) error
}

// Plugin registers a workqueue with a Prefab server for use by other plugins.
// The queue can be retrieved from the request context using FromContext.
func Plugin(wq WorkQueue) *WorkQueuePlugin {
	return &WorkQueuePlugin{
		WorkQueue: wq,
	}
}

// WorkQueuePlugin provides access to a work queue for plugins and components.
type WorkQueuePlugin struct {
	WorkQueue
}

// From prefab.Plugin.
func (p *WorkQueuePlugin) Name() string {
	return PluginName
}

// From prefab.OptionProvider.
func (p *WorkQueuePlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithRequestConfig(p.inject),
	}
}

// Shutdownable is implemented by WorkQueue implementations that need graceful shutdown.
type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

// From prefab.ShutdownPlugin.
func (p *WorkQueuePlugin) Shutdown(ctx context.Context) error {
	// If the queue implements Shutdownable, use that for graceful shutdown
	if queue, ok := p.WorkQueue.(Shutdownable); ok {
		err := queue.Shutdown(ctx)
		if err == nil {
			logging.Info(ctx, "Work queue drained")
		}
		return err
	}

	// Otherwise, just wait for completion
	err := p.Wait(ctx)
	if err == nil {
		logging.Info(ctx, "Work queue drained")
	}
	return err
}

func (p *WorkQueuePlugin) inject(ctx context.Context) context.Context {
	return context.WithValue(ctx, workQueueKey{}, p)
}

// FromContext retrieves the work queue from a context.
func FromContext(ctx context.Context) WorkQueue {
	if p, ok := ctx.Value(workQueueKey{}).(*WorkQueuePlugin); ok {
		return p.WorkQueue
	}
	return nil
}

type workQueueKey struct{}
