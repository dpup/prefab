// Package main demonstrates the OAuth plugin with a complete example server.
//
// This example shows:
// - Setting up an OAuth authorization server
// - Registering OAuth clients
// - Interposing an explicit consent step before code issuance
// - Protecting endpoints with OAuth scopes
// - Using the authorization code flow
//
// To test:
// 1. Run: go run ./examples/oauthserver
// 2. Visit: http://localhost:8080
// 3. Click "Start OAuth Flow" to test the authorization code flow
// 4. Or use curl to test the client credentials flow:
//
//	curl -X POST http://localhost:8080/oauth/token \
//	  -d "grant_type=client_credentials" \
//	  -d "client_id=demo-client" \
//	  -d "client_secret=demo-secret" \
//	  -d "scope=read"
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/auth/fakeauth"
	"github.com/dpup/prefab/plugins/oauth"
)

// consentSigningKey signs the CSRF/consent tokens handed to the browser. In a
// real deployment, pull this from config or KMS; hardcoding is fine for a demo.
var consentSigningKey = []byte("demo-consent-signing-key-change-me")

// consentCookieName holds the CSRF token that guards the consent form.
const consentCookieName = "oauth-consent-csrf"

func main() {
	// Create OAuth plugin with demo clients and a consent-enforcing handler.
	oauthPlugin := oauth.NewBuilder().
		// A confidential client for server-to-server communication
		WithClient(oauth.Client{
			ID:           "demo-client",
			Secret:       "demo-secret",
			Name:         "Demo Client",
			RedirectURIs: []string{"http://localhost:8080/callback"},
			Scopes:       []string{"read", "write", "admin"},
			Public:       false,
		}).
		// A public client for SPAs or mobile apps
		WithClient(oauth.Client{
			ID:           "public-client",
			Name:         "Public Client",
			RedirectURIs: []string{"http://localhost:3000/callback", "http://127.0.0.1:3000/callback"},
			Scopes:       []string{"read", "write"},
			Public:       true,
		}).
		WithAccessTokenExpiry(time.Hour).
		WithRefreshTokenExpiry(7 * 24 * time.Hour).
		WithIssuer("http://localhost:8080").
		// Require an explicit consent step rather than auto-approving any
		// authenticated user's request.
		WithUserAuthorizationHandler(consentGatedAuthorization).
		Build()

	server := prefab.New(
		prefab.WithPlugin(auth.Plugin()),
		prefab.WithPlugin(fakeauth.Plugin()),
		prefab.WithPlugin(oauthPlugin),
		prefab.WithHTTPHandler("/", homeHandler()),
		prefab.WithHTTPHandler("/consent", consentHandler()),
		prefab.WithHTTPHandler("/callback", callbackHandler()),
		prefab.WithHTTPHandler("/api/public", publicHandler()),
		prefab.WithHTTPHandler("/api/protected", protectedHandler()),
		prefab.WithHTTPHandler("/api/admin", adminHandler()),
		prefab.WithHTTPHandler("/api/userinfo", userinfoHandler()),
	)

	log.Println("Starting OAuth example server on http://localhost:8080")
	log.Println("Visit http://localhost:8080 to test the OAuth flow")

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// consentGatedAuthorization is the custom UserAuthorizationHandler that
// interposes a consent step between "user is authenticated" and "code is
// issued." It expects the request to carry a `consent` form value holding a
// signed CSRF token which must match the cookie set by the consent page.
// If no valid token is present, it redirects the browser to /consent with the
// original authorize parameters preserved.
func consentGatedAuthorization(w http.ResponseWriter, r *http.Request) (string, error) {
	identity, err := auth.IdentityFromContext(r.Context())
	if err != nil {
		// Not authenticated — let go-oauth2's default error redirect fire so
		// the client sees a proper error rather than a consent page.
		return "", err
	}

	submitted := r.FormValue("consent")
	cookie, cookieErr := r.Cookie(consentCookieName)

	// If the request carries a submitted token that matches the cookie and
	// has a valid HMAC, treat it as explicit approval.
	if submitted != "" && cookieErr == nil && submitted == cookie.Value {
		if verifyErr := prefab.VerifyCSRFToken(submitted, consentSigningKey); verifyErr == nil {
			// Consume the cookie so the approval can't be replayed.
			http.SetCookie(w, &http.Cookie{
				Name:     consentCookieName,
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			return identity.Subject, nil
		}
	}

	// No (or invalid) approval — bounce the browser through our consent page,
	// preserving the authorize params so we can replay them on approval.
	redirect := "/consent?" + r.URL.RawQuery
	http.Redirect(w, r, redirect, http.StatusFound)
	return "", nil
}

// consentHandler shows a consent page for GET requests and replays the
// authorize request with a signed consent token on POST (approval).
func consentHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			renderConsentPage(w, r)
		case http.MethodPost:
			handleConsentApproval(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// renderConsentPage shows the approval form. It mints a fresh CSRF token,
// sets it as a cookie, and embeds it in the form as a hidden field.
func renderConsentPage(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	clientID := params.Get("client_id")
	scope := params.Get("scope")

	if clientID == "" {
		http.Error(w, "missing client_id", http.StatusBadRequest)
		return
	}

	token := prefab.GenerateCSRFToken(consentSigningKey)
	http.SetCookie(w, &http.Cookie{
		Name:     consentCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600, // 10 minutes
	})

	// Build hidden fields that replay the original authorize params on approval.
	var hiddenFields strings.Builder
	for key, values := range params {
		for _, v := range values {
			fmt.Fprintf(&hiddenFields, `<input type="hidden" name="%s" value="%s">`,
				html.EscapeString(key), html.EscapeString(v))
		}
	}

	scopeList := "<em>(no specific scopes requested)</em>"
	if scope != "" {
		var items []string
		for _, s := range strings.Fields(scope) {
			items = append(items, "<li><code>"+html.EscapeString(s)+"</code></li>")
		}
		scopeList = "<ul>" + strings.Join(items, "") + "</ul>"
	}

	page := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Authorize %s</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        .card { padding: 30px; border: 1px solid #ddd; border-radius: 8px; background: #fafafa; }
        .btn { padding: 10px 20px; border: 0; border-radius: 4px; cursor: pointer; margin-right: 10px; font-size: 16px; }
        .btn-approve { background: #28a745; color: white; }
        .btn-deny { background: #dc3545; color: white; }
        .scope { background: #e9ecef; padding: 2px 6px; border-radius: 3px; font-family: monospace; }
    </style>
</head>
<body>
    <div class="card">
        <h1>Authorize <strong>%s</strong>?</h1>
        <p>This application is requesting the following permissions:</p>
        %s
        <form method="POST" action="/consent">
            %s
            <input type="hidden" name="consent" value="%s">
            <button class="btn btn-approve" name="decision" value="approve">Approve</button>
            <button class="btn btn-deny" name="decision" value="deny">Deny</button>
        </form>
        <p style="margin-top: 20px; color: #666; font-size: 14px;">
            You'll be redirected back to the application after you decide.
        </p>
    </div>
</body>
</html>`,
		html.EscapeString(clientID),
		html.EscapeString(clientID),
		scopeList,
		hiddenFields.String(),
		html.EscapeString(token))

	w.Header().Set("Content-Type", "text/html")
	_, _ = w.Write([]byte(page))
}

// handleConsentApproval handles the form POST. On approve, it replays the
// authorize request with the consent token attached so the oauth plugin's
// consentGatedAuthorization handler can verify and proceed.
func handleConsentApproval(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	decision := r.FormValue("decision")
	clientID := r.FormValue("client_id")
	redirectURI := r.FormValue("redirect_uri")
	state := r.FormValue("state")
	consent := r.FormValue("consent")

	cookie, err := r.Cookie(consentCookieName)
	if err != nil || cookie.Value == "" || cookie.Value != consent {
		http.Error(w, "missing or mismatched consent token", http.StatusForbidden)
		return
	}
	if err := prefab.VerifyCSRFToken(consent, consentSigningKey); err != nil {
		http.Error(w, "invalid consent token", http.StatusForbidden)
		return
	}

	if decision != "approve" {
		// User denied — redirect to the client's redirect_uri with an OAuth
		// access_denied error per RFC 6749.
		if redirectURI == "" {
			http.Error(w, "access denied", http.StatusForbidden)
			return
		}
		q := url.Values{}
		q.Set("error", "access_denied")
		q.Set("error_description", "The user denied the request")
		if state != "" {
			q.Set("state", state)
		}
		http.Redirect(w, r, redirectURI+"?"+q.Encode(), http.StatusFound)
		return
	}

	// Replay the original authorize request, preserving all params and
	// attaching the consent token so the plugin's handler lets it through.
	authorizeParams := url.Values{}
	for key, values := range r.PostForm {
		if key == "decision" {
			continue
		}
		for _, v := range values {
			authorizeParams.Add(key, v)
		}
	}
	if clientID == "" || redirectURI == "" {
		http.Error(w, "missing required authorize parameters", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/oauth/authorize?"+authorizeParams.Encode(), http.StatusFound)
}

// homeHandler serves a simple HTML page for testing OAuth flows.
func homeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		page := `<!DOCTYPE html>
<html>
<head>
    <title>OAuth Example Server</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .section { margin: 20px 0; padding: 20px; border: 1px solid #ddd; border-radius: 8px; }
        .btn { display: inline-block; padding: 10px 20px; background: #007bff; color: white; text-decoration: none; border-radius: 4px; margin: 5px; }
        .btn:hover { background: #0056b3; }
        .btn-secondary { background: #6c757d; }
        .btn-secondary:hover { background: #545b62; }
        pre { background: #f5f5f5; padding: 10px; overflow-x: auto; border-radius: 4px; }
        code { font-family: monospace; }
    </style>
</head>
<body>
    <h1>OAuth2 Example Server</h1>

    <div class="section">
        <h2>Test Authorization Code Flow</h2>
        <p>This initiates an authorization request. The server will show you a consent page before issuing a code:</p>
        <a class="btn" href="/oauth/authorize?client_id=demo-client&response_type=code&redirect_uri=http://localhost:8080/callback&scope=read%20write&state=test123">
            Start OAuth Flow
        </a>
        <p><small>Requests read + write scopes on behalf of demo-client.</small></p>
    </div>

    <div class="section">
        <h2>Test Client Credentials Flow</h2>
        <p>Get an access token using client credentials (no user involved, so no consent step):</p>
        <button class="btn" onclick="testClientCredentials()">Get Token</button>
        <pre id="token-result"></pre>
    </div>

    <div class="section">
        <h2>Test Protected Endpoints</h2>
        <p>Enter an access token to test the protected endpoints:</p>
        <input type="text" id="access-token" placeholder="Access Token" style="width: 100%; padding: 8px; margin: 10px 0;">
        <br>
        <button class="btn btn-secondary" onclick="testEndpoint('/api/public')">Public API</button>
        <button class="btn btn-secondary" onclick="testEndpoint('/api/protected')">Protected API</button>
        <button class="btn btn-secondary" onclick="testEndpoint('/api/admin')">Admin API</button>
        <button class="btn btn-secondary" onclick="testEndpoint('/api/userinfo')">User Info</button>
        <pre id="api-result"></pre>
    </div>

    <div class="section">
        <h2>OAuth Metadata</h2>
        <a class="btn btn-secondary" href="/.well-known/oauth-authorization-server" target="_blank">
            View Metadata
        </a>
    </div>

    <script>
        async function testClientCredentials() {
            const result = document.getElementById('token-result');
            result.textContent = 'Loading...';

            try {
                const response = await fetch('/oauth/token', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded',
                    },
                    body: new URLSearchParams({
                        grant_type: 'client_credentials',
                        client_id: 'demo-client',
                        client_secret: 'demo-secret',
                        scope: 'read write'
                    })
                });

                const data = await response.json();
                result.textContent = JSON.stringify(data, null, 2);

                if (data.access_token) {
                    document.getElementById('access-token').value = data.access_token;
                }
            } catch (err) {
                result.textContent = 'Error: ' + err.message;
            }
        }

        async function testEndpoint(endpoint) {
            const result = document.getElementById('api-result');
            const token = document.getElementById('access-token').value;

            result.textContent = 'Loading...';

            try {
                const headers = {};
                if (token) {
                    headers['Authorization'] = 'Bearer ' + token;
                }

                const response = await fetch(endpoint, { headers });
                const data = await response.json();
                result.textContent = JSON.stringify(data, null, 2);
            } catch (err) {
                result.textContent = 'Error: ' + err.message;
            }
        }
    </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	})
}

// callbackHandler handles the OAuth redirect callback.
func callbackHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := html.EscapeString(r.URL.Query().Get("code"))
		state := html.EscapeString(r.URL.Query().Get("state"))
		errorCode := html.EscapeString(r.URL.Query().Get("error"))
		errorDesc := html.EscapeString(r.URL.Query().Get("error_description"))

		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>OAuth Callback</title>
    <style>
        body { font-family: system-ui, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        pre { background: #f5f5f5; padding: 10px; overflow-x: auto; border-radius: 4px; }
        .btn { display: inline-block; padding: 10px 20px; background: #007bff; color: white; text-decoration: none; border-radius: 4px; }
        .error { color: red; }
        .success { color: green; }
    </style>
</head>
<body>
    <h1>OAuth Callback</h1>

    %s

    <p><a href="/" class="btn">Back to Home</a></p>

    <script>
        async function exchangeCode() {
            const code = '%s';
            if (!code) return;

            const result = document.getElementById('exchange-result');
            result.textContent = 'Exchanging code for token...';

            try {
                const response = await fetch('/oauth/token', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/x-www-form-urlencoded',
                    },
                    body: new URLSearchParams({
                        grant_type: 'authorization_code',
                        code: code,
                        client_id: 'demo-client',
                        client_secret: 'demo-secret',
                        redirect_uri: 'http://localhost:8080/callback'
                    })
                });

                const data = await response.json();
                result.textContent = JSON.stringify(data, null, 2);
            } catch (err) {
                result.textContent = 'Error: ' + err.message;
            }
        }

        if ('%s') {
            exchangeCode();
        }
    </script>
</body>
</html>`,
			func() string {
				if errorCode != "" {
					return fmt.Sprintf(`<div class="error">
        <h2>Error</h2>
        <p><strong>Error:</strong> %s</p>
        <p><strong>Description:</strong> %s</p>
    </div>`, errorCode, errorDesc)
				}
				if code != "" {
					return fmt.Sprintf(`<div class="success">
        <h2>Authorization Successful!</h2>
        <p><strong>Code:</strong> <code>%s</code></p>
        <p><strong>State:</strong> <code>%s</code></p>

        <h3>Exchange Code for Token</h3>
        <pre id="exchange-result">Click button or wait...</pre>
    </div>`, code, state)
				}
				return `<p>No authorization code received.</p>`
			}(),
			code, code)

		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(htmlContent))
	})
}

func publicHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]interface{}{
			"message": "This is a public endpoint",
			"time":    time.Now().Format(time.RFC3339),
		})
	})
}

func protectedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		identity, err := auth.IdentityFromContext(ctx)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		if oauth.IsOAuthRequest(ctx) {
			if !oauth.HasScope(ctx, "read") {
				jsonError(w, http.StatusForbidden, "Missing 'read' scope")
				return
			}
		}

		jsonResponse(w, map[string]interface{}{
			"message":   "This is a protected endpoint",
			"subject":   identity.Subject,
			"provider":  identity.Provider,
			"is_oauth":  oauth.IsOAuthRequest(ctx),
			"client_id": oauth.OAuthClientIDFromContext(ctx),
			"scopes":    oauth.OAuthScopesFromContext(ctx),
		})
	})
}

func adminHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		identity, err := auth.IdentityFromContext(ctx)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		if oauth.IsOAuthRequest(ctx) {
			if !oauth.HasScope(ctx, "admin") {
				jsonError(w, http.StatusForbidden, "Missing 'admin' scope")
				return
			}
		}

		jsonResponse(w, map[string]interface{}{
			"message":   "This is an admin endpoint",
			"subject":   identity.Subject,
			"is_oauth":  oauth.IsOAuthRequest(ctx),
			"client_id": oauth.OAuthClientIDFromContext(ctx),
			"scopes":    oauth.OAuthScopesFromContext(ctx),
		})
	})
}

func userinfoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		identity, err := auth.IdentityFromContext(ctx)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		jsonResponse(w, map[string]interface{}{
			"subject":        identity.Subject,
			"email":          identity.Email,
			"email_verified": identity.EmailVerified,
			"name":           identity.Name,
			"provider":       identity.Provider,
			"session_id":     identity.SessionID,
			"auth_time":      identity.AuthTime.Format(time.RFC3339),
			"is_oauth":       oauth.IsOAuthRequest(ctx),
			"oauth_client":   oauth.OAuthClientIDFromContext(ctx),
			"oauth_scopes":   oauth.OAuthScopesFromContext(ctx),
		})
	})
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
