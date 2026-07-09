package prefab

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConditionalResponse(t *testing.T) {
	t.Run("304 drops body and content headers", func(t *testing.T) {
		h := conditionalResponse(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Etag", `W/"v1"`)
			w.WriteHeader(http.StatusNotModified)
			// The Gateway would marshal the message here; it must be dropped.
			_, _ = w.Write([]byte("{}"))
		}))

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/thing", nil))

		res := rec.Result()
		assert.Equal(t, http.StatusNotModified, res.StatusCode)
		assert.Empty(t, rec.Body.String())
		assert.Empty(t, res.Header.Get("Content-Type"))
		// The validator header is preserved on a 304.
		assert.Equal(t, `W/"v1"`, res.Header.Get("Etag"))
	})

	t.Run("non-304 passes through untouched", func(t *testing.T) {
		h := conditionalResponse(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/thing", nil))

		res := rec.Result()
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.Equal(t, `{"ok":true}`, rec.Body.String())
		assert.Equal(t, "application/json", res.Header.Get("Content-Type"))
	})
}
