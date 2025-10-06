package authz

import (
	"context"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"

	"google.golang.org/grpc/codes"
)

// Compose combines multiple typed role describers into a single RoleDescriber.
// All describers are called and their results are merged.
//
// If the object implements ScopedObject, the scope is automatically validated
// before calling any describers. If the scope doesn't match, an empty role list
// is returned.
//
// Type safety: The generic type T ensures all describers operate on the same
// object type, eliminating manual type assertions.
//
// Example:
//
//	authz.Compose(
//	    authz.RoleOwner.IfOwner(func(doc *Document) string { return doc.OwnerID }),
//	    authz.StaticRole(authz.RoleViewer, func(_ context.Context, _ auth.Identity, doc *Document) bool {
//	        return doc.Published
//	    }),
//	)
func Compose[T any](describers ...TypedRoleDescriber[T]) RoleDescriber {
	return RoleDescriberFn(func(ctx context.Context, subject auth.Identity, object any, scope Scope) ([]Role, error) {
		// Type assertion with helpful error message
		typed, ok := object.(T)
		if !ok {
			var zero T
			return nil, errors.Codef(codes.Internal, "authz: expected type %T, got %T", zero, object)
		}

		// Auto-validate scope if object implements ScopedObject
		if scoped, ok := any(typed).(ScopedObject); ok {
			if scoped.ScopeID() != string(scope) {
				// Scope mismatch - return empty roles, not an error
				// This allows Compose to work with objects from different scopes
				return []Role{}, nil
			}
		}

		// Call all describers and merge results
		var allRoles []Role
		for _, describer := range describers {
			roles, err := describer(ctx, subject, typed, scope)
			if err != nil {
				return nil, err
			}
			allRoles = append(allRoles, roles...)
		}
		return allRoles, nil
	})
}

// ConditionalRole grants a role if an async predicate returns true.
// Use this pattern when role assignment requires I/O, database queries, or
// other async operations.
//
// Example:
//
//	const author = authz.Role("author")
//	author.When(func(ctx context.Context, subject auth.Identity, note *Note, scope authz.Scope) (bool, error) {
//	    user, err := fetchUser(ctx, subject.Subject)
//	    if err != nil {
//	        return false, err
//	    }
//	    return note.CreatedBy == user.ID, nil
//	})
func ConditionalRole[T any](role Role, predicate func(context.Context, auth.Identity, T, Scope) (bool, error)) TypedRoleDescriber[T] {
	return func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error) {
		match, err := predicate(ctx, subject, object, scope)
		if err != nil {
			return nil, err
		}
		if match {
			return []Role{role}, nil
		}
		return nil, nil
	}
}

// StaticRole grants a role if a sync predicate returns true.
// Use this pattern when role assignment is based purely on object attributes
// without requiring async operations.
//
// Example:
//
//	authz.StaticRole(authz.RoleViewer, func(_ context.Context, _ auth.Identity, doc *Document) bool {
//	    return doc.Published
//	})
func StaticRole[T any](role Role, predicate func(context.Context, auth.Identity, T) bool) TypedRoleDescriber[T] {
	return func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error) {
		if predicate(ctx, subject, object) {
			return []Role{role}, nil
		}
		return nil, nil
	}
}

// StaticRoles returns multiple roles based on object attributes.
// Use this pattern when you need to return different roles based on conditions.
//
// Example:
//
//	authz.StaticRoles(func(_ context.Context, subject auth.Identity, pr *PullRequest) []authz.Role {
//	    var roles []authz.Role
//	    for _, reviewer := range pr.Reviewers {
//	        if reviewer == subject.Subject {
//	            roles = append(roles, "reviewer")
//	        }
//	    }
//	    return roles
//	})
func StaticRoles[T any](getRoles func(context.Context, auth.Identity, T) []Role) TypedRoleDescriber[T] {
	return func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error) {
		return getRoles(ctx, subject, object), nil
	}
}

// GlobalRole grants a role based on context only, ignoring object and subject.
// This is useful for global overrides like superuser checks.
//
// Example:
//
//	const superuser = authz.Role("superuser")
//	superuser.IfGlobal(func(ctx context.Context) bool {
//	    return isSuperUser(ctx, secret)
//	})
func GlobalRole[T any](role Role, predicate func(context.Context) bool) TypedRoleDescriber[T] {
	return func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error) {
		if predicate(ctx) {
			return []Role{role}, nil
		}
		return nil, nil
	}
}

// OwnershipRole grants a role if the subject owns the object.
// This is a common pattern for granting elevated permissions to resource creators.
// Returns no roles for anonymous users (zero-value Identity).
//
// Example:
//
//	authz.RoleOwner.IfOwner(func(doc *Document) string {
//	    return doc.OwnerID
//	})
func OwnershipRole[T any](role Role, getOwnerID func(T) string) TypedRoleDescriber[T] {
	return StaticRole(role, func(_ context.Context, subject auth.Identity, object T) bool {
		if subject == (auth.Identity{}) {
			return false
		}
		return getOwnerID(object) == subject.Subject
	})
}

// IdentityOwnershipRole grants a role when the identity resolves to the object's owner.
// This pattern handles cases where identity-to-user mapping is workspace-scoped and requires
// async resolution (e.g., database lookup).
// Returns no roles for anonymous users (zero-value Identity).
//
// Unlike OwnershipRole which does simple string comparison (identity.Subject == ownerID),
// this pattern:
// - Supports async identity-to-user resolution (e.g., database lookup)
// - Handles workspace/scope-scoped user identities
// - Gracefully handles NotFound errors when identity isn't mapped in the workspace
//
// Example (workspace-scoped users):
//
//	authz.IdentityOwnershipRole(authz.RoleOwner,
//	    func(ctx context.Context, subject auth.Identity, note *Note) (string, error) {
//	        // Resolve identity to workspace-scoped user ID
//	        q := models.FromContext(ctx)
//	        viewer, err := q.UserByIdentityAndWorkspace(ctx, &models.UserByIdentityAndWorkspaceParams{
//	            WorkspaceID:      note.WorkspaceID,
//	            IdentitySub:      subject.Subject,
//	            IdentityProvider: subject.Provider,
//	        })
//	        if err != nil {
//	            return "", err
//	        }
//	        return viewer.ID, nil
//	    },
//	    func(note *Note) string { return note.OwnerID },
//	)
func IdentityOwnershipRole[T any](
	role Role,
	resolveUserID func(context.Context, auth.Identity, T) (string, error),
	getOwnerID func(T) string,
) TypedRoleDescriber[T] {
	return func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error) {
		if subject == (auth.Identity{}) {
			return []Role{}, nil
		}

		userID, err := resolveUserID(ctx, subject, object)
		if err != nil {
			// Handle NotFound gracefully - identity not mapped in this workspace
			if errors.Code(err) == codes.NotFound {
				return []Role{}, nil
			}
			return nil, err
		}

		if userID == getOwnerID(object) {
			return []Role{role}, nil
		}
		return []Role{}, nil
	}
}

// MembershipRoles returns roles based on membership in a parent object.
// The parent object is identified by a parent ID extracted from the current object.
// Returns no roles for anonymous users (zero-value Identity).
//
// This pattern is useful when roles are inherited from a parent resource
// (e.g., organization membership grants roles on all org documents).
//
// Example:
//
//	authz.MembershipRoles(
//	    func(doc *Document) string { return doc.OrgID },
//	    func(ctx context.Context, orgID string, subject auth.Identity) ([]authz.Role, error) {
//	        org, err := fetchOrg(ctx, orgID)
//	        if err != nil {
//	            return nil, err
//	        }
//	        return org.GetUserRoles(ctx, subject.Subject)
//	    },
//	)
func MembershipRoles[T any](getParentID func(T) string, getRoles func(context.Context, string, auth.Identity) ([]Role, error)) TypedRoleDescriber[T] {
	return func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error) {
		if subject == (auth.Identity{}) {
			return []Role{}, nil
		}
		parentID := getParentID(object)
		return getRoles(ctx, parentID, subject)
	}
}

// ScopeRoles returns roles based on the subject's relationship to the scope.
// Unlike MembershipRoles which uses a parent ID from the object, this uses
// the scope parameter directly.
// Returns no roles for anonymous users (zero-value Identity).
//
// This pattern is useful when the scope represents the "owner" of the object
// (e.g., Document=Object, Folder=Scope) and you want to grant roles based on
// the user's relationship to that scope.
//
// Example:
//
//	authz.ScopeRoles(
//	    func(note *Note) string { return note.WorkspaceID },
//	    func(ctx context.Context, workspaceID string, subject auth.Identity) ([]authz.Role, error) {
//	        return workspaceRoleDescriber.DescribeRoles(ctx, subject, workspaceID, authz.Scope(workspaceID))
//	    },
//	)
func ScopeRoles[T any](getScopeID func(T) string, getRoles func(context.Context, string, auth.Identity) ([]Role, error)) TypedRoleDescriber[T] {
	return func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error) {
		if subject == (auth.Identity{}) {
			return []Role{}, nil
		}
		scopeID := getScopeID(object)
		// Validate scope matches
		if string(scope) != scopeID {
			return nil, errors.Codef(codes.Internal, "authz: scope mismatch: expected %s, got %s", scopeID, scope)
		}
		return getRoles(ctx, scopeID, subject)
	}
}

// ValidateScope wraps a role describer to validate scope before calling it.
// If the object implements ScopedObject and the scope doesn't match, returns
// empty roles. Otherwise, delegates to the wrapped describer.
//
// Note: Compose already does this automatically, so this is only needed if
// you're not using Compose.
//
// Example:
//
//	authz.ValidateScope(myDescriber)
func ValidateScope[T any](describer TypedRoleDescriber[T]) TypedRoleDescriber[T] {
	return func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error) {
		// Validate scope if object implements ScopedObject
		if scoped, ok := any(object).(ScopedObject); ok {
			if scoped.ScopeID() != string(scope) {
				return []Role{}, nil
			}
		}
		return describer(ctx, subject, object, scope)
	}
}
