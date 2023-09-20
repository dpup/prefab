package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestPlugin struct {
	name string
}

func (tp *TestPlugin) Init(r *Registry) error {
	initOrder = append(initOrder, tp.name)
	return nil
}

var initOrder []string

func TestInit(t *testing.T) {
	// Resetting initOrder for the test
	initOrder = []string{}

	r := &Registry{
		plugins: make(map[any]entry),
	}

	// Register plugins with dependencies
	r.Register("A", &TestPlugin{"A"}, "B", "C")
	r.Register("B", &TestPlugin{"B"}, "C", "D")
	r.Register("C", &TestPlugin{"C"}, "D")
	r.Register("D", &TestPlugin{"D"})

	// Initialize plugins
	err := r.Init()
	assert.Nil(t, err, "initialization failed")

	// Check initialization order
	expectedOrder := []string{"D", "C", "B", "A"}
	for i, name := range initOrder {
		assert.Equal(t, expectedOrder[i], name, "out of order at index %d", i)
	}
}

func TestCycleDetection(t *testing.T) {
	// Resetting initOrder for the test
	initOrder = []string{}

	r := &Registry{
		plugins: make(map[any]entry),
	}

	// Register plugins with a cycle: A -> B -> C -> A
	r.Register("A", &TestPlugin{"A"}, "B")
	r.Register("B", &TestPlugin{"B"}, "C")
	r.Register("C", &TestPlugin{"C"}, "A")

	err := r.Init()
	assert.EqualError(t, err, "plugin: dependency cycle detected involving A")
}

func TestMissingDependency(t *testing.T) {
	// Resetting initOrder for the test
	initOrder = []string{}

	r := &Registry{
		plugins: make(map[any]entry),
	}

	// Register plugins with a missing dependency: A -> B -> XX
	r.Register("A", &TestPlugin{"A"}, "B")
	r.Register("B", &TestPlugin{"B"}, "XX")

	err := r.Init()
	assert.EqualError(t, err, "plugin: missing dependency, XX not registered")
}
