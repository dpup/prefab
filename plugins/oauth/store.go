package oauth

import (
	"context"
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
	if len(c.client.RedirectURIs) > 0 {
		return c.client.RedirectURIs[0]
	}
	return ""
}
func (c *clientAdapter) IsPublic() bool { return c.client.Public }
func (c *clientAdapter) GetUserID() string {
	return c.client.CreatedBy
}

// tokenStoreAdapter implements go-oauth2's TokenStore interface with in-memory storage.
type tokenStoreAdapter struct {
	mu           sync.RWMutex
	accessTokens map[string]oauth2.TokenInfo
	codes        map[string]oauth2.TokenInfo
	refresh      map[string]oauth2.TokenInfo
}

func newTokenStoreAdapter() *tokenStoreAdapter {
	return &tokenStoreAdapter{
		accessTokens: make(map[string]oauth2.TokenInfo),
		codes:        make(map[string]oauth2.TokenInfo),
		refresh:      make(map[string]oauth2.TokenInfo),
	}
}

// Create stores a new token.
func (s *tokenStoreAdapter) Create(ctx context.Context, info oauth2.TokenInfo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if code := info.GetCode(); code != "" {
		s.codes[code] = info
	}
	if access := info.GetAccess(); access != "" {
		s.accessTokens[access] = info
	}
	if refresh := info.GetRefresh(); refresh != "" {
		s.refresh[refresh] = info
	}
	return nil
}

// RemoveByCode removes a token by authorization code.
func (s *tokenStoreAdapter) RemoveByCode(ctx context.Context, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.codes, code)
	return nil
}

// RemoveByAccess removes a token by access token.
func (s *tokenStoreAdapter) RemoveByAccess(ctx context.Context, access string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.accessTokens, access)
	return nil
}

// RemoveByRefresh removes a token by refresh token.
func (s *tokenStoreAdapter) RemoveByRefresh(ctx context.Context, refresh string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.refresh, refresh)
	return nil
}

// GetByCode retrieves a token by authorization code.
func (s *tokenStoreAdapter) GetByCode(ctx context.Context, code string) (oauth2.TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.codes[code]
	if !ok {
		return nil, ErrInvalidGrant
	}
	return info, nil
}

// GetByAccess retrieves a token by access token.
func (s *tokenStoreAdapter) GetByAccess(ctx context.Context, access string) (oauth2.TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.accessTokens[access]
	if !ok {
		return nil, ErrInvalidGrant
	}
	return info, nil
}

// GetByRefresh retrieves a token by refresh token.
func (s *tokenStoreAdapter) GetByRefresh(ctx context.Context, refresh string) (oauth2.TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	info, ok := s.refresh[refresh]
	if !ok {
		return nil, ErrInvalidGrant
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
	// Trusted indicates if user consent can be skipped for this client.
	Trusted bool
	// CreatedBy is the user ID of who created this client (for user-registered clients).
	CreatedBy string
	// CreatedAt is when the client was registered.
	CreatedAt time.Time
}
