package auth

import (
	"context"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/storage"
)

type blocklistKey struct{}

// Blocklist is an interface for managed blocked tokens. By default identity
// tokens are valid until they expire. This interface allows applications to
// block tokens before they expire.
type Blocklist interface {
	// IsBlocked checks if a token with the given key is blocked.
	IsBlocked(key string) (bool, error)

	// Block adds a token to the blocklist. Key can be the token itself or a
	// unique ID.
	Block(key string) error
}

// IsBlocked checks if a token is blocked.
func IsBlocked(ctx context.Context, key string) (bool, error) {
	if bl, ok := ctx.Value(blocklistKey{}).(Blocklist); ok {
		return bl.IsBlocked(key)
	}
	return false, nil
}

// WithBlockist adds a blocklist to the context.
func WithBlockist(ctx context.Context, bl Blocklist) context.Context {
	return context.WithValue(ctx, blocklistKey{}, bl)
}

// MaybeBlock adds a token to the blocklist if a blocklist is present in the
// context.
func MaybeBlock(ctx context.Context, key string) error {
	if bl, ok := ctx.Value(blocklistKey{}).(Blocklist); ok {
		return bl.Block(key)
	}
	return nil
}

// NewBlocklist creates a basic implementation of the blocklist interface,
// backed via a storage.Store.
func NewBlocklist(store storage.Store) Blocklist {
	return &basicBlocklist{store: store}
}

type basicBlocklist struct {
	store storage.Store
}

func (b *basicBlocklist) IsBlocked(key string) (bool, error) {
	return b.store.Exists(key, &BlockedToken{})
}

func (b *basicBlocklist) Block(key string) error {
	err := b.store.Create(&BlockedToken{Key: key})
	if errors.Is(err, storage.ErrAlreadyExists) {
		return err
	}
	return nil
}

// BlockedToken is a model for storing blocked tokens.
type BlockedToken struct {
	Key string
}

// Implements storage.Model
func (bt *BlockedToken) PK() string {
	return bt.Key
}
