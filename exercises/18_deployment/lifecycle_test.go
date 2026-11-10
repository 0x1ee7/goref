package deployment_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	deployment "goref/exercises/18_deployment"
)

// callTimeout bounds every wait on Run so a broken implementation fails instead of hanging.
const callTimeout = 5 * time.Second

func loopbackListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}

// startRun runs deployment.Run in a goroutine and returns the channel carrying its result.
func startRun(ctx context.Context, srv *http.Server, ln net.Listener, grace time.Duration) <-chan error {
	done := make(chan error, 1)
	go func() { done <- deployment.Run(ctx, srv, ln, grace) }()
	return done
}

func awaitRun(t *testing.T, done <-chan error, within time.Duration) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(within):
		t.Fatalf("Run did not return within %v", within)
		return nil
	}
}

// waitHealthy polls /healthz until the server answers 200.
func waitHealthy(t *testing.T, base string) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(callTimeout)
	for {
		resp, err := client.Get(base + "/healthz")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("server at %s never became healthy (last err %v)", base, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func newLifecycleServer(t *testing.T) *http.Server {
	t.Helper()
	return newTestServer(t, loadTestConfig(t), deployment.Deps{})
}

func TestPart3ShutdownOnCancel(t *testing.T) {
	ln := loopbackListener(t)
	base := "http://" + ln.Addr().String()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := startRun(ctx, newLifecycleServer(t), ln, callTimeout)
	waitHealthy(t, base)

	cancel()
	if err := awaitRun(t, done, callTimeout); err != nil {
		t.Fatalf("Run after cancel: got %v, want nil for a clean shutdown", err)
	}

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err == nil {
		_ = conn.Close()
		t.Error("listener still accepts connections after Run returned")
	}
}

func TestPart3InFlightRequestFinishes(t *testing.T) {
	ln := loopbackListener(t)
	base := "http://" + ln.Addr().String()
	srv := newLifecycleServer(t)

	started := make(chan struct{})
	release := make(chan struct{})
	inner := srv.Handler
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slow" {
			inner.ServeHTTP(w, r)
			return
		}
		close(started)
		<-release
		_, _ = io.WriteString(w, "done")
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := startRun(ctx, srv, ln, callTimeout)
	waitHealthy(t, base)

	type result struct {
		status int
		body   string
		err    error
	}
	res := make(chan result, 1)
	go func() {
		resp, err := http.Get(base + "/slow")
		if err != nil {
			res <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		res <- result{status: resp.StatusCode, body: string(b), err: err}
	}()

	select {
	case <-started:
	case <-time.After(callTimeout):
		t.Fatal("slow handler never started")
	}

	cancel() // shutdown begins while the request is still in flight
	time.Sleep(100 * time.Millisecond)
	close(release)

	select {
	case r := <-res:
		if r.err != nil {
			t.Fatalf("in-flight request failed during shutdown: %v", r.err)
		}
		if r.status != http.StatusOK || r.body != "done" {
			t.Errorf("in-flight response = %d %q, want 200 \"done\"", r.status, r.body)
		}
	case <-time.After(callTimeout):
		t.Fatal("in-flight request did not complete")
	}

	if err := awaitRun(t, done, callTimeout); err != nil {
		t.Errorf("Run: got %v, want nil after a graceful drain", err)
	}
}

func TestPart3ShutdownDeadlineForcesClose(t *testing.T) {
	ln := loopbackListener(t)
	base := "http://" + ln.Addr().String()
	srv := newLifecycleServer(t)

	started := make(chan struct{})
	never := make(chan struct{})
	t.Cleanup(func() { close(never) }) // unblock the stuck handler once the test is over
	inner := srv.Handler
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stuck" {
			inner.ServeHTTP(w, r)
			return
		}
		close(started)
		select {
		case <-never:
		case <-r.Context().Done():
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := startRun(ctx, srv, ln, 100*time.Millisecond)
	waitHealthy(t, base)

	go func() {
		if resp, err := http.Get(base + "/stuck"); err == nil {
			_ = resp.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(callTimeout):
		t.Fatal("stuck handler never started")
	}

	cancel()
	err := awaitRun(t, done, callTimeout)
	if err == nil {
		t.Fatal("Run: got nil, want an error when the drain deadline is exceeded")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Run error = %v, want it to wrap context.DeadlineExceeded", err)
	}
}

func TestPart3IdleShutdownIsPrompt(t *testing.T) {
	ln := loopbackListener(t)
	base := "http://" + ln.Addr().String()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// A long grace period: an idle server must not wait for it to elapse.
	done := startRun(ctx, newLifecycleServer(t), ln, time.Minute)
	waitHealthy(t, base)

	start := time.Now()
	cancel()
	if err := awaitRun(t, done, callTimeout); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("idle shutdown took %v; it should return as soon as nothing is in flight", elapsed)
	}
}

func TestPart3ServeFailureIsReturned(t *testing.T) {
	ln := loopbackListener(t)
	if err := ln.Close(); err != nil {
		t.Fatalf("closing listener: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	err := awaitRun(t, startRun(ctx, newLifecycleServer(t), ln, callTimeout), callTimeout)
	if err == nil {
		t.Fatal("Run on a dead listener: got nil, want the serve error")
	}
	if errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run error = %v; ErrServerClosed means a clean shutdown, not a serve failure", err)
	}
}

func TestPart3PreCancelledContext(t *testing.T) {
	ln := loopbackListener(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := awaitRun(t, startRun(ctx, newLifecycleServer(t), ln, callTimeout), callTimeout)
	if err != nil {
		t.Errorf("Run with an already-cancelled context: got %v, want nil", err)
	}
}
