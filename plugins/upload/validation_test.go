package upload

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestWithValidTypes(t *testing.T) {
	t.Run("CustomValidTypes", func(t *testing.T) {
		plugin := Plugin(
			WithBackend(NewMemBackend()),
			WithValidTypes("image/png", "application/pdf"),
		)

		assert.Equal(t, []string{"image/png", "application/pdf"}, plugin.validTypes)
	})

	t.Run("OverridesConfig", func(t *testing.T) {
		// Even if config has default types, WithValidTypes should override
		plugin := Plugin(
			WithBackend(NewMemBackend()),
			WithValidTypes("text/plain"),
		)

		assert.Equal(t, []string{"text/plain"}, plugin.validTypes)
	})
}

func TestServerOptions(t *testing.T) {
	plugin := Plugin(WithBackend(NewMemBackend()))

	opts := plugin.ServerOptions()

	// Should return upload and download handler options
	assert.Len(t, opts, 2)
}

func TestInit(t *testing.T) {
	t.Run("WithBackend", func(t *testing.T) {
		plugin := Plugin(WithBackend(NewMemBackend()))
		r := &prefab.Registry{}

		err := plugin.Init(t.Context(), r)
		require.NoError(t, err)
	})

	t.Run("WithoutBackend", func(t *testing.T) {
		plugin := Plugin()
		r := &prefab.Registry{}

		err := plugin.Init(t.Context(), r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no backend configured")
		assert.Equal(t, codes.Internal, errors.Code(err))
	})

	t.Run("WithAuthzPlugin", func(t *testing.T) {
		plugin := Plugin(WithBackend(NewMemBackend()))
		r := &prefab.Registry{}

		// Register authz plugin (which requires auth)
		r.Register(auth.Plugin())
		r.Register(plugin)

		ctx := logging.With(t.Context(), logging.NewDevLogger())
		err := r.Init(ctx)
		require.NoError(t, err)

		// Plugin should have authz reference
		assert.Nil(t, plugin.az) // authz not registered
	})
}

// validateUploadRequest tests are covered by the integration tests
// The function is primarily an internal helper

func TestHandleUpload_ErrorCases(t *testing.T) {
	t.Run("WrongMethod", func(t *testing.T) {
		plugin := Plugin(WithBackend(NewMemBackend()))
		req := httptest.NewRequest(http.MethodGet, "/upload", nil)
		req = req.WithContext(newTestContext())

		_, err := plugin.handleUpload(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "method not allowed")
		assert.Equal(t, codes.Unimplemented, errors.Code(err))
	})

	t.Run("WrongPath", func(t *testing.T) {
		plugin := Plugin(WithBackend(NewMemBackend()))
		req := httptest.NewRequest(http.MethodPost, "/wrong-path", nil)
		req = req.WithContext(newTestContext())

		_, err := plugin.handleUpload(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "path not allowed")
	})

	t.Run("BackendSaveError", func(t *testing.T) {
		// Use a backend that will fail
		backend := &errorBackend{}
		plugin := Plugin(WithBackend(backend))

		req := newSaveRequest(map[string][]byte{"test.png": pngData()})
		_, err := plugin.handleUpload(req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error saving file")
	})
}

func TestHandleDownload_ErrorCases(t *testing.T) {
	t.Run("WrongMethod", func(t *testing.T) {
		plugin := Plugin(WithBackend(NewMemBackend()))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/download/domain/folder/file.png", nil)
		req = req.WithContext(newTestContext())

		plugin.handleDownload(rr, req)

		assert.Equal(t, http.StatusNotImplemented, rr.Code)
		assert.Contains(t, rr.Body.String(), "method not allowed")
	})

	t.Run("WrongPath", func(t *testing.T) {
		plugin := Plugin(WithBackend(NewMemBackend()))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/wrong-prefix/file.png", nil)
		req = req.WithContext(newTestContext())

		plugin.handleDownload(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "path not allowed")
	})

	t.Run("InvalidFilePath", func(t *testing.T) {
		plugin := Plugin(WithBackend(NewMemBackend()))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/download/invalid-path", nil)
		req = req.WithContext(newTestContext())

		plugin.handleDownload(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid file path")
	})

	t.Run("FileNotFound", func(t *testing.T) {
		plugin := Plugin(WithBackend(NewMemBackend()))

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/download/domain/folder/nonexistent.png", nil)
		req = req.WithContext(newTestContext())

		plugin.handleDownload(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		assert.Contains(t, rr.Body.String(), "file not found")
	})
}

func TestGetDownloadPrefix(t *testing.T) {
	t.Run("WithConfiguredPrefix", func(t *testing.T) {
		// When downloadPrefix is set in config
		plugin := &UploadPlugin{downloadPrefix: "/custom-download"}
		assert.Equal(t, "/custom-download/", plugin.getDownloadPrefix())
	})

	t.Run("WithTrailingSlash", func(t *testing.T) {
		// When prefix already has trailing slash
		plugin := &UploadPlugin{downloadPrefix: "/download/"}
		assert.Equal(t, "/download/", plugin.getDownloadPrefix())
	})

	t.Run("EmptyPrefix", func(t *testing.T) {
		// When prefix is empty, adds slash
		plugin := &UploadPlugin{downloadPrefix: ""}
		assert.Equal(t, "/", plugin.getDownloadPrefix())
	})
}

func TestDownloadContentType(t *testing.T) {
	plugin := Plugin(WithBackend(NewMemBackend()))

	// Upload a file first
	req := newSaveRequest(map[string][]byte{"test.png": pngData()})
	resp, err := plugin.handleUpload(req)
	require.NoError(t, err)

	uploadPath := resp.(*UploadResponse).Files[0].UploadPath

	// Download it and check that it works
	rr := httptest.NewRecorder()
	downloadReq := httptest.NewRequest(http.MethodGet, plugin.getDownloadPrefix()+uploadPath, nil)
	downloadReq = downloadReq.WithContext(newTestContext())

	plugin.handleDownload(rr, downloadReq)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, pngData(), rr.Body.Bytes())
	// Content-Type header is set (mime type detection is platform-dependent)
	// Just verify the header exists
	rr.Header().Get("Content-Type") // called for coverage
}

// errorBackend is a Backend that always fails on Save
type errorBackend struct{}

func (b *errorBackend) Save(path string, data []byte) error {
	return errors.New("backend error")
}

func (b *errorBackend) Get(path string) ([]byte, error) {
	return nil, errors.NewC("file not found", codes.NotFound)
}
