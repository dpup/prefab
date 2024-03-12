package serverutil

import (
	"context"
	"net/http"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestHeaderMatcher(t *testing.T) {
	tests := []struct {
		name           string
		headers        []string
		key            string
		expectedResult string
		expectedMatch  bool
	}{
		{
			name:           "Default header not present in setup args",
			headers:        []string{"Content-Type", "Accept"},
			key:            "Authorization",
			expectedResult: "grpcgateway-Authorization",
			expectedMatch:  true,
		},
		{
			name:           "Empty list of headers",
			headers:        []string{},
			key:            "Any-Key",
			expectedResult: "",
			expectedMatch:  false,
		},
		{
			name:           "Key with different case",
			headers:        []string{"X-Request-Id"},
			key:            "x-request-id",
			expectedResult: MetadataHeaderPrefix + "X-Request-Id",
			expectedMatch:  true,
		},
		{
			name:           "Key with non-standard capitalization",
			headers:        []string{"X-Custom-Header"},
			key:            "X-CUSTOM-HEADER",
			expectedResult: MetadataHeaderPrefix + "X-Custom-Header",
			expectedMatch:  true,
		},
		{
			name:           "Key with lowercase",
			headers:        []string{"x-custom-header"},
			key:            "x-custom-header",
			expectedResult: MetadataHeaderPrefix + "X-Custom-Header",
			expectedMatch:  true,
		},
		{
			name:           "Empty string as input key",
			headers:        []string{"Content-Type"},
			key:            "",
			expectedResult: "",
			expectedMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := HeaderMatcher(tt.headers)
			result, match := matcher(tt.key)

			if result != tt.expectedResult || match != tt.expectedMatch {
				t.Errorf("HeaderMatcher(%v)(%v) = %v, %v, want %v, %v", tt.headers, tt.key, result, match, tt.expectedResult, tt.expectedMatch)
			}
		})
	}
}

func TestHttpMethodAndMetadataAnnotator(t *testing.T) {
	req, err := http.NewRequest("POST", "/some/path", nil)
	if err != nil {
		t.Fatal(err)
	}

	md := HttpMetadataAnnotator(context.Background(), req)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	if method := HttpMethod(ctx); method != "POST" {
		t.Errorf("Expected HTTP method 'POST', got '%s'", method)
	}

	if method := HttpMethod(context.Background()); method != "" {
		t.Errorf("Expected no HTTP method, got '%s'", method)
	}
}

func TestHttpHeader(t *testing.T) {
	tests := []struct {
		name            string
		contextMetadata metadata.MD
		header          string
		want            string
	}{
		{
			name:            "application specific headers",
			contextMetadata: metadata.MD{MetadataHeaderPrefix + "test-header": []string{"test-value"}},
			header:          "test-header",
			want:            "test-value",
		},
		{
			name:            "application specific headers, case insensitive",
			contextMetadata: metadata.MD{MetadataHeaderPrefix + "test-header": []string{"test-value"}},
			header:          "Test-Header",
			want:            "test-value",
		},
		{
			name:            "permanent header via runtime",
			contextMetadata: metadata.MD{"grpcgateway-authorization": []string{"auth-value"}},
			header:          "authorization",
			want:            "auth-value",
		},
		{
			name:            "non-header metadata ignored",
			contextMetadata: metadata.MD{"other-metadata": []string{"other-value"}},
			header:          "other-metadata",
			want:            "",
		},
		{
			name:            "no metadata",
			contextMetadata: metadata.MD{},
			header:          "missing-header",
			want:            "",
		},
		{
			name:            "multiple values, first returned",
			contextMetadata: metadata.MD{MetadataHeaderPrefix + "multi-header": []string{"first-value", "second-value"}},
			header:          "multi-header",
			want:            "first-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), tt.contextMetadata)
			if got := HttpHeader(ctx, tt.header); got != tt.want {
				t.Errorf("HttpHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}
