package pwdauth

import (
	"context"
	"testing"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPlugin(t *testing.T) {
	tests := []struct {
		name           string
		opts           []PwdAuthOption
		validateHasher bool
	}{
		{
			name:           "default configuration",
			opts:           nil,
			validateHasher: true,
		},
		{
			name: "custom hasher",
			opts: []PwdAuthOption{
				WithHasher(TestHasher),
			},
			validateHasher: false,
		},
		{
			name: "with account finder",
			opts: []PwdAuthOption{
				WithAccountFinder(&mockAccountFinder{}),
			},
			validateHasher: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Plugin(tt.opts...)
			assert.NotNil(t, p)
			assert.Equal(t, PluginName, p.Name())
			if tt.validateHasher {
				assert.NotNil(t, p.hasher)
			}
		})
	}
}

func TestPwdAuthPlugin_Name(t *testing.T) {
	p := Plugin()
	assert.Equal(t, PluginName, p.Name())
}

func TestPwdAuthPlugin_Deps(t *testing.T) {
	p := Plugin()
	deps := p.Deps()
	assert.Len(t, deps, 1)
	assert.Contains(t, deps, auth.PluginName)
}

func TestPwdAuthPlugin_Init(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupPlugin   func() *PwdAuthPlugin
		expectedError string
	}{
		{
			name: "successful initialization",
			setupPlugin: func() *PwdAuthPlugin {
				return Plugin(WithAccountFinder(&mockAccountFinder{}))
			},
			expectedError: "",
		},
		{
			name: "missing account finder",
			setupPlugin: func() *PwdAuthPlugin {
				return Plugin()
			},
			expectedError: "pwdauth: plugin requires an account finder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authPlugin := auth.Plugin()
			registry := &prefab.Registry{}
			registry.Register(authPlugin)

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

func TestPwdAuthPlugin_handleLogin(t *testing.T) {
	ctx := context.Background()
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)

	tests := []struct {
		name          string
		req           *auth.LoginRequest
		accountFinder AccountFinder
		hasher        Hasher
		expectedError bool
		expectedCode  codes.Code
		validateResp  func(*testing.T, *auth.LoginResponse)
	}{
		{
			name: "wrong provider",
			req: &auth.LoginRequest{
				Provider: "wrong-provider",
			},
			accountFinder: &mockAccountFinder{},
			expectedError: true,
			expectedCode:  codes.InvalidArgument,
		},
		{
			name: "missing email",
			req: &auth.LoginRequest{
				Provider: ProviderName,
				Creds: map[string]string{
					"password": "test",
				},
			},
			accountFinder: &mockAccountFinder{},
			expectedError: true,
			expectedCode:  codes.InvalidArgument,
		},
		{
			name: "missing password",
			req: &auth.LoginRequest{
				Provider: ProviderName,
				Creds: map[string]string{
					"email": "test@example.com",
				},
			},
			accountFinder: &mockAccountFinder{},
			expectedError: true,
			expectedCode:  codes.InvalidArgument,
		},
		{
			name: "account not found",
			req: &auth.LoginRequest{
				Provider: ProviderName,
				Creds: map[string]string{
					"email":    "notfound@example.com",
					"password": "password",
				},
				IssueToken: true,
			},
			accountFinder: &mockAccountFinder{
				findFunc: func(ctx context.Context, email string) (*Account, error) {
					return nil, status.Error(codes.NotFound, "account not found")
				},
			},
			expectedError: true,
			expectedCode:  codes.Unauthenticated,
		},
		{
			name: "account finder internal error",
			req: &auth.LoginRequest{
				Provider: ProviderName,
				Creds: map[string]string{
					"email":    "test@example.com",
					"password": "password",
				},
				IssueToken: true,
			},
			accountFinder: &mockAccountFinder{
				findFunc: func(ctx context.Context, email string) (*Account, error) {
					return nil, status.Error(codes.Internal, "database error")
				},
			},
			expectedError: true,
			expectedCode:  codes.Internal,
		},
		{
			name: "incorrect password",
			req: &auth.LoginRequest{
				Provider: ProviderName,
				Creds: map[string]string{
					"email":    "test@example.com",
					"password": "wrong-password",
				},
				IssueToken: true,
			},
			accountFinder: &mockAccountFinder{
				findFunc: func(ctx context.Context, email string) (*Account, error) {
					return &Account{
						ID:             "user123",
						Email:          email,
						Name:           "Test User",
						EmailVerified:  true,
						HashedPassword: hashedPassword,
					}, nil
				},
			},
			hasher:        DefaultHasher,
			expectedError: true,
			expectedCode:  codes.Unauthenticated,
		},
		{
			name: "successful login with token",
			req: &auth.LoginRequest{
				Provider: ProviderName,
				Creds: map[string]string{
					"email":    "test@example.com",
					"password": "correct-password",
				},
				IssueToken: true,
			},
			accountFinder: &mockAccountFinder{
				findFunc: func(ctx context.Context, email string) (*Account, error) {
					return &Account{
						ID:             "user123",
						Email:          email,
						Name:           "Test User",
						EmailVerified:  true,
						HashedPassword: hashedPassword,
					}, nil
				},
			},
			hasher:        DefaultHasher,
			expectedError: false,
			validateResp: func(t *testing.T, resp *auth.LoginResponse) {
				assert.True(t, resp.Issued)
				assert.NotEmpty(t, resp.Token)
			},
		},
		{
			name: "successful login with test hasher",
			req: &auth.LoginRequest{
				Provider: ProviderName,
				Creds: map[string]string{
					"email":    "test@example.com",
					"password": "plaintext-password",
				},
				IssueToken: true,
			},
			accountFinder: &mockAccountFinder{
				findFunc: func(ctx context.Context, email string) (*Account, error) {
					return &Account{
						ID:             "user456",
						Email:          email,
						Name:           "Test User 2",
						EmailVerified:  false,
						HashedPassword: []byte("plaintext-password"),
					}, nil
				},
			},
			hasher:        TestHasher,
			expectedError: false,
			validateResp: func(t *testing.T, resp *auth.LoginResponse) {
				assert.True(t, resp.Issued)
				assert.NotEmpty(t, resp.Token)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []PwdAuthOption{WithAccountFinder(tt.accountFinder)}
			if tt.hasher != nil {
				opts = append(opts, WithHasher(tt.hasher))
			}
			p := Plugin(opts...)

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

func TestIdentityFromAccount(t *testing.T) {
	account := &Account{
		ID:             "user123",
		Email:          "test@example.com",
		Name:           "Test User",
		EmailVerified:  true,
		HashedPassword: []byte("hashed"),
	}

	identity := identityFromAccount(account)

	assert.Equal(t, ProviderName, identity.Provider)
	assert.Equal(t, "user123", identity.Subject)
	assert.Equal(t, "test@example.com", identity.Email)
	assert.Equal(t, "Test User", identity.Name)
	assert.True(t, identity.EmailVerified)
	assert.NotEmpty(t, identity.SessionID)
	assert.False(t, identity.AuthTime.IsZero())
}

func TestWithHasher(t *testing.T) {
	customHasher := TestHasher
	p := Plugin(WithHasher(customHasher))
	assert.Equal(t, customHasher, p.hasher)
}

func TestWithAccountFinder(t *testing.T) {
	finder := &mockAccountFinder{}
	p := Plugin(WithAccountFinder(finder))
	assert.Equal(t, finder, p.accountFinder)
}

// Mock AccountFinder for testing
type mockAccountFinder struct {
	findFunc func(ctx context.Context, email string) (*Account, error)
}

func (m *mockAccountFinder) FindAccount(ctx context.Context, email string) (*Account, error) {
	if m.findFunc != nil {
		return m.findFunc(ctx, email)
	}
	return &Account{
		ID:             "default-user",
		Email:          email,
		Name:           "Default User",
		EmailVerified:  true,
		HashedPassword: []byte("default-hashed-password"),
	}, nil
}
