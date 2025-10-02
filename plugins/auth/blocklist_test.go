package auth

import (
	"testing"

	"github.com/dpup/prefab/plugins/storage"
	"github.com/dpup/prefab/plugins/storage/memstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBlocklist(t *testing.T) {
	store := memstore.New()
	bl := NewBlocklist(store)
	assert.NotNil(t, bl)
}

func TestBlocklist_Block(t *testing.T) {
	store := memstore.New()
	bl := NewBlocklist(store)

	// Block a token
	err := bl.Block("token123")
	require.NoError(t, err)

	// Verify it's blocked
	blocked, err := bl.IsBlocked("token123")
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestBlocklist_Block_AlreadyExists(t *testing.T) {
	store := memstore.New()
	bl := NewBlocklist(store)

	// Block a token
	err := bl.Block("token123")
	require.NoError(t, err)

	// Try to block the same token again - should return AlreadyExists error
	err = bl.Block("token123")
	require.Error(t, err)
	assert.ErrorIs(t, err, storage.ErrAlreadyExists)
}

func TestBlocklist_IsBlocked_NotBlocked(t *testing.T) {
	store := memstore.New()
	bl := NewBlocklist(store)

	blocked, err := bl.IsBlocked("nonexistent-token")
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestIsBlocked_WithContext(t *testing.T) {
	store := memstore.New()
	bl := NewBlocklist(store)

	// Block a token
	err := bl.Block("token123")
	require.NoError(t, err)

	// Check with context
	ctx := WithBlockist(t.Context(), bl)
	blocked, err := IsBlocked(ctx, "token123")
	require.NoError(t, err)
	assert.True(t, blocked)

	// Check unblocked token
	blocked, err = IsBlocked(ctx, "token456")
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestIsBlocked_NoBlocklistInContext(t *testing.T) {
	ctx := t.Context()

	// Should return false when no blocklist is present
	blocked, err := IsBlocked(ctx, "any-token")
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestMaybeBlock(t *testing.T) {
	store := memstore.New()
	bl := NewBlocklist(store)

	ctx := WithBlockist(t.Context(), bl)

	// Block via MaybeBlock
	err := MaybeBlock(ctx, "token789")
	require.NoError(t, err)

	// Verify it's blocked
	blocked, err := bl.IsBlocked("token789")
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestMaybeBlock_NoBlocklistInContext(t *testing.T) {
	ctx := t.Context()

	// Should not error when no blocklist is present
	err := MaybeBlock(ctx, "any-token")
	require.NoError(t, err)
}

func TestBlockedToken_PK(t *testing.T) {
	bt := &BlockedToken{Key: "test-key-123"}
	assert.Equal(t, "test-key-123", bt.PK())
}

func TestBlocklist_Integration(t *testing.T) {
	// Test full integration flow
	store := memstore.New()
	bl := NewBlocklist(store)

	// Block multiple tokens
	tokens := []string{"token1", "token2", "token3"}
	for _, token := range tokens {
		err := bl.Block(token)
		require.NoError(t, err)
	}

	// Verify all are blocked
	for _, token := range tokens {
		blocked, err := bl.IsBlocked(token)
		require.NoError(t, err)
		assert.True(t, blocked, "token %s should be blocked", token)
	}

	// Verify unblocked token
	blocked, err := bl.IsBlocked("unblocked-token")
	require.NoError(t, err)
	assert.False(t, blocked)
}
