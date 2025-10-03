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

type TestPluginWithOptDeps struct {
	name    string
	deps    []string
	optDeps []string
}

func (tp *TestPluginWithOptDeps) Name() string {
	return tp.name
}

func (tp *TestPluginWithOptDeps) Deps() []string {
	return tp.deps
}

func (tp *TestPluginWithOptDeps) OptDeps() []string {
	return tp.optDeps
}

func (tp *TestPluginWithOptDeps) Init(ctx context.Context, r *Registry) error {
	initOrder = append(initOrder, tp.name)
	return nil
}

// TestOptionalDependencyInitOrder verifies that optional dependencies are
// initialized before the plugin that declares them, if they are registered.
func TestOptionalDependencyInitOrder(t *testing.T) {
	ctx := t.Context()

	t.Run("optional dependency initialized first when registered", func(t *testing.T) {
		initOrder = []string{}
		r := &Registry{}

		// Plugin A has optional dependency on B
		// If B is registered, it should initialize before A
		r.Register(&TestPluginWithOptDeps{name: "A", optDeps: []string{"B"}})
		r.Register(&TestPlugin{name: "B"})

		err := r.Init(ctx)
		require.NoError(t, err)

		// B should initialize before A
		assert.Equal(t, []string{"B", "A"}, initOrder)
	})

	t.Run("optional dependency not required if missing", func(t *testing.T) {
		initOrder = []string{}
		r := &Registry{}

		// Plugin A has optional dependency on B, but B is not registered
		// This should not cause an error
		r.Register(&TestPluginWithOptDeps{name: "A", optDeps: []string{"B"}})

		err := r.Init(ctx)
		require.NoError(t, err)

		// Only A should initialize
		assert.Equal(t, []string{"A"}, initOrder)
	})

	t.Run("optional dependencies with transitive required deps", func(t *testing.T) {
		initOrder = []string{}
		r := &Registry{}

		// A has optional dep on B
		// B has required dep on C
		// C has required dep on D
		// Expected order: D, C, B, A
		r.Register(&TestPluginWithOptDeps{name: "A", optDeps: []string{"B"}})
		r.Register(&TestPlugin{name: "B", deps: []string{"C"}})
		r.Register(&TestPlugin{name: "C", deps: []string{"D"}})
		r.Register(&TestPlugin{name: "D"})

		err := r.Init(ctx)
		require.NoError(t, err)

		assert.Equal(t, []string{"D", "C", "B", "A"}, initOrder)
	})

	t.Run("mixed required and optional dependencies", func(t *testing.T) {
		initOrder = []string{}
		r := &Registry{}

		// A has required dep on B and optional dep on C
		// Both should initialize before A
		// B has no deps, C has no deps
		// Order should be: B, C, A (or C, B, A - both are valid)
		r.Register(&TestPluginWithOptDeps{name: "A", deps: []string{"B"}, optDeps: []string{"C"}})
		r.Register(&TestPlugin{name: "B"})
		r.Register(&TestPlugin{name: "C"})

		err := r.Init(ctx)
		require.NoError(t, err)

		// A should be last
		assert.Equal(t, "A", initOrder[2])
		// B and C should both come before A
		assert.Contains(t, initOrder[:2], "B")
		assert.Contains(t, initOrder[:2], "C")
	})

	t.Run("optional dependency cycle detection", func(t *testing.T) {
		initOrder = []string{}
		r := &Registry{}

		// Create a cycle using optional dependencies
		// A optionally depends on B, B optionally depends on A
		r.Register(&TestPluginWithOptDeps{name: "A", optDeps: []string{"B"}})
		r.Register(&TestPluginWithOptDeps{name: "B", optDeps: []string{"A"}})

		err := r.Init(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cycle")
	})

	t.Run("registration order independence with optional deps", func(t *testing.T) {
		// This is the key test - demonstrates the bug that we're fixing
		// Currently, if A is registered before Storage, A may initialize first
		// even though it has an optional dependency on Storage

		t.Run("storage registered first", func(t *testing.T) {
			initOrder = []string{}
			r := &Registry{}

			// Register in "good" order
			r.Register(&TestPlugin{name: "Storage"})
			r.Register(&TestPluginWithOptDeps{name: "Auth", optDeps: []string{"Storage"}})

			err := r.Init(ctx)
			require.NoError(t, err)
			assert.Equal(t, []string{"Storage", "Auth"}, initOrder)
		})

		t.Run("auth registered first", func(t *testing.T) {
			initOrder = []string{}
			r := &Registry{}

			// Register in "bad" order - this should still work correctly
			r.Register(&TestPluginWithOptDeps{name: "Auth", optDeps: []string{"Storage"}})
			r.Register(&TestPlugin{name: "Storage"})

			err := r.Init(ctx)
			require.NoError(t, err)

			// This is what we want to ensure: Storage still initializes before Auth
			// even though Auth was registered first
			assert.Equal(t, []string{"Storage", "Auth"}, initOrder)
		})
	})
}

type TestShutdownPlugin struct {
	name string
	deps []string
}

func (tp *TestShutdownPlugin) Name() string {
	return tp.name
}

func (tp *TestShutdownPlugin) Deps() []string {
	return tp.deps
}

func (tp *TestShutdownPlugin) Init(ctx context.Context, r *Registry) error {
	initOrder = append(initOrder, tp.name)
	return nil
}

func (tp *TestShutdownPlugin) Shutdown(ctx context.Context) error {
	shutdownOrder = append(shutdownOrder, tp.name)
	return nil
}

var shutdownOrder []string

// TestShutdownOrder verifies that plugins shut down in reverse dependency order.
// If A depends on B, then A should shut down before B (so B is still available).
func TestShutdownOrder(t *testing.T) {
	ctx := t.Context()

	t.Run("shutdown in reverse initialization order", func(t *testing.T) {
		initOrder = []string{}
		shutdownOrder = []string{}
		r := &Registry{}

		// Register in a different order than dependency order
		// to expose the bug
		// D <- C <- B <- A (dependency chain)
		// Register order: D, C, A, B (mixed up)
		// Init order will be: D, C, B, A (dependency order)
		// Current buggy shutdown: D, C, A, B (registration order) - WRONG!
		// Correct shutdown should be: A, B, C, D (reverse init order)
		r.Register(&TestShutdownPlugin{name: "D"})
		r.Register(&TestShutdownPlugin{name: "C", deps: []string{"D"}})
		r.Register(&TestShutdownPlugin{name: "A", deps: []string{"B", "C"}})
		r.Register(&TestShutdownPlugin{name: "B", deps: []string{"C", "D"}})

		err := r.Init(ctx)
		require.NoError(t, err)

		err = r.Shutdown(ctx)
		require.NoError(t, err)

		// Verify init order (dependency order)
		t.Logf("Init order: %v", initOrder)
		assert.Equal(t, []string{"D", "C", "B", "A"}, initOrder)

		// Verify shutdown is in reverse init order
		// A should shut down first (it depends on B and C)
		// Then B (depends on C and D)
		// Then C (depends on D)
		// Finally D (no dependencies)
		t.Logf("Shutdown order: %v", shutdownOrder)
		assert.Equal(t, []string{"A", "B", "C", "D"}, shutdownOrder)
	})

	t.Run("shutdown only affects plugins that implement ShutdownPlugin", func(t *testing.T) {
		initOrder = []string{}
		shutdownOrder = []string{}
		r := &Registry{}

		// Mix of plugins with and without shutdown
		r.Register(&TestShutdownPlugin{name: "A"})
		r.Register(&TestPlugin{name: "B"}) // No shutdown
		r.Register(&TestShutdownPlugin{name: "C"})

		err := r.Init(ctx)
		require.NoError(t, err)

		err = r.Shutdown(ctx)
		require.NoError(t, err)

		// Only A and C should be in shutdown order
		assert.Contains(t, shutdownOrder, "A")
		assert.Contains(t, shutdownOrder, "C")
		assert.NotContains(t, shutdownOrder, "B")
	})
}

// TestGetPlugin verifies type-based plugin retrieval.
func TestGetPlugin(t *testing.T) {
	t.Run("find plugin by type", func(t *testing.T) {
		r := &Registry{}
		plugin := &TestPlugin{name: "test"}
		r.Register(plugin)

		result, ok := GetPlugin[*TestPlugin](r)
		assert.True(t, ok, "should find plugin by type")
		assert.Equal(t, plugin, result)
		assert.Equal(t, "test", result.Name())
	})

	t.Run("plugin not found", func(t *testing.T) {
		r := &Registry{}
		r.Register(&TestPlugin{name: "test"})

		result, ok := GetPlugin[*TestShutdownPlugin](r)
		assert.False(t, ok, "should return false when no plugin matches type")
		assert.Nil(t, result)
	})

	t.Run("empty registry", func(t *testing.T) {
		r := &Registry{}

		result, ok := GetPlugin[*TestPlugin](r)
		assert.False(t, ok, "should return false for empty registry")
		assert.Nil(t, result)
	})

	t.Run("multiple plugins returns first match", func(t *testing.T) {
		r := &Registry{}
		plugin1 := &TestPlugin{name: "first"}
		plugin2 := &TestPlugin{name: "second"}
		other := &TestShutdownPlugin{name: "other"}

		r.Register(other)
		r.Register(plugin1)
		r.Register(plugin2)

		result, ok := GetPlugin[*TestPlugin](r)
		assert.True(t, ok, "should find plugin")
		assert.NotNil(t, result)
		// Should return one of the TestPlugin instances
		assert.True(t, result.Name() == "first" || result.Name() == "second")
	})

	t.Run("find among mixed types", func(t *testing.T) {
		r := &Registry{}
		r.Register(&TestPlugin{name: "a"})
		target := &TestShutdownPlugin{name: "target"}
		r.Register(target)
		r.Register(&TestPlugin{name: "b"})

		result, ok := GetPlugin[*TestShutdownPlugin](r)
		assert.True(t, ok, "should find the shutdown plugin")
		assert.Equal(t, target, result)
		assert.Equal(t, "target", result.Name())
	})
}
