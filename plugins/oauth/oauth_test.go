package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestOAuthPlugin_Builder(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Scopes:       []string{"read", "write"},
		}).
		WithAccessTokenExpiry(2 * time.Hour).
		WithRefreshTokenExpiry(7 * 24 * time.Hour).
		WithAuthCodeExpiry(5 * time.Minute).
		WithIssuer("https://auth.example.com").
		Build()

	assert.Equal(t, PluginName, plugin.Name())
	assert.Equal(t, 2*time.Hour, plugin.accessTokenExpiry)
	assert.Equal(t, 7*24*time.Hour, plugin.refreshTokenExpiry)
	assert.Equal(t, 5*time.Minute, plugin.authCodeExpiry)
	assert.Equal(t, "https://auth.example.com", plugin.issuer)

	// Verify client was added
	client, err := plugin.GetClientStore().GetClient(context.Background(), "test-client")
	require.NoError(t, err)
	assert.Equal(t, "test-client", client.ID)
}

func TestOAuthPlugin_MetadataHandler(t *testing.T) {
	plugin := NewBuilder().
		WithIssuer("https://auth.example.com").
		Build()

	handler := plugin.metadataHandler()

	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var meta map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &meta)
	require.NoError(t, err)

	assert.Equal(t, "https://auth.example.com", meta["issuer"])
	assert.Equal(t, "https://auth.example.com/oauth/authorize", meta["authorization_endpoint"])
	assert.Equal(t, "https://auth.example.com/oauth/token", meta["token_endpoint"])
}

func TestMemoryClientStore(t *testing.T) {
	store := newMemoryClientStore()
	ctx := context.Background()

	// Create client
	client := &Client{
		ID:           "test-client",
		Secret:       "secret",
		Name:         "Test Client",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read", "write"},
		CreatedBy:    "user-123",
	}

	err := store.CreateClient(ctx, client)
	require.NoError(t, err)

	// Get client
	retrieved, err := store.GetClient(ctx, "test-client")
	require.NoError(t, err)
	assert.Equal(t, client.ID, retrieved.ID)
	assert.Equal(t, client.Secret, retrieved.Secret)

	// Test adapter wraps store correctly
	adapter := newClientStoreAdapter(store)
	clientInfo, err := adapter.GetByID(ctx, "test-client")
	require.NoError(t, err)
	assert.Equal(t, "test-client", clientInfo.GetID())
	assert.Equal(t, "secret", clientInfo.GetSecret())

	// Try to create duplicate
	err = store.CreateClient(ctx, client)
	assert.Error(t, err)

	// Get non-existent client
	_, err = store.GetClient(ctx, "non-existent")
	assert.Error(t, err)

	// Update client
	client.Name = "Updated Name"
	err = store.UpdateClient(ctx, client)
	require.NoError(t, err)

	// List by user
	clients, err := store.ListClientsByUser(ctx, "user-123")
	require.NoError(t, err)
	assert.Len(t, clients, 1)

	// Delete client
	err = store.DeleteClient(ctx, "test-client")
	require.NoError(t, err)

	_, err = store.GetClient(ctx, "test-client")
	assert.Error(t, err)
}

func TestTokenStoreAdapter(t *testing.T) {
	store := newTokenStoreAdapter()
	ctx := context.Background()

	// Create a mock token using go-oauth2's models
	ti := &mockTokenInfo{
		clientID: "client-1",
		userID:   "user-1",
		scope:    "read write",
		access:   "test-access-token",
	}

	err := store.Create(ctx, ti)
	require.NoError(t, err)

	// Verify we can retrieve it
	retrieved, err := store.GetByAccess(ctx, "test-access-token")
	require.NoError(t, err)
	assert.Equal(t, "client-1", retrieved.GetClientID())
}

// mockTokenInfo implements oauth2.TokenInfo for testing
type mockTokenInfo struct {
	clientID string
	userID   string
	scope    string
	access   string
	refresh  string
	code     string
}

func (m *mockTokenInfo) New() oauth2.TokenInfo         { return &mockTokenInfo{} }
func (m *mockTokenInfo) GetClientID() string           { return m.clientID }
func (m *mockTokenInfo) SetClientID(s string)          { m.clientID = s }
func (m *mockTokenInfo) GetUserID() string             { return m.userID }
func (m *mockTokenInfo) SetUserID(s string)            { m.userID = s }
func (m *mockTokenInfo) GetScope() string              { return m.scope }
func (m *mockTokenInfo) SetScope(s string)             { m.scope = s }
func (m *mockTokenInfo) GetCode() string               { return m.code }
func (m *mockTokenInfo) SetCode(s string)              { m.code = s }
func (m *mockTokenInfo) GetAccess() string             { return m.access }
func (m *mockTokenInfo) SetAccess(s string)            { m.access = s }
func (m *mockTokenInfo) GetRefresh() string            { return m.refresh }
func (m *mockTokenInfo) SetRefresh(s string)           { m.refresh = s }
func (m *mockTokenInfo) GetRedirectURI() string        { return "" }
func (m *mockTokenInfo) SetRedirectURI(string)         {}
func (m *mockTokenInfo) GetAccessCreateAt() time.Time  { return time.Now() }
func (m *mockTokenInfo) SetAccessCreateAt(time.Time)   {}
func (m *mockTokenInfo) GetAccessExpiresIn() time.Duration { return time.Hour }
func (m *mockTokenInfo) SetAccessExpiresIn(time.Duration) {}
func (m *mockTokenInfo) GetRefreshCreateAt() time.Time  { return time.Now() }
func (m *mockTokenInfo) SetRefreshCreateAt(time.Time)   {}
func (m *mockTokenInfo) GetRefreshExpiresIn() time.Duration { return 24 * time.Hour }
func (m *mockTokenInfo) SetRefreshExpiresIn(time.Duration) {}
func (m *mockTokenInfo) GetCodeCreateAt() time.Time     { return time.Now() }
func (m *mockTokenInfo) SetCodeCreateAt(time.Time)      {}
func (m *mockTokenInfo) GetCodeExpiresIn() time.Duration { return 10 * time.Minute }
func (m *mockTokenInfo) SetCodeExpiresIn(time.Duration) {}
func (m *mockTokenInfo) GetCodeChallenge() string       { return "" }
func (m *mockTokenInfo) SetCodeChallenge(string)        {}
func (m *mockTokenInfo) GetCodeChallengeMethod() oauth2.CodeChallengeMethod { return "" }
func (m *mockTokenInfo) SetCodeChallengeMethod(oauth2.CodeChallengeMethod) {}

func TestScopeHelpers(t *testing.T) {
	// Test ParseScopes
	scopes := ParseScopes("read write admin")
	assert.Equal(t, []string{"read", "write", "admin"}, scopes)

	emptyScopes := ParseScopes("")
	assert.Nil(t, emptyScopes)

	// Test FormatScopes
	formatted := FormatScopes([]string{"read", "write"})
	assert.Equal(t, "read write", formatted)

	// Test HasScope with context
	ctx := context.Background()
	ctx = WithOAuthScopes(ctx, []string{"read", "write"})

	assert.True(t, HasScope(ctx, "read"))
	assert.True(t, HasScope(ctx, "write"))
	assert.False(t, HasScope(ctx, "admin"))

	// Test HasAnyScope
	assert.True(t, HasAnyScope(ctx, "admin", "read"))
	assert.False(t, HasAnyScope(ctx, "admin", "delete"))

	// Test HasAllScopes
	assert.True(t, HasAllScopes(ctx, "read", "write"))
	assert.False(t, HasAllScopes(ctx, "read", "admin"))

	// Test IsOAuthRequest
	assert.False(t, IsOAuthRequest(ctx))

	ctx = WithOAuthClientID(ctx, "test-client")
	assert.True(t, IsOAuthRequest(ctx))
	assert.Equal(t, "test-client", OAuthClientIDFromContext(ctx))
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		md       metadata.MD
		expected string
	}{
		{
			name:     "bearer token",
			md:       metadata.Pairs("authorization", "Bearer my-token"),
			expected: "my-token",
		},
		{
			name:     "bearer lowercase",
			md:       metadata.Pairs("authorization", "bearer my-token"),
			expected: "my-token",
		},
		{
			name:     "no prefix",
			md:       metadata.Pairs("authorization", "my-token"),
			expected: "my-token",
		},
		{
			name:     "empty",
			md:       metadata.MD{},
			expected: "",
		},
		{
			name:     "empty value",
			md:       metadata.Pairs("authorization", ""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBearerToken(tt.md)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestOAuthPlugin_ClientCredentialsFlow(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Scopes:       []string{"read", "write"},
		}).
		Build()

	handler := plugin.tokenHandler()

	// Test client credentials grant
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", "test-client")
	form.Set("client_secret", "test-secret")
	form.Set("scope", "read")

	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// go-oauth2 should handle this and return a token
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response["access_token"])
	assert.Equal(t, "Bearer", response["token_type"])
}

func TestOAuthPlugin_InvalidClient(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	handler := plugin.tokenHandler()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", "wrong-client")
	form.Set("client_secret", "wrong-secret")

	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail with invalid client
	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestOAuthPlugin_AddClient(t *testing.T) {
	plugin := NewBuilder().Build()

	// Add client dynamically
	plugin.AddClient(Client{
		ID:           "dynamic-client",
		Secret:       "secret",
		RedirectURIs: []string{"http://localhost/callback"},
	})

	// Verify it was added
	client, err := plugin.GetClientStore().GetClient(context.Background(), "dynamic-client")
	require.NoError(t, err)
	assert.Equal(t, "dynamic-client", client.ID)
}
