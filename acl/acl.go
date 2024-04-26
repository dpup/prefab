// Package acl provides a plugin for implementing basic access controls. It uses
// the service descriptor to define the ACL, which is then enforced by a GRPC
// interceptor.
//
// ACL Policies are defined in terms of roles and actions, both of which are
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
// Role Describers can also be configured to accept a `domain_id` from the
// request. This is optional and is intended to simplify the implementation of
// mutlti-tenant systems or systems where a user might be part of multiple
// workspaces or groups, each with different permissions.
//
// To map an incoming request to a resource, the ACL plugin uses "Object
// Fetchers". Fetchers can be registered against a key, which can be an
// arbitrary string, or derived from `reflect.Type`. The fetcher is then called
// with the value of a request parameter, per the field option.
//
// RPCs can be configured with a default effect of Allow. For example, a page
// might be configured to allow all users to view it, except those on mobile
// devices (this is a bit of a tenuous example, but you get the idea).
package acl

import (
	"context"

	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/serverutil"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

type Role string

type Action string

type Domain string

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

// Combine returns the combined effect of a list of effects. If the list is empty,
// the default effect is returned. If the list contains the default effect, the
// default effect is returned. Otherwise, the default effect is reversed.
//
// In otherwords, the entire list must be the reverse of the default effect for
// it to override the default.
func (e effectList) Combine(defaultEffect Effect) Effect {
	if len(e) == 0 {
		return defaultEffect
	}
	for _, effect := range e {
		if effect == defaultEffect {
			return defaultEffect
		}
	}
	return defaultEffect.Reverse()
}

// Fetches an object based on a request parameter.
type ObjectFetcher func(ctx context.Context, key any) (any, error)

// Describes a role relative to a type.
type RoleDescriber func(ctx context.Context, subject auth.Identity, object any, domain Domain) ([]Role, error)

// MethodOptions returns ACL related method options from the method descriptor
// associated with the given info.
func MethodOptions(info *grpc.UnaryServerInfo) (objectKey string, action Action, defaultEffect Effect) {
	if v, ok := serverutil.MethodOption(info, E_ObjectKey); ok {
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

// FieldOptions returns proto fields that are tagged with ACL related options.
func FieldOptions(req proto.Message) (string, string, error) {
	var objectID, domainID string
	if v, ok := serverutil.FieldOption(req, E_ObjectId); ok {
		if len(v) != 1 {
			return "", "", errors.Codef(codes.Internal, "acl error: require exactly one object_id on request descriptor: %s", req.ProtoReflect().Descriptor().FullName())
		}
		objectID = v[0].FieldValue.(string)
	} else {
		return "", "", errors.Codef(codes.Internal, "acl error: object_id required on request descriptor: %s", req.ProtoReflect().Descriptor().FullName())
	}
	if v, ok := serverutil.FieldOption(req, E_DomainId); ok {
		if len(v) != 1 {
			return "", "", errors.Codef(codes.Internal, "acl error: expected exactly one domain_id on request descriptor: %s", req.ProtoReflect().Descriptor().FullName())
		}
		domainID = v[0].FieldValue.(string)
	}
	return objectID, domainID, nil
}
