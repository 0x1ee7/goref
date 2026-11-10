package goroutines_test

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// testTimeout bounds every wait in this package. A stub or a bug fails the test
// instead of hanging the whole test binary.
const testTimeout = 5 * time.Second

// recvOrFail receives from ch or fails the test after testTimeout. Under
// synctest the timeout is a fake timer, so it cannot deadlock the bubble.
func recvOrFail[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(testTimeout):
		t.Fatalf("timed out waiting for %s", what)
		var zero T
		return zero
	}
}

// callWithin runs fn on a new goroutine and waits for it to return. The test
// fails if fn does not return within testTimeout.
func callWithin(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	recvOrFail(t, done, what)
}

// settleGoroutines waits until the goroutine count drops back to baseline. It is
// a settle check for leak detection, not a coordination mechanism: goroutines
// that exited have already signalled their owners through channels or WaitGroups.
func settleGoroutines(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for {
		n := runtime.NumGoroutine()
		if n <= baseline {
			return
		}
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			buf = buf[:runtime.Stack(buf, true)]
			t.Fatalf("goroutine leak: %d running, want <= %d\n%s", n, baseline, buf)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// gate proves that n operations run at the same time, without sleeping: every
// caller blocks in arrive until n callers have arrived, then all are released.
type gate struct {
	n       int64
	arrived atomic.Int64
	release chan struct{}
}

func newGate(n int) *gate {
	return &gate{n: int64(n), release: make(chan struct{})}
}

// arrive records one caller and blocks until the gate opens or ctx ends.
func (g *gate) arrive(ctx context.Context) error {
	if g.arrived.Add(1) == g.n {
		close(g.release)
	}
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
