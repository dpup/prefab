package prefab

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestPlugin struct {
	name string
	deps []string
}

func (tp *TestPlugin) Name() string {
	return tp.name
}

func (tp *TestPlugin) Deps() []string {
	return tp.deps
}

func (tp *TestPlugin) Init(ctx context.Context, r *Registry) error {
	initOrder = append(initOrder, tp.name)
	return nil
}

var initOrder []string

func TestInit(t *testing.T) {
	ctx := t.Context()

	// Resetting initOrder for the test
	initOrder = []string{}
	r := &Registry{}

	// Register plugins with dependencies
	r.Register(&TestPlugin{name: "A", deps: []string{"B", "C"}})
	r.Register(&TestPlugin{name: "B", deps: []string{"C", "D"}})
	r.Register(&TestPlugin{name: "C", deps: []string{"D"}})
	r.Register(&TestPlugin{name: "D"})

	// Initialize plugins
	err := r.Init(ctx)
	require.NoError(t, err, "initialization failed")

	// Check initialization order
	expectedOrder := []string{"D", "C", "B", "A"}
	for i, name := range initOrder {
		assert.Equal(t, expectedOrder[i], name, "out of order at index %d", i)
	}
}

func TestCycleDetection(t *testing.T) {
	ctx := t.Context()

	// Resetting initOrder for the test
	initOrder = []string{}

	r := &Registry{}

	// Register plugins with a cycle: A -> B -> C -> A
	r.Register(&TestPlugin{name: "A", deps: []string{"B"}})
	r.Register(&TestPlugin{name: "B", deps: []string{"C"}})
	r.Register(&TestPlugin{name: "C", deps: []string{"A"}})

	err := r.Init(ctx)
	assert.EqualError(t, err, "plugin: dependency cycle detected involving 'A'")
}

func TestMissingDependency(t *testing.T) {
	ctx := t.Context()

	// Resetting initOrder for the test
	initOrder = []string{}

	r := &Registry{}

	// Register plugins with a missing dependency: A -> B -> XX
	r.Register(&TestPlugin{name: "A", deps: []string{"B"}})
	r.Register(&TestPlugin{name: "B", deps: []string{"XX"}})

	err := r.Init(ctx)
	assert.EqualError(t, err, "plugin: missing dependency, 'XX' not registered")
}
