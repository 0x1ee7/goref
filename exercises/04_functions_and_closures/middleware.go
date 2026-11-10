package funcs

import (
	"context"
	"net/http"
	"time"
)

// Middleware wraps an http.Handler with additional behaviour.
type Middleware func(http.Handler) http.Handler

// Sample is what Instrument records for one request.
type Sample struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
}

// Chain composes middlewares. The first argument is the outermost:
// Chain(a, b)(h) behaves as a(b(h)). With no middlewares it returns the
// handler unchanged.
func Chain(mws ...Middleware) Middleware {
	return func(next http.Handler) http.Handler { return next }
}

// RequestID uses the incoming X-Request-Id header when present (gen is not
// called); otherwise it calls gen. The id is set on the response header and
// stored in the request context for RequestIDFrom.
func RequestID(gen func() string) Middleware {
	return func(next http.Handler) http.Handler { return next }
}

// RequestIDFrom returns the id stored by RequestID, or "" if there is none.
func RequestIDFrom(ctx context.Context) string { return "" }

// Recover converts a panic in the wrapped handler into a logged 500 response.
// It logs via logf with format "panic serving %s %s: %v\n%s" (method, path,
// panic value, stack). If response headers were already written it does not
// write a second status. A panic value of http.ErrAbortHandler is re-panicked
// without logging or responding.
func Recover(logf func(format string, args ...any)) Middleware {
	return func(next http.Handler) http.Handler { return next }
}

// Instrument calls now before and after the wrapped handler and calls record
// once with the Method, URL path, final status (200 if none was written) and
// elapsed duration. If the handler panics, record is not called.
func Instrument(now func() time.Time, record func(Sample)) Middleware {
	return func(next http.Handler) http.Handler { return next }
}
