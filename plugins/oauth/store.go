package oauth

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-oauth2/oauth2/v4"
)

// ClientStore defines the interface for OAuth client storage.
type ClientStore interface {
	GetClient(ctx context.Context, clientID string) (*Client, error)
	CreateClient(ctx context.Context, client *Client) error
	UpdateClient(ctx context.Context, client *Client) error
	DeleteClient(ctx context.Context, clientID string) error
	ListClientsByUser(ctx context.Context, userID string) ([]*Client, error)
}

// TokenStore defines the interface for OAuth token storage.
// Implement this interface to persist tokens in a database or other storage.
type TokenStore interface {
	// Create stores a new token. The token may have an access token, refresh token,
	// and/or authorization code set. All non-empty values should be indexed for lookup.
	Create(ctx context.Context, info TokenInfo) error
	// RemoveByCode removes a token by its authorization code.
	RemoveByCode(ctx context.Context, code string) error
	// RemoveByAccess removes a token by its access token.
	RemoveByAccess(ctx context.Context, access string) error
	// RemoveByRefresh removes a token by its refresh token.
	RemoveByRefresh(ctx context.Context, refresh string) error
	// GetByCode retrieves a token by its authorization code.
	GetByCode(ctx context.Context, code string) (TokenInfo, error)
	// GetByAccess retrieves a token by its access token.
	GetByAccess(ctx context.Context, access string) (TokenInfo, error)
	// GetByRefresh retrieves a token by its refresh token.
	GetByRefresh(ctx context.Context, refresh string) (TokenInfo, error)
}

// TokenInfo represents the data stored for an OAuth token.
// This is a simplified version of oauth2.TokenInfo for storage purposes.
type TokenInfo struct {
	ClientID            string
	UserID              string
	Scope               string
	Code                string
	CodeCreateAt        time.Time
	CodeExpiresIn       time.Duration
	CodeChallenge       string
	CodeChallengeMethod string
	Access              string
	AccessCreateAt      time.Time
	AccessExpiresIn     time.Duration
	Refresh             string
	RefreshCreateAt     time.Time
	RefreshExpiresIn    time.Duration
	RedirectURI         string
}

// clientStoreAdapter adapts ClientStore to go-oauth2's ClientStore interface.
type clientStoreAdapter struct {
	store ClientStore
}

func newClientStoreAdapter(store ClientStore) *clientStoreAdapter {
	return &clientStoreAdapter{store: store}
}

// GetByID implements oauth2.ClientStore.
func (s *clientStoreAdapter) GetByID(ctx context.Context, id string) (oauth2.ClientInfo, error) {
	client, err := s.store.GetClient(ctx, id)
	if err != nil {
		return nil, err
	}
	return &clientAdapter{client: *client}, nil
}

// memoryClientStore is an in-memory implementation of ClientStore.
type memoryClientStore struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func newMemoryClientStore() *memoryClientStore {
	return &memoryClientStore{
		clients: make(map[string]*Client),
	}
}

// GetClient retrieves a client by ID.
func (s *memoryClientStore) GetClient(ctx context.Context, clientID string) (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	client, ok := s.clients[clientID]
	if !ok {
		return nil, ErrInvalidClient
	}
	return client, nil
}

// CreateClient creates a new client.
func (s *memoryClientStore) CreateClient(ctx context.Context, client *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[client.ID]; exists {
		return ErrInvalidClient
	}
	c := *client
	s.clients[client.ID] = &c
	return nil
}

// UpdateClient updates an existing client.
func (s *memoryClientStore) UpdateClient(ctx context.Context, client *Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[client.ID]; !exists {
		return ErrInvalidClient
	}
	c := *client
	s.clients[client.ID] = &c
	return nil
}

// DeleteClient deletes a client.
func (s *memoryClientStore) DeleteClient(ctx context.Context, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.clients, clientID)
	return nil
}

// ListClientsByUser lists all clients created by a user.
func (s *memoryClientStore) ListClientsByUser(ctx context.Context, userID string) ([]*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var clients []*Client
	for _, client := range s.clients {
		if client.CreatedBy == userID {
			c := *client
			clients = append(clients, &c)
		}
	}
	return clients, nil
}

// clientAdapter adapts our Client to go-oauth2's ClientInfo interface.
type clientAdapter struct {
	client Client
}

func (c *clientAdapter) GetID() string     { return c.client.ID }
func (c *clientAdapter) GetSecret() string { return c.client.Secret }
func (c *clientAdapter) GetDomain() string {
	// Return all redirect URIs joined by newline for custom validation handler
	return strings.Join(c.client.RedirectURIs, "\n")
}
func (c *clientAdapter) IsPublic() bool { return c.client.Public }
func (c *clientAdapter) GetUserID() string {
	return c.client.CreatedBy
}

// tokenStoreAdapter adapts TokenStore to go-oauth2's TokenStore interface.
type tokenStoreAdapter struct {
	store TokenStore
}

func newTokenStoreAdapter(store TokenStore) *tokenStoreAdapter {
	return &tokenStoreAdapter{store: store}
}

// Create stores a new token.
func (s *tokenStoreAdapter) Create(ctx context.Context, info oauth2.TokenInfo) error {
	return s.store.Create(ctx, tokenInfoFromOAuth2(info))
}

// RemoveByCode removes a token by authorization code.
func (s *tokenStoreAdapter) RemoveByCode(ctx context.Context, code string) error {
	return s.store.RemoveByCode(ctx, code)
}

// RemoveByAccess removes a token by access token.
func (s *tokenStoreAdapter) RemoveByAccess(ctx context.Context, access string) error {
	return s.store.RemoveByAccess(ctx, access)
}

// RemoveByRefresh removes a token by refresh token.
func (s *tokenStoreAdapter) RemoveByRefresh(ctx context.Context, refresh string) error {
	return s.store.RemoveByRefresh(ctx, refresh)
}

// GetByCode retrieves a token by authorization code.
func (s *tokenStoreAdapter) GetByCode(ctx context.Context, code string) (oauth2.TokenInfo, error) {
	info, err := s.store.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return &tokenInfoAdapter{info: info}, nil
}

// GetByAccess retrieves a token by access token.
func (s *tokenStoreAdapter) GetByAccess(ctx context.Context, access string) (oauth2.TokenInfo, error) {
	info, err := s.store.GetByAccess(ctx, access)
	if err != nil {
		return nil, err
	}
	return &tokenInfoAdapter{info: info}, nil
}

// GetByRefresh retrieves a token by refresh token.
func (s *tokenStoreAdapter) GetByRefresh(ctx context.Context, refresh string) (oauth2.TokenInfo, error) {
	info, err := s.store.GetByRefresh(ctx, refresh)
	if err != nil {
		return nil, err
	}
	return &tokenInfoAdapter{info: info}, nil
}

// tokenInfoFromOAuth2 converts oauth2.TokenInfo to our TokenInfo.
func tokenInfoFromOAuth2(info oauth2.TokenInfo) TokenInfo {
	return TokenInfo{
		ClientID:            info.GetClientID(),
		UserID:              info.GetUserID(),
		Scope:               info.GetScope(),
		Code:                info.GetCode(),
		CodeCreateAt:        info.GetCodeCreateAt(),
		CodeExpiresIn:       info.GetCodeExpiresIn(),
		CodeChallenge:       info.GetCodeChallenge(),
		CodeChallengeMethod: string(info.GetCodeChallengeMethod()),
		Access:              info.GetAccess(),
		AccessCreateAt:      info.GetAccessCreateAt(),
		AccessExpiresIn:     info.GetAccessExpiresIn(),
		Refresh:             info.GetRefresh(),
		RefreshCreateAt:     info.GetRefreshCreateAt(),
		RefreshExpiresIn:    info.GetRefreshExpiresIn(),
		RedirectURI:         info.GetRedirectURI(),
	}
}

// tokenInfoAdapter adapts our TokenInfo to oauth2.TokenInfo interface.
type tokenInfoAdapter struct {
	info TokenInfo
}

func (t *tokenInfoAdapter) New() oauth2.TokenInfo                             { return &tokenInfoAdapter{} }
func (t *tokenInfoAdapter) GetClientID() string                               { return t.info.ClientID }
func (t *tokenInfoAdapter) SetClientID(s string)                              { t.info.ClientID = s }
func (t *tokenInfoAdapter) GetUserID() string                                 { return t.info.UserID }
func (t *tokenInfoAdapter) SetUserID(s string)                                { t.info.UserID = s }
func (t *tokenInfoAdapter) GetScope() string                                  { return t.info.Scope }
func (t *tokenInfoAdapter) SetScope(s string)                                 { t.info.Scope = s }
func (t *tokenInfoAdapter) GetCode() string                                   { return t.info.Code }
func (t *tokenInfoAdapter) SetCode(s string)                                  { t.info.Code = s }
func (t *tokenInfoAdapter) GetCodeCreateAt() time.Time                        { return t.info.CodeCreateAt }
func (t *tokenInfoAdapter) SetCodeCreateAt(s time.Time)                       { t.info.CodeCreateAt = s }
func (t *tokenInfoAdapter) GetCodeExpiresIn() time.Duration                   { return t.info.CodeExpiresIn }
func (t *tokenInfoAdapter) SetCodeExpiresIn(s time.Duration)                  { t.info.CodeExpiresIn = s }
func (t *tokenInfoAdapter) GetCodeChallenge() string                          { return t.info.CodeChallenge }
func (t *tokenInfoAdapter) SetCodeChallenge(s string)                         { t.info.CodeChallenge = s }
func (t *tokenInfoAdapter) GetCodeChallengeMethod() oauth2.CodeChallengeMethod {
	return oauth2.CodeChallengeMethod(t.info.CodeChallengeMethod)
}
func (t *tokenInfoAdapter) SetCodeChallengeMethod(s oauth2.CodeChallengeMethod) {
	t.info.CodeChallengeMethod = string(s)
}
func (t *tokenInfoAdapter) GetAccess() string                  { return t.info.Access }
func (t *tokenInfoAdapter) SetAccess(s string)                 { t.info.Access = s }
func (t *tokenInfoAdapter) GetAccessCreateAt() time.Time       { return t.info.AccessCreateAt }
func (t *tokenInfoAdapter) SetAccessCreateAt(s time.Time)      { t.info.AccessCreateAt = s }
func (t *tokenInfoAdapter) GetAccessExpiresIn() time.Duration  { return t.info.AccessExpiresIn }
func (t *tokenInfoAdapter) SetAccessExpiresIn(s time.Duration) { t.info.AccessExpiresIn = s }
func (t *tokenInfoAdapter) GetRefresh() string                 { return t.info.Refresh }
func (t *tokenInfoAdapter) SetRefresh(s string)                { t.info.Refresh = s }
func (t *tokenInfoAdapter) GetRefreshCreateAt() time.Time      { return t.info.RefreshCreateAt }
func (t *tokenInfoAdapter) SetRefreshCreateAt(s time.Time)     { t.info.RefreshCreateAt = s }
func (t *tokenInfoAdapter) GetRefreshExpiresIn() time.Duration { return t.info.RefreshExpiresIn }
func (t *tokenInfoAdapter) SetRefreshExpiresIn(s time.Duration) { t.info.RefreshExpiresIn = s }
func (t *tokenInfoAdapter) GetRedirectURI() string             { return t.info.RedirectURI }
func (t *tokenInfoAdapter) SetRedirectURI(s string)            { t.info.RedirectURI = s }

// memoryTokenStore is an in-memory implementation of TokenStore.
type memoryTokenStore struct {
	mu           sync.RWMutex
	accessTokens map[string]TokenInfo
	codes        map[string]TokenInfo
	refresh      map[string]TokenInfo
}

// NewMemoryTokenStore creates a new in-memory token store.
func NewMemoryTokenStore() TokenStore {
	return &memoryTokenStore{
		accessTokens: make(map[string]TokenInfo),
		codes:        make(map[string]TokenInfo),
		refresh:      make(map[string]TokenInfo),
	}
}

// Create stores a new token.
func (s *memoryTokenStore) Create(ctx context.Context, info TokenInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if info.Code != "" {
		s.codes[info.Code] = info
	}
	if info.Access != "" {
		s.accessTokens[info.Access] = info
	}
	if info.Refresh != "" {
		s.refresh[info.Refresh] = info
	}
	return nil
}

// RemoveByCode removes a token by authorization code.
func (s *memoryTokenStore) RemoveByCode(ctx context.Context, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.codes, code)
	return nil
}

// RemoveByAccess removes a token by access token.
func (s *memoryTokenStore) RemoveByAccess(ctx context.Context, access string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.accessTokens, access)
	return nil
}

// RemoveByRefresh removes a token by refresh token.
func (s *memoryTokenStore) RemoveByRefresh(ctx context.Context, refresh string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.refresh, refresh)
	return nil
}

// GetByCode retrieves a token by authorization code.
func (s *memoryTokenStore) GetByCode(ctx context.Context, code string) (TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.codes[code]
	if !ok {
		return TokenInfo{}, ErrInvalidGrant
	}
	return info, nil
}

// GetByAccess retrieves a token by access token.
func (s *memoryTokenStore) GetByAccess(ctx context.Context, access string) (TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.accessTokens[access]
	if !ok {
		return TokenInfo{}, ErrInvalidGrant
	}
	return info, nil
}

// GetByRefresh retrieves a token by refresh token.
func (s *memoryTokenStore) GetByRefresh(ctx context.Context, refresh string) (TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.refresh[refresh]
	if !ok {
		return TokenInfo{}, ErrInvalidGrant
	}
	return info, nil
}

// Client represents an OAuth2 client application.
type Client struct {
	// ID is the unique client identifier.
	ID string
	// Secret is the client secret for confidential clients. Leave empty for public clients.
	Secret string
	// Name is a human-readable name for the client.
	Name string
	// RedirectURIs is the list of allowed redirect URIs for authorization code flow.
	RedirectURIs []string
	// Scopes is the list of allowed scopes for this client.
	Scopes []string
	// Public indicates if this is a public client (e.g., mobile/SPA apps without a secret).
	Public bool
	// CreatedBy is the user ID of who created this client (for user-registered clients).
	CreatedBy string
	// CreatedAt is when the client was registered.
	CreatedAt time.Time
}
