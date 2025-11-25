package google

import (
	"context"
	"testing"
	"time"

	"github.com/dpup/prefab/plugins/auth"
	"github.com/stretchr/testify/assert"
)

func TestOAuthToken_HasRefreshToken(t *testing.T) {
	tests := []struct {
		name     string
		token    OAuthToken
		expected bool
	}{
		{
			name:     "with refresh token",
			token:    OAuthToken{RefreshToken: "refresh-123"},
			expected: true,
		},
		{
			name:     "without refresh token",
			token:    OAuthToken{},
			expected: false,
		},
		{
			name:     "empty refresh token",
			token:    OAuthToken{RefreshToken: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.token.HasRefreshToken())
		})
	}
}

func TestOAuthToken_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		token    OAuthToken
		expected bool
	}{
		{
			name:     "zero expiry (never expires)",
			token:    OAuthToken{},
			expected: false,
		},
		{
			name:     "expired token",
			token:    OAuthToken{Expiry: time.Now().Add(-time.Hour)},
			expected: true,
		},
		{
			name:     "valid token",
			token:    OAuthToken{Expiry: time.Now().Add(time.Hour)},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.token.IsExpired())
		})
	}
}

func TestWithOfflineAccess(t *testing.T) {
	p := Plugin(
		WithClient("test-id", "test-secret"),
		WithOfflineAccess(),
	)
	assert.True(t, p.offlineAccess)
}

func TestWithScopes(t *testing.T) {
	p := Plugin(
		WithClient("test-id", "test-secret"),
		WithScopes("https://www.googleapis.com/auth/gmail.readonly"),
	)
	assert.Contains(t, p.extraScopes, "https://www.googleapis.com/auth/gmail.readonly")
}

func TestWithScopes_Multiple(t *testing.T) {
	p := Plugin(
		WithClient("test-id", "test-secret"),
		WithScopes(
			"https://www.googleapis.com/auth/gmail.readonly",
			"https://www.googleapis.com/auth/calendar.readonly",
		),
	)
	assert.Len(t, p.extraScopes, 2)
	assert.Contains(t, p.extraScopes, "https://www.googleapis.com/auth/gmail.readonly")
	assert.Contains(t, p.extraScopes, "https://www.googleapis.com/auth/calendar.readonly")
}

func TestWithScopes_Chained(t *testing.T) {
	p := Plugin(
		WithClient("test-id", "test-secret"),
		WithScopes("https://www.googleapis.com/auth/gmail.readonly"),
		WithScopes("https://www.googleapis.com/auth/calendar.readonly"),
	)
	assert.Len(t, p.extraScopes, 2)
}

func TestWithTokenHandler(t *testing.T) {
	called := false
	handler := func(ctx context.Context, identity auth.Identity, token OAuthToken) error {
		called = true
		return nil
	}

	p := Plugin(
		WithClient("test-id", "test-secret"),
		WithTokenHandler(handler),
	)

	assert.NotNil(t, p.tokenHandler)

	// Call the handler to verify it's set correctly
	err := p.tokenHandler(context.Background(), auth.Identity{}, OAuthToken{})
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestWithTokenHandler_ReceivesCorrectData(t *testing.T) {
	var receivedIdentity auth.Identity
	var receivedToken OAuthToken

	handler := func(ctx context.Context, identity auth.Identity, token OAuthToken) error {
		receivedIdentity = identity
		receivedToken = token
		return nil
	}

	p := Plugin(
		WithClient("test-id", "test-secret"),
		WithTokenHandler(handler),
	)

	testIdentity := auth.Identity{
		Subject: "user-123",
		Email:   "test@example.com",
	}
	testToken := OAuthToken{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	err := p.tokenHandler(context.Background(), testIdentity, testToken)
	assert.NoError(t, err)
	assert.Equal(t, testIdentity.Subject, receivedIdentity.Subject)
	assert.Equal(t, testIdentity.Email, receivedIdentity.Email)
	assert.Equal(t, testToken.AccessToken, receivedToken.AccessToken)
	assert.Equal(t, testToken.RefreshToken, receivedToken.RefreshToken)
}

func TestPlugin_CombinedOptions(t *testing.T) {
	handlerCalled := false
	p := Plugin(
		WithClient("my-client-id", "my-client-secret"),
		WithOfflineAccess(),
		WithScopes("https://www.googleapis.com/auth/gmail.readonly"),
		WithTokenHandler(func(ctx context.Context, identity auth.Identity, token OAuthToken) error {
			handlerCalled = true
			return nil
		}),
	)

	assert.Equal(t, "my-client-id", p.clientID)
	assert.Equal(t, "my-client-secret", p.clientSecret)
	assert.True(t, p.offlineAccess)
	assert.Contains(t, p.extraScopes, "https://www.googleapis.com/auth/gmail.readonly")
	assert.NotNil(t, p.tokenHandler)

	// Verify handler works
	_ = p.tokenHandler(context.Background(), auth.Identity{}, OAuthToken{})
	assert.True(t, handlerCalled)
}

func TestWithTokenHandler_ErrorAborts(t *testing.T) {
	expectedErr := assert.AnError

	handler := func(ctx context.Context, identity auth.Identity, token OAuthToken) error {
		return expectedErr
	}

	p := Plugin(
		WithClient("test-id", "test-secret"),
		WithTokenHandler(handler),
	)

	// Verify handler returns the error
	err := p.tokenHandler(context.Background(), auth.Identity{}, OAuthToken{})
	assert.ErrorIs(t, err, expectedErr)
}
