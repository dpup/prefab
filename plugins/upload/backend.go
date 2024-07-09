package upload

import (
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
