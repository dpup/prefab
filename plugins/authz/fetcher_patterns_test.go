package authz_test

import (
	"context"
	"testing"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/authz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

// Test types for fetcher patterns
type testObject struct {
	id      string
	name    string
	deleted bool
}

func TestFetcher(t *testing.T) {
	t.Run("creates type-safe fetcher from function", func(t *testing.T) {
		fetcher := authz.Fetcher(func(ctx context.Context, id string) (*testObject, error) {
			if id == "1" {
				return &testObject{id: "1", name: "Object 1"}, nil
			}
			return nil, errors.NewC("not found", codes.NotFound)
		})

		obj, err := fetcher(t.Context(), "1")
		require.NoError(t, err)
		assert.Equal(t, "1", obj.id)
		assert.Equal(t, "Object 1", obj.name)
	})

	t.Run("propagates errors from fetch function", func(t *testing.T) {
		expectedErr := errors.NewC("not found", codes.NotFound)
		fetcher := authz.Fetcher(func(ctx context.Context, id string) (*testObject, error) {
			return nil, expectedErr
		})

		obj, err := fetcher(t.Context(), "missing")
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, obj)
	})
}

func TestMapFetcher(t *testing.T) {
	staticObjects := map[string]*testObject{
		"1": {id: "1", name: "Object 1"},
		"2": {id: "2", name: "Object 2"},
	}

	fetcher := authz.MapFetcher(staticObjects)

	t.Run("fetches existing object from map", func(t *testing.T) {
		obj, err := fetcher(t.Context(), "1")
		require.NoError(t, err)
		assert.Equal(t, "1", obj.id)
		assert.Equal(t, "Object 1", obj.name)
	})

	t.Run("returns NotFound for missing key", func(t *testing.T) {
		obj, err := fetcher(t.Context(), "missing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, obj)
	})

	t.Run("works with different key types", func(t *testing.T) {
		intMap := map[int]*testObject{
			1: {id: "1", name: "Object 1"},
			2: {id: "2", name: "Object 2"},
		}
		intFetcher := authz.MapFetcher(intMap)

		obj, err := intFetcher(t.Context(), 1)
		require.NoError(t, err)
		assert.Equal(t, "1", obj.id)
	})
}

func TestValidatedFetcher(t *testing.T) {
	baseFetcher := authz.MapFetcher(map[string]*testObject{
		"1": {id: "1", name: "Active", deleted: false},
		"2": {id: "2", name: "Deleted", deleted: true},
	})

	validatedFetcher := authz.ValidatedFetcher(
		baseFetcher,
		func(obj *testObject) error {
			if obj.deleted {
				return errors.NewC("object deleted", codes.NotFound)
			}
			return nil
		},
	)

	t.Run("returns valid object", func(t *testing.T) {
		obj, err := validatedFetcher(t.Context(), "1")
		require.NoError(t, err)
		assert.Equal(t, "Active", obj.name)
	})

	t.Run("rejects invalid object", func(t *testing.T) {
		obj, err := validatedFetcher(t.Context(), "2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deleted")
		assert.Nil(t, obj)
	})

	t.Run("propagates fetch errors", func(t *testing.T) {
		obj, err := validatedFetcher(t.Context(), "missing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, obj)
	})
}

func TestComposeFetchers(t *testing.T) {
	cache := map[string]*testObject{
		"1": {id: "1", name: "Cached Object"},
	}

	database := map[string]*testObject{
		"2": {id: "2", name: "DB Object"},
	}

	api := map[string]*testObject{
		"3": {id: "3", name: "API Object"},
	}

	composed := authz.ComposeFetchers(
		authz.MapFetcher(cache),
		authz.MapFetcher(database),
		authz.MapFetcher(api),
	)

	t.Run("tries cache first", func(t *testing.T) {
		obj, err := composed(t.Context(), "1")
		require.NoError(t, err)
		assert.Equal(t, "Cached Object", obj.name)
	})

	t.Run("falls back to database if not in cache", func(t *testing.T) {
		obj, err := composed(t.Context(), "2")
		require.NoError(t, err)
		assert.Equal(t, "DB Object", obj.name)
	})

	t.Run("falls back to API if not in cache or database", func(t *testing.T) {
		obj, err := composed(t.Context(), "3")
		require.NoError(t, err)
		assert.Equal(t, "API Object", obj.name)
	})

	t.Run("returns last error if all fetchers fail", func(t *testing.T) {
		obj, err := composed(t.Context(), "missing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, obj)
	})
}

func TestTransformKey(t *testing.T) {
	// Fetcher that expects int keys
	intFetcher := authz.MapFetcher(map[int]*testObject{
		1: {id: "1", name: "Object 1"},
		2: {id: "2", name: "Object 2"},
	})

	// Transform string keys to int keys
	stringFetcher := authz.TransformKey(
		func(id string) int {
			switch id {
			case "one":
				return 1
			case "two":
				return 2
			default:
				return 0
			}
		},
		intFetcher,
	)

	t.Run("transforms key and fetches object", func(t *testing.T) {
		obj, err := stringFetcher(t.Context(), "one")
		require.NoError(t, err)
		assert.Equal(t, "1", obj.id)
		assert.Equal(t, "Object 1", obj.name)
	})

	t.Run("returns error for invalid transformed key", func(t *testing.T) {
		obj, err := stringFetcher(t.Context(), "invalid")
		require.Error(t, err)
		assert.Nil(t, obj)
	})
}

func TestDefaultFetcher(t *testing.T) {
	baseFetcher := authz.MapFetcher(map[string]*testObject{
		"1": {id: "1", name: "Object 1"},
	})

	defaultObj := &testObject{id: "default", name: "Default Object"}
	fetcherWithDefault := authz.DefaultFetcher(baseFetcher, defaultObj)

	t.Run("returns fetched object when found", func(t *testing.T) {
		obj, err := fetcherWithDefault(t.Context(), "1")
		require.NoError(t, err)
		assert.Equal(t, "1", obj.id)
		assert.Equal(t, "Object 1", obj.name)
	})

	t.Run("returns default when object not found", func(t *testing.T) {
		obj, err := fetcherWithDefault(t.Context(), "missing")
		require.NoError(t, err)
		assert.Equal(t, "default", obj.id)
		assert.Equal(t, "Default Object", obj.name)
	})
}

// Integration test: Real-world composition
func TestRealWorldComposition(t *testing.T) {
	// Simulate cache
	cache := map[string]*testObject{
		"cached": {id: "cached", name: "Cached", deleted: false},
	}

	// Simulate database
	database := map[string]*testObject{
		"cached":  {id: "cached", name: "Cached", deleted: false},
		"active":  {id: "active", name: "Active", deleted: false},
		"deleted": {id: "deleted", name: "Deleted", deleted: true},
	}

	// Build a realistic fetcher: cache → database with validation
	fetcher := authz.ComposeFetchers(
		// Try cache first
		authz.MapFetcher(cache),

		// Fall back to validated database fetch
		authz.ValidatedFetcher(
			authz.MapFetcher(database),
			func(obj *testObject) error {
				if obj.deleted {
					return errors.NewC("object deleted", codes.NotFound)
				}
				return nil
			},
		),
	)

	t.Run("fetches from cache when available", func(t *testing.T) {
		obj, err := fetcher(t.Context(), "cached")
		require.NoError(t, err)
		assert.Equal(t, "Cached", obj.name)
	})

	t.Run("fetches from database when not in cache", func(t *testing.T) {
		obj, err := fetcher(t.Context(), "active")
		require.NoError(t, err)
		assert.Equal(t, "Active", obj.name)
	})

	t.Run("validates database objects", func(t *testing.T) {
		obj, err := fetcher(t.Context(), "deleted")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deleted")
		assert.Nil(t, obj)
	})

	t.Run("returns error when not found anywhere", func(t *testing.T) {
		obj, err := fetcher(t.Context(), "missing")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Nil(t, obj)
	})
}

// Test integration with authz registration
func TestFetcherWithAuthzPlugin(t *testing.T) {
	staticObjects := map[string]*testObject{
		"1": {id: "1", name: "Object 1"},
		"2": {id: "2", name: "Object 2"},
	}

	plugin := authz.Plugin(
		// Register using the new pattern
		authz.WithObjectFetcher("test", authz.AsObjectFetcher(
			authz.MapFetcher(staticObjects),
		)),
	)

	// Verify the fetcher was registered correctly
	assert.NotNil(t, plugin)
}
