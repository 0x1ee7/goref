package goroutines_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"goref/exercises/14_goroutines_and_sync"
)

func TestPart4NewLRUInvalidCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		_, err := goroutines.NewLRU[string, int](capacity)
		if !errors.Is(err, goroutines.ErrInvalidCapacity) {
			t.Errorf("NewLRU(%d) error = %v, want ErrInvalidCapacity", capacity, err)
		}
	}
}

func TestPart4LRUGetPutLen(t *testing.T) {
	c, err := goroutines.NewLRU[string, int](3)
	if err != nil {
		t.Fatalf("NewLRU() error = %v", err)
	}
	c.Put("a", 1)
	c.Put("b", 2)
	if c.Len() != 2 {
		t.Errorf("Len() = %d, want 2", c.Len())
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a) = %d, %v; want 1, true", v, ok)
	}
	if _, ok := c.Get("missing"); ok {
		t.Errorf("Get(missing) reported a hit")
	}
}

func TestPart4LRUEviction(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		run      func(c *goroutines.LRU[string, int])
		evicted  string
		kept     []string
	}{
		{
			name:     "least recently used is evicted",
			capacity: 2,
			run: func(c *goroutines.LRU[string, int]) {
				c.Put("a", 1)
				c.Put("b", 2)
				c.Get("a") // a is now most recent
				c.Put("c", 3)
			},
			evicted: "b",
			kept:    []string{"a", "c"},
		},
		{
			name:     "Peek does not change recency",
			capacity: 2,
			run: func(c *goroutines.LRU[string, int]) {
				c.Put("a", 1)
				c.Put("b", 2)
				c.Peek("a") // a stays least recent
				c.Put("c", 3)
			},
			evicted: "a",
			kept:    []string{"b", "c"},
		},
		{
			name:     "updating a key promotes it and does not grow the cache",
			capacity: 2,
			run: func(c *goroutines.LRU[string, int]) {
				c.Put("a", 1)
				c.Put("b", 2)
				c.Put("a", 10)
				c.Put("c", 3)
			},
			evicted: "b",
			kept:    []string{"a", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := goroutines.NewLRU[string, int](tt.capacity)
			if err != nil {
				t.Fatalf("NewLRU() error = %v", err)
			}
			tt.run(c)
			if c.Len() != tt.capacity {
				t.Errorf("Len() = %d, want %d", c.Len(), tt.capacity)
			}
			if _, ok := c.Peek(tt.evicted); ok {
				t.Errorf("%q still cached, want evicted", tt.evicted)
			}
			for _, k := range tt.kept {
				if _, ok := c.Peek(k); !ok {
					t.Errorf("%q evicted, want kept", k)
				}
			}
		})
	}
}

func TestPart4LRUConcurrentUse(t *testing.T) {
	c, err := goroutines.NewLRU[int, int](16)
	if err != nil {
		t.Fatalf("NewLRU() error = %v", err)
	}
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 200 {
				c.Put(g*1000+i, i)
				c.Get(i)
			}
		})
	}
	callWithin(t, "concurrent LRU use", wg.Wait)
	if c.Len() != 16 {
		t.Errorf("Len() = %d after concurrent puts, want 16", c.Len())
	}
}

// loaderFor returns a Loader that yields "v-<key>" and counts nothing itself.
func loaderFor() goroutines.Loader[string] {
	return func(_ context.Context, key string) (string, error) {
		return "v-" + key, nil
	}
}

func TestPart4NewCacheInvalidCapacity(t *testing.T) {
	_, err := goroutines.NewCache(0, loaderFor())
	if !errors.Is(err, goroutines.ErrInvalidCapacity) {
		t.Errorf("NewCache(0) error = %v, want ErrInvalidCapacity", err)
	}
}

func TestPart4CacheHitsAvoidLoader(t *testing.T) {
	c, err := goroutines.NewCache(4, loaderFor())
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	for range 3 {
		v, err := c.GetOrLoad(t.Context(), "k")
		if err != nil || v != "v-k" {
			t.Fatalf("GetOrLoad(k) = %q, %v; want v-k, nil", v, err)
		}
	}
	if n := c.Loads(); n != 1 {
		t.Errorf("Loads() = %d after 3 calls for one key, want 1", n)
	}
}

func TestPart4CacheEvictionReloads(t *testing.T) {
	c, err := goroutines.NewCache(1, loaderFor())
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	for _, key := range []string{"a", "b", "a"} {
		if _, err := c.GetOrLoad(t.Context(), key); err != nil {
			t.Fatalf("GetOrLoad(%s) error = %v", key, err)
		}
	}
	if n := c.Loads(); n != 3 {
		t.Errorf("Loads() = %d, want 3 (capacity 1 evicts a when b arrives)", n)
	}
}

func TestPart4CacheLoaderErrorNotCached(t *testing.T) {
	calls := 0
	c, err := goroutines.NewCache(4, func(_ context.Context, key string) (string, error) {
		calls++
		if calls == 1 {
			return "", errBoom
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	if _, err := c.GetOrLoad(t.Context(), "k"); !errors.Is(err, errBoom) {
		t.Fatalf("first GetOrLoad() error = %v, want errBoom", err)
	}
	v, err := c.GetOrLoad(t.Context(), "k")
	if err != nil || v != "ok" {
		t.Errorf("second GetOrLoad() = %q, %v; want ok, nil (errors are not cached)", v, err)
	}
	if n := c.Loads(); n != 2 {
		t.Errorf("Loads() = %d, want 2", n)
	}
}

func TestPart4CacheSingleFlight(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		c, err := goroutines.NewCache(4, func(ctx context.Context, key string) (string, error) {
			select {
			case <-release:
				return "v-" + key, nil
			case <-time.After(testTimeout):
				return "", errors.New("loader never released")
			}
		})
		if err != nil {
			t.Fatalf("NewCache() error = %v", err)
		}
		type result struct {
			v   string
			err error
		}
		results := make(chan result, 8)
		for range 8 {
			go func() {
				v, err := c.GetOrLoad(t.Context(), "hot")
				results <- result{v, err}
			}()
		}
		// Every caller is now durably blocked behind the one in-flight load.
		synctest.Wait()
		if n := c.Loads(); n != 1 {
			t.Errorf("Loads() = %d with 8 concurrent callers, want 1 (single-flight)", n)
		}
		close(release)
		for range 8 {
			r := recvOrFail(t, results, "GetOrLoad result")
			if r.err != nil || r.v != "v-hot" {
				t.Errorf("GetOrLoad() = %q, %v; want v-hot, nil", r.v, r.err)
			}
		}
		if n := c.Loads(); n != 1 {
			t.Errorf("Loads() = %d after completion, want 1", n)
		}
	})
}

func TestPart4CacheCallerCancelDoesNotCancelLoad(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		c, err := goroutines.NewCache(4, func(ctx context.Context, key string) (string, error) {
			select {
			case <-release:
			case <-time.After(testTimeout):
				return "", errors.New("loader never released")
			}
			// The load must not inherit the cancellation of the caller that started it.
			if err := ctx.Err(); err != nil {
				return "", err
			}
			return "v-" + key, nil
		})
		if err != nil {
			t.Fatalf("NewCache() error = %v", err)
		}

		ctxA, cancelA := context.WithCancel(t.Context())
		defer cancelA()
		aDone := make(chan error, 1)
		go func() {
			_, err := c.GetOrLoad(ctxA, "k")
			aDone <- err
		}()
		bDone := make(chan error, 1)
		bVal := make(chan string, 1)
		go func() {
			v, err := c.GetOrLoad(t.Context(), "k")
			bVal <- v
			bDone <- err
		}()
		synctest.Wait()

		cancelA()
		if err := recvOrFail(t, aDone, "cancelled caller"); !errors.Is(err, context.Canceled) {
			t.Errorf("cancelled GetOrLoad() error = %v, want context.Canceled", err)
		}

		close(release)
		if err := recvOrFail(t, bDone, "surviving caller"); err != nil {
			t.Fatalf("surviving GetOrLoad() error = %v, want nil (load must survive A's cancel)", err)
		}
		if v := recvOrFail(t, bVal, "surviving value"); v != "v-k" {
			t.Errorf("surviving GetOrLoad() = %q, want v-k", v)
		}

		// The abandoned load's result was still cached.
		v, err := c.GetOrLoad(t.Context(), "k")
		if err != nil || v != "v-k" {
			t.Errorf("later GetOrLoad() = %q, %v; want cached v-k, nil", v, err)
		}
		if n := c.Loads(); n != 1 {
			t.Errorf("Loads() = %d, want 1", n)
		}
	})
}

func TestPart4CacheCancelledWaiterGetsCtxErr(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		c, err := goroutines.NewCache(4, func(ctx context.Context, key string) (string, error) {
			select {
			case <-release:
				return "v", nil
			case <-time.After(testTimeout):
				return "", errors.New("loader never released")
			}
		})
		if err != nil {
			t.Fatalf("NewCache() error = %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = c.GetOrLoad(ctx, "k")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("GetOrLoad(already cancelled) error = %v, want context.Canceled", err)
		}
		close(release)
	})
}
