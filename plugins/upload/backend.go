package upload

import (
	"os"
	"path/filepath"

	"github.com/dpup/prefab/errors"
	"google.golang.org/grpc/codes"
)

// Backend is an interface for storing and retrieving files.
type Backend interface {
	// Save saves the file to the backend.
	Save(path string, data []byte) error

	// Get retrieves the file from the backend.
	Get(path string) ([]byte, error)
}

// MemBackend is a Backend that stores files in memory. Intended for testing
// and development. Not threadsafe.
type MemBackend struct {
	files map[string][]byte
}

// NewMemBackend returns a new MemBackend.
func NewMemBackend() Backend {
	return &MemBackend{
		files: make(map[string][]byte),
	}
}

func (b *MemBackend) Save(path string, data []byte) error {
	b.files[path] = data
	return nil
}

func (b *MemBackend) Get(path string) ([]byte, error) {
	data, ok := b.files[path]
	if !ok {
		return nil, errors.NewC("file not found", codes.NotFound)
	}
	return data, nil
}

// FSBackend is a Backend that stores files on the filesystem.
type FSBackend struct {
	rootDir string
}

// NewFSBackend returns a new FSBackend.
func NewFSBackend(rootDir string) Backend {
	return &FSBackend{rootDir: rootDir}
}

func (b *FSBackend) Save(path string, data []byte) error {
	p := filepath.Join(b.rootDir, path)

	// Ensure folder exists.
	err := os.MkdirAll(filepath.Dir(p), 0755)
	if err != nil {
		return errors.WrapPrefix(err, "upload: failed to create directory", 0)
	}

	// Write file.
	err = os.WriteFile(p, data, 0644)
	if err != nil {
		return errors.WrapPrefix(err, "upload: failed to write file", 0)
	}

	return nil
}

func (b *FSBackend) Get(path string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(b.rootDir, path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.NewC("upload: file not found", codes.NotFound)
		}
		return nil, errors.WrapPrefix(err, "upload: failed to read file", 0)
	}
	return data, nil
}
