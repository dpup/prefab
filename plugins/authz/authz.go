// Package authz provides a plugin for implementing role-based access control (RBAC).
// It uses protocol buffer annotations to define authorization rules that are
// enforced by a GRPC interceptor.
//
// # Getting Started.
//
// The simplest way to get started with authorization is to define policies and
// register object fetchers and role describers:
//
//	authzPlugin := authz.Plugin(
//		// Define policies (effect, role, action)
//		authz.WithPolicy(authz.Allow, authz.RoleViewer, authz.Action("documents.view")),
//		authz.WithPolicy(authz.Allow, authz.RoleEditor, authz.Action("documents.edit")),
//		authz.WithPolicy(authz.Allow, authz.RoleAdmin, authz.Action("*")),
//
//		// Register object fetcher
//		authz.WithObjectFetcher("document", authz.AsObjectFetcher(
//			authz.Fetcher(db.GetDocumentByID),
//		)),
//
//		// Register role describer
//		authz.WithRoleDescriber("document", authz.Compose(
//			authz.OwnershipRole(authz.RoleOwner, func(d *Document) string {
//				return d.OwnerID
//			}),
//		)),
//	)
//
//	// Add to your Prefab server.
//	server := prefab.New(
//		prefab.WithPlugin(authzPlugin),
//		// Other plugins and options.
//	)
//
// # Core Concepts
//
// Authz Policies are defined in terms of roles and actions, both of which are
// application defined strings. For example, an "editor" role might be allowed
// to perform the "document.edit" action.
//
// Roles are context dependent and determined by application provided functions
// called "Role Describers". Role Describers return a list of roles for a given
// authenticated identity and object. For example, a user may have the role
// "owner" for a specific document and "admin" for their workspace.
//
// Role Describers can chose to restrict whether a role is granted, based on
// other attributes. For example, an "admin" role could only be granted if the
// request comes from a specific IP address.
//
// Role Describers can also be configured to accept a `scope` from the
// request. This is optional and is intended to simplify the implementation of
// multi-tenant systems or systems where a user might be part of multiple
// workspaces or groups, each with different permissions. The scope represents
// the "container" of the object being accessed (e.g., Document=Object, Folder=Scope).
//
// To map an incoming request to a resource, the Authz plugin uses "Object
// Fetchers". Fetchers can be registered against a key, which can be an
// arbitrary string, or derived from `reflect.Type`. The fetcher is then called
// with the value of a request parameter, per the field option.
//
// RPCs can be configured with a default effect of Allow. For example, a page
// might be configured to allow all users to view it, except those on mobile
// devices.
//
// # Protocol Buffer Annotations
//
// To use authorization, you need to annotate your protocol buffer definitions:
//
//	rpc GetDocument(GetDocumentRequest) returns (GetDocumentResponse) {
//	  option (prefab.authz.action) = "documents.view";
//	  option (prefab.authz.resource) = "document";
//	  option (prefab.authz.default_effect) = "deny"; // Optional, defaults to "deny"
//	}
//
//
//
//	message GetDocumentRequest {
//	  string org_id = 1 [(prefab.authz.scope) = true]; // Optional scope (e.g., workspace, org).
//	  string document_id = 2 [(prefab.authz.id) = true]; // Required to identify resource.
//	}
//
// # Common Patterns
//
// This package provides several common patterns to simplify authorization setup:
//
// - Builder pattern: Use `NewBuilder()` for a fluent configuration interface.
// - Predefined roles: `RoleAdmin`, `RoleEditor`, `RoleViewer`, etc.
// - Common CRUD actions: `ActionCreate`, `ActionRead`, etc.
// - Type-safe interfaces: Use the typed helpers for compile-time type safety.
//
// # Role Describer Patterns
//
// The package provides composable, type-safe patterns for building role describers
// that eliminate boilerplate and manual type assertions:
//
//	authz.Compose(
//	    // Grant owner role if user owns the document
//	    authz.OwnershipRole(authz.RoleOwner, func(doc *Document) string {
//	        return doc.OwnerID
//	    }),
//
//	    // Grant viewer role if document is published
//	    authz.StaticRole(authz.RoleViewer, func(_ context.Context, _ auth.Identity, doc *Document) bool {
//	        return doc.Published
//	    }),
//
//	    // Grant roles based on organization membership
//	    authz.MembershipRoles(
//	        func(doc *Document) string { return doc.OrgID },
//	        func(ctx context.Context, orgID string, identity auth.Identity) ([]authz.Role, error) {
//	            org, err := fetchOrg(ctx, orgID)
//	            if err != nil {
//	                return nil, err
//	            }
//	            return org.GetUserRoles(ctx, identity.Subject)
//	        },
//	    ),
//	)
//
// Available patterns:
// - Compose: Combines multiple describers with automatic scope validation
// - OwnershipRole: Grants role if user owns the resource
// - ConditionalRole: Grants role based on async predicate (for database queries)
// - StaticRole: Grants role based on sync predicate (for simple conditions)
// - StaticRoles: Returns multiple roles based on conditions
// - GlobalRole: Grants role based on context only (e.g., superuser checks)
// - MembershipRoles: Grants roles based on parent resource membership
// - ScopeRoles: Grants roles based on scope relationship
//
// See role_patterns.go for detailed documentation on each pattern.
//
// # Object Fetcher Patterns
//
// The package provides composable, type-safe patterns for building object fetchers
// that eliminate boilerplate and manual type assertions:
//
//	// Simple map-based fetcher (common for tests/examples)
//	builder.WithObjectFetcher("document", authz.AsObjectFetcher(
//	    authz.MapFetcher(staticDocuments),
//	))
//
//	// Database fetcher with type safety
//	builder.WithObjectFetcher("user", authz.AsObjectFetcher(
//	    authz.Fetcher(func(ctx context.Context, id string) (*User, error) {
//	        return db.GetUserByID(ctx, id)
//	    }),
//	))
//
//	// Composed fetcher with caching and validation
//	builder.WithObjectFetcher("org", authz.AsObjectFetcher(
//	    authz.ComposeFetchers(
//	        authz.MapFetcher(cache),           // Try cache first
//	        authz.ValidatedFetcher(            // Then validated DB fetch
//	            authz.Fetcher(db.GetOrgByID),
//	            func(org *Org) error {
//	                if org.Deleted {
//	                    return errors.NewC("org deleted", codes.NotFound)
//	                }
//	                return nil
//	            },
//	        ),
//	    ),
//	))
//
// Available patterns:
// - Fetcher: Type-safe wrapper for fetch functions
// - MapFetcher: Fetch from static maps
// - ValidatedFetcher: Add validation to fetched objects
// - ComposeFetchers: Try multiple fetchers in order (cache → DB → API)
// - TransformKey: Transform key before fetching
// - DefaultFetcher: Return default instead of error
//
// See fetcher_patterns.go for detailed documentation on each pattern.
//
// # Role Hierarchy
//
// You can establish a role hierarchy where parent roles inherit child roles:
//
//	authz.WithRoleHierarchy(RoleAdmin, RoleEditor, RoleViewer, RoleUser)
//
// In this example, admins inherit all editor permissions, editors inherit viewer
// permissions, and viewers inherit user permissions.
//
// # Examples
//
// For complete examples, see:
// - examples/authz/custom/authzexample.go (fully custom configuration)
// - examples/authz/builder/authzexample.go (common builder pattern)
// - examples/authz/patterns/authzexample.go (role describer patterns)
package authz

import (
	"context"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/serverutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

type Role string

type Action string

// Scope defines the context in which authorization occurs (e.g., organization, workspace).
type Scope string

type Effect int

const (
	Deny Effect = iota
	Allow
)

func (e Effect) Reverse() Effect {
	if e == Deny {
		return Allow
	}
	return Deny
}

func (e Effect) String() string {
	if e == Deny {
		return "DENY"
	}
	return "ALLOW"
}

type effectList []Effect

// Combine returns the combined effect using AWS IAM-style precedence:
// 1. Explicit Deny always wins (security first)
// 2. Explicit Allow wins if no Deny exists
// 3. Default effect if no policies match
//
// This provides intuitive, predictable authorization:
// - Deny policies can block access even if other roles grant it
// - Allow policies grant access unless explicitly denied
// - Safe defaults protect resources when no policies match
func (e effectList) Combine(defaultEffect Effect) Effect {
	if len(e) == 0 {
		return defaultEffect
	}

	// Step 1: Explicit Deny always wins
	for _, effect := range e {
		if effect == Deny {
			return Deny
		}
	}

	// Step 2: If no Deny, any Allow wins
	for _, effect := range e {
		if effect == Allow {
			return Allow
		}
	}

	// Step 3: No policies matched (should not reach here if effects is non-empty)
	return defaultEffect
}

// AuthzObject is the base interface for all objects used in authorization. While
// not strictly necessary, it is recommended to implement this interface for
// type safety.
type AuthzObject interface {
	// AuthzType returns a string identifier for the object type
	AuthzType() string
}

// OwnedObject represents objects that have an owner.
type OwnedObject interface {
	AuthzObject
	// OwnerID returns the ID of the object's owner
	OwnerID() string
}

// ScopedObject represents objects that belong to a specific scope.
type ScopedObject interface {
	AuthzObject
	// ScopeID returns the scope ID
	ScopeID() string
}

// ObjectFetcher is an interface for fetching objects based on a request parameter.
type ObjectFetcher interface {
	// FetchObject retrieves an object based on the provided key
	FetchObject(ctx context.Context, key any) (any, error)
}

// RoleDescriber is an interface for describing roles relative to a type.
type RoleDescriber interface {
	// DescribeRoles determines the roles a subject has relative to an object in a scope
	DescribeRoles(ctx context.Context, subject auth.Identity, object any, scope Scope) ([]Role, error)
}

// TypedObjectFetcher is a function type for fetching objects with type safety.
type TypedObjectFetcher[K comparable, T any] func(ctx context.Context, key K) (T, error)

// TypedRoleDescriber is a function type for describing roles with type safety.
type TypedRoleDescriber[T any] func(ctx context.Context, subject auth.Identity, object T, scope Scope) ([]Role, error)

// MethodOptions returns Authz related method options from the method descriptor.
// associated with the given info..
func MethodOptions(info *grpc.UnaryServerInfo) (objectKey string, action Action, defaultEffect Effect) {
	if v, ok := serverutil.MethodOption(info, E_Resource); ok {
		objectKey = v.(string)
	} else {
		objectKey = "*"
	}
	if v, ok := serverutil.MethodOption(info, E_Action); ok {
		action = Action(v.(string))
	}
	if v, ok := serverutil.MethodOption(info, E_DefaultEffect); ok {
		switch v.(string) {
		case "allow":
			defaultEffect = Allow
		case "deny":
			defaultEffect = Deny
		default:
			// TODO: Consider erroring instead of defaulting to deny.
			defaultEffect = Deny
		}
	}
	return
}

// FieldOptions returns proto fields that are tagged with Authz related options.
// It returns the object ID and scope string.
func FieldOptions(req proto.Message) (any, string, error) {
	var id any
	var scope string
	if v, ok := serverutil.FieldOption(req, E_Id); ok {
		if len(v) != 1 {
			return "", "", errors.Codef(codes.Internal, "authz error: require exactly one id on request descriptor: %s", req.ProtoReflect().Descriptor().FullName())
		}
		id = v[0].FieldValue
	}

	// Keep checking for deprecated domain tag.
	if v, ok := serverutil.FieldOption(req, E_Domain); ok {
		if len(v) != 1 {
			return "", "", errors.Codef(codes.Internal, "authz error: expected exactly one domain on request descriptor: %s", req.ProtoReflect().Descriptor().FullName())
		}
		scope = v[0].FieldValue.(string)
	}
	// End deprecation.

	if v, ok := serverutil.FieldOption(req, E_Scope); ok {
		if len(v) != 1 {
			return "", "", errors.Codef(codes.Internal, "authz error: expected exactly one scope on request descriptor: %s", req.ProtoReflect().Descriptor().FullName())
		}
		scope = v[0].FieldValue.(string)
	}
	return id, scope, nil
}
