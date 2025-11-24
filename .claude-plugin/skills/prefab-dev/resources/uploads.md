# File Uploads

The upload plugin provides HTTP endpoints for uploading and downloading files with optional authorization.

## Setup

```go
import (
    "github.com/dpup/prefab"
    "github.com/dpup/prefab/plugins/upload"
)

s := prefab.New(
    prefab.WithPlugin(upload.Plugin(
        upload.WithBackend(myBackend),
    )),
)
```

## Configuration

Via YAML:
```yaml
upload:
  path: /upload
  downloadPrefix: /download
  maxFiles: 10
  maxMemory: 4194304  # 4MB
  validTypes:
    - image/jpeg
    - image/png
    - image/gif
    - image/webp
```

Via functional options:
```go
upload.Plugin(
    upload.WithBackend(backend),
    upload.WithValidTypes("image/jpeg", "image/png", "application/pdf"),
)
```

## Storage Backends

Implement the `Backend` interface:

```go
type Backend interface {
    Save(path string, data []byte) error
    Get(path string) ([]byte, error)
}
```

### File System Backend

```go
type fsBackend struct {
    root string
}

func (b *fsBackend) Save(path string, data []byte) error {
    fullPath := filepath.Join(b.root, path)
    if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
        return err
    }
    return os.WriteFile(fullPath, data, 0644)
}

func (b *fsBackend) Get(path string) ([]byte, error) {
    return os.ReadFile(filepath.Join(b.root, path))
}
```

## Client Usage

### Upload

```javascript
const formData = new FormData();
formData.append('file', fileInput.files[0]);

const response = await fetch('/upload?domain=workspace1&folder=user123', {
    method: 'POST',
    body: formData,
});

const result = await response.json();
// { files: [{ originalName: "photo.jpg", uploadPath: "workspace1/user123/abc123.jpg" }] }
```

### Download

```javascript
const downloadUrl = `/download/${uploadPath}`;
```

## Authorization Integration

The upload plugin integrates with authz for access control:

```go
s := prefab.New(
    prefab.WithPlugin(auth.Plugin()),
    prefab.WithPlugin(authz.Plugin(
        // Allow users to upload/download in their folder
        authz.WithPolicy(authz.Allow, "user", authz.Action("upload.save")),
        authz.WithPolicy(authz.Allow, "user", authz.Action("upload.download")),

        // Register role describer for uploads
        authz.WithRoleDescriber("upload", authz.Compose(
            authz.MembershipRoles(
                func(folder any) string { return folder.(string) }, // folder is the object
                func(ctx context.Context, folder, userID string) ([]authz.Role, error) {
                    // Return "user" role if userID matches folder
                    if folder == userID {
                        return []authz.Role{"user"}, nil
                    }
                    return nil, nil
                },
            ),
        )),
    )),
    prefab.WithPlugin(upload.Plugin(
        upload.WithBackend(myBackend),
    )),
)
```

## File Naming

Files are stored using a salted SHA-256 hash of their contents:
- Content-addressable (same content = same path)
- Not guessable without the salt
- Path format: `{domain}/{folder}/{hash}{extension}`

## Multiple File Upload

```javascript
const formData = new FormData();
formData.append('files', file1);
formData.append('files', file2);
formData.append('files', file3);

const response = await fetch('/upload?domain=workspace&folder=project', {
    method: 'POST',
    body: formData,
});
```

Maximum files per request is configurable via `upload.maxFiles`.
