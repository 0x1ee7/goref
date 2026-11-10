package goroutines

import (
	"context"
	"errors"
)

// ErrInvalidConcurrency is returned by NewDownloader when maxConcurrent < 1.
var ErrInvalidConcurrency = errors.New("downloader: maxConcurrent must be >= 1")

// BodyFetcher downloads the body of url. It must return promptly once ctx is done.
type BodyFetcher func(ctx context.Context, url string) ([]byte, error)

// Downloader fetches bodies with at most maxConcurrent fetches in flight across
// all callers of the same Downloader. It is safe for concurrent use.
type Downloader struct {
	// engineer-owned fields
}

// NewDownloader returns a Downloader that runs fetch with at most maxConcurrent
// concurrent calls. It returns ErrInvalidConcurrency if maxConcurrent < 1.
func NewDownloader(fetch BodyFetcher, maxConcurrent int) (*Downloader, error) {
	return nil, errors.New("not implemented")
}

// Download fetches url, waiting for a free slot first. If ctx is already done,
// it returns ctx.Err() without calling fetch. A wait for a slot that ends
// because ctx is done returns ctx.Err() as well. Fetch errors are returned as is.
func (d *Downloader) Download(ctx context.Context, url string) ([]byte, error) {
	return nil, errors.New("not implemented")
}

// DownloadAll fetches every URL and returns the bodies in input order. On the
// first failure it cancels the remaining work, waits for every started fetch to
// finish, and returns (nil, err) where err wraps the first failure and names its
// URL. Goroutines are bounded by maxConcurrent for this call. Empty input returns
// an empty slice and no error.
func (d *Downloader) DownloadAll(ctx context.Context, urls []string) ([][]byte, error) {
	return nil, errors.New("not implemented")
}

// InFlight returns the number of fetch calls running right now.
func (d *Downloader) InFlight() int64 {
	return 0
}

// PeakConcurrency returns the highest value InFlight has ever reached. It is 0
// before any call and never exceeds maxConcurrent.
func (d *Downloader) PeakConcurrency() int64 {
	return 0
}
