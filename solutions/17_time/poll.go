// Part 7 (continued): timers and tickers with a stop condition.
package timeutil

import (
	"context"
	"fmt"
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
	if interval <= 0 {
		return fmt.Errorf("%w: non-positive interval %v", ErrInvalid, interval)
	}
	if fn == nil {
		return fmt.Errorf("%w: nil function", ErrInvalid)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case tick := <-ticker.C:
			if err := fn(ctx, tick); err != nil {
				return err
			}
		}
	}
}
