package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Storage defines an abstract interface for file storage operations.
type Storage interface {
	// Upload stores the content from reader under the given key and returns
	// the canonical URL or path to the stored object.
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error)

	// Download retrieves the content stored under the given key.
	// The caller is responsible for closing the returned ReadCloser.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object stored under the given key.
	Delete(ctx context.Context, key string) error

	// GetURL returns a URL (or path) that can be used to access the stored object.
	GetURL(ctx context.Context, key string) (string, error)
}

// LocalStorage implements Storage using the local filesystem.
type LocalStorage struct {
	basePath string
}

// NewLocalStorage creates a LocalStorage rooted at basePath.
// The base directory is created if it does not exist.
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("storage: resolve base path: %w", err)
	}

	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: create base directory: %w", err)
	}

	return &LocalStorage{basePath: abs}, nil
}

// Upload writes the content from reader to the local filesystem under key.
// Intermediate directories are created as needed.
// The contentType parameter is ignored for local storage.
func (s *LocalStorage) Upload(_ context.Context, key string, reader io.Reader, _ string) (string, error) {
	fullPath := s.resolve(key)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("storage: create directory %s: %w", dir, err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("storage: create file %s: %w", fullPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		// Best-effort cleanup on write failure.
		_ = os.Remove(fullPath)
		return "", fmt.Errorf("storage: write file %s: %w", fullPath, err)
	}

	return fullPath, nil
}

// Download opens the file at key for reading.
// The caller must close the returned ReadCloser.
func (s *LocalStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	fullPath := s.resolve(key)

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("storage: open file %s: %w", fullPath, err)
	}
	return f, nil
}

// Delete removes the file at key from the local filesystem.
func (s *LocalStorage) Delete(_ context.Context, key string) error {
	fullPath := s.resolve(key)

	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: delete file %s: %w", fullPath, err)
	}
	return nil
}

// GetURL returns the absolute filesystem path for the given key.
func (s *LocalStorage) GetURL(_ context.Context, key string) (string, error) {
	fullPath := s.resolve(key)

	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("storage: stat file %s: %w", fullPath, err)
	}
	return fullPath, nil
}

// resolve joins the base path with the key, cleaning the result to prevent
// directory traversal attacks.
func (s *LocalStorage) resolve(key string) string {
	// Clean the key to remove any ".." or other traversal components.
	clean := filepath.Clean("/" + key)
	return filepath.Join(s.basePath, clean)
}
