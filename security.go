package prefab

import (
	"fmt"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"

	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"google.golang.org/grpc/codes"
)

type XFramesOptions string

const (
	XFramesOptionsNone       XFramesOptions = ""
	XFramesOptionsDeny       XFramesOptions = "DENY"
	XFramesOptionsSameOrigin XFramesOptions = "SAMEORIGIN"
)

var (
	// HSTS requires a minimum expiration of 1 year for preload.
	ErrBadHSTSExpiration = errors.NewC("prefab: HSTS preload requires expiration of at least 1 year", codes.FailedPrecondition)

	// ErrCORSWildcardCredentials is returned when the wildcard origin "*" is
	// combined with credentialed CORS. The Fetch spec forbids the pairing
	// (browsers reject "*" alongside Access-Control-Allow-Credentials: true), so
	// prefab refuses it at startup rather than shipping a policy that is either
	// unusable or — if an implementation "fixed" it by reflecting the origin
	// instead — a credential-leaking hole.
	ErrCORSWildcardCredentials = errors.NewC(`prefab: CORS wildcard origin "*" cannot be combined with CORSAllowCredentials`, codes.FailedPrecondition)
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
	// resource and what methods are allowed. Origins are matched exactly, with
	// one special value: a single "*" entry opts into wildcard CORS, emitting a
	// literal Access-Control-Allow-Origin: * for every request. That is distinct
	// from — and safer than — reflecting the caller's Origin: "*" cannot be
	// paired with credentials (see ErrCORSWildcardCredentials), so it can never
	// silently become a credentialed cross-origin hole.
	// TODO: Should this allow for prefix/suffix patterns too, not just "*"?
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
	corsWildcard     bool       // CORSOrigins contains "*": emit a literal ACAO: *.
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
		// Wildcard emits a constant "*" for every request; otherwise the caller's
		// Origin is echoed back only when it exactly matches the allowlist.
		allowOrigin := ""
		if s.corsWildcard {
			allowOrigin = "*"
		} else if s.allowedOrigins[r.Header.Get("Origin")] {
			allowOrigin = r.Header.Get("Origin")
		}
		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			if r.Method == http.MethodOptions {
				for k, v := range s.preflightHeaders {
					w.Header().Set(k, v)
				}
			} else if len(s.CORSExposeHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(s.CORSExposeHeaders, ", "))
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

	// Detect wildcard CORS and reject the credentialed pairing. Done before the
	// staticHeaders memoization guard below so the invariant is re-checked on
	// every call — a returned error must not stop recurring just because a prior
	// call already populated staticHeaders.
	s.corsWildcard = false
	for _, origin := range s.CORSOrigins {
		if origin == "*" {
			s.corsWildcard = true
			break
		}
	}
	if s.corsWildcard && s.CORSAllowCredentials {
		return errors.Mark(ErrCORSWildcardCredentials, 0)
	}

	if s.staticHeaders == nil {
		s.staticHeaders = make(map[string]string)
		s.staticHeaders["X-Content-Type-Options"] = "nosniff"
		s.staticHeaders["Referrer-Policy"] = "strict-origin-when-cross-origin"

		if s.XFramesOptions != XFramesOptionsNone {
			s.staticHeaders["X-Frame-Options"] = string(s.XFramesOptions)
		}

		if s.HSTSExpiration > 0 {
			if err := s.computeHSTSHeaders(); err != nil {
				return err
			}
		}

		if len(s.CORSOrigins) > 0 {
			s.computeCORSHeaders()
		}
	}
	return nil
}

func (s *SecurityHeaders) computeHSTSHeaders() error {
	h := fmt.Sprintf("max-age=%.0f", s.HSTSExpiration.Seconds())
	if s.HSTSIncludeSubdomains {
		h += "; includeSubDomains"
	}
	if s.HSTSPreload {
		if s.HSTSExpiration < time.Hour*24*365 {
			return errors.Mark(ErrBadHSTSExpiration, 0)
		}
		h += "; preload"
	}
	s.staticHeaders["Strict-Transport-Security"] = h
	return nil
}

// See https://fetch.spec.whatwg.org/#http-responses for details on
// headers required on preflight and non-preflight requests.
func (s *SecurityHeaders) computeCORSHeaders() {
	// Vary: Origin matters only for the reflected-origin path, where the response
	// depends on the request's Origin. A wildcard emits a constant "*", so Vary
	// would needlessly defeat shared-cache reuse.
	if !s.corsWildcard {
		s.staticHeaders["Vary"] = "Origin"
	}

	if s.CORSAllowCredentials {
		s.staticHeaders["Access-Control-Allow-Credentials"] = "true"
	}

	s.preflightHeaders = make(map[string]string)
	if len(s.CORSAllowMethods) > 0 {
		s.preflightHeaders["Access-Control-Allow-Methods"] = strings.Join(s.CORSAllowMethods, ", ")
	} else {
		s.preflightHeaders["Access-Control-Allow-Methods"] = "GET, POST, PUT, DELETE, PATCH"
	}
	if len(s.CORSAllowHeaders) > 0 {
		s.preflightHeaders["Access-Control-Allow-Headers"] = strings.Join(s.CORSAllowHeaders, ", ")
	}
	if s.CORSMaxAge > 0 {
		s.preflightHeaders["Access-Control-Max-Age"] = fmt.Sprintf("%.0f", s.CORSMaxAge.Seconds())
	}

	s.allowedOrigins = map[string]bool{}
	for _, origin := range s.CORSOrigins {
		s.allowedOrigins[origin] = true
	}
}

func (s *SecurityHeaders) normalizeHeaders(h []string) {
	for i, v := range h {
		h[i] = textproto.CanonicalMIMEHeaderKey(v)
	}
}

func securityMiddleware(h http.Handler, sh *SecurityHeaders) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := sh.Apply(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logging.Errorw(r.Context(), "Failed to apply security headers", "error", err)
			return
		}
		if r.Method == http.MethodOptions {
			return // Only send headers on OPTIONS requests.
		}

		h.ServeHTTP(w, r)
	})
}
