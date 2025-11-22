package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/eventbus"
	"github.com/dpup/prefab/serverutil"
	"google.golang.org/grpc/codes"
)

// LoginHandler is a function which allows delegation of login requests.
type LoginHandler func(ctx context.Context, req *LoginRequest) (*LoginResponse, error)

func New() AuthServiceServer {
	return &impl{}
}

// IdentityValidator is an optional hook that can validate whether a target identity
// exists and is valid before allowing delegation. This allows applications to prevent
// delegation to non-existent or suspended users.
type IdentityValidator func(ctx context.Context, provider, subject string) error

// Implements AuthServiceServer and Plugin interfaces.
type impl struct {
	UnimplementedAuthServiceServer
	handlers map[string]LoginHandler

	// Delegation configuration (injected from AuthPlugin)
	delegationEnabled    bool
	requireReason        bool
	delegationExpiration time.Duration
	adminChecker         AdminChecker
	identityValidator    IdentityValidator
}

func (s *impl) AddLoginHandler(provider string, h LoginHandler) {
	if s.handlers == nil {
		s.handlers = map[string]LoginHandler{}
	}
	s.handlers[provider] = h
}

func (s *impl) Login(ctx context.Context, in *LoginRequest) (*LoginResponse, error) {
	logging.Track(ctx, "auth.provider", in.Provider)
	logging.Track(ctx, "auth.issueToken", in.IssueToken)
	logging.Track(ctx, "auth.redirectUri", in.RedirectUri)
	logging.Info(ctx, "Login attempt")

	if in.RedirectUri != "" && in.IssueToken {
		return nil, errors.NewC("auth: `issue_token` not compatible with `redirect_uri`", codes.InvalidArgument)
	}

	// TODO: Verify redirect_uri is a path or has a valid host.

	if h, ok := s.handlers[in.Provider]; ok {
		resp, err := h(ctx, in)

		// TODO: If the handler returns an error we may still want to send to the
		// redirect_uri with an error message, so the user doesn't end on a raw JSON
		// response.

		if resp != nil && resp.RedirectUri != "" {
			// Send a 302 redirect.
			logging.Infow(ctx, "Sending redirect", "redirectUri", resp.RedirectUri)
			if e := serverutil.SendStatusCode(ctx, http.StatusFound); e != nil {
				logging.Errorw(ctx, "auth: failed to send status code", "error", e)
			}
			if e := serverutil.SendHeader(ctx, "location", resp.RedirectUri); e != nil {
				logging.Errorw(ctx, "auth: failed to send header", "error", e)
			}
		}

		return resp, err
	}

	return nil, errors.NewC("auth: unknown or unregistered provider", codes.InvalidArgument)
}

func (s *impl) Logout(ctx context.Context, in *LogoutRequest) (*LogoutResponse, error) {
	id, err := identityFromCookie(ctx)
	if err != nil {
		// TODO: Should double logout be idempotent?
		return nil, err
	}

	// If enabled, block this token from future use.
	if err := MaybeBlock(ctx, id.SessionID); err != nil {
		logging.Errorw(ctx, "auth: failed to block tokenfor logout", "error", err)
	}

	address := serverutil.AddressFromContext(ctx)
	isSecure := strings.HasPrefix(address, "https")

	// Try to clear the cookie.
	if err := serverutil.SendCookie(ctx, &http.Cookie{
		Name:     IdentityTokenCookieName,
		Value:    "[invalidated]",
		Path:     "/",
		Secure:   isSecure,
		HttpOnly: true,
		Expires:  time.Now().Add(-24 * time.Hour),
		SameSite: http.SameSiteLaxMode,
	}); err != nil {
		return nil, err
	}

	r := in.RedirectUri
	if r == "" {
		r = address
	}

	if bus := eventbus.FromContext(ctx); bus != nil {
		bus.Publish(LogoutEvent, NewAuthEvent(id))
	}

	// For gateway requests, send the HTTP headers.
	serverutil.SendStatusCode(ctx, http.StatusFound)
	serverutil.SendHeader(ctx, "location", r)
	logging.Infow(ctx, "Sending logout redirect", "redirectUri", r)

	return &LogoutResponse{
		RedirectUri: r,
	}, nil
}

func (s *impl) Identity(ctx context.Context, in *IdentityRequest) (*IdentityResponse, error) {
	i, err := IdentityFromContext(ctx)
	if err != nil {
		return nil, err
	}
	resp := &IdentityResponse{
		Provider:      i.Provider,
		Subject:       i.Subject,
		Email:         i.Email,
		EmailVerified: i.EmailVerified,
		Name:          i.Name,
	}
	if i.Delegation != nil {
		resp.Delegation = i.Delegation
	}
	return resp, nil
}

func (s *impl) AssumeIdentity(ctx context.Context, in *AssumeIdentityRequest) (*AssumeIdentityResponse, error) {
	adminIdentity, err := s.validateDelegationRequest(ctx, in)
	if err != nil {
		return nil, err
	}

	assumedIdentity, err := s.createDelegatedIdentity(ctx, adminIdentity, in)
	if err != nil {
		return nil, err
	}

	token, err := s.generateDelegationToken(ctx, adminIdentity, assumedIdentity, in.Reason)
	if err != nil {
		return nil, err
	}

	return &AssumeIdentityResponse{Token: token}, nil
}

// validateDelegationRequest performs all validation checks for delegation.
func (s *impl) validateDelegationRequest(ctx context.Context, in *AssumeIdentityRequest) (Identity, error) {
	// Check delegation is enabled
	if !s.delegationEnabled {
		return Identity{}, errors.NewC("delegation not enabled", codes.FailedPrecondition)
	}

	// Extract and validate admin identity
	adminIdentity, err := IdentityFromContext(ctx)
	if err != nil {
		return Identity{}, errors.Wrap(err, 0).
			Append("authentication required to assume identity").
			WithCode(codes.Unauthenticated)
	}

	// Prevent delegation chaining
	if IsDelegated(adminIdentity) {
		return Identity{}, errors.NewC(
			"delegation chaining not allowed: delegated identities cannot assume other identities",
			codes.PermissionDenied,
		)
	}

	// Check admin authorization
	if err := s.checkAdminAuthorization(ctx, adminIdentity); err != nil {
		return Identity{}, err
	}

	// Validate request inputs
	if err := s.validateAssumeIdentityRequest(in); err != nil {
		return Identity{}, err
	}

	return adminIdentity, nil
}

// checkAdminAuthorization verifies the admin has permission to delegate.
func (s *impl) checkAdminAuthorization(ctx context.Context, identity Identity) error {
	if s.adminChecker == nil {
		return errors.NewC(
			"delegation requires authz plugin or custom admin checker",
			codes.FailedPrecondition,
		)
	}

	isAdmin, err := s.adminChecker(ctx, identity)
	if err != nil {
		return errors.Wrap(err, 0).
			Append("authorization check failed").
			WithCode(codes.Internal)
	}
	if !isAdmin {
		return errors.NewC(
			"insufficient permissions: delegation requires admin role",
			codes.PermissionDenied,
		)
	}

	return nil
}

// validateAssumeIdentityRequest validates the request parameters.
func (s *impl) validateAssumeIdentityRequest(in *AssumeIdentityRequest) error {
	if in.Subject == "" || in.Provider == "" {
		return errors.NewC("subject and provider required", codes.InvalidArgument)
	}
	if s.requireReason && in.Reason == "" {
		return errors.NewC("reason required for delegation", codes.InvalidArgument)
	}
	// Limit reason length to prevent DoS via excessive memory/storage
	const maxReasonLength = 1000
	if len(in.Reason) > maxReasonLength {
		return errors.NewC("reason exceeds maximum length", codes.InvalidArgument)
	}
	return nil
}

// createDelegatedIdentity creates an identity with delegation info.
func (s *impl) createDelegatedIdentity(ctx context.Context, adminIdentity Identity, in *AssumeIdentityRequest) (Identity, error) {
	// Call optional validator hook if configured
	if s.identityValidator != nil {
		if err := s.identityValidator(ctx, in.Provider, in.Subject); err != nil {
			return Identity{}, errors.Wrap(err, 0).
				Append("target identity validation failed").
				WithCode(codes.InvalidArgument)
		}
	}

	// Use single timestamp for consistency between AuthTime and DelegatedAt
	now := timeFunc()

	return Identity{
		Provider:  in.Provider,
		Subject:   in.Subject,
		SessionID: generateSessionID(),
		AuthTime:  now,
		// Note: Email, Name, EmailVerified are NOT populated
		// The assumed identity only has provider + subject
		Delegation: &DelegationInfo{
			DelegatorSub:       adminIdentity.Subject,
			DelegatorProvider:  adminIdentity.Provider,
			DelegatorSessionId: adminIdentity.SessionID,
			Reason:             in.Reason,
			DelegatedAt:        now.Unix(),
		},
	}, nil
}

// generateDelegationToken generates a JWT and publishes audit events.
func (s *impl) generateDelegationToken(ctx context.Context, adminIdentity, assumedIdentity Identity, reason string) (string, error) {
	// Use delegation-specific expiration if configured, otherwise inherit from context
	tokenCtx := ctx
	if s.delegationExpiration > 0 {
		tokenCtx = injectExpiration(s.delegationExpiration)(ctx)
	}

	token, err := IdentityToken(tokenCtx, assumedIdentity)
	if err != nil {
		return "", err
	}

	// Publish delegation event for audit trail
	if bus := eventbus.FromContext(ctx); bus != nil {
		bus.Publish(DelegationEvent, DelegationEventData{
			Admin:           adminIdentity,
			AssumedIdentity: assumedIdentity,
			Reason:          reason,
		})
	}

	// Log with additional audit context
	s.logDelegation(ctx, adminIdentity, assumedIdentity, reason)

	return token, nil
}

// logDelegation logs delegation with audit context.
func (s *impl) logDelegation(ctx context.Context, adminIdentity, assumedIdentity Identity, reason string) {
	logging.Infow(ctx, "Identity assumed",
		"admin_sub", adminIdentity.Subject,
		"admin_provider", adminIdentity.Provider,
		"admin_session", adminIdentity.SessionID,
		"assumed_sub", assumedIdentity.Subject,
		"assumed_provider", assumedIdentity.Provider,
		"assumed_session", assumedIdentity.SessionID,
		"reason", reason,
	)
}
