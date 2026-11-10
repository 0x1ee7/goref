// Package ctxflow is the context and cancellation exercise: a small service
// that talks to flaky upstreams and must bound, cancel, and shut down its work.
package ctxflow

import "errors"

// ErrInvalidArgument is wrapped by constructors and calls given a
// configuration that cannot work (non-positive budgets, negative sizes).
var ErrInvalidArgument = errors.New("ctxflow: invalid argument")

// ErrClosed is returned when a pool, broker, or subscription is used after it
// has been closed or shut down.
var ErrClosed = errors.New("ctxflow: closed")
