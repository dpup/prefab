package authz

import (
	"context"
	"slices"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
)

// Constant name for identifying the core Authz plugin.
const PluginName = "authz"

var (
	ErrPermissionDenied = errors.Codef(codes.PermissionDenied, "you are not authorized to perform this action")
	ErrUnauthenticated  = errors.Codef(codes.Unauthenticated, "the requested action requires authentication")
)

// Configuration options for the Authz Plugin.
type AuthzOption func(*AuthzPlugin)

// Builder provides a fluent interface for configuring the authz plugin.
type Builder struct {
	plugin *AuthzPlugin
}

// NewBuilder creates a new builder for the authz plugin.
func NewBuilder() *Builder {
	return &Builder{
		plugin: &AuthzPlugin{},
	}
}

// Build finalizes the builder and returns the configured plugin.
func (b *Builder) Build() *AuthzPlugin {
	return b.plugin
}

// WithRoleHierarchy adds a role hierarchy to the builder.
func (b *Builder) WithRoleHierarchy(roles ...Role) *Builder {
	b.plugin.SetRoleHierarchy(roles...)
	return b
}

// WithPolicy adds a policy to the builder.
func (b *Builder) WithPolicy(effect Effect, role Role, action Action) *Builder {
	b.plugin.DefinePolicy(effect, role, action)
	return b
}

// WithObjectFetcher adds an object fetcher to the builder.
func (b *Builder) WithObjectFetcher(objectKey string, fetcher ObjectFetcher) *Builder {
	b.plugin.RegisterObjectFetcher(objectKey, fetcher)
	return b
}

// WithRoleDescriber adds a role describer to the builder.
func (b *Builder) WithRoleDescriber(objectKey string, describer RoleDescriber) *Builder {
	b.plugin.RegisterRoleDescriber(objectKey, describer)
	return b
}

// WithObjectFetcherFn adds a function-based object fetcher to the builder.
func (b *Builder) WithObjectFetcherFn(objectKey string, fetcher func(ctx context.Context, key any) (any, error)) *Builder {
	b.plugin.RegisterObjectFetcher(objectKey, ObjectFetcherFn(fetcher))
	return b
}

// WithRoleDescriberFn adds a function-based role describer to the builder.
func (b *Builder) WithRoleDescriberFn(objectKey string, describer func(ctx context.Context, subject auth.Identity, object any, scope Scope) ([]Role, error)) *Builder {
	b.plugin.RegisterRoleDescriber(objectKey, RoleDescriberFn(describer))
	return b
}

// We can't use generic methods directly, so we'll provide package level functions instead

// Plugin returns a new AuthzPlugin.
func Plugin(opts ...AuthzOption) *AuthzPlugin {
	ap := &AuthzPlugin{}
	for _, opt := range opts {
		opt(ap)
	}
	return ap
}

// WithRoleHierarchy configures the plugin with a hierarchy of roles.
//
// The first role is the most powerful, and the last role has no hierarchy from
// a single call. Multiple calls can be used to define a tree hierarchies.
//
// Example:
//
//	WithRoleHierarchy("owner", "admin", "editor", "viewer", "member")
//	WithRoleHierarchy("suggester", "viewer")
//
// In this example, the "owner" role is an "admin", "editor", "viewer", and
// "member". An "admin" is an "editor", "viewer", and "member". An "editor" is
// also a "viewer" and a "member".
//
// A "suggester" is a "viewer" and a "member", since the ancestry of "viewer"
// was defined by the previous call.
func WithRoleHierarchy(roles ...Role) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.SetRoleHierarchy(roles...)
	}
}

// WithPolicy adds an Authz policy to the plugin.
func WithPolicy(effect Effect, role Role, action Action) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.DefinePolicy(effect, role, action)
	}
}

// WithObjectFetcher adds an object fetcher to the plugin.
func WithObjectFetcher(objectKey string, fetcher ObjectFetcher) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterObjectFetcher(objectKey, fetcher)
	}
}

// WithRoleDescriber adds a role describer to the plugin.
func WithRoleDescriber(objectKey string, describer RoleDescriber) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterRoleDescriber(objectKey, describer)
	}
}

// WithFunctionObjectFetcher adds a function-based object fetcher to the plugin.
func WithFunctionObjectFetcher(objectKey string, fetcher func(ctx context.Context, key any) (any, error)) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterObjectFetcher(objectKey, ObjectFetcherFn(fetcher))
	}
}

// WithFunctionRoleDescriber adds a function-based role describer to the plugin.
func WithFunctionRoleDescriber(objectKey string, describer func(ctx context.Context, subject auth.Identity, object any, scope Scope) ([]Role, error)) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterRoleDescriber(objectKey, RoleDescriberFn(describer))
	}
}

// AuthzPlugin provides functionality for authorizing requests and access to resources.
type AuthzPlugin struct {
	policies       map[Action]map[Role]Effect
	objectFetchers map[string]ObjectFetcher
	roleDescribers map[string]RoleDescriber
	roleParents    map[Role]Role
}

// From plugin.Plugin.
func (ap *AuthzPlugin) Name() string {
	return PluginName
}

// From plugin.DependentPlugin.
func (ap *AuthzPlugin) Deps() []string {
	return []string{auth.PluginName}
}

// From prefab.OptionProvider, registers an additional interceptor.
func (ap *AuthzPlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithGRPCInterceptor(ap.Interceptor),
		prefab.WithHTTPHandlerFunc("/debug/authz", ap.DebugHandler),
	}
}

// DefinePolicy defines an policy which allows/denies the given role to perform
// the action.
func (ap *AuthzPlugin) DefinePolicy(effect Effect, role Role, action Action) {
	if ap.policies == nil {
		ap.policies = make(map[Action]map[Role]Effect)
	}
	if ap.policies[action] == nil {
		ap.policies[action] = make(map[Role]Effect)
	}
	ap.policies[action][role] = effect
}

// RegisterObjectFetcher registers an object fetcher for a specified object key.
// '*' can be used as a wildcard to match any key which doesn't have a more specific fetcher.
func (ap *AuthzPlugin) RegisterObjectFetcher(objectKey string, fetcher ObjectFetcher) {
	if ap.objectFetchers == nil {
		ap.objectFetchers = make(map[string]ObjectFetcher)
	}
	ap.objectFetchers[objectKey] = fetcher
}

// RegisterRoleDescriber registers a role describer for a specified object key.
// '*' can be used as a wildcard to match any key which doesn't have a more specific describer.
func (ap *AuthzPlugin) RegisterRoleDescriber(objectKey string, describer RoleDescriber) {
	if ap.roleDescribers == nil {
		ap.roleDescribers = make(map[string]RoleDescriber)
	}
	ap.roleDescribers[objectKey] = describer
}

// SetRoleHierarchy sets the hierarchy of roles.
func (ap *AuthzPlugin) SetRoleHierarchy(roles ...Role) {
	if len(roles) <= 1 {
		return
	}
	if ap.roleParents == nil {
		ap.roleParents = map[Role]Role{}
	}
	for i := range len(roles) - 1 {
		if _, exists := ap.roleParents[roles[i]]; exists {
			panic("role '" + roles[i] + "' is already part of an established hierarchy")
		}
		if slices.Contains(roles[i+1:], roles[i]) {
			panic("cycle detected for role '" + roles[i] + "' in new hierarchy")
		}
		if slices.Contains(ap.RoleHierarchy(roles[i+1]), roles[i]) {
			panic("cycle detected for role '" + roles[i] + "' in established hierarchy")
		}
		ap.roleParents[roles[i]] = roles[i+1]
	}
}

// RoleHierarchy returns the ancestry of a single role.
func (ap *AuthzPlugin) RoleHierarchy(role Role) []Role {
	roles := []Role{role}
	for parent := ap.roleParents[role]; parent != Role(""); parent = ap.roleParents[parent] {
		roles = append(roles, parent)
	}
	return roles
}

// RoleTree returns the hierarchy of roles in tree form.
func (ap *AuthzPlugin) RoleTree() map[Role][]Role {
	children := make(map[Role][]Role)
	for child, parent := range ap.roleParents {
		children[parent] = append(children[parent], child)
	}
	return children
}

func (ap *AuthzPlugin) fetcherForKey(objectKey string) ObjectFetcher {
	if fetcher, ok := ap.objectFetchers[objectKey]; ok {
		return fetcher
	}
	if fetcher, ok := ap.objectFetchers["*"]; ok {
		return fetcher
	}
	return nil
}

func (ap *AuthzPlugin) describerForKey(objectKey string) RoleDescriber {
	if describer, ok := ap.roleDescribers[objectKey]; ok {
		return describer
	}
	if describer, ok := ap.roleDescribers["*"]; ok {
		return describer
	}
	return nil
}

// Interceptor that enforces authorization policies configured on the GRPC
// service descriptors.
//
// This interceptor:
// 1. Uses method options to get object key and action.
// 2. Uses proto field options to get an object id and optionally scope.
// 3. Fetches the object based on the object key and id (ObjectFetcher).
// 4. Gets the user's role relative to the object (RoleDescriber).
// 5. Checks if the role can perform the action on the object.
func (ap *AuthzPlugin) Interceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	// Get the Authz spec from the method descriptor.
	objectKey, action, defaultEffect := MethodOptions(info)
	if action == "" {
		// No policies to enforce.
		return handler(ctx, req)
	}

	// Get the object and scope from the request object.
	objectID, scopeStr, err := FieldOptions(req.(proto.Message))
	if err != nil {
		return nil, err
	}

	if err := ap.Authorize(ctx, AuthorizeParams{
		ObjectKey:     objectKey,
		ObjectID:      objectID,
		Scope:         Scope(scopeStr),
		Action:        action,
		DefaultEffect: defaultEffect,
		Info:          info.FullMethod,
	}); err != nil {
		return nil, err
	}

	return handler(ctx, req)
}

// Parameters for the Authorize method.
type AuthorizeParams struct {
	ObjectKey     string
	ObjectID      any
	Scope         Scope
	Action        Action
	DefaultEffect Effect
	Info          string
}

// Authorize takes the configuration and verifies that the caller is authorized
// to perform the action on the object.
func (ap *AuthzPlugin) Authorize(ctx context.Context, cfg AuthorizeParams) error {
	if ap.policies[cfg.Action] == nil {
		return errors.Codef(codes.Internal, "authz error: no policies configured for '%s' on %s", cfg.Action, cfg.Info)
	}
	fetcher := ap.fetcherForKey(cfg.ObjectKey)
	if fetcher == nil {
		return errors.Codef(codes.Internal, "authz error: no object fetcher for key '%s' on %s", cfg.ObjectKey, cfg.Info)
	}
	describer := ap.describerForKey(cfg.ObjectKey)
	if describer == nil {
		return errors.Codef(codes.Internal, "authz error: no role describer for key '%s' on %s", cfg.ObjectKey, cfg.Info)
	}

	// Fetch the object that the action is being performed on.
	object, err := fetcher.FetchObject(ctx, cfg.ObjectID)
	if err != nil {
		return err
	}

	defaultError := ErrPermissionDenied

	// Get the caller's identity.
	identity, err := auth.IdentityFromContext(ctx)
	if err != nil {
		if !errors.Is(err, auth.ErrNotFound) {
			logging.Track(ctx, "authz.reason", "authentication error")
			return err
		}
		// If the request is unauthenticated, still try to run the policy, but change
		// the default error type to Unauthenticated instead of Permission Denied.
		defaultError = ErrUnauthenticated
	}

	// Get the user's roles relative to the object.
	roles, err := describer.DescribeRoles(ctx, identity, object, cfg.Scope)
	if err != nil {
		logging.Track(ctx, "authz.reason", "failed to describe roles")
		return err
	}

	logging.Track(ctx, "authz.action", cfg.Action)
	logging.Track(ctx, "authz.objectID", cfg.ObjectID)
	logging.Track(ctx, "authz.object", object)
	logging.Track(ctx, "authz.scope", cfg.Scope)
	logging.Track(ctx, "authz.roles", roles)

	if len(roles) == 0 {
		logging.Track(ctx, "authz.reason", "no roles")
		return errors.Mark(defaultError, 0)
	}

	if ap.DetermineEffect(cfg.Action, roles, cfg.DefaultEffect) == Allow {
		logging.Track(ctx, "authz.reason", "allowed by role")
		return nil
	}

	logging.Track(ctx, "authz.reason", "denied by role")
	return errors.Mark(defaultError, 0)
}

// DetermineEffect checks to see if there are any policies which explicitly
// apply to this role and action. If there are, then all roles must explicitly
// revert the default effect.
//
// In otherwords, if an RPC is default deny and two roles explicitly match a
// policy, then both roles must allow access. This can be used to create
// exclusion groups: e.g. all admins, except nyc-admins.
func (ap *AuthzPlugin) DetermineEffect(action Action, roles []Role, defaultEffect Effect) Effect {
	if len(roles) == 0 {
		return defaultEffect
	}
	var effects effectList
	for _, role := range roles {
		inheritedRoles := ap.RoleHierarchy(role)
		for _, r := range inheritedRoles {
			if roleEffect, ok := ap.policies[action][r]; ok {
				effects = append(effects, roleEffect)
			}
		}
	}
	return effects.Combine(defaultEffect)
}
