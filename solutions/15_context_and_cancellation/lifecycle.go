package ctxflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Component is a long-running function owned by its starter. It runs until it
// returns or is stopped, and Wait reports its result.
type Component struct {
	cancel context.CancelCauseFunc
	done   chan struct{}
	err    error
	cause  error
}

// Start runs run in a new goroutine under a context derived from parent.
func Start(parent context.Context, run func(ctx context.Context) error) *Component {
	ctx, cancel := context.WithCancelCause(parent)
	c := &Component{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(c.done)
		c.err = run(ctx)
		c.cause = context.Cause(ctx)
		cancel(nil)
	}()
	return c
}

// Stop cancels the component's context with cause. Only the first cause is
// kept; later calls are no-ops.
func (c *Component) Stop(cause error) {
	c.cancel(cause)
}

// Wait blocks until run returns and reports its error.
func (c *Component) Wait() error {
	<-c.done
	return c.err
}

// Cause reports why the component's context ended. It returns nil until the
// component has finished, and nil if the context was never cancelled.
func (c *Component) Cause() error {
	select {
	case <-c.done:
		return c.cause
	default:
		return nil
	}
}

// RunWithCleanup runs body, then cleanup. If ctx is already done, body is
// skipped and its error is context.Cause(ctx). Cleanup always runs, under a
// context that is not cancelled by ctx but is bounded by cleanupTimeout, so
// it can still flush and close after cancellation. The result joins body's and
// cleanup's errors.
func RunWithCleanup(ctx context.Context, cleanupTimeout time.Duration, body, cleanup func(ctx context.Context) error) error {
	if cleanupTimeout <= 0 {
		return fmt.Errorf("%w: cleanup timeout must be positive", ErrInvalidArgument)
	}
	var bodyErr error
	if ctx.Err() != nil {
		bodyErr = context.Cause(ctx)
	} else {
		bodyErr = body(ctx)
	}
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	return errors.Join(bodyErr, cleanup(cctx))
}

// CloseOnDone closes c when ctx ends. onErr, if non-nil, receives a Close
// error. The returned stop function prevents the close if it has not started
// yet; it reports true if it did so and false if the close already ran or is
// running.
func CloseOnDone(ctx context.Context, c io.Closer, onErr func(error)) (stop func() bool) {
	return context.AfterFunc(ctx, func() {
		if err := c.Close(); err != nil && onErr != nil {
			onErr(err)
		}
	})
}
