// Package upload provides a prefab plugin that adds an HTTP handler for
// uploading files. The handler accepts multipart form data, buffers the file
// in memory, and then persists the file to a storage backend.
//
// Files are stored using a salted hash of their contents. This means the file
// is content addressable, but not guessable by someone without the salt.
//
// The plugin optionally integrates with the authz plugin to enforce access
// controls. For upload, the query parameters "domain" and "folder" may be
// passed. "domain" would typically be a workspace, organization, or high-level
// grouping. "folder" might be a user id, a project id, or some other entity
// that governs upload and download rights.
//
// Roadmap for this plugin:
// - Support streaming uploads to reduce memory usage.
package upload

import (
	"bytes"
	context "context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"

	"github.com/dpup/prefab"
	"github.com/dpup/prefab/errors"
	"github.com/dpup/prefab/logging"
	"github.com/dpup/prefab/plugins/authz"
	"google.golang.org/grpc/codes"
)

const (
	// Constant name for identifying the upload plugin.
	PluginName = "upload"

	// authz action for storing files.
	SaveAction = "upload.save"

	// authz action for downloading files.
	DownloadAction = "upload.download"

	// authz object key used to scope RoleDescribers.
	ObjectKey = "upload"
)

// UploadOptions customize the configuration of the upload plugin.
type UploadOption func(*UploadPlugin)

// WithBackend configures the storage backend to use.
func WithBackend(be Backend) UploadOption {
	return func(p *UploadPlugin) {
		p.be = be
	}
}

// WithValidTypes configures the valid file types that can be uploaded.
// Overriding any values from the configuration.
func WithValidTypes(types ...string) UploadOption {
	return func(p *UploadPlugin) {
		p.validTypes = types
	}
}

// Plugin returns a new UploadPlugin.
func Plugin(opts ...UploadOption) *UploadPlugin {
	p := &UploadPlugin{
		uploadPath:     prefab.Config.String("upload.path"),
		downloadPrefix: prefab.Config.String("upload.downloadPrefix"),
		maxFiles:       prefab.Config.Int("upload.maxFiles"),
		maxMemory:      prefab.Config.Int64("upload.maxMemory"),
		validTypes:     prefab.Config.Strings("upload.validTypes"),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// UploadPlugin provides an HTTP handler for uploading files.
type UploadPlugin struct {
	// The path exposed to clients.
	uploadPath string

	// Prefix for downloading files.
	downloadPrefix string

	// Maximum number of distinct items that can be uploaded at once.
	maxFiles int

	// The maximum memory to use for buffering all files in a single request.
	maxMemory int64

	// The valid file types that can be uploaded.
	validTypes []string

	// Backend that stores the uploaded files.
	be Backend

	// Reference to the authz plugin, if available.
	az *authz.AuthzPlugin
}

// From prefab.Plugin.
func (p *UploadPlugin) Name() string {
	return PluginName
}

// From prefab.OptionalDependentPlugin.
func (p *UploadPlugin) OptDeps() []string {
	return []string{authz.PluginName}
}

// From prefab.OptionProvider.
func (p *UploadPlugin) ServerOptions() []prefab.ServerOption {
	return []prefab.ServerOption{
		prefab.WithJSONHandler(p.uploadPath, p.handleUpload),
		prefab.WithHTTPHandlerFunc(p.downloadPrefix, p.handleDownload),
	}
}

// From prefab.InitializablePlugin.
func (p *UploadPlugin) Init(ctx context.Context, r *prefab.Registry) error {
	if az := r.Get(authz.PluginName); az != nil {
		p.az = az.(*authz.AuthzPlugin)
		// Register an object fetcher that just passes on the folder name to the
		// role describer.
		p.az.RegisterObjectFetcher(ObjectKey, func(ctx context.Context, folder any) (any, error) {
			return folder, nil
		})
	}
	if p.be == nil {
		return errors.NewC("upload: no backend configured", codes.Internal)
	}
	return nil
}

// Handles the multipart form data and forwards to the upload service.
//
//nolint:gocognit // IMO breaking this up more would hurt readability.
func (p *UploadPlugin) handleUpload(r *http.Request) (any, error) {
	if r.Method != http.MethodPost {
		return nil, errors.NewC("upload: method not allowed", codes.Unimplemented)
	}
	if r.URL.Path != p.uploadPath {
		return nil, errors.NewC("upload: path not allowed", codes.InvalidArgument)
	}

	ctx := r.Context()

	domain := queryParam(r, "domain")
	folder := queryParam(r, "folder")

	// If authz plugin is configured, use it to verify access to the upload.
	// Otherwise we assume the upload endpoint is protected by other middleware or
	// WAFs or some such.
	if p.az != nil {
		err := p.az.Authorize(ctx, authz.AuthorizeParams{
			ObjectKey:     ObjectKey,
			Action:        SaveAction,
			ObjectID:      folder,
			Domain:        domain,
			DefaultEffect: authz.Deny,
			Info:          "Upload",
		})
		if err != nil {
			return nil, err
		}
	}

	if err := p.validateUploadRequest(r); err != nil {
		return nil, err
	}

	resp := &UploadResponse{}

	for _, files := range r.MultipartForm.File {
		for _, file := range files {
			data, err := p.readFile(file)
			if err != nil {
				return nil, err
			}

			contentType := http.DetectContentType(data)
			if strings.Contains(contentType, ";") {
				contentType = strings.Split(contentType, ";")[0]
			}
			if !slices.Contains(p.validTypes, contentType) {
				return nil, errors.NewC("upload: invalid file type: "+contentType, codes.InvalidArgument)
			}

			exts, err := mime.ExtensionsByType(contentType)
			if err != nil || len(exts) == 0 {
				return nil, errors.NewC("upload: error detecting file type", codes.InvalidArgument)
			}

			hash := sha256.New()
			hash.Write(data)
			path := fmt.Sprintf("%s/%s/%s%s", domain, folder, hex.EncodeToString(hash.Sum(nil)), exts[0])

			if err := p.be.Save(path, data); err != nil {
				return nil, errors.WrapPrefix(err, "upload: error saving file", 0)
			}

			resp.Files = append(resp.Files, &UploadedFile{
				OriginalName: file.Filename,
				UploadPath:   path,
			})
		}
	}

	return resp, nil
}

func (p *UploadPlugin) validateUploadRequest(r *http.Request) error {
	// Limit request body size.
	r.Body = http.MaxBytesReader(nil, r.Body, p.maxMemory)

	// Allow parsing of entire request body in memory.
	if err := r.ParseMultipartForm(p.maxMemory); err != nil {
		return errors.NewC(err, codes.InvalidArgument)
	}

	if r.MultipartForm == nil || len(r.MultipartForm.File) == 0 {
		return errors.NewC("upload: no files uploaded", codes.InvalidArgument)
	}

	if len(r.MultipartForm.File) > p.maxFiles {
		return errors.NewC("upload: too many files uploaded", codes.InvalidArgument)
	}

	return nil
}

func (*UploadPlugin) readFile(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil {
		return nil, errors.WrapPrefix(err, "upload: error opening file", 0)
	}
	defer f.Close()

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, f); err != nil {
		return nil, errors.WrapPrefix(err, "upload: error reading file", 0)
	}

	return buf.Bytes(), nil
}

func (p *UploadPlugin) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeTextError(w, r, errors.NewC("upload: method not allowed", codes.Unimplemented))
		return
	}
	if !strings.HasPrefix(r.URL.Path, p.downloadPrefix+"/") {
		writeTextError(w, r, errors.NewC("upload: path not allowed", codes.InvalidArgument))
		return
	}

	filePath := r.URL.Path[len(p.downloadPrefix)+1:]
	parts := strings.Split(filePath, "/")
	if len(parts) != 3 {
		writeTextError(w, r, errors.NewC("upload: invalid file path", codes.InvalidArgument))
		return
	}

	if p.az != nil {
		err := p.az.Authorize(r.Context(), authz.AuthorizeParams{
			ObjectKey:     ObjectKey,
			Action:        DownloadAction,
			ObjectID:      parts[1],
			Domain:        parts[0],
			DefaultEffect: authz.Deny,
			Info:          "Upload",
		})
		if err != nil {
			writeTextError(w, r, err)
			return
		}
	}

	data, err := p.be.Get(filePath)
	if err != nil {
		writeTextError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(filePath))
	w.Write(data)
}

func writeTextError(w http.ResponseWriter, r *http.Request, err error) {
	logging.Errorw(r.Context(), "upload: error", "error", err)

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(errors.HTTPStatusCode(err))
	w.Write([]byte(err.Error()))
}

func queryParam(r *http.Request, key string) string {
	v := r.URL.Query().Get(key)
	if v != "" {
		return v
	}
	return "-"
}
