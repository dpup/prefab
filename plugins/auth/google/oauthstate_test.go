package google

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOauthState_Encode(t *testing.T) {
	state := &oauthState{
		OriginalState: "client-state",
		RequestUri:    "/dashboard",
		TimeStamp:     time.Now(),
		Signature:     "test-signature",
	}

	encoded := state.Encode()
	assert.NotEmpty(t, encoded)

	// Verify it's valid base64
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)

	// Verify it's valid JSON
	var decodedState oauthState
	err = json.Unmarshal(decoded, &decodedState)
	require.NoError(t, err)

	assert.Equal(t, state.OriginalState, decodedState.OriginalState)
	assert.Equal(t, state.RequestUri, decodedState.RequestUri)
	assert.Equal(t, state.Signature, decodedState.Signature)
}

func TestGooglePlugin_newOauthState(t *testing.T) {
	p := &GooglePlugin{
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	code := "original-code"
	redirectUri := "/redirect-uri"

	state := p.newOauthState(code, redirectUri)

	assert.NotNil(t, state)
	assert.Equal(t, redirectUri, state.OriginalState)
	assert.Equal(t, code, state.RequestUri)
	assert.NotEmpty(t, state.Signature)
	assert.False(t, state.TimeStamp.IsZero())

	// Verify signature is a hex-encoded string
	assert.Len(t, state.Signature, 64) // SHA256 produces 32 bytes, hex encoded = 64 chars
}

func TestGooglePlugin_parseState(t *testing.T) {
	p := &GooglePlugin{
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	tests := []struct {
		name          string
		setupState    func() string
		expectedError bool
		validateState func(*testing.T, *oauthState)
	}{
		{
			name: "valid state",
			setupState: func() string {
				state := p.newOauthState("test-code", "/dashboard")
				return state.Encode()
			},
			expectedError: false,
			validateState: func(t *testing.T, s *oauthState) {
				assert.Equal(t, "/dashboard", s.OriginalState)
				assert.Equal(t, "test-code", s.RequestUri)
			},
		},
		{
			name: "empty state",
			setupState: func() string {
				return ""
			},
			expectedError: true,
		},
		{
			name: "invalid base64",
			setupState: func() string {
				return "not-valid-base64!!!"
			},
			expectedError: true,
		},
		{
			name: "invalid JSON",
			setupState: func() string {
				return base64.StdEncoding.EncodeToString([]byte("not json"))
			},
			expectedError: true,
		},
		{
			name: "expired state",
			setupState: func() string {
				state := &oauthState{
					OriginalState: "test",
					RequestUri:    "/test",
					TimeStamp:     time.Now().Add(-10 * time.Minute),
				}

				// Re-parse to get the signature
				p2 := &GooglePlugin{clientSecret: "test-client-secret"}
				fullState := p2.newOauthState(state.RequestUri, state.OriginalState)
				fullState.TimeStamp = state.TimeStamp

				return fullState.Encode()
			},
			expectedError: true,
		},
		{
			name: "wrong signature",
			setupState: func() string {
				// Create state with correct signature
				state := p.newOauthState("test-code", "/dashboard")

				// Tamper with the state by changing the request URI
				decoded, _ := base64.StdEncoding.DecodeString(state.Encode())
				var s oauthState
				json.Unmarshal(decoded, &s)
				s.RequestUri = "/tampered"

				tampered, _ := json.Marshal(s)
				return base64.StdEncoding.EncodeToString(tampered)
			},
			expectedError: true,
		},
		{
			name: "invalid signature format",
			setupState: func() string {
				state := &oauthState{
					OriginalState: "test",
					RequestUri:    "/test",
					TimeStamp:     time.Now(),
					Signature:     "not-hex",
				}
				encoded, _ := json.Marshal(state)
				return base64.StdEncoding.EncodeToString(encoded)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateStr := tt.setupState()
			state, err := p.parseState(stateStr)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, state)
				if tt.validateState != nil {
					tt.validateState(t, state)
				}
			}
		})
	}
}

func TestOauthState_RoundTrip(t *testing.T) {
	p := &GooglePlugin{
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	originalCode := "test-code-123"
	originalRedirect := "/original-redirect"

	// Create and encode state
	state := p.newOauthState(originalCode, originalRedirect)
	encoded := state.Encode()

	// Parse it back
	parsed, err := p.parseState(encoded)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, originalRedirect, parsed.OriginalState)
	assert.Equal(t, originalCode, parsed.RequestUri)
	assert.WithinDuration(t, state.TimeStamp, parsed.TimeStamp, time.Second)
}

func TestOauthState_DifferentSecrets(t *testing.T) {
	p1 := &GooglePlugin{clientSecret: "secret1"}
	p2 := &GooglePlugin{clientSecret: "secret2"}

	// Create state with p1
	state := p1.newOauthState("code", "redirect")
	encoded := state.Encode()

	// Try to parse with p2 (different secret)
	_, err := p2.parseState(encoded)
	require.Error(t, err, "should reject state signed with different secret")
}
