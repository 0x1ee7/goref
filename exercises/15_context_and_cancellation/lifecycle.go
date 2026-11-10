package ctxflow

import (
	"context"
	"io"
	"time"
)

// Component runs one long-lived function under a context that can be stopped
// with a cause.
type Component struct{}

// Start runs run in a new goroutine with a context derived from parent. The
// context is cancelled by Stop or by parent, whichever comes first.
func Start(parent context.Context, run func(ctx context.Context) error) *Component {
	return nil
}

// Stop cancels the component's context with cause. Only the first Stop
// (or parent cancellation) sets the cause; a nil cause means context.Canceled.
func (c *Component) Stop(cause error) {}

// Wait blocks until run returns and returns run's error.
func (c *Component) Wait() error { return nil }

// Cause returns the cancellation cause of the component's context, or nil
// while it is still running.
func (c *Component) Cause() error { return nil }

// RunWithCleanup runs body(ctx) and then cleanup, and cleanup runs exactly once
// on every path.
//
// If ctx is already done when RunWithCleanup is called, body is not started and
// the body error is context.Cause(ctx). Cleanup runs with a context that keeps
// the values of ctx but not its cancellation, bounded by cleanupTimeout.
// The result is errors.Join(bodyErr, cleanupErr), which is nil when both are nil.
// cleanupTimeout <= 0 wraps ErrInvalidArgument before anything runs.
func RunWithCleanup(ctx context.Context, cleanupTimeout time.Duration, body func(ctx context.Context) error, cleanup func(ctx context.Context) error) error {
	return nil
}

// CloseOnDone closes c once ctx is done, unless the returned stop function runs
// first. stop has the semantics of context.AfterFunc's stop: it reports true if
// it prevented the close. A Close error is passed to onErr when onErr is non-nil.
func CloseOnDone(ctx context.Context, c io.Closer, onErr func(error)) (stop func() bool) {
	return func() bool { return false }
}
