// Package main demonstrates the OAuth plugin with a complete example server.
//
// This example shows:
// - Setting up an OAuth authorization server
// - Registering OAuth clients
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
	"log"
	"net/http"
	"time"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/auth/fakeauth"
	"github.com/dpup/prefab/plugins/oauth"
)

func main() {
	// Create OAuth plugin with demo clients
	oauthPlugin := oauth.NewBuilder().
		// A confidential client for server-to-server communication
		WithClient(oauth.Client{
			ID:           "demo-client",
			Secret:       "demo-secret",
			Name:         "Demo Client",
			RedirectURIs: []string{"http://localhost:8080/callback"},
			Scopes:       []string{"read", "write", "admin"},
			Public:       false,
			Trusted:      true, // Skip consent screen
		}).
		// A public client for SPAs or mobile apps
		WithClient(oauth.Client{
			ID:           "public-client",
			Secret:       "",
			Name:         "Public Client",
			RedirectURIs: []string{"http://localhost:3000/callback", "http://127.0.0.1:3000/callback"},
			Scopes:       []string{"read", "write"},
			Public:       true,
			Trusted:      false,
		}).
		WithAccessTokenExpiry(time.Hour).
		WithRefreshTokenExpiry(7 * 24 * time.Hour).
		WithIssuer("http://localhost:8080").
		Build()

	// Create server with auth and OAuth plugins
	server := prefab.New(
		prefab.WithPlugin(auth.Plugin()),
		prefab.WithPlugin(fakeauth.Plugin()), // For easy testing - auto-login
		prefab.WithPlugin(oauthPlugin),
		prefab.WithHTTPHandler("/", homeHandler()),
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

// homeHandler serves a simple HTML page for testing OAuth flows.
func homeHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		html := `<!DOCTYPE html>
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
        #result { margin-top: 20px; }
    </style>
</head>
<body>
    <h1>OAuth2 Example Server</h1>

    <div class="section">
        <h2>Test Authorization Code Flow</h2>
        <p>This will redirect you through the OAuth authorization flow:</p>
        <a class="btn" href="/oauth/authorize?client_id=demo-client&response_type=code&redirect_uri=http://localhost:8080/callback&scope=read%20write&state=test123">
            Start OAuth Flow
        </a>
        <p><small>Uses demo-client with scopes: read, write</small></p>
    </div>

    <div class="section">
        <h2>Test Client Credentials Flow</h2>
        <p>Get an access token using client credentials:</p>
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
        <p>View the OAuth server metadata:</p>
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
		w.Write([]byte(html))
	})
}

// callbackHandler handles the OAuth redirect callback.
func callbackHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errorCode := r.URL.Query().Get("error")
		errorDesc := r.URL.Query().Get("error_description")

		html := fmt.Sprintf(`<!DOCTYPE html>
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

        // Auto-exchange if we have a code
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
		w.Write([]byte(html))
	})
}

// publicHandler is accessible without authentication.
func publicHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]interface{}{
			"message": "This is a public endpoint",
			"time":    time.Now().Format(time.RFC3339),
		})
	})
}

// protectedHandler requires authentication and "read" scope.
func protectedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Check if user is authenticated
		identity, err := auth.IdentityFromContext(ctx)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Check OAuth scope if this is an OAuth request
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

// adminHandler requires authentication and "admin" scope.
func adminHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Check if user is authenticated
		identity, err := auth.IdentityFromContext(ctx)
		if err != nil {
			jsonError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Check OAuth scope if this is an OAuth request
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

// userinfoHandler returns information about the authenticated user.
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
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
