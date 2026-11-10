package ctxflow_test

import (
	"testing"
	"time"
)

// deadline bounds every blocking step of a test. Inside testing/synctest it is
// fake time, so a missing event fails immediately instead of hanging.
const deadline = 2 * time.Second

// recv receives one value from ch or fails the test after deadline.
func recv[T any](t *testing.T, ch <-chan T) (T, bool) {
	t.Helper()
	select {
	case v, ok := <-ch:
		return v, ok
	case <-time.After(deadline):
		t.Errorf("timed out after %v waiting for a value", deadline)
		var zero T
		return zero, false
	}
}

// callWithin runs f on its own goroutine and returns its result, failing the
// test if f does not return within deadline.
func callWithin[T any](t *testing.T, f func() T) (T, bool) {
	t.Helper()
	ch := make(chan T, 1)
	go func() { ch <- f() }()
	return recv(t, (<-chan T)(ch))
}
