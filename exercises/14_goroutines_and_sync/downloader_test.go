package goroutines_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/14_goroutines_and_sync"
)

// bodyFor is a deterministic fake body for url.
func bodyFor(url string) []byte { return []byte("body:" + url) }

func okBody(_ context.Context, url string) ([]byte, error) { return bodyFor(url), nil }

func newDownloaderOrFatal(t *testing.T, fetch goroutines.BodyFetcher, maxConcurrent int) *goroutines.Downloader {
	t.Helper()
	d, err := goroutines.NewDownloader(fetch, maxConcurrent)
	if err != nil {
		t.Fatalf("NewDownloader(maxConcurrent=%d) error = %v", maxConcurrent, err)
	}
	return d
}

func TestPart2NewDownloaderInvalid(t *testing.T) {
	for _, n := range []int{0, -1} {
		t.Run(fmt.Sprintf("maxConcurrent=%d", n), func(t *testing.T) {
			d, err := goroutines.NewDownloader(okBody, n)
			if !errors.Is(err, goroutines.ErrInvalidConcurrency) {
				t.Errorf("NewDownloader() error = %v, want ErrInvalidConcurrency", err)
			}
			if d != nil {
				t.Errorf("NewDownloader() returned a non-nil Downloader on error")
			}
		})
	}
}

func TestPart2DownloadReturnsBody(t *testing.T) {
	d := newDownloaderOrFatal(t, okBody, 2)
	var got []byte
	var err error
	callWithin(t, "Download", func() {
		got, err = d.Download(t.Context(), "https://a.example/x")
	})
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if diff := cmp.Diff(bodyFor("https://a.example/x"), got); diff != "" {
		t.Errorf("Download() body mismatch (-want +got):\n%s", diff)
	}
}

func TestPart2DownloadErrors(t *testing.T) {
	t.Run("fetch error returned as is", func(t *testing.T) {
		d := newDownloaderOrFatal(t, func(_ context.Context, url string) ([]byte, error) {
			return nil, errBoom
		}, 1)
		var err error
		callWithin(t, "Download", func() {
			_, err = d.Download(t.Context(), "u")
		})
		if !errors.Is(err, errBoom) {
			t.Errorf("Download() error = %v, want errBoom", err)
		}
	})
	t.Run("cancelled context fails without calling fetch", func(t *testing.T) {
		var calls atomic.Int64
		d := newDownloaderOrFatal(t, func(_ context.Context, url string) ([]byte, error) {
			calls.Add(1)
			return bodyFor(url), nil
		}, 1)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		var err error
		callWithin(t, "Download", func() {
			_, err = d.Download(ctx, "u")
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Download() error = %v, want context.Canceled", err)
		}
		if n := calls.Load(); n != 0 {
			t.Errorf("fetch called %d times, want 0", n)
		}
	})
}

func TestPart2DownloadWaitsForSlotAndHonoursCancel(t *testing.T) {
	// The only slot is held by a blocked fetch. A second Download must wait for
	// the slot, and cancelling its context must end that wait with ctx.Err().
	hold := make(chan struct{})
	entered := make(chan struct{}, 1)
	d := newDownloaderOrFatal(t, func(ctx context.Context, url string) ([]byte, error) {
		if url == "hold" {
			entered <- struct{}{}
			select {
			case <-hold:
			case <-ctx.Done():
			}
		}
		return bodyFor(url), nil
	}, 1)

	holderDone := make(chan error, 1)
	go func() {
		_, err := d.Download(t.Context(), "hold")
		holderDone <- err
	}()
	recvOrFail(t, entered, "holder to enter fetch")

	ctx, cancel := context.WithCancel(t.Context())
	waiterDone := make(chan error, 1)
	go func() {
		_, err := d.Download(ctx, "waiter")
		waiterDone <- err
	}()
	cancel()
	if err := recvOrFail(t, waiterDone, "waiter to give up"); !errors.Is(err, context.Canceled) {
		t.Errorf("waiting Download() error = %v, want context.Canceled", err)
	}

	close(hold)
	if err := recvOrFail(t, holderDone, "holder to finish"); err != nil {
		t.Errorf("holder Download() error = %v", err)
	}
}

func TestPart2DownloadAllOrderedResults(t *testing.T) {
	urls := make([]string, 30)
	want := make([][]byte, len(urls))
	for i := range urls {
		urls[i] = fmt.Sprintf("https://f.example/%02d", i)
		want[i] = bodyFor(urls[i])
	}
	d := newDownloaderOrFatal(t, okBody, 5)
	var got [][]byte
	var err error
	callWithin(t, "DownloadAll", func() {
		got, err = d.DownloadAll(t.Context(), urls)
	})
	if err != nil {
		t.Fatalf("DownloadAll() error = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DownloadAll() mismatch (-want +got):\n%s", diff)
	}
}

func TestPart2DownloadAllEmpty(t *testing.T) {
	d := newDownloaderOrFatal(t, okBody, 3)
	var got [][]byte
	var err error
	callWithin(t, "DownloadAll", func() {
		got, err = d.DownloadAll(t.Context(), nil)
	})
	if err != nil {
		t.Fatalf("DownloadAll(nil) error = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("DownloadAll(nil) = %#v, want non-nil empty slice", got)
	}
}

func TestPart2DownloadAllFirstErrorCancelsRest(t *testing.T) {
	// "bad" fails at once. The other fetches block until their context is
	// cancelled, so the call can only return if the first error cancels siblings.
	d := newDownloaderOrFatal(t, func(ctx context.Context, url string) ([]byte, error) {
		if url == "bad" {
			return nil, errBoom
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}, 4)
	urls := []string{"slow-1", "bad", "slow-2", "slow-3"}
	var got [][]byte
	var err error
	callWithin(t, "DownloadAll", func() {
		got, err = d.DownloadAll(t.Context(), urls)
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("DownloadAll() error = %v, want wrapped errBoom", err)
	}
	if want := "download bad"; !strings.Contains(err.Error(), want) {
		t.Errorf("DownloadAll() error = %q, want it to name the failing URL (contains %q)", err, want)
	}
	if got != nil {
		t.Errorf("DownloadAll() results = %v on error, want nil", got)
	}
}

func TestPart2DownloadAllBoundedConcurrency(t *testing.T) {
	const limit = 3
	urls := make([]string, 12)
	for i := range urls {
		urls[i] = fmt.Sprintf("p%d", i)
	}
	// The first `limit` fetches must all be in flight together before any returns.
	g := newGate(limit)
	d := newDownloaderOrFatal(t, func(ctx context.Context, url string) ([]byte, error) {
		if err := g.arrive(ctx); err != nil {
			return nil, err
		}
		return bodyFor(url), nil
	}, limit)
	var err error
	callWithin(t, "DownloadAll", func() {
		_, err = d.DownloadAll(t.Context(), urls)
	})
	if err != nil {
		t.Fatalf("DownloadAll() error = %v", err)
	}
	if got := d.PeakConcurrency(); got != limit {
		t.Errorf("PeakConcurrency() = %d, want exactly %d", got, limit)
	}
	if got := d.InFlight(); got != 0 {
		t.Errorf("InFlight() = %d after DownloadAll returned, want 0", got)
	}
}

func TestPart2SharedLimitAcrossCallers(t *testing.T) {
	// Two concurrent DownloadAll calls share one limit of 2: the peak across both is 2.
	const limit = 2
	g := newGate(limit)
	d := newDownloaderOrFatal(t, func(ctx context.Context, url string) ([]byte, error) {
		if err := g.arrive(ctx); err != nil {
			return nil, err
		}
		return bodyFor(url), nil
	}, limit)
	a := []string{"a1", "a2", "a3"}
	b := []string{"b1", "b2", "b3"}
	errA := make(chan error, 1)
	errB := make(chan error, 1)
	go func() {
		_, err := d.DownloadAll(t.Context(), a)
		errA <- err
	}()
	go func() {
		_, err := d.DownloadAll(t.Context(), b)
		errB <- err
	}()
	if err := recvOrFail(t, errA, "first DownloadAll"); err != nil {
		t.Errorf("first DownloadAll() error = %v", err)
	}
	if err := recvOrFail(t, errB, "second DownloadAll"); err != nil {
		t.Errorf("second DownloadAll() error = %v", err)
	}
	if got := d.PeakConcurrency(); got != limit {
		t.Errorf("PeakConcurrency() = %d, want exactly %d", got, limit)
	}
}

func TestPart2DownloadAllNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()
	d := newDownloaderOrFatal(t, func(ctx context.Context, url string) ([]byte, error) {
		if url == "bad" {
			return nil, errBoom
		}
		return bodyFor(url), nil
	}, 4)
	urls := make([]string, 25)
	for i := range urls {
		urls[i] = fmt.Sprintf("n%d", i)
	}
	callWithin(t, "DownloadAll ok", func() {
		_, _ = d.DownloadAll(t.Context(), urls)
	})
	callWithin(t, "DownloadAll failing", func() {
		_, _ = d.DownloadAll(t.Context(), append(urls, "bad"))
	})
	settleGoroutines(t, baseline)
}


