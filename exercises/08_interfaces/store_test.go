package interfaces_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"goref/exercises/08_interfaces"
)

// Compile-time contract checks: these lines fail to build if a method is missing.
var (
	_ interfaces.BlobStore  = (*interfaces.MemoryStore)(nil)
	_ interfaces.BlobStore  = (*interfaces.FileStore)(nil)
	_ interfaces.BlobGetter = (*interfaces.MemoryStore)(nil)
	_ interfaces.Lister     = (*interfaces.MemoryStore)(nil)
	_ interfaces.Sizer      = (*interfaces.MemoryStore)(nil)
	_ interfaces.Lister     = (*interfaces.FileStore)(nil)
	_ io.Closer             = (*interfaces.FileStore)(nil)
)

// plain embeds only the BlobStore interface, so it hides every optional
// capability of the store it wraps.
type plain struct{ interfaces.BlobStore }

// closingStore adds io.Closer to any store.
type closingStore struct {
	interfaces.BlobStore
	closes int
}

func (c *closingStore) Close() error { c.closes++; return nil }

// getterFunc adapts a function to the read-only BlobGetter contract.
type getterFunc func(ctx context.Context, key string) ([]byte, error)

func (f getterFunc) Get(ctx context.Context, key string) ([]byte, error) { return f(ctx, key) }

func mustPut(t *testing.T, s interfaces.BlobStore, key, data string) {
	t.Helper()
	if err := s.Put(t.Context(), key, []byte(data)); err != nil {
		t.Fatalf("Put(%q): %v", key, err)
	}
}

func mustGet(t *testing.T, s interfaces.BlobStore, key string) string {
	t.Helper()
	got, err := s.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("Get(%q): %v", key, err)
	}
	return string(got)
}

func newFileStore(t *testing.T, dir string) *interfaces.FileStore {
	t.Helper()
	s, err := interfaces.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore(%q): %v", dir, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestPart1MemoryStore(t *testing.T) {
	t.Run("put then get round-trips", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		mustPut(t, s, "a/b.txt", "hello")
		if got := mustGet(t, s, "a/b.txt"); got != "hello" {
			t.Errorf("Get = %q, want %q", got, "hello")
		}
	})

	t.Run("zero value is ready to use", func(t *testing.T) {
		var s interfaces.MemoryStore
		mustPut(t, &s, "k", "v")
		if got := mustGet(t, &s, "k"); got != "v" {
			t.Errorf("Get = %q, want %q", got, "v")
		}
	})

	t.Run("overwrite replaces the value", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		mustPut(t, s, "k", "v1")
		mustPut(t, s, "k", "v2")
		if got := mustGet(t, s, "k"); got != "v2" {
			t.Errorf("Get = %q, want %q", got, "v2")
		}
	})

	t.Run("missing key wraps ErrNotFound", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		_, err := s.Get(t.Context(), "nope")
		if !errors.Is(err, interfaces.ErrNotFound) {
			t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
		}
	})

	t.Run("bytes are copied in and out", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		in := []byte("abc")
		if err := s.Put(t.Context(), "k", in); err != nil {
			t.Fatal(err)
		}
		in[0] = 'X' // caller mutates its buffer after Put
		out, err := s.Get(t.Context(), "k")
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "abc" {
			t.Fatalf("stored value follows caller's buffer: got %q, want %q", out, "abc")
		}
		out[1] = 'Y' // caller mutates the returned slice
		if again := mustGet(t, s, "k"); again != "abc" {
			t.Errorf("stored value follows returned slice: got %q, want %q", again, "abc")
		}
	})

	t.Run("delete removes the key; second delete is ErrNotFound", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		mustPut(t, s, "k", "v")
		if err := s.Delete(t.Context(), "k"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := s.Get(t.Context(), "k"); !errors.Is(err, interfaces.ErrNotFound) {
			t.Errorf("Get after Delete err = %v, want ErrNotFound", err)
		}
		if err := s.Delete(t.Context(), "k"); !errors.Is(err, interfaces.ErrNotFound) {
			t.Errorf("second Delete err = %v, want ErrNotFound", err)
		}
	})

	t.Run("invalid keys are rejected", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		mustPut(t, s, "ok/key", "x")
		for _, key := range []string{"", "/abs", "../escape", "a//b", "a/./b", "a/../b", `a\b`} {
			err := s.Put(t.Context(), key, []byte("x"))
			if !errors.Is(err, interfaces.ErrInvalidKey) {
				t.Errorf("Put(%q) err = %v, want ErrInvalidKey", key, err)
			}
		}
	})

	t.Run("canceled context is honoured", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		mustPut(t, s, "k", "v")
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := s.Put(ctx, "k", []byte("w")); !errors.Is(err, context.Canceled) {
			t.Fatalf("Put with canceled ctx err = %v, want context.Canceled", err)
		}
		if got := mustGet(t, s, "k"); got != "v" {
			t.Errorf("value after rejected Put = %q, want %q", got, "v")
		}
	})
}

func TestPart1FileStore(t *testing.T) {
	t.Run("writes nested keys as files under the root", func(t *testing.T) {
		dir := t.TempDir()
		s := newFileStore(t, dir)
		mustPut(t, s, "reports/2026/q1.txt", "revenue")
		raw, err := os.ReadFile(filepath.Join(dir, "reports", "2026", "q1.txt"))
		if err != nil {
			t.Fatalf("on-disk file missing: %v", err)
		}
		if string(raw) != "revenue" {
			t.Errorf("on-disk content = %q, want %q", raw, "revenue")
		}
		if got := mustGet(t, s, "reports/2026/q1.txt"); got != "revenue" {
			t.Errorf("Get = %q, want %q", got, "revenue")
		}
	})

	t.Run("missing key wraps ErrNotFound", func(t *testing.T) {
		s := newFileStore(t, t.TempDir())
		if _, err := s.Get(t.Context(), "nope.txt"); !errors.Is(err, interfaces.ErrNotFound) {
			t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
		}
	})

	t.Run("delete twice: second is ErrNotFound", func(t *testing.T) {
		s := newFileStore(t, t.TempDir())
		mustPut(t, s, "k.txt", "v")
		if err := s.Delete(t.Context(), "k.txt"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if err := s.Delete(t.Context(), "k.txt"); !errors.Is(err, interfaces.ErrNotFound) {
			t.Errorf("second Delete err = %v, want ErrNotFound", err)
		}
	})

	t.Run("keys cannot escape the root", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "store")
		s := newFileStore(t, root)
		mustPut(t, s, "ok.txt", "x") // sanity: valid keys work
		err := s.Put(t.Context(), "../escaped.txt", []byte("boom"))
		if !errors.Is(err, interfaces.ErrInvalidKey) {
			t.Fatalf("Put(../escaped.txt) err = %v, want ErrInvalidKey", err)
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escaped.txt")); !os.IsNotExist(err) {
			t.Errorf("file escaped the root: stat err = %v", err)
		}
	})

	t.Run("NewFileStore creates a missing directory", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "a", "b")
		newFileStore(t, dir)
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			t.Fatalf("directory %s not created: %v", dir, err)
		}
	})

	t.Run("close releases the handle; reopening sees earlier writes", func(t *testing.T) {
		dir := t.TempDir()
		s, err := interfaces.NewFileStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		mustPut(t, s, "k", "v")
		if err := s.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		reopened := newFileStore(t, dir)
		if got := mustGet(t, reopened, "k"); got != "v" {
			t.Errorf("Get after reopen = %q, want %q", got, "v")
		}
	})
}

func TestPart1Transfer(t *testing.T) {
	source := map[string]string{"a": "1", "b": "2"}
	src := getterFunc(func(_ context.Context, key string) ([]byte, error) {
		v, ok := source[key]
		if !ok {
			return nil, fmt.Errorf("source %q: %w", key, interfaces.ErrNotFound)
		}
		return []byte(v), nil
	})

	t.Run("copies every key in order", func(t *testing.T) {
		dst := interfaces.NewMemoryStore()
		n, err := interfaces.Transfer(t.Context(), dst, src, "a", "b")
		if err != nil {
			t.Fatalf("Transfer: %v", err)
		}
		if n != 2 {
			t.Errorf("copied = %d, want 2", n)
		}
		if got := mustGet(t, dst, "b"); got != "2" {
			t.Errorf("dst[b] = %q, want %q", got, "2")
		}
	})

	t.Run("stops at the first failing key", func(t *testing.T) {
		dst := interfaces.NewMemoryStore()
		n, err := interfaces.Transfer(t.Context(), dst, src, "a", "missing", "b")
		if !errors.Is(err, interfaces.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if n != 1 {
			t.Errorf("copied = %d, want 1", n)
		}
		if _, err := dst.Get(t.Context(), "b"); !errors.Is(err, interfaces.ErrNotFound) {
			t.Errorf("key after failure was copied: err = %v", err)
		}
	})

	t.Run("any BlobStore can be the source", func(t *testing.T) {
		from := interfaces.NewMemoryStore()
		mustPut(t, from, "x", "payload")
		dst := interfaces.NewMemoryStore()
		n, err := interfaces.Transfer(t.Context(), dst, from, "x")
		if err != nil || n != 1 {
			t.Fatalf("Transfer = %d, %v; want 1, nil", n, err)
		}
		if got := mustGet(t, dst, "x"); got != "payload" {
			t.Errorf("dst[x] = %q, want %q", got, "payload")
		}
	})
}

func TestPart2ListKeys(t *testing.T) {
	t.Run("memory store lists matching keys sorted", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		for _, k := range []string{"img/b.png", "img/a.png", "doc/x.txt"} {
			mustPut(t, s, k, "x")
		}
		got, err := interfaces.ListKeys(t.Context(), s, "img/")
		if err != nil {
			t.Fatalf("ListKeys: %v", err)
		}
		if diff := cmp.Diff([]string{"img/a.png", "img/b.png"}, got); diff != "" {
			t.Errorf("ListKeys mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("file store lists nested keys with forward slashes", func(t *testing.T) {
		s := newFileStore(t, t.TempDir())
		for _, k := range []string{"img/a.png", "img/sub/c.png", "doc/x.txt"} {
			mustPut(t, s, k, "x")
		}
		got, err := interfaces.ListKeys(t.Context(), s, "img/")
		if err != nil {
			t.Fatalf("ListKeys: %v", err)
		}
		if diff := cmp.Diff([]string{"img/a.png", "img/sub/c.png"}, got); diff != "" {
			t.Errorf("ListKeys mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("store without Lister reports ErrUnsupported", func(t *testing.T) {
		mem := interfaces.NewMemoryStore()
		mustPut(t, mem, "k", "v")
		if got, err := interfaces.ListKeys(t.Context(), mem, ""); err != nil || len(got) != 1 {
			t.Fatalf("sanity: ListKeys on MemoryStore = %v, %v; want one key", got, err)
		}
		_, err := interfaces.ListKeys(t.Context(), plain{mem}, "")
		if !errors.Is(err, errors.ErrUnsupported) {
			t.Errorf("ListKeys on non-Lister err = %v, want errors.ErrUnsupported", err)
		}
	})
}

func TestPart2SizeOf(t *testing.T) {
	t.Run("uses Sizer when the store has one", func(t *testing.T) {
		s := interfaces.NewMemoryStore()
		mustPut(t, s, "k", "hello")
		got, err := interfaces.SizeOf(t.Context(), s, "k")
		if err != nil || got != 5 {
			t.Errorf("SizeOf = %d, %v; want 5, nil", got, err)
		}
	})

	t.Run("falls back to reading the blob", func(t *testing.T) {
		mem := interfaces.NewMemoryStore()
		mustPut(t, mem, "k", "hello")
		got, err := interfaces.SizeOf(t.Context(), plain{mem}, "k")
		if err != nil || got != 5 {
			t.Errorf("SizeOf fallback = %d, %v; want 5, nil", got, err)
		}
	})

	t.Run("fallback preserves ErrNotFound", func(t *testing.T) {
		mem := interfaces.NewMemoryStore()
		mustPut(t, mem, "present", "x")
		_, err := interfaces.SizeOf(t.Context(), plain{mem}, "missing")
		if !errors.Is(err, interfaces.ErrNotFound) {
			t.Errorf("SizeOf(missing) err = %v, want ErrNotFound", err)
		}
	})
}

func TestPart2Capabilities(t *testing.T) {
	mem := interfaces.NewMemoryStore()
	fileStore := newFileStore(t, t.TempDir())
	cases := []struct {
		name  string
		store interfaces.BlobStore
		want  []string
	}{
		{"memory", mem, []string{"list", "size"}},
		{"file", fileStore, []string{"list", "close"}},
		{"plain", plain{mem}, nil},
		{"closer only", &closingStore{BlobStore: mem}, []string{"close"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := interfaces.Capabilities(tc.store)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Capabilities mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart2CloseStore(t *testing.T) {
	t.Run("calls Close on io.Closer stores", func(t *testing.T) {
		c := &closingStore{BlobStore: interfaces.NewMemoryStore()}
		if err := interfaces.CloseStore(c); err != nil {
			t.Fatalf("CloseStore: %v", err)
		}
		if c.closes != 1 {
			t.Errorf("Close called %d times, want 1", c.closes)
		}
	})

	t.Run("no-op for stores without Close", func(t *testing.T) {
		c := &closingStore{BlobStore: interfaces.NewMemoryStore()}
		if err := interfaces.CloseStore(c); err != nil || c.closes != 1 {
			t.Fatalf("sanity: closer not closed: err=%v closes=%d", err, c.closes)
		}
		if err := interfaces.CloseStore(plain{interfaces.NewMemoryStore()}); err != nil {
			t.Errorf("CloseStore(plain) = %v, want nil", err)
		}
	})
}
