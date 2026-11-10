// Package goroutines covers goroutines, channels and package sync through
// production-shaped problems: a link checker, a bounded downloader, shared
// state, a cache with request coalescing, pipelines, CPU-bound parallel map
// and a periodic scheduler.
package goroutines

import (
	"context"
	"errors"
	"sync"
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
	return r.Err == nil && r.Status >= 200 && r.Status < 400
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
	if workers < 1 {
		return nil, ErrInvalidWorkers
	}

	// Deduplicate: each distinct URL gets one slot in unique.
	index := make(map[string]int, len(urls))
	unique := make([]string, 0, len(urls))
	for _, u := range urls {
		if _, ok := index[u]; !ok {
			index[u] = len(unique)
			unique = append(unique, u)
		}
	}

	out := make([]LinkResult, len(unique))
	done := make([]bool, len(unique))

	jobs := make(chan int)
	var wg sync.WaitGroup
	for range min(workers, len(unique)) {
		wg.Go(func() {
			for i := range jobs {
				// A fetch must not start once ctx is done.
				if ctx.Err() != nil {
					continue
				}
				status, err := fetch(ctx, unique[i])
				if err != nil {
					status = 0
				}
				out[i] = LinkResult{URL: unique[i], Status: status, Err: err}
				done[i] = true
			}
		})
	}

	for i := range unique {
		if ctx.Err() != nil {
			break
		}
		select {
		case <-ctx.Done():
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()

	// Every worker has exited, so reading out and done is race-free.
	if ctxErr := ctx.Err(); ctxErr != nil {
		for i := range unique {
			if !done[i] {
				out[i] = LinkResult{URL: unique[i], Err: ctxErr}
			}
		}
	}

	results := make([]LinkResult, len(urls))
	for i, u := range urls {
		results[i] = out[index[u]]
	}
	return results, ctx.Err()
}
