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
	memStore := NewMemoryTokenStore()
	store := newTokenStoreAdapter(memStore)
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

func TestMemoryTokenStore(t *testing.T) {
	store := NewMemoryTokenStore()
	ctx := context.Background()

	// Create token
	info := TokenInfo{
		ClientID: "client-1",
		UserID:   "user-1",
		Scope:    "read write",
		Access:   "test-access-token",
		Refresh:  "test-refresh-token",
		Code:     "test-code",
	}

	err := store.Create(ctx, info)
	require.NoError(t, err)

	// Test GetByAccess
	retrieved, err := store.GetByAccess(ctx, "test-access-token")
	require.NoError(t, err)
	assert.Equal(t, "client-1", retrieved.ClientID)

	// Test GetByRefresh
	retrieved, err = store.GetByRefresh(ctx, "test-refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "client-1", retrieved.ClientID)

	// Test GetByCode
	retrieved, err = store.GetByCode(ctx, "test-code")
	require.NoError(t, err)
	assert.Equal(t, "client-1", retrieved.ClientID)

	// Test RemoveByAccess
	err = store.RemoveByAccess(ctx, "test-access-token")
	require.NoError(t, err)
	_, err = store.GetByAccess(ctx, "test-access-token")
	assert.Error(t, err)

	// Test RemoveByRefresh
	err = store.RemoveByRefresh(ctx, "test-refresh-token")
	require.NoError(t, err)
	_, err = store.GetByRefresh(ctx, "test-refresh-token")
	assert.Error(t, err)

	// Test RemoveByCode
	err = store.RemoveByCode(ctx, "test-code")
	require.NoError(t, err)
	_, err = store.GetByCode(ctx, "test-code")
	assert.Error(t, err)
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

func (m *mockTokenInfo) New() oauth2.TokenInfo                              { return &mockTokenInfo{} }
func (m *mockTokenInfo) GetClientID() string                                { return m.clientID }
func (m *mockTokenInfo) SetClientID(s string)                               { m.clientID = s }
func (m *mockTokenInfo) GetUserID() string                                  { return m.userID }
func (m *mockTokenInfo) SetUserID(s string)                                 { m.userID = s }
func (m *mockTokenInfo) GetScope() string                                   { return m.scope }
func (m *mockTokenInfo) SetScope(s string)                                  { m.scope = s }
func (m *mockTokenInfo) GetCode() string                                    { return m.code }
func (m *mockTokenInfo) SetCode(s string)                                   { m.code = s }
func (m *mockTokenInfo) GetAccess() string                                  { return m.access }
func (m *mockTokenInfo) SetAccess(s string)                                 { m.access = s }
func (m *mockTokenInfo) GetRefresh() string                                 { return m.refresh }
func (m *mockTokenInfo) SetRefresh(s string)                                { m.refresh = s }
func (m *mockTokenInfo) GetRedirectURI() string                             { return "" }
func (m *mockTokenInfo) SetRedirectURI(string)                              {}
func (m *mockTokenInfo) GetAccessCreateAt() time.Time                       { return time.Now() }
func (m *mockTokenInfo) SetAccessCreateAt(time.Time)                        {}
func (m *mockTokenInfo) GetAccessExpiresIn() time.Duration                  { return time.Hour }
func (m *mockTokenInfo) SetAccessExpiresIn(time.Duration)                   {}
func (m *mockTokenInfo) GetRefreshCreateAt() time.Time                      { return time.Now() }
func (m *mockTokenInfo) SetRefreshCreateAt(time.Time)                       {}
func (m *mockTokenInfo) GetRefreshExpiresIn() time.Duration                 { return 24 * time.Hour }
func (m *mockTokenInfo) SetRefreshExpiresIn(time.Duration)                  {}
func (m *mockTokenInfo) GetCodeCreateAt() time.Time                         { return time.Now() }
func (m *mockTokenInfo) SetCodeCreateAt(time.Time)                          {}
func (m *mockTokenInfo) GetCodeExpiresIn() time.Duration                    { return 10 * time.Minute }
func (m *mockTokenInfo) SetCodeExpiresIn(time.Duration)                     {}
func (m *mockTokenInfo) GetCodeChallenge() string                           { return "" }
func (m *mockTokenInfo) SetCodeChallenge(string)                            {}
func (m *mockTokenInfo) GetCodeChallengeMethod() oauth2.CodeChallengeMethod { return "" }
func (m *mockTokenInfo) SetCodeChallengeMethod(oauth2.CodeChallengeMethod)  {}

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
	require.NoError(t, plugin.AddClient(Client{
		ID:           "dynamic-client",
		Secret:       "secret",
		RedirectURIs: []string{"http://localhost/callback"},
	}))

	// Verify it was added
	client, err := plugin.GetClientStore().GetClient(context.Background(), "dynamic-client")
	require.NoError(t, err)
	assert.Equal(t, "dynamic-client", client.ID)
}

func TestOAuthPlugin_ScopeValidation(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "restricted-client",
			Secret:       "secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Scopes:       []string{"read", "write"}, // Only these scopes allowed
		}).
		WithClient(Client{
			ID:           "unrestricted-client",
			Secret:       "secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Scopes:       []string{}, // No restrictions
		}).
		Build()

	ctx := context.Background()

	// Test restricted client - allowed scopes
	scope, err := plugin.validateScopes(ctx, "restricted-client", "read")
	require.NoError(t, err)
	assert.Equal(t, "read", scope)

	scope, err = plugin.validateScopes(ctx, "restricted-client", "read write")
	require.NoError(t, err)
	assert.Equal(t, "read write", scope)

	// Test restricted client - disallowed scope
	_, err = plugin.validateScopes(ctx, "restricted-client", "admin")
	assert.ErrorIs(t, err, ErrInvalidScope)

	_, err = plugin.validateScopes(ctx, "restricted-client", "read admin")
	assert.ErrorIs(t, err, ErrInvalidScope)

	// Test unrestricted client - any scope allowed
	scope, err = plugin.validateScopes(ctx, "unrestricted-client", "anything goes")
	require.NoError(t, err)
	assert.Equal(t, "anything goes", scope)

	// Test empty scope
	scope, err = plugin.validateScopes(ctx, "restricted-client", "")
	require.NoError(t, err)
	assert.Equal(t, "", scope)
}

func TestOAuthPlugin_RedirectURIValidation(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:     "multi-redirect-client",
			Secret: "secret",
			RedirectURIs: []string{
				"http://localhost:8080/callback",
				"http://localhost:3000/callback",
				"https://example.com/oauth/callback",
			},
		}).
		Build()

	// Get the client to verify GetDomain returns all URIs
	clientInfo, err := plugin.clientStore.GetByID(context.Background(), "multi-redirect-client")
	require.NoError(t, err)

	domain := clientInfo.GetDomain()
	assert.Contains(t, domain, "http://localhost:8080/callback")
	assert.Contains(t, domain, "http://localhost:3000/callback")
	assert.Contains(t, domain, "https://example.com/oauth/callback")
}

func TestOAuthPlugin_PKCEEnforcement(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "public-client",
			Secret:       "",
			RedirectURIs: []string{"http://localhost/callback"},
			Public:       true,
		}).
		WithClient(Client{
			ID:           "confidential-client",
			Secret:       "secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Public:       false,
		}).
		WithEnforcePKCE(true).
		Build()

	// Test that public client without code_challenge is rejected
	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=public-client&response_type=code&redirect_uri=http://localhost/callback", nil)
	err := plugin.validatePKCERequired(req)
	assert.ErrorIs(t, err, ErrPKCERequired)

	// Test that public client with code_challenge is accepted
	req = httptest.NewRequest("GET", "/oauth/authorize?client_id=public-client&response_type=code&redirect_uri=http://localhost/callback&code_challenge=abc123&code_challenge_method=S256", nil)
	err = plugin.validatePKCERequired(req)
	assert.NoError(t, err)

	// Test that confidential client without code_challenge is accepted (PKCE not required)
	req = httptest.NewRequest("GET", "/oauth/authorize?client_id=confidential-client&response_type=code&redirect_uri=http://localhost/callback", nil)
	err = plugin.validatePKCERequired(req)
	assert.NoError(t, err)

	// Test invalid code_challenge_method
	req = httptest.NewRequest("GET", "/oauth/authorize?client_id=public-client&response_type=code&redirect_uri=http://localhost/callback&code_challenge=abc123&code_challenge_method=invalid", nil)
	err = plugin.validatePKCERequired(req)
	assert.ErrorIs(t, err, ErrInvalidGrant)
}

func TestOAuthPlugin_PKCEEnforcementDisabled(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "public-client",
			Secret:       "",
			RedirectURIs: []string{"http://localhost/callback"},
			Public:       true,
		}).
		WithEnforcePKCE(false).
		Build()

	// When PKCE is not enforced, public client without code_challenge should be allowed
	assert.False(t, plugin.shouldEnforcePKCE())
}

func TestOAuthPlugin_TokenRevocation(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	// Create a token in the store
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:         "test-client",
		UserID:           "user-1",
		Scope:            "read write",
		Access:           "test-access-token-revoke",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Hour,
		Refresh:          "test-refresh-token-revoke",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: 24 * time.Hour,
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	// Verify token exists
	_, err = plugin.tokenStore.store.GetByAccess(ctx, "test-access-token-revoke")
	require.NoError(t, err)

	handler := plugin.revokeHandler()

	// Test revocation with Basic auth
	form := url.Values{}
	form.Set("token", "test-access-token-revoke")
	form.Set("token_type_hint", "access_token")

	req := httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify token was removed
	_, err = plugin.tokenStore.store.GetByAccess(ctx, "test-access-token-revoke")
	assert.Error(t, err)
}

func TestOAuthPlugin_TokenRevocation_InvalidClient(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	handler := plugin.revokeHandler()

	form := url.Values{}
	form.Set("token", "some-token")

	req := httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("wrong-client", "wrong-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestOAuthPlugin_TokenRevocation_NonExistentToken(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	handler := plugin.revokeHandler()

	form := url.Values{}
	form.Set("token", "non-existent-token")

	req := httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// RFC 7009: Always return 200 OK even for non-existent tokens
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestOAuthPlugin_TokenIntrospection(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		WithIssuer("https://auth.example.com").
		Build()

	// Create a token in the store
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:         "test-client",
		UserID:           "user-123",
		Scope:            "read write",
		Access:           "test-access-token-introspect",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Hour,
		Refresh:          "test-refresh-token-introspect",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: 24 * time.Hour,
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	handler := plugin.introspectHandler()

	// Test introspection of valid access token
	form := url.Values{}
	form.Set("token", "test-access-token-introspect")

	req := httptest.NewRequest("POST", "/oauth/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, true, response["active"])
	assert.Equal(t, "test-client", response["client_id"])
	assert.Equal(t, "user-123", response["sub"])
	assert.Equal(t, "read write", response["scope"])
	assert.Equal(t, "Bearer", response["token_type"])
	assert.Equal(t, "https://auth.example.com", response["iss"])
}

func TestOAuthPlugin_TokenIntrospection_ExpiredToken(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	// Create an expired token
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:        "test-client",
		UserID:          "user-123",
		Access:          "expired-token",
		AccessCreateAt:  time.Now().Add(-2 * time.Hour),
		AccessExpiresIn: time.Hour, // Expired 1 hour ago
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	handler := plugin.introspectHandler()

	form := url.Values{}
	form.Set("token", "expired-token")

	req := httptest.NewRequest("POST", "/oauth/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, false, response["active"])
}

func TestOAuthPlugin_TokenIntrospection_NotFound(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	handler := plugin.introspectHandler()

	form := url.Values{}
	form.Set("token", "non-existent-token")

	req := httptest.NewRequest("POST", "/oauth/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, false, response["active"])
}

func TestOAuthPlugin_MetadataIncludesNewEndpoints(t *testing.T) {
	plugin := NewBuilder().
		WithIssuer("https://auth.example.com").
		Build()

	handler := plugin.metadataHandler()

	req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var meta map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &meta)
	require.NoError(t, err)

	assert.Equal(t, "https://auth.example.com/oauth/revoke", meta["revocation_endpoint"])
	assert.Equal(t, "https://auth.example.com/oauth/introspect", meta["introspection_endpoint"])
}

func TestOAuthPlugin_TokenRevocation_OwnershipValidation(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "client-a",
			Secret:       "secret-a",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		WithClient(Client{
			ID:           "client-b",
			Secret:       "secret-b",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	// Create a token owned by client-a
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:         "client-a",
		UserID:           "user-1",
		Scope:            "read write",
		Access:           "token-owned-by-client-a",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Hour,
		Refresh:          "refresh-owned-by-client-a",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: 24 * time.Hour,
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	handler := plugin.revokeHandler()

	// client-b tries to revoke client-a's token - should not work
	form := url.Values{}
	form.Set("token", "token-owned-by-client-a")

	req := httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("client-b", "secret-b")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// RFC 7009: Always return 200 OK (but token should NOT be revoked)
	assert.Equal(t, http.StatusOK, w.Code)

	// Token should still exist
	_, err = plugin.tokenStore.store.GetByAccess(ctx, "token-owned-by-client-a")
	require.NoError(t, err, "token should not have been revoked by wrong client")

	// Now client-a revokes its own token - should work
	req = httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("client-a", "secret-a")
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Token should be gone now
	_, err = plugin.tokenStore.store.GetByAccess(ctx, "token-owned-by-client-a")
	assert.Error(t, err, "token should have been revoked by owning client")
}

func TestOAuthPlugin_TokenIntrospection_OwnershipValidation(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "client-a",
			Secret:       "secret-a",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		WithClient(Client{
			ID:           "client-b",
			Secret:       "secret-b",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		WithIssuer("https://auth.example.com").
		Build()

	// Create a token owned by client-a
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:         "client-a",
		UserID:           "user-1",
		Scope:            "read write",
		Access:           "introspect-token-client-a",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Hour,
		Refresh:          "introspect-refresh-client-a",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: 24 * time.Hour,
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	handler := plugin.introspectHandler()

	// client-b tries to introspect client-a's token - should return inactive
	form := url.Values{}
	form.Set("token", "introspect-token-client-a")

	req := httptest.NewRequest("POST", "/oauth/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("client-b", "secret-b")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, false, response["active"], "token should appear inactive to wrong client")

	// client-a introspects its own token - should return active
	req = httptest.NewRequest("POST", "/oauth/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("client-a", "secret-a")
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["active"], "token should appear active to owning client")
	assert.Equal(t, "client-a", response["client_id"])
}

func TestOAuthPlugin_RefreshTokenIntrospection(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	// Create a token with refresh token
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:         "test-client",
		UserID:           "user-1",
		Scope:            "read",
		Access:           "access-for-refresh-test",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Hour,
		Refresh:          "refresh-token-to-introspect",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: 24 * time.Hour,
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	handler := plugin.introspectHandler()

	// Introspect refresh token with hint
	form := url.Values{}
	form.Set("token", "refresh-token-to-introspect")
	form.Set("token_type_hint", "refresh_token")

	req := httptest.NewRequest("POST", "/oauth/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["active"])
	assert.Equal(t, "test-client", response["client_id"])
	assert.Equal(t, "read", response["scope"])
	// Refresh tokens don't have token_type
	_, hasTokenType := response["token_type"]
	assert.False(t, hasTokenType, "refresh tokens should not have token_type")
}

func TestOAuthPlugin_RevokeRefreshToken(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	// Create a token
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:         "test-client",
		UserID:           "user-1",
		Access:           "access-token-for-refresh-revoke",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Hour,
		Refresh:          "refresh-token-to-revoke",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: 24 * time.Hour,
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	handler := plugin.revokeHandler()

	// Revoke the refresh token with hint
	form := url.Values{}
	form.Set("token", "refresh-token-to-revoke")
	form.Set("token_type_hint", "refresh_token")

	req := httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Refresh token should be gone
	_, err = plugin.tokenStore.store.GetByRefresh(ctx, "refresh-token-to-revoke")
	assert.Error(t, err)
}

func TestOAuthPlugin_AuthorizeHandler_PKCE(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "public-client",
			Secret:       "",
			RedirectURIs: []string{"http://localhost/callback"},
			Public:       true,
		}).
		WithEnforcePKCE(true).
		Build()

	handler := plugin.authorizeHandler()

	// Request without PKCE should fail for public client
	req := httptest.NewRequest("GET", "/oauth/authorize?client_id=public-client&response_type=code&redirect_uri=http://localhost/callback", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "invalid_request", response["error"])
}

func TestOAuthPlugin_RevokeMethodNotAllowed(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	handler := plugin.revokeHandler()

	// GET request should fail
	req := httptest.NewRequest("GET", "/oauth/revoke?token=some-token", nil)
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestOAuthPlugin_IntrospectMethodNotAllowed(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	handler := plugin.introspectHandler()

	// GET request should fail
	req := httptest.NewRequest("GET", "/oauth/introspect?token=some-token", nil)
	req.SetBasicAuth("test-client", "test-secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestOAuthPlugin_FormEncodedClientCredentials(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
		}).
		Build()

	// Create a token
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:        "test-client",
		UserID:          "user-1",
		Access:          "token-for-form-auth",
		AccessCreateAt:  time.Now(),
		AccessExpiresIn: time.Hour,
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	handler := plugin.introspectHandler()

	// Use form-encoded credentials instead of Basic auth
	form := url.Values{}
	form.Set("token", "token-for-form-auth")
	form.Set("client_id", "test-client")
	form.Set("client_secret", "test-secret")

	req := httptest.NewRequest("POST", "/oauth/introspect", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, true, response["active"])
}

func TestOAuthPlugin_PublicClientRevocation(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "public-client",
			Secret:       "",
			RedirectURIs: []string{"http://localhost/callback"},
			Public:       true,
		}).
		Build()

	// Create a token for the public client
	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:        "public-client",
		UserID:          "user-1",
		Access:          "public-client-token",
		AccessCreateAt:  time.Now(),
		AccessExpiresIn: time.Hour,
	}
	err := plugin.tokenStore.store.Create(ctx, tokenInfo)
	require.NoError(t, err)

	handler := plugin.revokeHandler()

	// Public client revokes its token (no secret required)
	form := url.Values{}
	form.Set("token", "public-client-token")
	form.Set("client_id", "public-client")

	req := httptest.NewRequest("POST", "/oauth/revoke", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Token should be revoked
	_, err = plugin.tokenStore.store.GetByAccess(ctx, "public-client-token")
	assert.Error(t, err)
}

// TestOAuthPlugin_RefreshToken_RequiresClientAuth verifies that the refresh
// token grant requires valid client credentials. Without this, anyone who
// obtains a refresh token can mint new access tokens.
func TestOAuthPlugin_RefreshToken_RequiresClientAuth(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "confidential",
			Secret:       "correct-secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Scopes:       []string{"read", "write"},
		}).
		Build()

	ctx := context.Background()
	tokenInfo := TokenInfo{
		ClientID:         "confidential",
		UserID:           "user-1",
		Scope:            "read write",
		Access:           "stolen-access",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Hour,
		Refresh:          "stolen-refresh",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: 24 * time.Hour,
	}
	require.NoError(t, plugin.tokenStore.store.Create(ctx, tokenInfo))

	handler := plugin.tokenHandler()

	t.Run("no client auth is rejected", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", "stolen-refresh")
		req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"refresh without client credentials must be rejected")
	})

	t.Run("wrong secret is rejected", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", "stolen-refresh")
		form.Set("client_id", "confidential")
		form.Set("client_secret", "wrong-secret")
		req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"refresh with wrong client secret must be rejected")
	})

	t.Run("cross-client refresh is rejected", func(t *testing.T) {
		plugin := NewBuilder().
			WithClient(Client{
				ID:           "victim",
				Secret:       "victim-secret",
				RedirectURIs: []string{"http://localhost/callback"},
			}).
			WithClient(Client{
				ID:           "attacker",
				Secret:       "attacker-secret",
				RedirectURIs: []string{"http://localhost/callback"},
			}).
			Build()

		ctx := context.Background()
		require.NoError(t, plugin.tokenStore.store.Create(ctx, TokenInfo{
			ClientID:         "victim",
			UserID:           "user-1",
			Access:           "victim-access",
			AccessCreateAt:   time.Now(),
			AccessExpiresIn:  time.Hour,
			Refresh:          "victim-refresh",
			RefreshCreateAt:  time.Now(),
			RefreshExpiresIn: 24 * time.Hour,
		}))

		handler := plugin.tokenHandler()

		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", "victim-refresh")
		req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("attacker", "attacker-secret")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"attacker client must not be able to refresh another client's token")
	})

	t.Run("correct credentials succeed", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", "stolen-refresh")
		form.Set("client_id", "confidential")
		form.Set("client_secret", "correct-secret")
		req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "valid refresh should succeed; got body=%s", w.Body.String())
	})
}

// TestOAuthPlugin_ClientCredentialsScopeBypass verifies that the configured
// client scope allowlist is enforced for the client_credentials grant. Without
// this, clients could mint tokens with arbitrary scopes beyond their grant.
func TestOAuthPlugin_ClientCredentialsScopeBypass(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "limited",
			Secret:       "secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Scopes:       []string{"read"},
		}).
		Build()

	handler := plugin.tokenHandler()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", "limited")
	form.Set("client_secret", "secret")
	form.Set("scope", "read admin delete")

	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code,
		"client must not receive scopes beyond its allowlist")
}

// TestOAuthPlugin_RefreshScopeEscalation verifies that the refresh_token grant
// cannot escalate to scopes beyond those originally granted.
func TestOAuthPlugin_RefreshScopeEscalation(t *testing.T) {
	plugin := NewBuilder().
		WithClient(Client{
			ID:           "confidential",
			Secret:       "secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Scopes:       []string{"read", "write", "admin"},
		}).
		Build()

	ctx := context.Background()
	require.NoError(t, plugin.tokenStore.store.Create(ctx, TokenInfo{
		ClientID:         "confidential",
		UserID:           "user-1",
		Scope:            "read",
		Access:           "a1",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Hour,
		Refresh:          "r1",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: 24 * time.Hour,
	}))

	handler := plugin.tokenHandler()

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", "r1")
	form.Set("scope", "read admin") // attempts to gain 'admin'
	req := httptest.NewRequest("POST", "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("confidential", "secret")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code,
		"refresh must not grant scopes beyond the original token's scope")
}

// TestOAuthPlugin_EmptySecretConfidentialClient_Rejected verifies that a
// confidential client misconfigured with an empty secret is rejected at
// registration time and — as defense in depth — that a client injected
// directly into a store bypassing validation cannot authenticate with an
// empty secret.
func TestOAuthPlugin_EmptySecretConfidentialClient_Rejected(t *testing.T) {
	t.Run("WithClient panics on empty-secret confidential client", func(t *testing.T) {
		defer func() {
			r := recover()
			assert.NotNil(t, r, "expected panic for invalid static client")
		}()
		_ = NewBuilder().
			WithClient(Client{
				ID:           "misconfigured",
				Secret:       "",
				Public:       false,
				RedirectURIs: []string{"http://localhost/callback"},
			}).
			Build()
	})

	t.Run("authenticateClient rejects empty secret even if client bypasses validation", func(t *testing.T) {
		plugin := NewBuilder().Build()
		ctx := context.Background()

		// Bypass validation by injecting directly into the adapter's store via
		// a raw map-level memory store.
		raw := newMemoryClientStore()
		raw.clients["misconfigured"] = &Client{
			ID:           "misconfigured",
			Secret:       "",
			Public:       false,
			RedirectURIs: []string{"http://localhost/callback"},
		}
		plugin.clientStore = newClientStoreAdapter(raw)
		require.NoError(t, plugin.tokenStore.store.Create(ctx, TokenInfo{
			ClientID:        "misconfigured",
			UserID:          "user-1",
			Access:          "a1",
			AccessCreateAt:  time.Now(),
			AccessExpiresIn: time.Hour,
		}))

		handler := plugin.introspectHandler()
		form := url.Values{}
		form.Set("token", "a1")
		req := httptest.NewRequest("POST", "/oauth/introspect", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("misconfigured", "")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code,
			"confidential client with empty stored secret must not authenticate")
	})
}

// TestClient_Validate exercises the redirect URI and secret rules.
func TestClient_Validate(t *testing.T) {
	tests := []struct {
		name   string
		client Client
		ok     bool
	}{
		{
			name:   "confidential client without secret is rejected",
			client: Client{ID: "c1", RedirectURIs: []string{"http://localhost/cb"}},
			ok:     false,
		},
		{
			name:   "public client with secret is rejected",
			client: Client{ID: "c2", Secret: "s", Public: true, RedirectURIs: []string{"http://localhost/cb"}},
			ok:     false,
		},
		{
			name:   "redirect URI with embedded newline is rejected",
			client: Client{ID: "c3", Secret: "s", RedirectURIs: []string{"http://good/cb\nhttp://evil/cb"}},
			ok:     false,
		},
		{
			name:   "redirect URI with carriage return is rejected",
			client: Client{ID: "c4", Secret: "s", RedirectURIs: []string{"http://good/cb\revil"}},
			ok:     false,
		},
		{
			name:   "relative redirect URI is rejected",
			client: Client{ID: "c5", Secret: "s", RedirectURIs: []string{"/not-absolute"}},
			ok:     false,
		},
		{
			name:   "missing ID is rejected",
			client: Client{Secret: "s", RedirectURIs: []string{"http://localhost/cb"}},
			ok:     false,
		},
		{
			name:   "valid confidential client",
			client: Client{ID: "ok", Secret: "s", RedirectURIs: []string{"http://localhost/cb"}},
			ok:     true,
		},
		{
			name:   "valid public client with no secret",
			client: Client{ID: "ok", Public: true, RedirectURIs: []string{"myapp://cb"}},
			ok:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.Validate()
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
