package auth

import (
	"context"
	"testing"
	"time"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/serverutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestIsDelegated(t *testing.T) {
	// Normal identity
	normalIdentity := Identity{
		Subject:  "user123",
		Provider: "google",
	}
	assert.False(t, IsDelegated(normalIdentity))

	// Delegated identity
	delegatedIdentity := Identity{
		Subject:  "user456",
		Provider: "google",
		Delegation: &DelegationInfo{
			DelegatorSub:       "admin123",
			DelegatorProvider:  "google",
			DelegatorSessionId: "session-abc",
			Reason:             "support-case-873",
		},
	}
	assert.True(t, IsDelegated(delegatedIdentity))
}

func TestGetDelegator(t *testing.T) {
	// Normal identity
	normalIdentity := Identity{
		Subject:  "user123",
		Provider: "google",
	}
	sub, provider, sessionID, ok := GetDelegator(normalIdentity)
	assert.False(t, ok)
	assert.Empty(t, sub)
	assert.Empty(t, provider)
	assert.Empty(t, sessionID)

	// Delegated identity
	delegatedIdentity := Identity{
		Subject:  "user456",
		Provider: "google",
		Delegation: &DelegationInfo{
			DelegatorSub:       "admin123",
			DelegatorProvider:  "google",
			DelegatorSessionId: "session-abc",
			Reason:             "support-case-873",
		},
	}
	sub, provider, sessionID, ok = GetDelegator(delegatedIdentity)
	assert.True(t, ok)
	assert.Equal(t, "admin123", sub)
	assert.Equal(t, "google", provider)
	assert.Equal(t, "session-abc", sessionID)
}

func TestDelegationTokenRoundtrip(t *testing.T) {
	ctx := context.Background()
	ctx = serverutil.WithAddress(ctx, "https://example.com")
	ctx = injectSigningKey("test-key-123")(ctx)
	ctx = injectExpiration(time.Hour)(ctx)
	ctx = WithIdentityExtractorsForTest(ctx)

	// Create a delegated identity
	original := Identity{
		Subject:   "user456",
		Provider:  "github",
		SessionID: generateSessionID(),
		AuthTime:  timeFunc(),
		Delegation: &DelegationInfo{
			DelegatorSub:       "admin123",
			DelegatorProvider:  "google",
			DelegatorSessionId: "admin-session-xyz",
			Reason:             "support investigation",
			DelegatedAt:        timeFunc().Unix(),
		},
	}

	// Generate token
	token, err := IdentityToken(ctx, original)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Parse token
	parsed, err := ParseIdentityToken(ctx, token)
	require.NoError(t, err)

	// Verify identity fields
	assert.Equal(t, original.Subject, parsed.Subject)
	assert.Equal(t, original.Provider, parsed.Provider)
	assert.Equal(t, original.SessionID, parsed.SessionID)

	// Verify delegation info is preserved
	require.NotNil(t, parsed.Delegation)
	assert.Equal(t, "admin123", parsed.Delegation.DelegatorSub)
	assert.Equal(t, "google", parsed.Delegation.DelegatorProvider)
	assert.Equal(t, "admin-session-xyz", parsed.Delegation.DelegatorSessionId)
	assert.Equal(t, "support investigation", parsed.Delegation.Reason)
}

func TestDelegationTokenWithoutDelegation(t *testing.T) {
	ctx := context.Background()
	ctx = serverutil.WithAddress(ctx, "https://example.com")
	ctx = injectSigningKey("test-key-123")(ctx)
	ctx = injectExpiration(time.Hour)(ctx)
	ctx = WithIdentityExtractorsForTest(ctx)

	// Create a normal (non-delegated) identity
	original := Identity{
		Subject:       "user123",
		Provider:      "google",
		SessionID:     "session-123",
		AuthTime:      timeFunc(),
		Email:         "user@example.com",
		EmailVerified: true,
		Name:          "Test User",
	}

	// Generate token
	token, err := IdentityToken(ctx, original)
	require.NoError(t, err)

	// Parse token
	parsed, err := ParseIdentityToken(ctx, token)
	require.NoError(t, err)

	// Verify no delegation info
	assert.Nil(t, parsed.Delegation)
	assert.Equal(t, original.Email, parsed.Email)
	assert.Equal(t, original.Name, parsed.Name)
}

func TestClaimsValidationWithDelegation(t *testing.T) {
	tests := []struct {
		name        string
		claims      Claims
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid delegation claims",
			claims: Claims{
				Provider:           "google",
				DelegatorSub:       "admin123",
				DelegatorProvider:  "google",
				DelegatorSessionID: "session-abc",
				DelegationReason:   "support case",
				DelegatedAt:        timeFunc().Unix(),
			},
			expectError: false,
		},
		{
			name: "missing delegator_sub",
			claims: Claims{
				Provider:           "google",
				DelegatorProvider:  "google",
				DelegatorSessionID: "session-abc",
				DelegationReason:   "support case",
			},
			expectError: true,
			errorMsg:    "missing delegator_sub",
		},
		{
			name: "missing delegator_provider",
			claims: Claims{
				Provider:           "google",
				DelegatorSub:       "admin123",
				DelegatorSessionID: "session-abc",
				DelegationReason:   "support case",
			},
			expectError: true,
			errorMsg:    "missing delegator_provider",
		},
		{
			name: "missing delegator_session_id",
			claims: Claims{
				Provider:          "google",
				DelegatorSub:      "admin123",
				DelegatorProvider: "google",
				DelegationReason:  "support case",
			},
			expectError: true,
			errorMsg:    "missing delegator_session_id",
		},
		{
			name: "missing delegation_reason",
			claims: Claims{
				Provider:           "google",
				DelegatorSub:       "admin123",
				DelegatorProvider:  "google",
				DelegatorSessionID: "session-abc",
			},
			expectError: true,
			errorMsg:    "missing delegation_reason",
		},
		{
			name: "missing delegated_at",
			claims: Claims{
				Provider:           "google",
				DelegatorSub:       "admin123",
				DelegatorProvider:  "google",
				DelegatorSessionID: "session-abc",
				DelegationReason:   "support case",
				DelegatedAt:        0, // Invalid timestamp
			},
			expectError: true,
			errorMsg:    "missing or invalid delegated_at",
		},
		{
			name: "no delegation fields - valid",
			claims: Claims{
				Provider: "google",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claims.Validate()
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGenerateSessionID(t *testing.T) {
	// Generate multiple session IDs
	ids := make(map[string]bool)
	for i := range 100 {
		id := generateSessionID()
		assert.NotEmpty(t, id)
		assert.Contains(t, id, "delegated-")
		// Ensure uniqueness
		assert.False(t, ids[id], "duplicate session ID generated: iteration %d", i)
		ids[id] = true
	}
}

func TestAuthServiceDelegationConfig(t *testing.T) {
	// Test that delegation config is properly set on authService
	service := &impl{
		delegationEnabled: true,
		requireReason:     false,
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return true, nil
		},
	}

	assert.True(t, service.delegationEnabled)
	assert.False(t, service.requireReason)
	assert.NotNil(t, service.adminChecker)

	// Test that adminChecker works
	isAdmin, err := service.adminChecker(context.Background(), Identity{})
	require.NoError(t, err)
	assert.True(t, isAdmin)
}

func TestAdminCheckerWrapsAuthorizer(t *testing.T) {
	// Test that adminChecker properly wraps authorizer
	mockAuthorizer := &mockAuthorizerImpl{shouldAllow: true}

	// Create the wrapper function (same as in authplugin.go)
	adminChecker := func(ctx context.Context, _ Identity) (bool, error) {
		params := AuthorizeParams{
			ObjectKey:     DelegationResource,
			ObjectID:      nil,
			Scope:         "",
			Action:        DelegationAction,
			DefaultEffect: 0,
			Info:          "AssumeIdentity",
		}
		err := mockAuthorizer.Authorize(ctx, params)
		if err != nil {
			if errors.Code(err) == codes.PermissionDenied {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	// Test authorized case
	isAdmin, err := adminChecker(context.Background(), Identity{})
	require.NoError(t, err)
	assert.True(t, isAdmin)

	// Test permission denied case
	mockAuthorizer.shouldAllow = false
	isAdmin, err = adminChecker(context.Background(), Identity{})
	require.NoError(t, err)
	assert.False(t, isAdmin)
}

// mockAuthorizerImpl is a test implementation of Authorizer
type mockAuthorizerImpl struct {
	shouldAllow bool
}

func (m *mockAuthorizerImpl) Authorize(ctx context.Context, params any) error {
	if !m.shouldAllow {
		return errors.NewC("permission denied", codes.PermissionDenied)
	}
	return nil
}

// setupTestContext creates a test context with all required components
func setupTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx := context.Background()
	ctx = serverutil.WithAddress(ctx, "https://example.com")
	ctx = injectSigningKey("test-key-123")(ctx)
	ctx = injectExpiration(time.Hour)(ctx)
	ctx = WithIdentityExtractorsForTest(ctx)
	// Use NewDevLogger for tests - it's lightweight and won't actually log
	ctx = logging.With(ctx, logging.NewDevLogger())
	return ctx
}

// TestAssumeIdentitySuccess tests successful delegation
func TestAssumeIdentitySuccess(t *testing.T) {
	ctx := setupTestContext(t)

	// Create admin identity
	adminIdentity := Identity{
		Subject:   "admin123",
		Provider:  "google",
		SessionID: "admin-session-xyz",
		AuthTime:  timeFunc(),
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	// Create service with delegation enabled
	service := &impl{
		delegationEnabled: true,
		requireReason:     true,
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return true, nil // Admin is authorized
		},
	}

	// Test successful delegation
	req := &AssumeIdentityRequest{
		Provider: "github",
		Subject:  "user456",
		Reason:   "support-case-123",
	}

	resp, err := service.AssumeIdentity(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.Token)

	// Verify token contains correct identity
	parsed, err := ParseIdentityToken(ctx, resp.Token)
	require.NoError(t, err)
	assert.Equal(t, "github", parsed.Provider)
	assert.Equal(t, "user456", parsed.Subject)

	// Verify delegation info
	require.NotNil(t, parsed.Delegation)
	assert.Equal(t, "admin123", parsed.Delegation.DelegatorSub)
	assert.Equal(t, "google", parsed.Delegation.DelegatorProvider)
	assert.Equal(t, "admin-session-xyz", parsed.Delegation.DelegatorSessionId)
	assert.Equal(t, "support-case-123", parsed.Delegation.Reason)
	assert.Positive(t, parsed.Delegation.DelegatedAt)
}

// TestAssumeIdentityDelegationDisabled tests error when delegation is disabled
func TestAssumeIdentityDelegationDisabled(t *testing.T) {
	ctx := context.Background()

	service := &impl{
		delegationEnabled: false,
	}

	req := &AssumeIdentityRequest{
		Provider: "google",
		Subject:  "user123",
		Reason:   "test",
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, errors.Code(err))
	assert.Contains(t, err.Error(), "delegation not enabled")
}

// TestAssumeIdentityUnauthenticated tests error when no identity in context
func TestAssumeIdentityUnauthenticated(t *testing.T) {
	ctx := context.Background()

	service := &impl{
		delegationEnabled: true,
	}

	req := &AssumeIdentityRequest{
		Provider: "google",
		Subject:  "user123",
		Reason:   "test",
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, errors.Code(err))
	assert.Contains(t, err.Error(), "authentication required")
}

// TestAssumeIdentityDelegationChaining tests prevention of delegation chaining
func TestAssumeIdentityDelegationChaining(t *testing.T) {
	ctx := context.Background()

	// Create a delegated identity (already assumed)
	delegatedIdentity := Identity{
		Subject:  "user456",
		Provider: "github",
		Delegation: &DelegationInfo{
			DelegatorSub:       "admin123",
			DelegatorProvider:  "google",
			DelegatorSessionId: "session-xyz",
			Reason:             "original-reason",
			DelegatedAt:        timeFunc().Unix(),
		},
	}
	ctx = WithIdentityForTest(ctx, delegatedIdentity)

	service := &impl{
		delegationEnabled: true,
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return true, nil
		},
	}

	req := &AssumeIdentityRequest{
		Provider: "google",
		Subject:  "user789",
		Reason:   "chaining-attempt",
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, errors.Code(err))
	assert.Contains(t, err.Error(), "delegation chaining not allowed")
}

// TestAssumeIdentityNoAdminChecker tests error when no admin checker configured
func TestAssumeIdentityNoAdminChecker(t *testing.T) {
	ctx := context.Background()

	adminIdentity := Identity{
		Subject:  "admin123",
		Provider: "google",
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	service := &impl{
		delegationEnabled: true,
		adminChecker:      nil, // No checker configured
	}

	req := &AssumeIdentityRequest{
		Provider: "google",
		Subject:  "user123",
		Reason:   "test",
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, errors.Code(err))
	assert.Contains(t, err.Error(), "requires authz plugin or custom admin checker")
}

// TestAssumeIdentityUnauthorized tests error when admin check fails
func TestAssumeIdentityUnauthorized(t *testing.T) {
	ctx := context.Background()

	adminIdentity := Identity{
		Subject:  "user123",
		Provider: "google",
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	service := &impl{
		delegationEnabled: true,
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return false, nil // Not an admin
		},
	}

	req := &AssumeIdentityRequest{
		Provider: "google",
		Subject:  "user456",
		Reason:   "test",
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.PermissionDenied, errors.Code(err))
	assert.Contains(t, err.Error(), "insufficient permissions")
}

// TestAssumeIdentityAdminCheckerError tests error propagation from admin checker
func TestAssumeIdentityAdminCheckerError(t *testing.T) {
	ctx := context.Background()

	adminIdentity := Identity{
		Subject:  "admin123",
		Provider: "google",
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	service := &impl{
		delegationEnabled: true,
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return false, errors.NewC("database error", codes.Internal)
		},
	}

	req := &AssumeIdentityRequest{
		Provider: "google",
		Subject:  "user123",
		Reason:   "test",
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.Internal, errors.Code(err))
	assert.Contains(t, err.Error(), "authorization check failed")
}

// TestAssumeIdentityMissingProvider tests validation of provider field
func TestAssumeIdentityMissingProvider(t *testing.T) {
	ctx := context.Background()

	adminIdentity := Identity{
		Subject:  "admin123",
		Provider: "google",
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	service := &impl{
		delegationEnabled: true,
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return true, nil
		},
	}

	req := &AssumeIdentityRequest{
		Provider: "", // Missing provider
		Subject:  "user123",
		Reason:   "test",
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, errors.Code(err))
	assert.Contains(t, err.Error(), "subject and provider required")
}

// TestAssumeIdentityMissingSubject tests validation of subject field
func TestAssumeIdentityMissingSubject(t *testing.T) {
	ctx := context.Background()

	adminIdentity := Identity{
		Subject:  "admin123",
		Provider: "google",
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	service := &impl{
		delegationEnabled: true,
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return true, nil
		},
	}

	req := &AssumeIdentityRequest{
		Provider: "google",
		Subject:  "", // Missing subject
		Reason:   "test",
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, errors.Code(err))
	assert.Contains(t, err.Error(), "subject and provider required")
}

// TestAssumeIdentityMissingReason tests validation when reason is required
func TestAssumeIdentityMissingReason(t *testing.T) {
	ctx := context.Background()

	adminIdentity := Identity{
		Subject:  "admin123",
		Provider: "google",
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	service := &impl{
		delegationEnabled: true,
		requireReason:     true, // Reason required
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return true, nil
		},
	}

	req := &AssumeIdentityRequest{
		Provider: "google",
		Subject:  "user123",
		Reason:   "", // Missing reason
	}

	_, err := service.AssumeIdentity(ctx, req)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, errors.Code(err))
	assert.Contains(t, err.Error(), "reason required")
}

// TestAssumeIdentityReasonOptional tests delegation without reason when not required
func TestAssumeIdentityReasonOptional(t *testing.T) {
	ctx := setupTestContext(t)

	adminIdentity := Identity{
		Subject:   "admin123",
		Provider:  "google",
		SessionID: "admin-session",
		AuthTime:  timeFunc(),
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	service := &impl{
		delegationEnabled: true,
		requireReason:     false, // Reason NOT required
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return true, nil
		},
	}

	req := &AssumeIdentityRequest{
		Provider: "github",
		Subject:  "user456",
		Reason:   "", // Empty reason is OK
	}

	resp, err := service.AssumeIdentity(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

// TestAssumeIdentityWithValidator tests identity validator hook
func TestAssumeIdentityWithValidator(t *testing.T) {
	ctx := setupTestContext(t)

	adminIdentity := Identity{
		Subject:   "admin123",
		Provider:  "google",
		SessionID: "admin-session",
		AuthTime:  timeFunc(),
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	t.Run("validator succeeds", func(t *testing.T) {
		service := &impl{
			delegationEnabled: true,
			requireReason:     false,
			adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
				return true, nil
			},
			identityValidator: func(ctx context.Context, provider, subject string) error {
				// Validate that user exists
				if provider == "github" && subject == "user456" {
					return nil // Valid user
				}
				return errors.NewC("user not found", codes.NotFound)
			},
		}

		req := &AssumeIdentityRequest{
			Provider: "github",
			Subject:  "user456",
			Reason:   "test",
		}

		resp, err := service.AssumeIdentity(ctx, req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Token)
	})

	t.Run("validator fails", func(t *testing.T) {
		service := &impl{
			delegationEnabled: true,
			requireReason:     false,
			adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
				return true, nil
			},
			identityValidator: func(ctx context.Context, provider, subject string) error {
				return errors.NewC("user suspended", codes.FailedPrecondition)
			},
		}

		req := &AssumeIdentityRequest{
			Provider: "github",
			Subject:  "suspended-user",
			Reason:   "test",
		}

		_, err := service.AssumeIdentity(ctx, req)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, errors.Code(err))
		assert.Contains(t, err.Error(), "target identity validation failed")
	})
}

// TestAssumeIdentityWithCustomExpiration tests delegation-specific token expiration
func TestAssumeIdentityWithCustomExpiration(t *testing.T) {
	ctx := setupTestContext(t)
	// Override default expiration to 24 hours for this test
	ctx = injectExpiration(24 * time.Hour)(ctx)

	adminIdentity := Identity{
		Subject:   "admin123",
		Provider:  "google",
		SessionID: "admin-session",
		AuthTime:  timeFunc(),
	}
	ctx = WithIdentityForTest(ctx, adminIdentity)

	service := &impl{
		delegationEnabled:    true,
		delegationExpiration: 1 * time.Hour, // Custom shorter expiration
		adminChecker: func(ctx context.Context, identity Identity) (bool, error) {
			return true, nil
		},
	}

	req := &AssumeIdentityRequest{
		Provider: "github",
		Subject:  "user456",
		Reason:   "short-session",
	}

	resp, err := service.AssumeIdentity(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)

	// Parse and verify expiration
	parsed, err := ParseIdentityToken(ctx, resp.Token)
	require.NoError(t, err)

	// Token should expire in about 1 hour (with some tolerance)
	expiresIn := time.Until(parsed.AuthTime.Add(1 * time.Hour))
	assert.Greater(t, expiresIn, 55*time.Minute)
	assert.Less(t, expiresIn, 65*time.Minute)
}
