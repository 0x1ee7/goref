// Part 7 (continued): timers and tickers with a stop condition.
package timeutil

import (
	"context"
	"time"
)

// Poll calls fn once per interval tick until ctx is done or fn returns an error.
//
// fn runs on the polling goroutine, never concurrently with itself. Ticks that
// arrive while fn is running are dropped rather than queued, so a slow fn
// never causes a burst of back-to-back calls. fn's error stops the loop and is
// returned unchanged. When ctx is done, Poll returns ctx.Err(). A non-positive
// interval or a nil fn returns [ErrInvalid]. It never panics.
func Poll(ctx context.Context, interval time.Duration, fn func(ctx context.Context, tick time.Time) error) error {
	return errNotImplemented
}
