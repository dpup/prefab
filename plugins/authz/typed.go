package authz

import (
	"context"

	"github.com/dpup/prefab/plugins/auth"
)

// Package level functions to create typed fetchers and describers.

// WithTypedObjectFetcher adds a type-safe object fetcher to the builder..
// Since Go doesn't support generic methods, this is provided as a package function..
func WithTypedObjectFetcher[K comparable, T any](objectKey string, fetcher TypedObjectFetcher[K, T]) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterObjectFetcher(objectKey, AsObjectFetcher(fetcher))
	}
}

// WithTypedRoleDescriber adds a type-safe role describer to the builder..
// Since Go doesn't support generic methods, this is provided as a package function..
func WithTypedRoleDescriber[T any](objectKey string, describer TypedRoleDescriber[T]) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterRoleDescriber(objectKey, AsRoleDescriber(describer))
	}
}

// WithTypeRegistration registers a role describer and object fetcher for a specific type..
// Since Go doesn't support generic methods, this is provided as a package function..
func WithTypeRegistration[K comparable, T any](objectKey string, fetcher TypedObjectFetcher[K, T], describer TypedRoleDescriber[T]) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterObjectFetcher(objectKey, AsObjectFetcher(fetcher))
		ap.RegisterRoleDescriber(objectKey, AsRoleDescriber(describer))
	}
}

// Extension methods for the builder to use the typed functions.

// WithTypedObjectFetcher adds a type-safe object fetcher using a helper function.
func (b *Builder) WithTypedObjectFetcher(objectKey string, fetcher interface{}) *Builder {
	switch f := fetcher.(type) {
	case ObjectFetcher:
		b.plugin.RegisterObjectFetcher(objectKey, f)
	default:
		// The actual generic function is handled via the package function
		// This is just a proxy method to maintain the fluent interface
		panic("WithTypedObjectFetcher must be used with package function, not directly on Builder")
	}
	return b
}

// WithTypedRoleDescriber adds a type-safe role describer using a helper function.
func (b *Builder) WithTypedRoleDescriber(objectKey string, describer interface{}) *Builder {
	switch d := describer.(type) {
	case RoleDescriber:
		b.plugin.RegisterRoleDescriber(objectKey, d)
	default:
		// The actual generic function is handled via the package function
		// This is just a proxy method to maintain the fluent interface
		panic("WithTypedRoleDescriber must be used with package function, not directly on Builder")
	}
	return b
}

// TypedBuilder is a specialized builder wrapper for working with a specific type.
// This allows for compile-time type checking.
type TypedBuilder[T any] struct {
	builder *Builder
}

// NewTypedBuilder creates a new typed builder for a specific type.
func NewTypedBuilder[T any]() *TypedBuilder[T] {
	return &TypedBuilder[T]{
		builder: NewBuilder(),
	}
}

// Build finalizes the typed builder and returns the plugin.
func (tb *TypedBuilder[T]) Build() *AuthzPlugin {
	return tb.builder.Build()
}

// WithRoleHierarchy adds a role hierarchy to the builder.
func (tb *TypedBuilder[T]) WithRoleHierarchy(roles ...Role) *TypedBuilder[T] {
	tb.builder.WithRoleHierarchy(roles...)
	return tb
}

// WithPolicy adds a policy to the builder.
func (tb *TypedBuilder[T]) WithPolicy(effect Effect, role Role, action Action) *TypedBuilder[T] {
	tb.builder.WithPolicy(effect, role, action)
	return tb
}

// Go doesn't support generic methods on generic types, so we need specialized methods.

// WithStringObjectFetcher adds a type-safe object fetcher with string keys.
func (tb *TypedBuilder[T]) WithStringObjectFetcher(objectKey string, fetcher func(ctx context.Context, key string) (T, error)) *TypedBuilder[T] {
	tb.builder.plugin.RegisterObjectFetcher(objectKey, AsObjectFetcher(TypedObjectFetcher[string, T](fetcher)))
	return tb
}

// WithIntObjectFetcher adds a type-safe object fetcher with int keys.
func (tb *TypedBuilder[T]) WithIntObjectFetcher(objectKey string, fetcher func(ctx context.Context, key int) (T, error)) *TypedBuilder[T] {
	tb.builder.plugin.RegisterObjectFetcher(objectKey, AsObjectFetcher(TypedObjectFetcher[int, T](fetcher)))
	return tb
}

// WithRoleDescriber adds a type-safe role describer.
func (tb *TypedBuilder[T]) WithRoleDescriber(objectKey string, describer func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error)) *TypedBuilder[T] {
	tb.builder.plugin.RegisterRoleDescriber(objectKey, AsRoleDescriber(TypedRoleDescriber[T](describer)))
	return tb
}
