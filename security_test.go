package prefab

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name            string
		req             *http.Request
		conf            *SecurityHeaders
		expectedHeaders map[string]string
		expectedError   error
	}{
		{
			name: "empty",
			conf: &SecurityHeaders{},
			expectedHeaders: map[string]string{
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"X-Content-Type-Options": "nosniff",
			},
			expectedError: nil,
		},

		{
			name: "x-frame-options-deny",
			conf: &SecurityHeaders{XFramesOptions: XFramesOptionsDeny},
			expectedHeaders: map[string]string{
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "DENY",
			},
			expectedError: nil,
		},
		{
			name: "x-frame-options-sameorigin",
			conf: &SecurityHeaders{XFramesOptions: XFramesOptionsSameOrigin},
			expectedHeaders: map[string]string{
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"X-Content-Type-Options": "nosniff",
				"X-Frame-Options":        "SAMEORIGIN",
			},
			expectedError: nil,
		},

		{
			name: "hsts expiration only",
			conf: &SecurityHeaders{HSTSExpiration: time.Hour * 24},
			expectedHeaders: map[string]string{
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"X-Content-Type-Options":    "nosniff",
				"Strict-Transport-Security": "max-age=86400",
			},
			expectedError: nil,
		},
		{
			name: "hsts full",
			conf: &SecurityHeaders{
				HSTSExpiration:        time.Hour * 24 * 365,
				HSTSIncludeSubdomains: true,
				HSTSPreload:           true,
			},
			expectedHeaders: map[string]string{
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"X-Content-Type-Options":    "nosniff",
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
			},
			expectedError: nil,
		},
		{
			name: "hsts full short expiration",
			conf: &SecurityHeaders{
				HSTSExpiration:        time.Hour * 24,
				HSTSIncludeSubdomains: true,
				HSTSPreload:           true,
			},
			expectedHeaders: map[string]string{},
			expectedError:   ErrBadHSTSExpiration,
		},

		{
			name: "cors different origin",
			req:  httptest.NewRequest(http.MethodOptions, "https://example.com", nil),
			conf: &SecurityHeaders{
				CORSOrigins: []string{"https://something.com"},
			},
			expectedHeaders: map[string]string{
				"Vary":                   "Origin",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"X-Content-Type-Options": "nosniff",
			},
		},
		{
			name: "cors only origin",
			req:  httptest.NewRequest(http.MethodOptions, "https://example.com/foobar", nil),
			conf: &SecurityHeaders{
				CORSOrigins: []string{"https://example.com"},
			},
			expectedHeaders: map[string]string{
				"Vary":                         "Origin",
				"Referrer-Policy":              "strict-origin-when-cross-origin",
				"X-Content-Type-Options":       "nosniff",
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, PATCH",
			},
		},
		{
			name: "cors method override",
			req:  httptest.NewRequest(http.MethodOptions, "https://example.com/foobar", nil),
			conf: &SecurityHeaders{
				CORSOrigins:      []string{"https://example.com"},
				CORSAllowMethods: []string{"GET"},
			},
			expectedHeaders: map[string]string{
				"Vary":                         "Origin",
				"Referrer-Policy":              "strict-origin-when-cross-origin",
				"X-Content-Type-Options":       "nosniff",
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET",
			},
		},
		{
			name: "cors allow headers",
			req:  httptest.NewRequest(http.MethodOptions, "https://example.com/foobar", nil),
			conf: &SecurityHeaders{
				CORSOrigins:      []string{"https://example.com"},
				CORSAllowHeaders: []string{"X-Custom-Header", "X-Another-Header"},
			},
			expectedHeaders: map[string]string{
				"Vary":                         "Origin",
				"Referrer-Policy":              "strict-origin-when-cross-origin",
				"X-Content-Type-Options":       "nosniff",
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, PATCH",
				"Access-Control-Allow-Headers": "X-Custom-Header, X-Another-Header",
			},
		},
		{
			name: "cors allow credentials",
			req:  httptest.NewRequest(http.MethodOptions, "https://example.com/foobar", nil),
			conf: &SecurityHeaders{
				CORSOrigins:          []string{"https://example.com"},
				CORSAllowCredentials: true,
			},
			expectedHeaders: map[string]string{
				"Vary":                             "Origin",
				"Referrer-Policy":                  "strict-origin-when-cross-origin",
				"X-Content-Type-Options":           "nosniff",
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Methods":     "GET, POST, PUT, DELETE, PATCH",
				"Access-Control-Allow-Credentials": "true",
			},
		},
		{
			name: "cors max age",
			req:  httptest.NewRequest(http.MethodOptions, "https://example.com/foobar", nil),
			conf: &SecurityHeaders{
				CORSOrigins: []string{"https://example.com"},
				CORSMaxAge:  time.Hour,
			},
			expectedHeaders: map[string]string{
				"Vary":                         "Origin",
				"Referrer-Policy":              "strict-origin-when-cross-origin",
				"X-Content-Type-Options":       "nosniff",
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, PATCH",
				"Access-Control-Max-Age":       "3600",
			},
		},
		{
			name: "cors expose headers not on options request",
			req:  httptest.NewRequest(http.MethodOptions, "https://example.com/foobar", nil),
			conf: &SecurityHeaders{
				CORSOrigins:       []string{"https://example.com"},
				CORSAllowHeaders:  []string{"X-Allowed"},
				CORSExposeHeaders: []string{"X-Exposed"},
			},
			expectedHeaders: map[string]string{
				"Vary":                         "Origin",
				"Referrer-Policy":              "strict-origin-when-cross-origin",
				"X-Content-Type-Options":       "nosniff",
				"Access-Control-Allow-Origin":  "https://example.com",
				"Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, PATCH",
				"Access-Control-Allow-Headers": "X-Allowed",
			},
		},
		{
			name: "cors expose headers on get request",
			req:  httptest.NewRequest(http.MethodGet, "https://example.com/foobar", nil),
			conf: &SecurityHeaders{
				CORSOrigins:       []string{"https://example.com"},
				CORSAllowHeaders:  []string{"X-Allowed"},
				CORSExposeHeaders: []string{"X-Exposed"},
			},
			expectedHeaders: map[string]string{
				"Vary":                          "Origin",
				"Referrer-Policy":               "strict-origin-when-cross-origin",
				"X-Content-Type-Options":        "nosniff",
				"Access-Control-Allow-Origin":   "https://example.com",
				"Access-Control-Expose-Headers": "X-Exposed",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.req
			if r != nil {
				r.Header.Set("Origin", r.URL.Scheme+"://"+r.URL.Hostname())
			}
			w := httptest.NewRecorder()

			err := tt.conf.Apply(w, r)
			require.ErrorIs(t, err, tt.expectedError, "unexpected error")

			if err == nil {
				result := w.Result()

				// Assert all expected headers are present.
				for k, v := range tt.expectedHeaders {
					assert.Equal(t, v, result.Header.Get(k), "unexpected header value: %s", k)
				}

				// Assert no unexpected headers are present.
				for k := range result.Header {
					_, ok := tt.expectedHeaders[k]
					assert.True(t, ok, "unexpected header present: %s", k)
				}
			}
		})
	}
}
