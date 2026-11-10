package goroutines_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/14_goroutines_and_sync"
)

var errBoom = errors.New("boom")

// row is a comparable view of a LinkResult (error values compare by message).
type row struct {
	URL    string
	Status int
	Err    string
}

func rows(rs []goroutines.LinkResult) []row {
	out := make([]row, 0, len(rs))
	for _, r := range rs {
		msg := ""
		if r.Err != nil {
			msg = r.Err.Error()
		}
		out = append(out, row{URL: r.URL, Status: r.Status, Err: msg})
	}
	return out
}

// statusFor is a deterministic fake fetch result: distinct for URLs of different length.
func statusFor(url string) int { return 200 + len(url) }

func okFetch(_ context.Context, url string) (int, error) { return statusFor(url), nil }

func TestPart1CheckLinks(t *testing.T) {
	fifty := make([]string, 50)
	var wantFifty []row
	for i := range fifty {
		fifty[i] = fmt.Sprintf("https://docs.example/%0*d", i%7+1, i)
		wantFifty = append(wantFifty, row{URL: fifty[i], Status: statusFor(fifty[i])})
	}

	tests := []struct {
		name    string
		urls    []string
		workers int
		fetch   goroutines.Fetcher
		want    []row
	}{
		{
			name:    "empty input",
			urls:    nil,
			workers: 4,
			fetch:   okFetch,
			want:    []row{},
		},
		{
			name:    "results follow input order with 8 workers",
			urls:    fifty,
			workers: 8,
			fetch:   okFetch,
			want:    wantFifty,
		},
		{
			name:    "more workers than URLs",
			urls:    []string{"a", "bb", "ccc"},
			workers: 100,
			fetch:   okFetch,
			want: []row{
				{URL: "a", Status: 201},
				{URL: "bb", Status: 202},
				{URL: "ccc", Status: 203},
			},
		},
		{
			name:    "duplicates repeated in place",
			urls:    []string{"a", "bb", "a", "ccc", "a"},
			workers: 2,
			fetch:   okFetch,
			want: []row{
				{URL: "a", Status: 201},
				{URL: "bb", Status: 202},
				{URL: "a", Status: 201},
				{URL: "ccc", Status: 203},
				{URL: "a", Status: 201},
			},
		},
		{
			name:    "fetch error recorded per URL and status is zeroed",
			urls:    []string{"ok-1", "bad", "ok-22"},
			workers: 3,
			fetch: func(_ context.Context, url string) (int, error) {
				if url == "bad" {
					return 500, errBoom
				}
				return statusFor(url), nil
			},
			want: []row{
				{URL: "ok-1", Status: statusFor("ok-1")},
				{URL: "bad", Status: 0, Err: "boom"},
				{URL: "ok-22", Status: statusFor("ok-22")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []goroutines.LinkResult
			var err error
			callWithin(t, "CheckLinks", func() {
				got, err = goroutines.CheckLinks(t.Context(), tt.urls, tt.fetch, tt.workers)
			})
			if err != nil {
				t.Fatalf("CheckLinks() error = %v, want nil", err)
			}
			if diff := cmp.Diff(tt.want, rows(got)); diff != "" {
				t.Errorf("CheckLinks() rows mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart1CheckLinksDedupeFetchesOnce(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	fetch := func(_ context.Context, url string) (int, error) {
		mu.Lock()
		calls[url]++
		mu.Unlock()
		return 200, nil
	}
	var err error
	callWithin(t, "CheckLinks", func() {
		_, err = goroutines.CheckLinks(t.Context(), []string{"a", "b", "a", "a"}, fetch, 4)
	})
	if err != nil {
		t.Fatalf("CheckLinks() error = %v", err)
	}
	if calls["a"] != 1 || calls["b"] != 1 || len(calls) != 2 {
		t.Errorf("fetch calls = %v, want a:1 b:1", calls)
	}
}

func TestPart1CheckLinksInvalidWorkers(t *testing.T) {
	for _, workers := range []int{0, -3} {
		t.Run(fmt.Sprintf("workers=%d", workers), func(t *testing.T) {
			var got []goroutines.LinkResult
			var err error
			callWithin(t, "CheckLinks", func() {
				got, err = goroutines.CheckLinks(t.Context(), []string{"a"}, okFetch, workers)
			})
			if !errors.Is(err, goroutines.ErrInvalidWorkers) {
				t.Fatalf("CheckLinks() error = %v, want ErrInvalidWorkers", err)
			}
			if len(got) != 0 {
				t.Errorf("CheckLinks() returned %d results with invalid workers, want none", len(got))
			}
		})
	}
}

func TestPart1CheckLinksBoundedConcurrency(t *testing.T) {
	const workers = 3
	urls := make([]string, 9)
	for i := range urls {
		urls[i] = fmt.Sprintf("u%d", i)
	}
	// The first three fetches block until all three have arrived, which proves
	// that three fetches really run at once; the high-water mark then must be exactly 3.
	g := newGate(workers)
	var inFlight, peak atomic.Int64
	fetch := func(ctx context.Context, url string) (int, error) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		if err := g.arrive(ctx); err != nil {
			return 0, err
		}
		return 200, nil
	}
	var err error
	callWithin(t, "CheckLinks", func() {
		_, err = goroutines.CheckLinks(t.Context(), urls, fetch, workers)
	})
	if err != nil {
		t.Fatalf("CheckLinks() error = %v", err)
	}
	if got := peak.Load(); got != workers {
		t.Errorf("peak concurrent fetches = %d, want exactly %d", got, workers)
	}
}

func TestPart1CheckLinksCancelledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var calls atomic.Int64
	fetch := func(_ context.Context, url string) (int, error) {
		calls.Add(1)
		return 200, nil
	}
	var got []goroutines.LinkResult
	var err error
	callWithin(t, "CheckLinks", func() {
		got, err = goroutines.CheckLinks(ctx, []string{"a", "b", "c"}, fetch, 2)
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("CheckLinks() error = %v, want context.Canceled", err)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("fetch called %d times after cancellation, want 0", n)
	}
	want := []row{
		{URL: "a", Err: "context canceled"},
		{URL: "b", Err: "context canceled"},
		{URL: "c", Err: "context canceled"},
	}
	if diff := cmp.Diff(want, rows(got)); diff != "" {
		t.Errorf("rows mismatch (-want +got):\n%s", diff)
	}
}

func TestPart1CheckLinksCancelMidRun(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var calls atomic.Int64
	fetch := func(ctx context.Context, url string) (int, error) {
		calls.Add(1)
		if url == "stop" {
			cancel()
			return 0, ctx.Err()
		}
		return statusFor(url), nil
	}
	var got []goroutines.LinkResult
	var err error
	callWithin(t, "CheckLinks", func() {
		got, err = goroutines.CheckLinks(ctx, []string{"a", "stop", "b", "c"}, fetch, 1)
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("CheckLinks() error = %v, want context.Canceled", err)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("fetch calls = %d, want 2 (a and stop only)", n)
	}
	want := []row{
		{URL: "a", Status: 201},
		{URL: "stop", Err: "context canceled"},
		{URL: "b", Err: "context canceled"},
		{URL: "c", Err: "context canceled"},
	}
	if diff := cmp.Diff(want, rows(got)); diff != "" {
		t.Errorf("rows mismatch (-want +got):\n%s", diff)
	}
}

func TestPart1CheckLinksNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()
	urls := make([]string, 40)
	for i := range urls {
		urls[i] = fmt.Sprintf("x%d", i)
	}
	var (
		got []goroutines.LinkResult
		err error
	)
	callWithin(t, "CheckLinks", func() {
		got, err = goroutines.CheckLinks(t.Context(), urls, okFetch, 6)
	})
	if err != nil || len(got) != len(urls) {
		t.Fatalf("CheckLinks() = %d results, err %v; want %d results and nil error", len(got), err, len(urls))
	}
	settleGoroutines(t, baseline)
}

func TestPart1LinkResultOK(t *testing.T) {
	tests := []struct {
		name string
		r    goroutines.LinkResult
		want bool
	}{
		{name: "200", r: goroutines.LinkResult{URL: "u", Status: 200}, want: true},
		{name: "301 redirect is healthy", r: goroutines.LinkResult{URL: "u", Status: 301}, want: true},
		{name: "399 is healthy", r: goroutines.LinkResult{URL: "u", Status: 399}, want: true},
		{name: "400 is broken", r: goroutines.LinkResult{URL: "u", Status: 400}, want: false},
		{name: "199 is not success", r: goroutines.LinkResult{URL: "u", Status: 199}, want: false},
		{name: "status 0 without error", r: goroutines.LinkResult{URL: "u"}, want: false},
		{name: "error wins over status", r: goroutines.LinkResult{URL: "u", Status: 200, Err: errBoom}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !(goroutines.LinkResult{URL: "u", Status: 200}).OK() {
				t.Fatal("control: a 200 result must be OK")
			}
			if got := tt.r.OK(); got != tt.want {
				t.Errorf("OK() = %v, want %v", got, tt.want)
			}
		})
	}
}
