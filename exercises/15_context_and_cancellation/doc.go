// Package ctxflow is the context-and-cancellation exercise: the internals of a
// service that talks to flaky upstreams. Each part builds one time-bounded
// component; the README is the spec and the tests are its executable form.
//
// Conventions shared by every part:
//
//   - Context-driven errors are context.Cause(ctx), not ctx.Err(), so a caller
//     can recover why a context was cancelled (see context.WithCancelCause).
//   - Configuration errors wrap ErrInvalidArgument.
//   - Using a component after Close or Shutdown returns ErrClosed.
package ctxflow

import "errors"

// ErrInvalidArgument is wrapped by every constructor or method that rejects a
// configuration value (non-positive timeouts, sizes, rates, ...).
var ErrInvalidArgument = errors.New("invalid argument")

// ErrClosed is returned by a component that was closed or shut down.
var ErrClosed = errors.New("closed")
