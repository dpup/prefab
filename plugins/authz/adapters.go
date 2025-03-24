package authz

import (
	"context"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
)

// RoleDescriberFn adapts a function to the RoleDescriber interface..
type RoleDescriberFn func(ctx context.Context, subject auth.Identity, object any, scope Scope) ([]Role, error)

// DescribeRoles implements the RoleDescriber interface.
func (f RoleDescriberFn) DescribeRoles(ctx context.Context, subject auth.Identity, object any, scope Scope) ([]Role, error) {
	return f(ctx, subject, object, scope)
}

// ObjectFetcherFn adapts a function to the ObjectFetcher interface.
type ObjectFetcherFn func(ctx context.Context, key any) (any, error)

// FetchObject implements the ObjectFetcher interface.
func (f ObjectFetcherFn) FetchObject(ctx context.Context, key any) (any, error) {
	return f(ctx, key)
}

// AsRoleDescriber converts a TypedRoleDescriber to the RoleDescriber interface.
func AsRoleDescriber[T any](describer TypedRoleDescriber[T]) RoleDescriber {
	return RoleDescriberFn(func(ctx context.Context, subject auth.Identity, object any, scope Scope) ([]Role, error) {
		typedObject, ok := object.(T)
		if !ok {
			return nil, errors.Codef(0, "expected object type %T, got %T", *new(T), object)
		}
		return describer(ctx, subject, typedObject, scope)
	})
}

// AsObjectFetcher converts a TypedObjectFetcher to the ObjectFetcher interface.
func AsObjectFetcher[K comparable, T any](fetcher TypedObjectFetcher[K, T]) ObjectFetcher {
	return ObjectFetcherFn(func(ctx context.Context, key any) (any, error) {
		typedKey, ok := key.(K)
		if !ok {
			return nil, errors.Codef(0, "expected key type %T, got %T", *new(K), key)
		}
		return fetcher(ctx, typedKey)
	})
}
