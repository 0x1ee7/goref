package goroutines

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// ErrInvalidConcurrency is returned by NewDownloader when maxConcurrent < 1.
var ErrInvalidConcurrency = errors.New("downloader: maxConcurrent must be >= 1")

// BodyFetcher downloads the body of url. It must return promptly once ctx is done.
type BodyFetcher func(ctx context.Context, url string) ([]byte, error)

// Downloader fetches bodies with at most maxConcurrent fetches in flight across
// all callers of the same Downloader. It is safe for concurrent use.
type Downloader struct {
	fetch    BodyFetcher
	max      int64
	sem      *semaphore.Weighted
	inFlight atomic.Int64
	peak     atomic.Int64
}

// NewDownloader returns a Downloader that runs fetch with at most maxConcurrent
// concurrent calls. It returns ErrInvalidConcurrency if maxConcurrent < 1.
func NewDownloader(fetch BodyFetcher, maxConcurrent int) (*Downloader, error) {
	if maxConcurrent < 1 {
		return nil, ErrInvalidConcurrency
	}
	return &Downloader{
		fetch: fetch,
		max:   int64(maxConcurrent),
		sem:   semaphore.NewWeighted(int64(maxConcurrent)),
	}, nil
}

// Download fetches url, waiting for a free slot first. If ctx is already done,
// it returns ctx.Err() without calling fetch. A wait for a slot that ends
// because ctx is done returns ctx.Err() as well. Fetch errors are returned as is.
func (d *Downloader) Download(ctx context.Context, url string) ([]byte, error) {
	// semaphore.Acquire may succeed on a done context, so check first.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := d.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer d.sem.Release(1)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Measure inside the slot so the peak can never exceed maxConcurrent.
	n := d.inFlight.Add(1)
	defer d.inFlight.Add(-1)
	for {
		p := d.peak.Load()
		if n <= p || d.peak.CompareAndSwap(p, n) {
			break
		}
	}
	return d.fetch(ctx, url)
}

// DownloadAll fetches every URL and returns the bodies in input order. On the
// first failure it cancels the remaining work, waits for every started fetch to
// finish, and returns (nil, err) where err wraps the first failure and names its
// URL. Goroutines are bounded by maxConcurrent for this call. Empty input returns
// an empty slice and no error.
func (d *Downloader) DownloadAll(ctx context.Context, urls []string) ([][]byte, error) {
	bodies := make([][]byte, len(urls))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(int(d.max))
	for i, u := range urls {
		g.Go(func() error {
			body, err := d.Download(gctx, u)
			if err != nil {
				return fmt.Errorf("download %s: %w", u, err)
			}
			bodies[i] = body
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return bodies, nil
}

// InFlight returns the number of fetch calls running right now.
func (d *Downloader) InFlight() int64 {
	return d.inFlight.Load()
}

// PeakConcurrency returns the highest value InFlight has ever reached. It is 0
// before any call and never exceeds maxConcurrent.
func (d *Downloader) PeakConcurrency() int64 {
	return d.peak.Load()
}
