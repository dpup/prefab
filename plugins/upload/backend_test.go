package upload

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/dpup/prefab/errors"
)

func TestNewFSBackend(t *testing.T) {
	rootDir := t.TempDir()
	backend := NewFSBackend(rootDir)

	require.NotNil(t, backend)
	assert.IsType(t, &FSBackend{}, backend)
}

func TestFSBackend_Save(t *testing.T) {
	t.Run("SaveFile", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		data := []byte("test file content")
		err := backend.Save("test/file.txt", data)
		require.NoError(t, err)

		// Verify file was written
		savedData, err := os.ReadFile(filepath.Join(rootDir, "test/file.txt"))
		require.NoError(t, err)
		assert.Equal(t, data, savedData)
	})

	t.Run("SaveFileCreatesDirectory", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		data := []byte("nested file")
		err := backend.Save("deep/nested/path/file.txt", data)
		require.NoError(t, err)

		// Verify directory structure was created
		savedData, err := os.ReadFile(filepath.Join(rootDir, "deep/nested/path/file.txt"))
		require.NoError(t, err)
		assert.Equal(t, data, savedData)
	})

	t.Run("SaveOverwritesExisting", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		// Save initial file
		err := backend.Save("file.txt", []byte("original"))
		require.NoError(t, err)

		// Overwrite with new content
		err = backend.Save("file.txt", []byte("updated"))
		require.NoError(t, err)

		savedData, err := os.ReadFile(filepath.Join(rootDir, "file.txt"))
		require.NoError(t, err)
		assert.Equal(t, []byte("updated"), savedData)
	})

	t.Run("SaveBinaryData", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		data := pngData()
		err := backend.Save("image.png", data)
		require.NoError(t, err)

		savedData, err := os.ReadFile(filepath.Join(rootDir, "image.png"))
		require.NoError(t, err)
		assert.Equal(t, data, savedData)
	})

	t.Run("SaveEmptyFile", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		err := backend.Save("empty.txt", []byte{})
		require.NoError(t, err)

		savedData, err := os.ReadFile(filepath.Join(rootDir, "empty.txt"))
		require.NoError(t, err)
		assert.Empty(t, savedData)
	})

	t.Run("SaveInvalidPath", func(t *testing.T) {
		// Use a path that can't be created (e.g., under a file)
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		// Create a regular file
		filePath := filepath.Join(rootDir, "file.txt")
		err := os.WriteFile(filePath, []byte("blocking file"), 0600)
		require.NoError(t, err)

		// Try to save under the file (should fail)
		err = backend.Save("file.txt/nested/file.txt", []byte("data"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create directory")
	})
}

func TestFSBackend_Get(t *testing.T) {
	t.Run("GetExistingFile", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		// Save a file first
		data := []byte("test content")
		err := backend.Save("test.txt", data)
		require.NoError(t, err)

		// Retrieve it
		retrieved, err := backend.Get("test.txt")
		require.NoError(t, err)
		assert.Equal(t, data, retrieved)
	})

	t.Run("GetNonExistentFile", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		data, err := backend.Get("nonexistent.txt")
		assert.Nil(t, data)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, errors.Code(err))
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("GetFromNestedPath", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		data := []byte("nested content")
		err := backend.Save("a/b/c/file.txt", data)
		require.NoError(t, err)

		retrieved, err := backend.Get("a/b/c/file.txt")
		require.NoError(t, err)
		assert.Equal(t, data, retrieved)
	})

	t.Run("GetBinaryFile", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		data := jpegData()
		err := backend.Save("image.jpg", data)
		require.NoError(t, err)

		retrieved, err := backend.Get("image.jpg")
		require.NoError(t, err)
		assert.Equal(t, data, retrieved)
	})

	t.Run("GetEmptyFile", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		err := backend.Save("empty.txt", []byte{})
		require.NoError(t, err)

		retrieved, err := backend.Get("empty.txt")
		require.NoError(t, err)
		assert.Empty(t, retrieved)
	})
}

func TestFSBackend_Integration(t *testing.T) {
	t.Run("SaveAndGetMultipleFiles", func(t *testing.T) {
		rootDir := t.TempDir()
		backend := NewFSBackend(rootDir)

		files := map[string][]byte{
			"file1.txt":       []byte("content 1"),
			"dir/file2.txt":   []byte("content 2"),
			"dir/file3.png":   pngData(),
			"other/file4.jpg": jpegData(),
		}

		// Save all files
		for path, data := range files {
			err := backend.Save(path, data)
			require.NoError(t, err)
		}

		// Retrieve and verify all files
		for path, expectedData := range files {
			retrieved, err := backend.Get(path)
			require.NoError(t, err, "failed to get %s", path)
			assert.Equal(t, expectedData, retrieved, "data mismatch for %s", path)
		}
	})
}

func TestMemBackend_Get_NotFound(t *testing.T) {
	backend := NewMemBackend()

	data, err := backend.Get("nonexistent.txt")
	assert.Nil(t, data)
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, errors.Code(err))
}
