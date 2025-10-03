package auth

import (
	"context"
	"testing"
	"time"

	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/serverutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestNew(t *testing.T) {
	svc := New()
	require.NotNil(t, svc)
	assert.IsType(t, &impl{}, svc)
}

func TestImplAddLoginHandler(t *testing.T) {
	svc := &impl{}

	handler := func(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
		return &LoginResponse{}, nil
	}

	svc.AddLoginHandler("test-provider", handler)
	assert.NotNil(t, svc.handlers)
	assert.Contains(t, svc.handlers, "test-provider")
}

func TestLogin_Success(t *testing.T) {
	svc := &impl{}

	// Mock login handler
	called := false
	handler := func(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
		called = true
		assert.Equal(t, "test-provider", req.Provider)
		return &LoginResponse{
			Token: "test-token",
		}, nil
	}

	svc.AddLoginHandler("test-provider", handler)

	ctx := logging.With(t.Context(), logging.NewDevLogger())
	ctx = serverutil.WithAddress(ctx, "http://localhost:8000")

	req := &LoginRequest{
		Provider: "test-provider",
	}

	resp, err := svc.Login(ctx, req)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "test-token", resp.Token)
}

func TestLogin_WithRedirectUri(t *testing.T) {
	svc := &impl{}

	handler := func(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
		assert.Equal(t, "/dashboard", req.RedirectUri)
		return &LoginResponse{
			RedirectUri: "/dashboard?logged_in=true",
		}, nil
	}

	svc.AddLoginHandler("test-provider", handler)

	ctx := logging.With(t.Context(), logging.NewDevLogger())
	ctx = serverutil.WithAddress(ctx, "http://localhost:8000")

	req := &LoginRequest{
		Provider:    "test-provider",
		RedirectUri: "/dashboard",
	}

	resp, err := svc.Login(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "/dashboard?logged_in=true", resp.RedirectUri)
}

func TestLogin_InvalidProvider(t *testing.T) {
	svc := &impl{}

	ctx := logging.With(t.Context(), logging.NewDevLogger())
	ctx = serverutil.WithAddress(ctx, "http://localhost:8000")

	req := &LoginRequest{
		Provider: "unknown-provider",
	}

	resp, err := svc.Login(ctx, req)
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown or unregistered provider")
}

func TestLogin_IncompatibleOptions(t *testing.T) {
	svc := &impl{}

	ctx := logging.With(t.Context(), logging.NewDevLogger())
	ctx = serverutil.WithAddress(ctx, "http://localhost:8000")

	req := &LoginRequest{
		Provider:    "test-provider",
		IssueToken:  true,
		RedirectUri: "/dashboard",
	}

	resp, err := svc.Login(ctx, req)
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "`issue_token` not compatible with `redirect_uri`")
}

func TestLogout_NoCookie(t *testing.T) {
	svc := &impl{}

	ctx := t.Context()
	ctx = serverutil.WithAddress(ctx, "http://localhost:8000")

	req := &LogoutRequest{}

	// Logout without a cookie should fail
	resp, err := svc.Logout(ctx, req)
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestIdentity_Success(t *testing.T) {
	svc := &impl{}

	identity := Identity{
		SessionID:     "session-789",
		Subject:       "user-101",
		Provider:      "google",
		AuthTime:      jwt.NewNumericDate(time.Now()).Time,
		Email:         "test@example.com",
		EmailVerified: true,
		Name:          "Test User",
	}

	ctx := WithIdentityExtractorsForTest(t.Context())
	ctx = serverutil.WithAddress(ctx, "http://localhost:8000")
	ctx = injectSigningKey("test-key")(ctx)
	ctx = injectExpiration(24 * time.Hour)(ctx)

	tokenString, err := IdentityToken(ctx, identity)
	require.NoError(t, err)

	md := metadata.Pairs("authorization", tokenString)
	ctx = metadata.NewIncomingContext(ctx, md)

	req := &IdentityRequest{}

	resp, err := svc.Identity(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "google", resp.Provider)
	assert.Equal(t, "user-101", resp.Subject)
	assert.Equal(t, "test@example.com", resp.Email)
	assert.True(t, resp.EmailVerified)
	assert.Equal(t, "Test User", resp.Name)
}

func TestIdentity_Unauthenticated(t *testing.T) {
	svc := &impl{}

	ctx := WithIdentityExtractorsForTest(t.Context())
	ctx = serverutil.WithAddress(ctx, "http://localhost:8000")

	req := &IdentityRequest{}

	resp, err := svc.Identity(ctx, req)
	assert.Nil(t, resp)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}
