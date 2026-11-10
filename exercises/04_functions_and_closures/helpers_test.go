package funcs_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// fakeClock returns its current time, then advances by step on every call to
// Now. Advance moves it explicitly.
type fakeClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.now
	c.now = c.now.Add(c.step)
	return t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// panicOf runs fn and returns the value it panicked with, or nil if it did not.
func panicOf(fn func()) (v any) {
	defer func() { v = recover() }()
	fn()
	return nil
}

// serve runs h against a fresh recorder. A panic escaping h fails the test
// instead of killing the test binary.
func serve(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	if v := panicOf(func() { h.ServeHTTP(rec, req) }); v != nil {
		t.Fatalf("panic escaped the handler chain: %v", v)
	}
	return rec
}
