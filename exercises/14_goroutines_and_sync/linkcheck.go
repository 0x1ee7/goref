// Package goroutines covers goroutines, channels and package sync through
// production-shaped problems: a link checker, a bounded downloader, shared
// state, a cache with request coalescing, pipelines, CPU-bound parallel map
// and a periodic scheduler.
package goroutines

import (
	"context"
	"errors"
)

// ErrInvalidWorkers is returned by CheckLinks when workers < 1.
var ErrInvalidWorkers = errors.New("linkcheck: workers must be >= 1")

// Fetcher performs one status check of url and returns an HTTP-style status
// code. It must return promptly once ctx is done.
type Fetcher func(ctx context.Context, url string) (int, error)

// LinkResult is the outcome for one input URL.
type LinkResult struct {
	URL    string
	Status int   // zero whenever Err != nil
	Err    error // the fetch error, or ctx.Err() for a URL that was never fetched
}

// OK reports whether the link is healthy: Err is nil and 200 <= Status < 400.
func (r LinkResult) OK() bool {
	return false
}

// CheckLinks fetches every URL with at most workers fetches in flight and
// returns one LinkResult per input element, in input order.
//
// Duplicate URLs are fetched once and their result is repeated in place.
// Per-URL fetch errors are recorded in LinkResult.Err and do not abort the
// run. If ctx is done, no new fetch starts; URLs that were not fetched get
// Err = ctx.Err() and CheckLinks returns ctx.Err(). It returns only after
// every worker has exited. Empty input returns an empty result and no error.
func CheckLinks(ctx context.Context, urls []string, fetch Fetcher, workers int) ([]LinkResult, error) {
	return nil, errors.New("not implemented")
}
