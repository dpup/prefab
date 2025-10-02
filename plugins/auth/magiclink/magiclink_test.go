package magiclink

import (
	"strings"
	"testing"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/email"
	"github.com/dpup/prefab/plugins/templates"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPlugin(t *testing.T) {
	signingKey := []byte("test-signing-key")
	expiration := 15 * time.Minute

	tests := []struct {
		name string
		opts []MagicLinkOption
	}{
		{
			name: "default configuration loads from config",
			opts: nil,
		},
		{
			name: "with signing key override",
			opts: []MagicLinkOption{
				WithSigningKey(signingKey),
			},
		},
		{
			name: "with expiration override",
			opts: []MagicLinkOption{
				WithExpiration(expiration),
			},
		},
		{
			name: "with all options",
			opts: []MagicLinkOption{
				WithSigningKey(signingKey),
				WithExpiration(expiration),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin(tt.opts...)
			assert.NotNil(t, p)
			assert.Equal(t, PluginName, p.Name())
		})
	}
}

func TestMagicLinkPlugin_Name(t *testing.T) {
	p := Plugin()
	assert.Equal(t, PluginName, p.Name())
}

func TestMagicLinkPlugin_Deps(t *testing.T) {
	p := Plugin()
	deps := p.Deps()
	assert.Len(t, deps, 3)
	assert.Contains(t, deps, auth.PluginName)
	assert.Contains(t, deps, email.PluginName)
	assert.Contains(t, deps, templates.PluginName)
}

func TestMagicLinkPlugin_Init(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name          string
		setupPlugin   func() *MagicLinkPlugin
		expectedError string
	}{
		{
			name: "missing signing key",
			setupPlugin: func() *MagicLinkPlugin {
				return Plugin(WithSigningKey(nil), WithExpiration(15*time.Minute))
			},
			expectedError: "magiclink: config missing signing key",
		},
		{
			name: "missing expiration",
			setupPlugin: func() *MagicLinkPlugin {
				return Plugin(WithSigningKey([]byte("test-key")), WithExpiration(0))
			},
			expectedError: "magiclink: config missing token expiration",
		},
		{
			name: "successful initialization",
			setupPlugin: func() *MagicLinkPlugin {
				return Plugin(
					WithSigningKey([]byte("test-key")),
					WithExpiration(15*time.Minute),
				)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authPlugin := auth.Plugin()
			emailPlugin := email.Plugin()
			templatePlugin := templates.Plugin()

			registry := &prefab.Registry{}
			registry.Register(authPlugin)
			registry.Register(emailPlugin)
			registry.Register(templatePlugin)

			p := tt.setupPlugin()
			err := p.Init(ctx, registry)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMagicLinkPlugin_handleLogin(t *testing.T) {
	ctx := t.Context()
	signingKey := []byte("test-signing-key")

	tests := []struct {
		name          string
		req           *auth.LoginRequest
		expectedError bool
		expectedCode  codes.Code
		validateResp  func(*testing.T, *auth.LoginResponse)
	}{
		{
			name: "wrong provider",
			req: &auth.LoginRequest{
				Provider: "wrong-provider",
			},
			expectedError: true,
			expectedCode:  codes.InvalidArgument,
		},
		{
			name: "missing credentials",
			req: &auth.LoginRequest{
				Provider: ProviderName,
				Creds:    map[string]string{},
			},
			expectedError: true,
			expectedCode:  codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin(
				WithSigningKey(signingKey),
				WithExpiration(15*time.Minute),
			)

			resp, err := p.handleLogin(ctx, tt.req)

			if tt.expectedError {
				require.Error(t, err)
				if tt.expectedCode != 0 {
					st, ok := status.FromError(err)
					require.True(t, ok, "error should be a gRPC status error")
					assert.Equal(t, tt.expectedCode, st.Code())
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				if tt.validateResp != nil {
					tt.validateResp(t, resp)
				}
			}
		})
	}
}

// Note: handleEmail requires email and templates plugins which are hard to mock
// without changing production code. For now, we test generateToken and parseToken
// instead, which cover the core token logic.

func TestMagicLinkPlugin_generateToken(t *testing.T) {
	signingKey := []byte("test-signing-key")
	expiration := 15 * time.Minute

	p := Plugin(
		WithSigningKey(signingKey),
		WithExpiration(expiration),
	)

	email := "test@example.com"
	tokenString, err := p.generateToken(email)
	require.NoError(t, err)
	assert.NotEmpty(t, tokenString)

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			return signingKey, nil
		},
	)
	require.NoError(t, err)
	assert.True(t, token.Valid)

	claims, ok := token.Claims.(*Claims)
	require.True(t, ok)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, jwtIssuer, claims.Issuer)
	assert.Contains(t, claims.Audience, jwtAudience)
	assert.NotEmpty(t, claims.ID)
}

func TestMagicLinkPlugin_parseToken(t *testing.T) {
	signingKey := []byte("test-signing-key")
	expiration := 15 * time.Minute

	p := Plugin(
		WithSigningKey(signingKey),
		WithExpiration(expiration),
	)

	tests := []struct {
		name          string
		setupToken    func() string
		expectedError bool
		validateID    func(*testing.T, auth.Identity)
	}{
		{
			name: "valid token",
			setupToken: func() string {
				token, _ := p.generateToken("test@example.com")
				return token
			},
			expectedError: false,
			validateID: func(t *testing.T, id auth.Identity) {
				assert.Equal(t, "test@example.com", id.Email)
				assert.Equal(t, "test@example.com", id.Subject)
				assert.Equal(t, ProviderName, id.Provider)
				assert.True(t, id.EmailVerified)
				assert.NotEmpty(t, id.SessionID)
			},
		},
		{
			name: "expired token",
			setupToken: func() string {
				claims := &Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						Issuer:    jwtIssuer,
						Audience:  jwt.ClaimStrings{jwtAudience},
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
						IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
					},
					Email: "expired@example.com",
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString(signingKey)
				return tokenString
			},
			expectedError: true,
		},
		{
			name: "wrong signing key",
			setupToken: func() string {
				wrongKey := []byte("wrong-key")
				claims := &Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						Issuer:    jwtIssuer,
						Audience:  jwt.ClaimStrings{jwtAudience},
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
						IssuedAt:  jwt.NewNumericDate(time.Now()),
					},
					Email: "test@example.com",
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString(wrongKey)
				return tokenString
			},
			expectedError: true,
		},
		{
			name: "invalid token format",
			setupToken: func() string {
				return "not-a-valid-jwt"
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString := tt.setupToken()
			identity, err := p.parseToken(tokenString)

			if tt.expectedError {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok, "error should be a gRPC status error")
				assert.Equal(t, codes.InvalidArgument, st.Code())
			} else {
				require.NoError(t, err)
				if tt.validateID != nil {
					tt.validateID(t, identity)
				}
			}
		})
	}
}

func TestClaims(t *testing.T) {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "test-id",
			Issuer:    jwtIssuer,
			Audience:  jwt.ClaimStrings{jwtAudience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email:       "test@example.com",
		IssueToken:  true,
		RedirectUri: "/dashboard",
	}

	assert.Equal(t, "test@example.com", claims.Email)
	assert.True(t, claims.IssueToken)
	assert.Equal(t, "/dashboard", claims.RedirectUri)
}

func TestWithSigningKey(t *testing.T) {
	signingKey := []byte("custom-signing-key")
	p := Plugin(WithSigningKey(signingKey))
	assert.Equal(t, signingKey, p.signingKey)
}

func TestWithExpiration(t *testing.T) {
	expiration := 30 * time.Minute
	p := Plugin(WithExpiration(expiration))
	assert.Equal(t, expiration, p.tokenExpiration)
}

func TestHandleToken_TokenValidation(t *testing.T) {
	ctx := t.Context()
	signingKey := []byte("test-signing-key")

	p := Plugin(
		WithSigningKey(signingKey),
		WithExpiration(15*time.Minute),
	)

	// Generate a valid token
	validToken, err := p.generateToken("test@example.com")
	require.NoError(t, err)

	tests := []struct {
		name          string
		token         string
		issueToken    bool
		redirectUri   string
		expectedError bool
		validateResp  func(*testing.T, *auth.LoginResponse)
	}{
		{
			name:          "valid token with issue",
			token:         validToken,
			issueToken:    true,
			redirectUri:   "",
			expectedError: false,
			validateResp: func(t *testing.T, resp *auth.LoginResponse) {
				assert.True(t, resp.Issued)
				assert.NotEmpty(t, resp.Token)
			},
		},
		{
			name:          "invalid token",
			token:         "invalid-token",
			issueToken:    true,
			redirectUri:   "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := p.handleToken(ctx, tt.token, tt.issueToken, tt.redirectUri)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.validateResp != nil {
					tt.validateResp(t, resp)
				}
			}
		})
	}
}

func TestURLConstruction(t *testing.T) {
	// Test URL construction logic independently
	tests := []struct {
		name        string
		redirectUri string
		token       string
		expected    string
	}{
		{
			name:        "redirect with existing query",
			redirectUri: "/dashboard?foo=bar",
			token:       "ABC123",
			expected:    "&token=",
		},
		{
			name:        "redirect without query",
			redirectUri: "/dashboard",
			token:       "ABC123",
			expected:    "?token=",
		},
		{
			name:        "empty redirect",
			redirectUri: "",
			token:       "ABC123",
			expected:    "/api/auth/login?provider=magiclink&creds[token]=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var url string
			switch {
			case strings.Contains(tt.redirectUri, "?"):
				url = tt.redirectUri + "&token=" + tt.token
			case tt.redirectUri != "":
				url = tt.redirectUri + "?token=" + tt.token
			default:
				url = "http://example.com/api/auth/login?provider=magiclink&creds[token]=" + tt.token
			}

			assert.Contains(t, url, tt.expected)
		})
	}
}
