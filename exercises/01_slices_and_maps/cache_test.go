package slicesmaps_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"goref/exercises/01_slices_and_maps"
)

func newCache(t *testing.T, maxsize int, ttl time.Duration, clk *fakeClock) *slicesmaps.TTLCache[string, int] {
	t.Helper()
	c, err := slicesmaps.NewTTLCache[string, int](maxsize, ttl, clk.Now)
	if err != nil {
		t.Fatalf("NewTTLCache(%d, %v): %v", maxsize, ttl, err)
	}
	return c
}

func TestPart3Constructor(t *testing.T) {
	clk := newClock()
	tests := []struct {
		name    string
		maxsize int
		ttl     time.Duration
		wantErr bool
	}{
		{"zero maxsize", 0, time.Minute, true},
		{"negative maxsize", -1, time.Minute, true},
		{"zero ttl", 3, 0, true},
		{"negative ttl", 3, -time.Second, true},
		{"valid", 3, time.Minute, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := slicesmaps.NewTTLCache[string, int](tc.maxsize, tc.ttl, clk.Now)
			if tc.wantErr {
				if !errors.Is(err, slicesmaps.ErrInvalidConfig) {
					t.Fatalf("NewTTLCache error = %v, want ErrInvalidConfig", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTTLCache: unexpected error %v", err)
			}
			c.Put("a", 1)
			if v, ok := c.Get("a"); !ok || v != 1 {
				t.Fatalf("Get(a) = %d, %v; want 1, true", v, ok)
			}
		})
	}

	t.Run("nil clock uses time.Now", func(t *testing.T) {
		c, err := slicesmaps.NewTTLCache[string, int](3, time.Minute, nil)
		if err != nil {
			t.Fatalf("NewTTLCache with nil clock: %v", err)
		}
		c.Put("a", 1)
		if v, ok := c.Get("a"); !ok || v != 1 {
			t.Fatalf("Get(a) = %d, %v; want 1, true", v, ok)
		}
	})
}

func TestPart3GetPutBasics(t *testing.T) {
	t.Run("miss on empty cache", func(t *testing.T) {
		c := newCache(t, 3, time.Minute, newClock())
		if _, ok := c.Get("missing"); ok {
			t.Fatal("Get(missing) hit on an empty cache")
		}
		c.Put("x", 9)
		if v, ok := c.Get("x"); !ok || v != 9 {
			t.Fatalf("control: Get(x) = %d, %v; want 9, true", v, ok)
		}
	})
	t.Run("hit returns stored value", func(t *testing.T) {
		c := newCache(t, 3, time.Minute, newClock())
		c.Put("a", 1)
		if v, ok := c.Get("a"); !ok || v != 1 {
			t.Fatalf("Get(a) = %d, %v; want 1, true", v, ok)
		}
	})
	t.Run("put replaces value", func(t *testing.T) {
		c := newCache(t, 3, time.Minute, newClock())
		c.Put("a", 1)
		c.Put("a", 2)
		if v, ok := c.Get("a"); !ok || v != 2 {
			t.Fatalf("Get(a) after replace = %d, %v; want 2, true", v, ok)
		}
		if c.Len() != 1 {
			t.Fatalf("Len() after replace = %d, want 1", c.Len())
		}
	})
	t.Run("zero value is a real value", func(t *testing.T) {
		c := newCache(t, 3, time.Minute, newClock())
		c.Put("a", 0)
		if v, ok := c.Get("a"); !ok || v != 0 {
			t.Fatalf("Get(a) = %d, %v; want 0, true", v, ok)
		}
	})
}

func TestPart3Expiry(t *testing.T) {
	const ttl = 10 * time.Second
	t.Run("expires exactly at written plus ttl", func(t *testing.T) {
		clk := newClock()
		c := newCache(t, 3, ttl, clk)
		c.Put("a", 1)
		clk.Advance(ttl - time.Nanosecond)
		if _, ok := c.Get("a"); !ok {
			t.Fatal("entry expired 1ns before written+ttl")
		}
		clk.Advance(time.Nanosecond)
		if _, ok := c.Get("a"); ok {
			t.Fatal("entry still live at exactly written+ttl")
		}
	})
	t.Run("expired entries are not counted by Len or Contains", func(t *testing.T) {
		clk := newClock()
		c := newCache(t, 3, ttl, clk)
		c.Put("a", 1)
		c.Put("b", 2)
		if c.Len() != 2 {
			t.Fatalf("control: Len() = %d, want 2", c.Len())
		}
		clk.Advance(ttl)
		if c.Contains("a") {
			t.Fatal("Contains(a) true after expiry")
		}
		if c.Len() != 0 {
			t.Fatalf("Len() after expiry = %d, want 0", c.Len())
		}
	})
	t.Run("reads do not extend ttl", func(t *testing.T) {
		clk := newClock()
		c := newCache(t, 3, ttl, clk)
		c.Put("a", 1)
		clk.Advance(6 * time.Second)
		if _, ok := c.Get("a"); !ok {
			t.Fatal("entry should be live 6s after write")
		}
		clk.Advance(5 * time.Second) // 11s after the write
		if _, ok := c.Get("a"); ok {
			t.Fatal("Get extended the TTL: entry live 11s after write")
		}
	})
	t.Run("replace refreshes ttl", func(t *testing.T) {
		clk := newClock()
		c := newCache(t, 3, ttl, clk)
		c.Put("a", 1)
		clk.Advance(8 * time.Second)
		c.Put("a", 2)
		clk.Advance(8 * time.Second) // 16s after first write, 8s after refresh
		if v, ok := c.Get("a"); !ok || v != 2 {
			t.Fatalf("Get(a) = %d, %v; want 2, true after refresh", v, ok)
		}
	})
}

func TestPart3LRU(t *testing.T) {
	const hour = time.Hour
	t.Run("evicts least recently used", func(t *testing.T) {
		c := newCache(t, 2, hour, newClock())
		c.Put("a", 1)
		c.Put("b", 2)
		c.Put("c", 3)
		if got, want := c.Keys(), []string{"b", "c"}; !slices.Equal(got, want) {
			t.Fatalf("Keys() = %v, want %v", got, want)
		}
		if _, ok := c.Get("a"); ok {
			t.Fatal("a should have been evicted")
		}
	})
	t.Run("get refreshes recency", func(t *testing.T) {
		c := newCache(t, 2, hour, newClock())
		c.Put("a", 1)
		c.Put("b", 2)
		c.Get("a")
		c.Put("c", 3)
		if got, want := c.Keys(), []string{"a", "c"}; !slices.Equal(got, want) {
			t.Fatalf("Keys() = %v, want %v (b should be evicted)", got, want)
		}
	})
	t.Run("replace refreshes recency", func(t *testing.T) {
		c := newCache(t, 2, hour, newClock())
		c.Put("a", 1)
		c.Put("b", 2)
		c.Put("a", 9)
		c.Put("c", 3)
		if got, want := c.Keys(), []string{"a", "c"}; !slices.Equal(got, want) {
			t.Fatalf("Keys() = %v, want %v", got, want)
		}
		if v, ok := c.Get("a"); !ok || v != 9 {
			t.Fatalf("Get(a) = %d, %v; want 9, true", v, ok)
		}
	})
	t.Run("contains and keys do not touch recency", func(t *testing.T) {
		c := newCache(t, 2, hour, newClock())
		c.Put("a", 1)
		c.Put("b", 2)
		c.Contains("a")
		_ = c.Keys()
		c.Put("c", 3)
		if got, want := c.Keys(), []string{"b", "c"}; !slices.Equal(got, want) {
			t.Fatalf("Keys() = %v, want %v (a should be evicted)", got, want)
		}
	})
	t.Run("expired entries do not count toward capacity", func(t *testing.T) {
		clk := newClock()
		c := newCache(t, 2, 10*time.Second, clk)
		c.Put("a", 1)
		clk.Advance(5 * time.Second)
		c.Put("b", 2)
		clk.Advance(6 * time.Second) // a is expired (11s old); b is live (6s old)
		if c.Len() != 1 {
			t.Fatalf("control: Len() = %d, want 1", c.Len())
		}
		c.Put("c", 3)
		if got, want := c.Keys(), []string{"b", "c"}; !slices.Equal(got, want) {
			t.Fatalf("Keys() = %v, want %v: expired entry must not force eviction of b", got, want)
		}
	})
	t.Run("maxsize one keeps only the newest", func(t *testing.T) {
		c := newCache(t, 1, hour, newClock())
		c.Put("a", 1)
		c.Put("b", 2)
		if got, want := c.Keys(), []string{"b"}; !slices.Equal(got, want) {
			t.Fatalf("Keys() = %v, want %v", got, want)
		}
		if _, ok := c.Get("a"); ok {
			t.Fatal("a should have been evicted")
		}
		if v, ok := c.Get("b"); !ok || v != 2 {
			t.Fatalf("Get(b) = %d, %v; want 2, true", v, ok)
		}
	})
}

func TestPart3Stats(t *testing.T) {
	t.Run("get outcomes are counted", func(t *testing.T) {
		c := newCache(t, 3, time.Minute, newClock())
		c.Put("a", 1)
		c.Get("a")
		c.Get("a")
		c.Get("z")
		if c.Hits() != 2 || c.Misses() != 1 {
			t.Fatalf("Hits, Misses = %d, %d; want 2, 1", c.Hits(), c.Misses())
		}
	})
	t.Run("expired get counts as a miss", func(t *testing.T) {
		clk := newClock()
		c := newCache(t, 3, 10*time.Second, clk)
		c.Put("a", 1)
		clk.Advance(10 * time.Second)
		c.Get("a")
		if c.Hits() != 0 || c.Misses() != 1 {
			t.Fatalf("Hits, Misses = %d, %d; want 0, 1", c.Hits(), c.Misses())
		}
	})
	t.Run("contains and put are not counted", func(t *testing.T) {
		c := newCache(t, 3, time.Minute, newClock())
		c.Put("a", 1)
		c.Contains("a")
		c.Contains("z")
		c.Put("b", 2)
		c.Get("a")
		if c.Hits() != 1 || c.Misses() != 0 {
			t.Fatalf("Hits, Misses = %d, %d; want 1, 0", c.Hits(), c.Misses())
		}
	})
}
