package upload

import (
	"bytes"
	context "context"
	"encoding/base64"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/auth"
	"github.com/dpup/prefab/plugins/authz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

const (
	pngBase64  = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAAXNSR0IArs4c6QAAAA1JREFUGFdj+L+U4T8ABu8CpCYJ1DQAAAAASUVORK5CYII="
	jpegBase64 = "/9j/2wBDAP//////////////////////////////////////////////////////////////////////////////////////wAALCAABAAEBAREA/8QAFAABAAAAAAAAAAAAAAAAAAAAA//EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAD8AN//Z"
)

func TestUpload(t *testing.T) {
	be := NewMemBackend()
	plugin := Plugin(WithBackend(be))

	req := newSaveRequest(map[string][]byte{
		"test.png": pngData(),
		"test.jpg": jpegData(),
	})

	resp, err := plugin.handleUpload(req)
	require.NoError(t, err)

	expectedResponse := []*UploadedFile{
		{
			OriginalName: "test.png",
			UploadPath:   "-/-/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png",
		},
		{
			OriginalName: "test.jpg",
			UploadPath:   "-/-/b9380faf5b9fde41b0625e88f42900c3ee955b36f91c1f3d59be5989a8f2b133.jpe",
		},
	}
	assert.ElementsMatch(t, expectedResponse, resp.(*UploadResponse).Files)

	b, err := be.Get("-/-/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png")
	require.NoError(t, err)
	assert.Equal(t, pngData(), b)
}

func TestUploadWithDomainAndFolder(t *testing.T) {
	be := NewMemBackend()
	plugin := Plugin(WithBackend(be))

	req := newSaveRequest(map[string][]byte{
		"test.png": pngData(),
	})
	req.URL.RawQuery = "domain=github.com&folder=dpup"

	resp, err := plugin.handleUpload(req)
	require.NoError(t, err)

	expectedResponse := []*UploadedFile{
		{
			OriginalName: "test.png",
			UploadPath:   "github.com/dpup/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png",
		},
	}

	assert.ElementsMatch(t, expectedResponse, resp.(*UploadResponse).Files)

	b, err := be.Get("github.com/dpup/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png")
	require.NoError(t, err)
	assert.Equal(t, pngData(), b)
}

func TestUploadACL_Success(t *testing.T) {
	be := NewMemBackend()
	plugin := Plugin(WithBackend(be))

	// Set up a minimal auth plugin that allows dpup to upload files to the dpup
	// folder.
	az := authz.Plugin()
	az.RegisterRoleDescriber(ObjectKey, func(ctx context.Context, subject auth.Identity, object any, domain authz.Domain) ([]authz.Role, error) {
		if object.(string) == "dpup" { // dpup would usually come from the subject.
			return []authz.Role{"owner"}, nil
		}
		return []authz.Role{}, nil
	})
	az.DefinePolicy(authz.Allow, "owner", SaveAction)

	r := &prefab.Registry{}
	r.Register(auth.Plugin()) // To satisfy authz deps, not exercised.
	r.Register(az)
	r.Register(plugin)

	require.NoError(t, r.Init(context.Background()))

	req := newSaveRequest(map[string][]byte{
		"test.png": pngData(),
	})
	req.URL.RawQuery = "domain=github.com&folder=dpup"

	resp, err := plugin.handleUpload(req)
	require.NoError(t, err)

	expectedResponse := []*UploadedFile{
		{
			OriginalName: "test.png",
			UploadPath:   "github.com/dpup/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png",
		},
	}

	assert.ElementsMatch(t, expectedResponse, resp.(*UploadResponse).Files)

	b, err := be.Get("github.com/dpup/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png")
	require.NoError(t, err)
	assert.Equal(t, pngData(), b)
}

func TestUploadACL_Failure(t *testing.T) {
	be := NewMemBackend()
	plugin := Plugin(WithBackend(be))

	az := authz.Plugin()
	az.RegisterRoleDescriber(ObjectKey, func(ctx context.Context, subject auth.Identity, object any, domain authz.Domain) ([]authz.Role, error) {
		if object.(string) == "haxor" {
			return []authz.Role{"owner"}, nil
		}
		return []authz.Role{}, nil
	})
	az.DefinePolicy(authz.Allow, "owner", SaveAction)

	r := &prefab.Registry{}
	r.Register(auth.Plugin()) // To satisfy authz deps, not exercised.
	r.Register(az)
	r.Register(plugin)

	require.NoError(t, r.Init(context.Background()))

	req := newSaveRequest(map[string][]byte{
		"test.png": pngData(),
	})
	req.URL.RawQuery = "domain=github.com&folder=dpup"

	_, err := plugin.handleUpload(req)
	require.Error(t, err)

	_, err = be.Get("github.com/dpup/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png")
	assert.Error(t, err)
}
func TestUploadInvalidType(t *testing.T) {
	be := NewMemBackend()
	plugin := Plugin(WithBackend(be))

	req := newSaveRequest(map[string][]byte{"test.txt": []byte("hello")})

	_, err := plugin.handleUpload(req)
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, errors.Code(err), "Handler returned wrong status code")
}

func TestDownloadACL_Success(t *testing.T) {
	plugin := Plugin(WithBackend(NewMemBackend()))

	userID := "dpup"

	// Set up ACL that allows anyone to view.
	az := authz.Plugin()
	az.RegisterRoleDescriber(ObjectKey, func(ctx context.Context, subject auth.Identity, object any, domain authz.Domain) ([]authz.Role, error) {
		if object.(string) == userID {
			return []authz.Role{"owner"}, nil
		}
		return []authz.Role{"viewer"}, nil
	})
	az.SetRoleHierarchy("owner", "viewer")
	az.DefinePolicy(authz.Allow, "owner", SaveAction)
	az.DefinePolicy(authz.Allow, "viewer", DownloadAction)

	r := &prefab.Registry{}
	r.Register(auth.Plugin()) // To satisfy authz deps, not exercised.
	r.Register(az)
	r.Register(plugin)

	require.NoError(t, r.Init(context.Background()))

	uploadReq := newSaveRequest(map[string][]byte{"test.png": pngData()})
	uploadReq.URL.RawQuery = "domain=github.com&folder=dpup"

	_, err := plugin.handleUpload(uploadReq)
	require.NoError(t, err)

	// Download as owner.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/download/github.com/dpup/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png", nil)
	req = req.WithContext(logging.EnsureLogger(context.Background()))

	plugin.handleDownload(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, pngData(), rr.Body.Bytes())

	// Download as viewer.
	userID = "haxor"
	rr = httptest.NewRecorder()
	plugin.handleDownload(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, pngData(), rr.Body.Bytes())
}

func TestDownloadACL_Failure(t *testing.T) {
	plugin := Plugin(WithBackend(NewMemBackend()))

	userID := "dpup"

	// Set up ACL that only allows the owner to view it.
	az := authz.Plugin()
	az.RegisterRoleDescriber(ObjectKey, func(ctx context.Context, subject auth.Identity, object any, domain authz.Domain) ([]authz.Role, error) {
		if object.(string) == userID {
			return []authz.Role{"owner"}, nil
		}
		return []authz.Role{"viewer"}, nil
	})
	az.DefinePolicy(authz.Allow, "owner", SaveAction)
	az.DefinePolicy(authz.Allow, "owner", DownloadAction)

	r := &prefab.Registry{}
	r.Register(auth.Plugin()) // To satisfy authz deps, not exercised.
	r.Register(az)
	r.Register(plugin)

	require.NoError(t, r.Init(context.Background()))

	uploadReq := newSaveRequest(map[string][]byte{"test.png": pngData()})
	uploadReq.URL.RawQuery = "domain=github.com&folder=dpup"

	_, err := plugin.handleUpload(uploadReq)
	require.NoError(t, err)

	// Download as owner.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/download/github.com/dpup/c57044605bd03a0ebd560736e76d5499ae9596a164db29b88fe096432d23d187.png", nil)
	req = req.WithContext(logging.EnsureLogger(context.Background()))

	plugin.handleDownload(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, pngData(), rr.Body.Bytes())

	// Download as other user.
	userID = "haxor"
	rr = httptest.NewRecorder()
	plugin.handleDownload(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, "the requested action requires authentication", rr.Body.String())
}

func newSaveRequest(files map[string][]byte) *http.Request {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for fileName, data := range files {
		part, err := writer.CreateFormFile("file", fileName)
		if err != nil {
			panic(err)
		}

		_, err = io.Copy(part, bytes.NewReader(data))
		if err != nil {
			panic(err)
		}
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req.WithContext(logging.EnsureLogger(context.Background()))
}

func jpegData() []byte {
	d, err := base64.StdEncoding.DecodeString(jpegBase64)
	if err != nil {
		panic(err)
	}
	return d
}

func pngData() []byte {
	d, err := base64.StdEncoding.DecodeString(pngBase64)
	if err != nil {
		panic(err)
	}
	return d
}
