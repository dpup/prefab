package authz

import (
	"context"
	"fmt"
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

// WithObjectFetcherFn adds a function-based object fetcher to the plugin.
func WithObjectFetcherFn(objectKey string, fetcher func(ctx context.Context, key any) (any, error)) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterObjectFetcher(objectKey, ObjectFetcherFn(fetcher))
	}
}

// WithRoleDescriberFn adds a function-based role describer to the plugin.
func WithRoleDescriberFn(objectKey string, describer func(ctx context.Context, subject auth.Identity, object any, scope Scope) ([]Role, error)) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.RegisterRoleDescriber(objectKey, RoleDescriberFn(describer))
	}
}

// WithAuditLogger configures an audit logger to receive all authorization decisions.
// The audit logger is called for both allowed and denied requests, providing complete
// visibility into authorization decisions for compliance and security monitoring.
//
// Example:
//
//	authz.WithAuditLogger(func(ctx context.Context, decision authz.AuthzDecision) {
//	    log.Printf("authz: user=%s action=%s resource=%s effect=%s",
//	        decision.Identity.Subject, decision.Action, decision.Resource, decision.Effect)
//	})
func WithAuditLogger(logger AuditLogger) AuthzOption {
	return func(ap *AuthzPlugin) {
		ap.auditLogger = logger
	}
}

// AuditLogger is a function that receives authorization decisions for audit logging.
type AuditLogger func(ctx context.Context, decision AuthzDecision)

// AuthzPlugin provides functionality for authorizing requests and access to resources.
type AuthzPlugin struct {
	policies       map[Action]map[Role]Effect
	objectFetchers map[string]ObjectFetcher
	roleDescribers map[string]RoleDescriber
	roleParents    map[Role]Role
	auditLogger    AuditLogger
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

	// Determine the authorization effect and track which policies were evaluated
	finalEffect, evaluatedPolicies := ap.DetermineEffect(cfg.Action, roles, cfg.DefaultEffect)
	logging.Track(ctx, "authz.evaluated_policies", evaluatedPolicies)
	logging.Track(ctx, "authz.effect", finalEffect.String())

	// Build decision context for audit logging
	decision := AuthzDecision{
		Action:            cfg.Action,
		Resource:          cfg.ObjectKey,
		ObjectID:          cfg.ObjectID,
		Scope:             cfg.Scope,
		Identity:          identity,
		Roles:             roles,
		Effect:            finalEffect,
		DefaultEffect:     cfg.DefaultEffect,
		EvaluatedPolicies: evaluatedPolicies,
	}

	if finalEffect == Allow {
		decision.Reason = "allowed by policy"
		logging.Track(ctx, "authz.reason", decision.Reason)

		// Call audit logger if configured
		if ap.auditLogger != nil {
			ap.auditLogger(ctx, decision)
		}

		return nil
	}

	// Access denied - build explanation for user
	if len(roles) == 0 {
		decision.Reason = "no roles"
	} else {
		decision.Reason = "denied by policy"
	}
	logging.Track(ctx, "authz.reason", decision.Reason)

	// Call audit logger if configured
	if ap.auditLogger != nil {
		ap.auditLogger(ctx, decision)
	}

	// Build user-friendly denial explanation
	explanation := buildDenialExplanation(cfg.Action, roles, evaluatedPolicies, cfg.DefaultEffect)
	return errors.WithUserPresentableMessage(
		errors.Mark(defaultError, 0),
		"Access denied: %s", explanation,
	)
}

// buildDenialExplanation creates a human-readable explanation for why access was denied.
func buildDenialExplanation(action Action, roles []Role, evaluated []PolicyEvaluation, defaultEffect Effect) string {
	if len(roles) == 0 {
		return "no roles assigned"
	}

	if len(evaluated) == 0 {
		return fmt.Sprintf("no policies match action '%s' for your roles", action)
	}

	// Check for explicit deny
	for _, policy := range evaluated {
		if policy.Effect == Deny {
			return fmt.Sprintf("explicitly denied by role '%s'", policy.Role)
		}
	}

	// All policies were allow, but default is deny
	return fmt.Sprintf("action '%s' not explicitly allowed (default: deny)", action)
}

// DetermineEffect determines if a user can perform an action using AWS IAM-style precedence:
//
// 1. Explicit Deny: If ANY role has a Deny policy for the action → Deny
// 2. Explicit Allow: If ANY role has an Allow policy (and no Deny) → Allow
// 3. Default Effect: If no policies match → Use the RPC's default_effect
//
// This precedence model makes authorization predictable and secure:
// - Deny policies can block access even when other roles would grant it
// - Useful for creating "blocklist" roles or temporary access restrictions
// - Aligns with AWS IAM and other industry-standard access control systems
//
// Example: User has roles [admin, blocked-user]
//   - Policy: admin → Allow write
//   - Policy: blocked-user → Deny write
//   - Result: Deny (explicit deny wins)
//
// Returns the final effect and a list of evaluated policies for debugging/auditing.
func (ap *AuthzPlugin) DetermineEffect(action Action, roles []Role, defaultEffect Effect) (Effect, []PolicyEvaluation) {
	if len(roles) == 0 {
		return defaultEffect, nil
	}
	var effects effectList
	var evaluated []PolicyEvaluation

	for _, role := range roles {
		inheritedRoles := ap.RoleHierarchy(role)
		for _, r := range inheritedRoles {
			if roleEffect, ok := ap.policies[action][r]; ok {
				effects = append(effects, roleEffect)
				evaluated = append(evaluated, PolicyEvaluation{Role: r, Effect: roleEffect})
			}
		}
	}
	return effects.Combine(defaultEffect), evaluated
}
