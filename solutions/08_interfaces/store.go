// Package interfaces is the Go refresher on interfaces: implicit satisfaction,
// consumer-side contracts, optional capabilities, decorators, the nil-interface
// trap, and interfaces versus generics.
package interfaces

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
)

// ErrNotFound is returned (wrapped) when a key does not exist.
var ErrNotFound = errors.New("blob not found")

// ErrInvalidKey is returned (wrapped) for keys that are not safe relative paths.
var ErrInvalidKey = errors.New("invalid blob key")

// Compile-time checks that the stores satisfy the contracts they advertise.
var (
	_ BlobStore  = (*MemoryStore)(nil)
	_ BlobGetter = (*MemoryStore)(nil)
	_ Lister     = (*MemoryStore)(nil)
	_ Sizer      = (*MemoryStore)(nil)
	_ BlobStore  = (*FileStore)(nil)
	_ Lister     = (*FileStore)(nil)
	_ io.Closer  = (*FileStore)(nil)
)

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
	for i, key := range keys {
		data, err := src.Get(ctx, key)
		if err != nil {
			return i, fmt.Errorf("transfer %q: %w", key, err)
		}
		if err := dst.Put(ctx, key, data); err != nil {
			return i, fmt.Errorf("transfer %q: %w", key, err)
		}
	}
	return len(keys), nil
}

// validateKey enforces the rules shared by every store: a non-empty,
// slash-separated relative path with no empty, "." or ".." segments and no
// backslashes.
func validateKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, `\`) {
		return fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("%w: %q", ErrInvalidKey, key)
		}
	}
	return nil
}

// checkKey honours cancellation and validates the key before an operation.
func checkKey(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateKey(key)
}

// MemoryStore keeps blobs in memory. Its zero value is ready to use and it is
// safe for concurrent use. Bytes are copied on the way in and out, so callers
// never share memory with the store.
type MemoryStore struct {
	mu    sync.RWMutex
	blobs map[string][]byte
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

// Get implements BlobStore.
func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := checkKey(ctx, key); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.blobs[key]
	if !ok {
		return nil, fmt.Errorf("get %q: %w", key, ErrNotFound)
	}
	return bytes.Clone(data), nil
}

// Put implements BlobStore.
func (s *MemoryStore) Put(ctx context.Context, key string, data []byte) error {
	if err := checkKey(ctx, key); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.blobs == nil {
		s.blobs = make(map[string][]byte)
	}
	s.blobs[key] = bytes.Clone(data)
	return nil
}

// Delete implements BlobStore. Deleting a missing key wraps ErrNotFound.
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	if err := checkKey(ctx, key); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.blobs[key]; !ok {
		return fmt.Errorf("delete %q: %w", key, ErrNotFound)
	}
	delete(s.blobs, key)
	return nil
}

// List implements Lister.
func (s *MemoryStore) List(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var keys []string
	for k := range s.blobs {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys, nil
}

// Size implements Sizer.
func (s *MemoryStore) Size(ctx context.Context, key string) (int64, error) {
	if err := checkKey(ctx, key); err != nil {
		return 0, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.blobs[key]
	if !ok {
		return 0, fmt.Errorf("size %q: %w", key, ErrNotFound)
	}
	return int64(len(data)), nil
}

// FileStore keeps each blob as a file below a root directory. Keys are relative
// slash-separated paths; os.Root confines every operation to the directory, so
// nothing can escape it even through symlinks.
type FileStore struct {
	root *os.Root
}

// NewFileStore opens (creating if needed) a FileStore rooted at dir.
func NewFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open store root: %w", err)
	}
	return &FileStore{root: root}, nil
}

// Get implements BlobStore.
func (s *FileStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := checkKey(ctx, key); err != nil {
		return nil, err
	}
	data, err := s.root.ReadFile(key)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("get %q: %w", key, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get %q: %w", key, err)
	}
	return data, nil
}

// Put implements BlobStore, creating intermediate directories as needed.
func (s *FileStore) Put(ctx context.Context, key string, data []byte) error {
	if err := checkKey(ctx, key); err != nil {
		return err
	}
	if dir := path.Dir(key); dir != "." {
		if err := s.root.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("put %q: %w", key, err)
		}
	}
	if err := s.root.WriteFile(key, data, 0o644); err != nil {
		return fmt.Errorf("put %q: %w", key, err)
	}
	return nil
}

// Delete implements BlobStore. Deleting a missing key wraps ErrNotFound.
func (s *FileStore) Delete(ctx context.Context, key string) error {
	if err := checkKey(ctx, key); err != nil {
		return err
	}
	err := s.root.Remove(key)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete %q: %w", key, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

// List implements Lister by walking the root.
func (s *FileStore) List(ctx context.Context, prefix string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var keys []string
	err := fs.WalkDir(s.root.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasPrefix(name, prefix) {
			keys = append(keys, name)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list %q: %w", prefix, err)
	}
	slices.Sort(keys)
	return keys, nil
}

// Close releases the root directory handle.
func (s *FileStore) Close() error { return s.root.Close() }

// ListKeys lists keys with prefix when s implements Lister; otherwise it returns
// an error wrapping errors.ErrUnsupported.
func ListKeys(ctx context.Context, s BlobStore, prefix string) ([]string, error) {
	l, ok := s.(Lister)
	if !ok {
		return nil, fmt.Errorf("list keys: %w", errors.ErrUnsupported)
	}
	return l.List(ctx, prefix)
}

// SizeOf returns the size of key using Sizer when available, falling back to
// reading the blob otherwise.
func SizeOf(ctx context.Context, s BlobStore, key string) (int64, error) {
	if sz, ok := s.(Sizer); ok {
		return sz.Size(ctx, key)
	}
	data, err := s.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	return int64(len(data)), nil
}

// Capabilities reports which optional interfaces s implements, as the names
// "list", "size" and "close", in that order.
func Capabilities(s BlobStore) []string {
	var caps []string
	if _, ok := s.(Lister); ok {
		caps = append(caps, "list")
	}
	if _, ok := s.(Sizer); ok {
		caps = append(caps, "size")
	}
	if _, ok := s.(io.Closer); ok {
		caps = append(caps, "close")
	}
	return caps
}

// CloseStore calls Close when s implements io.Closer and is a no-op otherwise.
func CloseStore(s BlobStore) error {
	if c, ok := s.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
