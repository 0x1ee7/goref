// Package interfaces is the Go refresher on interfaces: implicit satisfaction,
// consumer-side contracts, optional capabilities, decorators, the nil-interface
// trap, and interfaces versus generics. The exported surface is fixed by the
// tests; the bodies are yours. README.md is the spec.
package interfaces

import (
	"context"
	"errors"
)

// ErrNotFound is returned (wrapped) when a key does not exist.
var ErrNotFound = errors.New("blob not found")

// ErrInvalidKey is returned (wrapped) for keys that are not safe relative paths.
var ErrInvalidKey = errors.New("invalid blob key")

// BlobStore is the storage contract the application depends on. Keep it small:
// anything beyond it is an optional capability checked with a type assertion.
type BlobStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key string, data []byte) error
	Delete(ctx context.Context, key string) error
}

// BlobGetter is the consumer-side view Transfer needs: read-only access.
type BlobGetter interface {
	Get(ctx context.Context, key string) ([]byte, error)
}

// Lister is an optional capability: enumerate keys with a prefix, sorted.
type Lister interface {
	List(ctx context.Context, prefix string) ([]string, error)
}

// Sizer is an optional capability: report a blob's size without a full read.
type Sizer interface {
	Size(ctx context.Context, key string) (int64, error)
}

// Transfer copies keys from src to dst in the given order and returns how many
// were copied. It stops at the first failure and wraps that error with the key.
func Transfer(ctx context.Context, dst BlobStore, src BlobGetter, keys ...string) (int, error) {
	return 0, nil
}

// MemoryStore keeps blobs in memory. Its zero value is ready to use.
type MemoryStore struct{}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

// Get implements BlobStore.
func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) { return nil, nil }

// Put implements BlobStore.
func (s *MemoryStore) Put(ctx context.Context, key string, data []byte) error { return nil }

// Delete implements BlobStore.
func (s *MemoryStore) Delete(ctx context.Context, key string) error { return nil }

// List implements Lister (Part 2).
func (s *MemoryStore) List(ctx context.Context, prefix string) ([]string, error) { return nil, nil }

// Size implements Sizer (Part 2).
func (s *MemoryStore) Size(ctx context.Context, key string) (int64, error) { return 0, nil }

// FileStore keeps each blob as a file below a root directory. Keys are relative
// slash-separated paths; nothing may escape the root.
type FileStore struct{}

// NewFileStore opens (creating if needed) a FileStore rooted at dir.
func NewFileStore(dir string) (*FileStore, error) {
	return nil, errors.New("not implemented")
}

// Get implements BlobStore.
func (s *FileStore) Get(ctx context.Context, key string) ([]byte, error) { return nil, nil }

// Put implements BlobStore.
func (s *FileStore) Put(ctx context.Context, key string, data []byte) error { return nil }

// Delete implements BlobStore.
func (s *FileStore) Delete(ctx context.Context, key string) error { return nil }

// List implements Lister (Part 2).
func (s *FileStore) List(ctx context.Context, prefix string) ([]string, error) { return nil, nil }

// Close releases the root directory handle (Part 1).
func (s *FileStore) Close() error { return nil }

// ListKeys lists keys with prefix when s implements Lister; otherwise it returns
// an error wrapping errors.ErrUnsupported (Part 2).
func ListKeys(ctx context.Context, s BlobStore, prefix string) ([]string, error) {
	return nil, nil
}

// SizeOf returns the size of key using Sizer when available, falling back to
// reading the blob otherwise (Part 2).
func SizeOf(ctx context.Context, s BlobStore, key string) (int64, error) {
	return 0, nil
}

// Capabilities reports which optional interfaces s implements, as the names
// "list", "size" and "close", in that order (Part 2).
func Capabilities(s BlobStore) []string { return nil }

// CloseStore calls Close when s implements io.Closer and is a no-op otherwise
// (Part 2).
func CloseStore(s BlobStore) error { return nil }
