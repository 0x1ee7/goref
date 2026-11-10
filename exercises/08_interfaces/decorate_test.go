package interfaces_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/08_interfaces"
)

// spyStore counts Get calls that reach the wrapped MemoryStore.
type spyStore struct {
	*interfaces.MemoryStore
	gets int
}

func (s *spyStore) Get(ctx context.Context, key string) ([]byte, error) {
	s.gets++
	return s.MemoryStore.Get(ctx, key)
}

func TestPart3CacheStore(t *testing.T) {
	t.Run("write-through Put serves later Gets from the cache", func(t *testing.T) {
		spy := &spyStore{MemoryStore: interfaces.NewMemoryStore()}
		cache, err := interfaces.NewCacheStore(spy, 4)
		if err != nil {
			t.Fatal(err)
		}
		mustPut(t, cache, "k", "v")
		for range 2 {
			if got := mustGet(t, cache, "k"); got != "v" {
				t.Fatalf("Get = %q, want %q", got, "v")
			}
		}
		if spy.gets != 0 {
			t.Errorf("inner Get calls = %d, want 0", spy.gets)
		}
		if st := cache.Stats(); st.Hits != 2 || st.Misses != 0 {
			t.Errorf("Stats = %+v, want 2 hits and 0 misses", st)
		}
	})

	t.Run("miss reads through once, then hits", func(t *testing.T) {
		spy := &spyStore{MemoryStore: interfaces.NewMemoryStore()}
		mustPut(t, spy, "k", "v") // bypasses the cache
		cache, err := interfaces.NewCacheStore(spy, 4)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if got := mustGet(t, cache, "k"); got != "v" {
				t.Fatalf("Get = %q, want %q", got, "v")
			}
		}
		if spy.gets != 1 {
			t.Errorf("inner Get calls = %d, want 1", spy.gets)
		}
		if st := cache.Stats(); st.Hits != 1 || st.Misses != 1 {
			t.Errorf("Stats = %+v, want 1 hit and 1 miss", st)
		}
	})

	t.Run("failed reads are not cached", func(t *testing.T) {
		spy := &spyStore{MemoryStore: interfaces.NewMemoryStore()}
		cache, err := interfaces.NewCacheStore(spy, 4)
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := cache.Get(t.Context(), "nope"); !errors.Is(err, interfaces.ErrNotFound) {
				t.Fatalf("Get(missing) err = %v, want ErrNotFound", err)
			}
		}
		if spy.gets != 2 {
			t.Errorf("inner Get calls = %d, want 2", spy.gets)
		}
		if st := cache.Stats(); st.Misses != 2 || st.Hits != 0 {
			t.Errorf("Stats = %+v, want 2 misses", st)
		}
	})

	t.Run("Delete invalidates the entry", func(t *testing.T) {
		spy := &spyStore{MemoryStore: interfaces.NewMemoryStore()}
		cache, err := interfaces.NewCacheStore(spy, 4)
		if err != nil {
			t.Fatal(err)
		}
		mustPut(t, cache, "k", "v")
		if got := mustGet(t, cache, "k"); got != "v" {
			t.Fatalf("Get before Delete = %q, want %q", got, "v")
		}
		if err := cache.Delete(t.Context(), "k"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if _, err := cache.Get(t.Context(), "k"); !errors.Is(err, interfaces.ErrNotFound) {
			t.Errorf("Get after Delete err = %v, want ErrNotFound", err)
		}
		if spy.gets != 1 {
			t.Errorf("inner Get calls = %d, want 1 (the post-delete miss)", spy.gets)
		}
	})

	t.Run("bounded FIFO eviction", func(t *testing.T) {
		spy := &spyStore{MemoryStore: interfaces.NewMemoryStore()}
		cache, err := interfaces.NewCacheStore(spy, 2)
		if err != nil {
			t.Fatal(err)
		}
		mustPut(t, cache, "a", "1")
		mustPut(t, cache, "b", "2")
		mustPut(t, cache, "c", "3") // evicts a
		if st := cache.Stats(); st.Evictions != 1 {
			t.Fatalf("after 3 Puts Evictions = %d, want 1", st.Evictions)
		}
		if got := mustGet(t, cache, "a"); got != "1" { // miss, served by inner; evicts b
			t.Errorf("Get(a) = %q, want %q", got, "1")
		}
		if got := mustGet(t, cache, "c"); got != "3" { // still cached: hit
			t.Errorf("Get(c) = %q, want %q", got, "3")
		}
		want := interfaces.CacheStats{Hits: 1, Misses: 1, Evictions: 2}
		if diff := cmp.Diff(want, cache.Stats()); diff != "" {
			t.Errorf("Stats mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("maxEntries must be positive", func(t *testing.T) {
		if _, err := interfaces.NewCacheStore(interfaces.NewMemoryStore(), 0); err == nil {
			t.Error("maxEntries 0: err = nil, want error")
		}
		if _, err := interfaces.NewCacheStore(interfaces.NewMemoryStore(), -1); err == nil {
			t.Error("maxEntries -1: err = nil, want error")
		}
		if _, err := interfaces.NewCacheStore(interfaces.NewMemoryStore(), 1); err != nil {
			t.Errorf("maxEntries 1: err = %v, want nil", err)
		}
	})

	t.Run("cache copies bytes in and out", func(t *testing.T) {
		cache, err := interfaces.NewCacheStore(interfaces.NewMemoryStore(), 4)
		if err != nil {
			t.Fatal(err)
		}
		in := []byte("abc")
		if err := cache.Put(t.Context(), "k", in); err != nil {
			t.Fatal(err)
		}
		in[0] = 'X'
		if got := mustGet(t, cache, "k"); got != "abc" {
			t.Errorf("cached value follows caller's buffer: got %q", got)
		}
	})

	t.Run("caching decorator deliberately hides Lister", func(t *testing.T) {
		cache, err := interfaces.NewCacheStore(interfaces.NewMemoryStore(), 4)
		if err != nil {
			t.Fatal(err)
		}
		var s interfaces.BlobStore = cache
		if _, ok := s.(interfaces.Lister); ok {
			t.Error("CacheStore implements Lister; a cached listing would go stale")
		}
	})
}

func TestPart3Instrument(t *testing.T) {
	t.Run("counts operations by kind and skips ErrNotFound", func(t *testing.T) {
		m := &interfaces.Metrics{}
		s := interfaces.Instrument(interfaces.NewMemoryStore(), m)
		mustPut(t, s, "a", "1")
		mustGet(t, s, "a")
		_, _ = s.Get(t.Context(), "missing") // ErrNotFound is not a failure
		_ = s.Delete(t.Context(), "a")
		_ = s.Delete(t.Context(), "a") // ErrNotFound again
		if err := s.Put(t.Context(), "", []byte("x")); !errors.Is(err, interfaces.ErrInvalidKey) {
			t.Fatalf("Put(invalid) err = %v, want ErrInvalidKey", err)
		}
		want := interfaces.MetricsSnapshot{Gets: 2, Puts: 2, Deletes: 2, Errors: 1}
		if diff := cmp.Diff(want, m.Snapshot()); diff != "" {
			t.Errorf("Snapshot mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("preserves exactly the optional capabilities of the wrapped store", func(t *testing.T) {
		cases := []struct {
			name                  string
			inner                 interfaces.BlobStore
			lister, sizer, closer bool
		}{
			{"memory", interfaces.NewMemoryStore(), true, true, false},
			{"file", newFileStore(t, t.TempDir()), true, false, true},
			{"plain", plain{interfaces.NewMemoryStore()}, false, false, false},
			{"closer", &closingStore{BlobStore: interfaces.NewMemoryStore()}, false, false, true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := &interfaces.Metrics{}
				s := interfaces.Instrument(tc.inner, m)
				mustPut(t, s, "k", "v")
				_, gotL := s.(interfaces.Lister)
				_, gotS := s.(interfaces.Sizer)
				_, gotC := s.(io.Closer)
				if gotL != tc.lister || gotS != tc.sizer || gotC != tc.closer {
					t.Errorf("capabilities list/size/close = %v/%v/%v, want %v/%v/%v",
						gotL, gotS, gotC, tc.lister, tc.sizer, tc.closer)
				}
				if tc.lister {
					if _, err := interfaces.ListKeys(t.Context(), s, ""); err != nil {
						t.Fatalf("ListKeys through wrapper: %v", err)
					}
				}
				if tc.sizer {
					if _, err := interfaces.SizeOf(t.Context(), s, "k"); err != nil {
						t.Fatalf("SizeOf through wrapper: %v", err)
					}
				}
				snap := m.Snapshot()
				if snap.Puts != 1 {
					t.Errorf("Puts = %d, want 1", snap.Puts)
				}
				wantLists, wantSizes := int64(0), int64(0)
				if tc.lister {
					wantLists = 1
				}
				if tc.sizer {
					wantSizes = 1
				}
				if snap.Lists != wantLists || snap.Sizes != wantSizes {
					t.Errorf("Lists/Sizes = %d/%d, want %d/%d", snap.Lists, snap.Sizes, wantLists, wantSizes)
				}
			})
		}
	})
}
