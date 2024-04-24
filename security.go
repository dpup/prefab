package prefab

import (
	"fmt"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type XFramesOptions string

const (
	XFramesOptionsNone       XFramesOptions = ""
	XFramesOptionsDeny       XFramesOptions = "DENY"
	XFramesOptionsSameOrigin XFramesOptions = "SAMEORIGIN"
)

var (
	// HSTS requires a minimum expiration of 1 year for preload.
	ErrBadHSTSExpiration = status.Error(codes.FailedPrecondition, "prefab: HSTS preload requires expiration of at least 1 year")
)

// SecurityHeaders contains the security headers that should be set on HTTP
// responses.
type SecurityHeaders struct {
	// X-Frame-Options controls whether the browser should allow the page to be
	// rendered in a frame or iframe.
	XFramesOptions XFramesOptions

	// Strict-Transport-Security (HSTS) tells the browser to always use HTTPS
	// when connecting to the site.
	HSTSExpiration        time.Duration
	HSTSIncludeSubdomains bool
	HSTSPreload           bool

	// Access-Control headers define which origins are allowed to access the
	// resource and what methods are allowed.
	CORSOrigins          []string
	CORSAllowMethods     []string
	CORSAllowHeaders     []string
	CORSExposeHeaders    []string
	CORSAllowCredentials bool
	CORSMaxAge           time.Duration

	// Precomputed fields.
	staticHeaders    map[string]string
	preflightHeaders map[string]string
	allowedOrigins   map[string]bool
	mu               sync.Mutex // Protects precomputed fields.
}

// Apply the security headers to the given response.
func (s *SecurityHeaders) Apply(w http.ResponseWriter, r *http.Request) error {
	if err := s.compute(); err != nil {
		return err
	}
	for k, v := range s.staticHeaders {
		w.Header().Set(k, v)
	}

	if len(s.CORSOrigins) > 0 {
		origin := r.Header.Get("Origin")
		if s.allowedOrigins[origin] {
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				for k, v := range s.preflightHeaders {
					w.Header().Set(k, v)
				}
			} else {
				if len(s.CORSExposeHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(s.CORSExposeHeaders, ", "))
				}
			}
		}
	}

	return nil
}

func (s *SecurityHeaders) compute() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.normalizeHeaders(s.CORSAllowHeaders)
	s.normalizeHeaders(s.CORSExposeHeaders)

	if s.staticHeaders == nil {
		s.staticHeaders = make(map[string]string)
		s.staticHeaders["X-Content-Type-Options"] = "nosniff"
		s.staticHeaders["Referrer-Policy"] = "strict-origin-when-cross-origin"

		if s.XFramesOptions != XFramesOptionsNone {
			s.staticHeaders["X-Frame-Options"] = string(s.XFramesOptions)
		}

		if s.HSTSExpiration > 0 {
			h := fmt.Sprintf("max-age=%.0f", s.HSTSExpiration.Seconds())
			if s.HSTSIncludeSubdomains {
				h += "; includeSubDomains"
			}
			if s.HSTSPreload {
				if s.HSTSExpiration < time.Hour*24*365 {
					return ErrBadHSTSExpiration
				}
				h += "; preload"
			}
			s.staticHeaders["Strict-Transport-Security"] = h
		}

		if len(s.CORSOrigins) > 0 {
			s.staticHeaders["Vary"] = "Origin"

			s.preflightHeaders = make(map[string]string)
			s.preflightHeaders["Access-Control-Allow-Origin"] = s.CORSOrigins[0]
			if len(s.CORSAllowMethods) > 0 {
				s.preflightHeaders["Access-Control-Allow-Methods"] = strings.Join(s.CORSAllowMethods, ", ")
			} else {
				s.preflightHeaders["Access-Control-Allow-Methods"] = "GET, POST, PUT, DELETE, PATCH"
			}
			if len(s.CORSAllowHeaders) > 0 {
				s.preflightHeaders["Access-Control-Allow-Headers"] = strings.Join(s.CORSAllowHeaders, ", ")
			}
			if s.CORSAllowCredentials {
				s.preflightHeaders["Access-Control-Allow-Credentials"] = "true"
			}
			if s.CORSMaxAge > 0 {
				s.preflightHeaders["Access-Control-Max-Age"] = fmt.Sprintf("%.0f", s.CORSMaxAge.Seconds())
			}

			// TODO: Should this allow patterns instead of only exact matches?
			s.allowedOrigins = map[string]bool{}
			for _, origin := range s.CORSOrigins {
				s.allowedOrigins[origin] = true
			}
		}
	}
	return nil
}

func (s *SecurityHeaders) normalizeHeaders(h []string) {
	for i, v := range h {
		h[i] = textproto.CanonicalMIMEHeaderKey(v)
	}
}
