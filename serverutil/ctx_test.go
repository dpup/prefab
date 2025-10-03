package serverutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithAddress(t *testing.T) {
	t.Run("SetAddress", func(t *testing.T) {
		ctx := WithAddress(t.Context(), "https://example.com")

		address := AddressFromContext(ctx)
		assert.Equal(t, "https://example.com", address)
	})

	t.Run("SetHTTPAddress", func(t *testing.T) {
		ctx := WithAddress(t.Context(), "http://localhost:8000")

		address := AddressFromContext(ctx)
		assert.Equal(t, "http://localhost:8000", address)
	})

	t.Run("SetEmptyAddress", func(t *testing.T) {
		ctx := WithAddress(t.Context(), "")

		address := AddressFromContext(ctx)
		assert.Empty(t, address)
	})

	t.Run("OverwriteAddress", func(t *testing.T) {
		ctx := WithAddress(t.Context(), "https://first.com")
		ctx = WithAddress(ctx, "https://second.com")

		address := AddressFromContext(ctx)
		assert.Equal(t, "https://second.com", address)
	})
}

func TestAddressFromContext(t *testing.T) {
	t.Run("WithAddress", func(t *testing.T) {
		ctx := WithAddress(t.Context(), "https://api.example.com")

		address := AddressFromContext(ctx)
		assert.Equal(t, "https://api.example.com", address)
	})

	t.Run("WithoutAddress_ReturnsDefault", func(t *testing.T) {
		ctx := t.Context()

		address := AddressFromContext(ctx)
		assert.Equal(t, "http://localhost:8080", address)
	})

	t.Run("WithAddressInChain", func(t *testing.T) {
		// Test that address survives context chaining
		ctx := WithAddress(t.Context(), "https://chained.example.com")

		// Create a child context
		childCtx := WithAddress(ctx, "https://child.example.com")

		// Original context should still have original address
		assert.Equal(t, "https://chained.example.com", AddressFromContext(ctx))
		// Child context should have new address
		assert.Equal(t, "https://child.example.com", AddressFromContext(childCtx))
	})
}
