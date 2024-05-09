package acl

import (
	"context"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/auth"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

// Constant name for identifying the core ACL plugin
const PluginName = "acl"

var (
	ErrPermissionDenied = errors.Codef(codes.PermissionDenied, "you are not authorized to perform this action")
	ErrUnauthenticated  = errors.Codef(codes.Unauthenticated, "the requested action requires authentication")
)

// Configuration options for the ACL Plugin.
type AclOption func(*AclPlugin)

// Plugin returns a new AclPlugin.
func Plugin(opts ...AclOption) *AclPlugin {
	ap := &AclPlugin{}
	for _, opt := range opts {
		opt(ap)
	}
	return ap
}

// WithPolicy adds an ACL policy to the plugin.
func WithPolicy(effect Effect, role Role, action Action) AclOption {
	return func(ap *AclPlugin) {
		ap.DefinePolicy(effect, role, action)
	}
}

// WithRoleDescriber adds a role describer to the plugin.
func WithObjectFetcher(objectKey string, fn ObjectFetcher) AclOption {
	return func(ap *AclPlugin) {
		ap.RegisterObjectFetcher(objectKey, fn)
	}
}

// WithRoleDescriber adds a role describer to the plugin.
func WithRoleDescriber(objectKey string, fn RoleDescriber) AclOption {
	return func(ap *AclPlugin) {
		ap.RegisterRoleDescriber(objectKey, fn)
	}
}

// AclPlugin provides functionality for authorizing requests and access to resources.
type AclPlugin struct {
	policies       map[Action]map[Role]Effect
	objectFetchers map[string]ObjectFetcher
	roleDescribers map[string]RoleDescriber
}

// From plugin.Plugin
func (ap *AclPlugin) Name() string {
	return PluginName
}

// From plugin.DependentPlugin
func (ap *AclPlugin) Deps() []string {
	return []string{auth.PluginName}
}

// From prefab.OptionProvider, registers an additional interceptor.
func (ap *AclPlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithGRPCInterceptor(ap.Interceptor),
	}
}

// DefinePolicy defines an policy which allows/denies the given role to perform
// the action.
func (ap *AclPlugin) DefinePolicy(effect Effect, role Role, action Action) {
	if ap.policies == nil {
		ap.policies = make(map[Action]map[Role]Effect)
	}
	if ap.policies[action] == nil {
		ap.policies[action] = make(map[Role]Effect)
	}
	ap.policies[action][role] = effect
}

// RegisterObjectFetcher registers a function for fetching an object based on a
// request parameter that was specified in the proto descriptor. '*' can be used
// as a wildcard to match any key which doesn't have a more specific fetcher.
func (ap *AclPlugin) RegisterObjectFetcher(objectKey string, fn ObjectFetcher) {
	if ap.objectFetchers == nil {
		ap.objectFetchers = make(map[string]ObjectFetcher)
	}
	ap.objectFetchers[objectKey] = fn
}

// RegisterRoleDescriber registers a function for describing a role relative to
// an object.  '*' can be used as a wildcard to match any key which doesn't have
// a more specific describer.
func (ap *AclPlugin) RegisterRoleDescriber(objectKey string, fn RoleDescriber) {
	if ap.roleDescribers == nil {
		ap.roleDescribers = make(map[string]RoleDescriber)
	}
	ap.roleDescribers[objectKey] = fn
}

func (ap *AclPlugin) fetcherForKey(objectKey string) ObjectFetcher {
	if fn, ok := ap.objectFetchers[objectKey]; ok {
		return fn
	}
	if fn, ok := ap.objectFetchers["*"]; ok {
		return fn
	}
	return nil
}

func (ap *AclPlugin) describerForKey(objectKey string) RoleDescriber {
	if fn, ok := ap.roleDescribers[objectKey]; ok {
		return fn
	}
	if fn, ok := ap.roleDescribers["*"]; ok {
		return fn
	}
	return nil
}

// Interceptor that enforces ACLs configured on the GRPC service descriptors.
//
// This interceptor:
// 1. Uses method options to get object key and action.
// 2. Uses proto field options to get an object id and optionally domain.
// 3. Fetches the object based on the object key and id (ObjectFetcher).
// 4. Gets the user's role relative to the object (RoleDescriber).
// 5. Checks if the role can perform the action on the object.
func (ap *AclPlugin) Interceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	// Get the ACL spec from the method descriptor.
	objectKey, action, defaultEffect := MethodOptions(info)
	if action == "" {
		// No ACLs to enforce.
		return handler(ctx, req)
	}
	if ap.policies[action] == nil {
		return nil, errors.Codef(codes.Internal, "acl error: no policies configured for '%s' on %s", action, info.FullMethod)
	}
	fetcher := ap.fetcherForKey(objectKey)
	if fetcher == nil {
		return nil, errors.Codef(codes.Internal, "acl error: no object fetcher for key '%s' on %s", objectKey, info.FullMethod)
	}
	describer := ap.describerForKey(objectKey)
	if describer == nil {
		return nil, errors.Codef(codes.Internal, "acl error: no role describer for key '%s' on %s", objectKey, info.FullMethod)
	}

	// Get the object and domain from the request object.
	objectID, domainID, err := FieldOptions(req.(proto.Message))
	if err != nil {
		return nil, err
	}

	// Fetch the object that the action is being performed on.
	object, err := fetcher(ctx, objectID)
	if err != nil {
		return nil, err
	}

	defaultError := ErrPermissionDenied

	// Get the caller's identity.
	identity, err := auth.IdentityFromContext(ctx)
	if err != nil {
		if !errors.Is(err, auth.ErrNotFound) {
			return nil, err
		}
		// If the request is unauthenticated, still try to run the ACLs, but change
		// the default error type to Unauthenticated instead of Permission Denied.
		defaultError = ErrUnauthenticated
	}

	// Get the user's roles relative to the object.
	roles, err := describer(ctx, identity, object, Domain(domainID))
	if err != nil {
		return nil, err
	}

	logging.Track(ctx, "acl.action", action)
	logging.Track(ctx, "acl.object", objectID)
	logging.Track(ctx, "acl.domain", domainID)
	logging.Track(ctx, "acl.roles", roles)

	if len(roles) == 0 {
		return nil, errors.Mark(defaultError, 0)
	}

	if ap.DetermineEffect(action, roles, defaultEffect) == Allow {
		return handler(ctx, req)
	}
	return nil, errors.Mark(defaultError, 0)
}

// DetermineEffect checks to see if there are any policies which explicitly
// apply to this role and action. If there are, then all roles must explicitly
// revert the default effect.
//
// In otherwords, if an RPC is default deny and two roles explicitly match a
// policy, then both roles must allow access. This can be used to create
// exclusion groups: e.g. all admins, except nyc-admins.
func (ap *AclPlugin) DetermineEffect(action Action, roles []Role, defaultEffect Effect) Effect {
	if len(roles) == 0 {
		return defaultEffect
	}
	var effects effectList
	for _, role := range roles {
		if roleEffect, ok := ap.policies[action][role]; ok {
			effects = append(effects, roleEffect)
		}
	}
	return effects.Combine(defaultEffect)
}
