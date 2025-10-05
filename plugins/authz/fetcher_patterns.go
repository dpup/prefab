package authz

import (
	"context"

	"github.com/dpup/prefab/errors"

	"google.golang.org/grpc/codes"
)

// Fetcher creates a type-safe object fetcher from a function.
// This is the foundational pattern - it wraps your fetch logic with type safety,
// eliminating manual type assertions.
//
// Use this when you have a function that fetches an object by ID, whether from
// a database, API, cache, or any other source.
//
// Example:
//
//	// Database fetch
//	authz.Fetcher(func(ctx context.Context, id string) (*Document, error) {
//	    return db.GetDocumentByID(ctx, id)
//	})
//
//	// Or pass the function directly if signatures match
//	authz.Fetcher(db.GetDocumentByID)
func Fetcher[K comparable, T any](fetch func(context.Context, K) (T, error)) TypedObjectFetcher[K, T] {
	return fetch
}

// MapFetcher creates an object fetcher from a static map.
// This is commonly used in examples and tests, or for small static datasets.
//
// Returns NotFound error if the key doesn't exist in the map.
//
// Example:
//
//	staticDocuments := map[string]*Document{
//	    "1": {ID: "1", Title: "Doc 1"},
//	    "2": {ID: "2", Title: "Doc 2"},
//	}
//	authz.MapFetcher(staticDocuments)
func MapFetcher[K comparable, T any](m map[K]T) TypedObjectFetcher[K, T] {
	return func(ctx context.Context, key K) (T, error) {
		if val, ok := m[key]; ok {
			return val, nil
		}
		var zero T
		return zero, errors.Codef(codes.NotFound, "object not found for key: %v", key)
	}
}

// ValidatedFetcher wraps an object fetcher with validation logic.
// The validator is called after a successful fetch and can reject the object
// by returning an error.
//
// This is useful for enforcing additional constraints like soft-deletes,
// status checks, or business rules.
//
// Example:
//
//	authz.ValidatedFetcher(
//	    authz.Fetcher(db.GetDocumentByID),
//	    func(doc *Document) error {
//	        if doc.Deleted {
//	            return errors.NewC("document deleted", codes.NotFound)
//	        }
//	        if doc.Archived {
//	            return errors.NewC("document archived", codes.PermissionDenied)
//	        }
//	        return nil
//	    },
//	)
func ValidatedFetcher[K comparable, T any](
	fetcher TypedObjectFetcher[K, T],
	validate func(T) error,
) TypedObjectFetcher[K, T] {
	return func(ctx context.Context, key K) (T, error) {
		obj, err := fetcher(ctx, key)
		if err != nil {
			return obj, err
		}
		if err := validate(obj); err != nil {
			var zero T
			return zero, err
		}
		return obj, nil
	}
}

// ComposeFetchers tries multiple fetchers in order until one succeeds.
// This is useful for implementing fallback strategies like:
// - Try cache, then database
// - Try primary database, then replica
// - Try local store, then remote API
//
// Returns the first successful result, or the last error if all fail.
//
// Example:
//
//	authz.ComposeFetchers(
//	    authz.MapFetcher(cache),           // Try cache first
//	    authz.Fetcher(db.GetDocumentByID), // Then database
//	    authz.Fetcher(api.FetchDocument),  // Finally remote API
//	)
func ComposeFetchers[K comparable, T any](fetchers ...TypedObjectFetcher[K, T]) TypedObjectFetcher[K, T] {
	return func(ctx context.Context, key K) (T, error) {
		var lastErr error
		for _, fetcher := range fetchers {
			obj, err := fetcher(ctx, key)
			if err == nil {
				return obj, nil
			}
			lastErr = err
		}
		var zero T
		return zero, lastErr
	}
}

// TransformKey wraps a fetcher to transform the key before fetching.
// This is useful when the incoming key format differs from what the fetcher expects.
//
// Example:
//
//	// Convert string IDs to int IDs
//	authz.TransformKey(
//	    func(id string) int { return parseID(id) },
//	    authz.Fetcher(db.GetDocumentByNumericID),
//	)
func TransformKey[K1, K2 comparable, T any](
	transform func(K1) K2,
	fetcher TypedObjectFetcher[K2, T],
) TypedObjectFetcher[K1, T] {
	return func(ctx context.Context, key K1) (T, error) {
		transformedKey := transform(key)
		return fetcher(ctx, transformedKey)
	}
}

// DefaultFetcher wraps a fetcher to return a default value instead of an error.
// This is useful for optional resources or graceful degradation.
//
// Example:
//
//	// Return a guest user if user not found
//	authz.DefaultFetcher(
//	    authz.Fetcher(db.GetUserByID),
//	    &User{ID: "guest", Name: "Guest User"},
//	)
func DefaultFetcher[K comparable, T any](
	fetcher TypedObjectFetcher[K, T],
	defaultValue T,
) TypedObjectFetcher[K, T] {
	return func(ctx context.Context, key K) (T, error) {
		obj, err := fetcher(ctx, key)
		if err != nil {
			return defaultValue, nil //nolint:nilerr // intentionally returning default value on error
		}
		return obj, nil
	}
}
