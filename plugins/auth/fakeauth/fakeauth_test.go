package fakeauth

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/dpup/prefab/plugins/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFakeAuthPlugin_handleLogin(t *testing.T) {
	// Create the plugin directly
	plugin := Plugin()

	// We'll test just the handleLogin function directly
	t.Run("login with default identity", func(t *testing.T) {
		ctx := context.Background()

		// Call handleLogin directly
		resp, err := plugin.handleLogin(ctx, &auth.LoginRequest{
			Provider:   ProviderName,
			IssueToken: true,
		})

		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		if !resp.Issued {
			t.Errorf("Expected token to be issued")
		}

		if resp.Token == "" {
			t.Errorf("Expected token to be present")
		}
	})

	t.Run("login with custom identity using id", func(t *testing.T) {
		ctx := context.Background()
		customSubject := "custom-123"
		customEmail := "custom@example.com"
		customName := "Custom User"

		// Call handleLogin directly with custom creds
		resp, err := plugin.handleLogin(ctx, &auth.LoginRequest{
			Provider: ProviderName,
			Creds: map[string]string{
				"id":             customSubject,
				"email":          customEmail,
				"name":           customName,
				"email_verified": "false",
			},
			IssueToken: true,
		})

		if err != nil {
			t.Fatalf("Login failed: %v", err)
		}

		if !resp.Issued {
			t.Errorf("Expected token to be issued")
		}

		if resp.Token == "" {
			t.Errorf("Expected token to be present")
		}
	})

	t.Run("login with simulated error", func(t *testing.T) {
		ctx := context.Background()

		// Call handleLogin with error simulation
		_, err := plugin.handleLogin(ctx, &auth.LoginRequest{
			Provider: ProviderName,
			Creds: map[string]string{
				"error_code":    "3", // corresponds to codes.InvalidArgument
				"error_message": "test error message",
			},
			IssueToken: true,
		})

		// Verify error is returned
		if err == nil {
			t.Fatalf("Expected login to fail with simulated error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error, got %v", err)
		}

		if st.Message() != "test error message" {
			t.Errorf("Expected error message 'test error message', got '%s'", st.Message())
		}
	})

	t.Run("validation rejects login", func(t *testing.T) {
		ctx := context.Background()

		// Create plugin with rejecting validator
		rejectingPlugin := Plugin(WithIdentityValidator(func(ctx context.Context, creds map[string]string) error {
			return status.Error(codes.PermissionDenied, "rejected for testing")
		}))

		// Call handleLogin, which should be rejected
		_, err := rejectingPlugin.handleLogin(ctx, &auth.LoginRequest{
			Provider:   ProviderName,
			IssueToken: true,
		})

		// Verify rejection
		if err == nil {
			t.Fatalf("Expected login to be rejected")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.PermissionDenied {
			t.Errorf("Expected PermissionDenied error, got %v", err)
		}
	})

	t.Run("wrong provider", func(t *testing.T) {
		ctx := context.Background()

		// Call handleLogin with wrong provider
		_, err := plugin.handleLogin(ctx, &auth.LoginRequest{
			Provider:   "wrong-provider",
			IssueToken: true,
		})

		// Verify rejection
		if err == nil {
			t.Fatalf("Expected login to be rejected for wrong provider")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error, got %v", err)
		}
	})
}

// Basic test for the MustLogin helper just to verify it's functional
func TestMustLogin(t *testing.T) {
	mockLogin := func(ctx context.Context, req *auth.LoginRequest, opts ...grpc.CallOption) (*auth.LoginResponse, error) {
		return &auth.LoginResponse{
			Issued: true,
			Token:  "test-token",
		}, nil
	}

	mockClient := &mockAuthClient{
		LoginFunc: mockLogin,
	}

	// Test with new FakeOptions struct
	emailVerified := true
	token := MustLogin(context.Background(), mockClient, FakeOptions{
		ID:            "test-id-123",
		Email:         "test@example.com",
		Name:          "Test User",
		EmailVerified: &emailVerified,
	})

	if token != "test-token" {
		t.Errorf("Expected 'test-token', got '%s'", token)
	}

}

// Test error handling with FakeOptions
func TestMustLogin_Error(t *testing.T) {
	mockLogin := func(ctx context.Context, req *auth.LoginRequest, opts ...grpc.CallOption) (*auth.LoginResponse, error) {
		creds := req.GetCreds()
		if creds["error_code"] != "" {
			code, _ := strconv.Atoi(creds["error_code"])
			errorMsg := "simulated error"
			if msg, ok := creds["error_message"]; ok && msg != "" {
				errorMsg = msg
			}
			return nil, status.Error(codes.Code(code), errorMsg)
		}
		return &auth.LoginResponse{
			Issued: true,
			Token:  "test-token",
		}, nil
	}

	mockClient := &mockAuthClient{
		LoginFunc: mockLogin,
	}

	// Setup a deferred recovery to test panic
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Expected MustLogin to panic with error option, but it didn't")
		}
		if errMsg, ok := r.(string); ok {
			if !strings.Contains(errMsg, "custom error") {
				t.Errorf("Expected error message to contain 'custom error', got: %s", errMsg)
			}
		} else {
			t.Errorf("Expected panic message to be a string, got: %v", r)
		}
	}()

	// This should cause a panic that will be caught by the deferred recovery
	MustLogin(context.Background(), mockClient, FakeOptions{
		ID:           "test-error",
		ErrorCode:    codes.PermissionDenied,
		ErrorMessage: "custom error",
	})

	t.Fatal("Should not reach this point")
}

// Mock implementation for testing
type mockLoginFunc func(context.Context, *auth.LoginRequest, ...grpc.CallOption) (*auth.LoginResponse, error)

type mockAuthClient struct {
	auth.AuthServiceClient
	LoginFunc mockLoginFunc
}

func (m *mockAuthClient) Login(ctx context.Context, req *auth.LoginRequest, opts ...grpc.CallOption) (*auth.LoginResponse, error) {
	return m.LoginFunc(ctx, req, opts...)
}
