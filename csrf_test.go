package prefab

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Assuming generateCSRFToken and verifyCSRFToken are defined in the package.

func TestGenerateAndVerifyCSRFToken(t *testing.T) {
	signingKey := []byte("secret-key")

	// Generate a token
	token := generateCSRFToken(signingKey)
	if token == "" {
		t.Fatal("Generated token is empty")
	}

	// Verify the token
	err := verifyCSRFToken(token, signingKey)
	if err != nil {
		t.Fatalf("Failed to verify the generated token: %v", err)
	}
}

func TestVerifyCSRFTokenInvalidFormat(t *testing.T) {
	signingKey := []byte("secret-key")
	invalidToken := "invalidtokenformat"

	err := verifyCSRFToken(invalidToken, signingKey)
	if err == nil {
		t.Fatal("Expected an error for invalid token format, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("Expected FailedPrecondition error for invalid token format, got: %v", err)
	}
}

func TestVerifyCSRFTokenInvalidSignature(t *testing.T) {
	signingKey := []byte("secret-key")

	// Simulate a token with correct format but invalid signature.
	invalidSignature := "ZZZZ_ABCD1234"

	err := verifyCSRFToken(invalidSignature, signingKey)
	if err == nil {
		t.Fatal("Expected an error for invalid signature, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("Expected FailedPrecondition error for invalid data, got: %v", err)
	}
}

func TestVerifyCSRFTokenInvalidData(t *testing.T) {
	signingKey := []byte("secret-key")

	// Simulate a token with correct format but invalid data
	invalidData := "ABC123_ZZZZ"

	err := verifyCSRFToken(invalidData, signingKey)
	if err == nil {
		t.Fatal("Expected an error for invalid data, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("Expected FailedPrecondition error for invalid data, got: %v", err)
	}
}

func TestVerifyCSRFTokenSignatureMismatch(t *testing.T) {
	signingKey := []byte("secret-key")
	randomData := make([]byte, 32)
	_, _ = rand.Read(randomData) // Ignore error for simplicity in test setup

	// Simulate a token with correct format but invalid signature
	invalidSignature := hex.EncodeToString([]byte("invalidsignature")) + "_" + hex.EncodeToString(randomData)

	err := verifyCSRFToken(invalidSignature, signingKey)
	if err == nil {
		t.Fatal("Expected an error for signature mismatch, got nil")
	}

	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Fatalf("Expected FailedPrecondition error for signature mismatch, got: %v", err)
	}
}

func TestCsrfMetadataAnnotator(t *testing.T) {
	// Define a test CSRF token value
	testCsrfToken := "123456789"

	// Setup a dummy HTTP request with the CSRF token as a query parameter
	req, err := http.NewRequest("GET", "/?csrf-token="+testCsrfToken, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to act as a dummy ResponseWriter.
	rr := httptest.NewRecorder()

	// Simulate handling the request, though we're not serving anything real.
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		md := csrfMetadataAnnotator(context.Background(), r)

		// Extract the csrf_token value from the metadata
		values := md.Get(csrfParam)
		if len(values) == 0 {
			t.Fatalf("Expected metadata to contain %s, but it was missing", csrfParam)
		}

		// Check that the value matches our test token
		if values[0] != testCsrfToken {
			t.Errorf("Expected %s to be %s, got %s", csrfParam, testCsrfToken, values[0])
		}
	})

	// Serve the request to our dummy handler.
	handler.ServeHTTP(rr, req)
}

func TestCsrfTokenFromCookie(t *testing.T) {
	testToken := "test_csrf_token"

	// Mock cookies in the same way that GRPC maps HTTP headers to metadata.
	cookie := &http.Cookie{Name: csrfCookie, Value: testToken}
	md := metadata.New(map[string]string{"grpcgateway-cookie": cookie.String()})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	retrievedToken := csrfTokenFromCookie(ctx)

	if retrievedToken != testToken {
		t.Errorf("Expected CSRF token to be %s, got %s", testToken, retrievedToken)
	}
}

func TestCsrfTokenFromCookie_NoCookie(t *testing.T) {
	ctx := context.Background() // No cookies embedded
	retrievedToken := csrfTokenFromCookie(ctx)
	if retrievedToken != "" {
		t.Errorf("Expected CSRF token to be empty, got %s", retrievedToken)
	}
}

func TestSendCSRFToken(t *testing.T) {
	signingKey := []byte("test_signing_key")
	mockTransport := &mockServerTransportStream{}
	ctx := grpc.NewContextWithServerTransportStream(context.Background(), mockTransport)
	generatedToken := SendCSRFToken(ctx, signingKey)
	if generatedToken == "" {
		t.Errorf("Expected a generated CSRF token, got an empty string")
	}
	sentCookies := mockTransport.md.Get("grpc-metadata-set-cookie")
	if len(sentCookies) == 0 {
		t.Fatalf("Expected set-cookie metadata, but it was not found")
	}

	expectedPrefix := "pf-ct=" + generatedToken + ";"
	if !strings.HasPrefix(sentCookies[0], expectedPrefix) {
		t.Errorf("Expected cookie to have prefix %s, got %s", expectedPrefix, sentCookies[0])
	}
}

func TestVerifyCSRF_EverythingMissing(t *testing.T) {
	signingKey := []byte("test_signing_key")
	ctx := createContextWithCSRF("", "", "")
	if err := VerifyCSRF(ctx, signingKey); err == nil {
		t.Error("VerifyCSRF passed when it should have failed")
	}
}

func TestVerifyCSRF_MissingCookie(t *testing.T) {
	signingKey := []byte("test_signing_key")
	ct := generateCSRFToken(signingKey)
	ctx := createContextWithCSRF("", ct, "")
	if err := VerifyCSRF(ctx, signingKey); err == nil {
		t.Error("VerifyCSRF passed when it should have failed")
	}
}

func TestVerifyCSRF_CookieParamMismatch(t *testing.T) {
	signingKey := []byte("test_signing_key")
	ct1 := generateCSRFToken(signingKey)
	ct2 := generateCSRFToken([]byte("attacker_key"))
	ctx := createContextWithCSRF("", ct1, ct2)
	if err := VerifyCSRF(ctx, signingKey); err == nil {
		t.Error("VerifyCSRF passed when it should have failed")
	}
}

func TestVerifyCSRF_BadKey(t *testing.T) {
	ct := generateCSRFToken([]byte("attacker_key"))
	ctx := createContextWithCSRF("", ct, ct)
	if err := VerifyCSRF(ctx, []byte("real_key")); err == nil {
		t.Error("VerifyCSRF passed when it should have failed")
	}
}

func TestVerifyCSRF_Success_XHRHeader(t *testing.T) {
	signingKey := []byte("test_signing_key")
	ctx := createContextWithCSRF("1", "", "")
	if err := VerifyCSRF(ctx, signingKey); err != nil {
		t.Errorf("VerifyCSRF failed when it should have succeeded: %v", err)
	}
}

func TestVerifyCSRF_Success_MatchingParamAndCookie(t *testing.T) {
	signingKey := []byte("test_signing_key")
	ct := generateCSRFToken(signingKey)
	ctx := createContextWithCSRF("", ct, ct)
	if err := VerifyCSRF(ctx, signingKey); err != nil {
		t.Errorf("VerifyCSRF failed when it should have succeeded: %v", err)
	}
}

func createContextWithCSRF(headerValue, paramValue, cookieValue string) context.Context {
	md := metadata.New(map[string]string{})
	if headerValue != "" {
		md.Set("pf-header-x-csrf-protection", headerValue)
	}
	if paramValue != "" {
		md.Set("csrf-token", paramValue)
	}
	if cookieValue != "" {
		cookie := &http.Cookie{Name: csrfCookie, Value: cookieValue}
		md.Set("grpcgateway-cookie", cookie.String())
	}
	return metadata.NewIncomingContext(context.Background(), md)
}

type mockServerTransportStream struct {
	md *metadata.MD
}

func (m *mockServerTransportStream) Method() string {
	return "test"
}

func (m *mockServerTransportStream) SetHeader(md metadata.MD) error {
	m.md = &md
	return nil
}

func (m *mockServerTransportStream) SendHeader(md metadata.MD) error {
	panic("Not implemented")
}

func (m *mockServerTransportStream) SetTrailer(md metadata.MD) error {
	panic("Not implemented")
}
