package shoplog

import (
	"context"
	"log/slog"
)

// WithRequestID returns a context carrying the request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return ctx
}

// RequestIDFromContext returns the id stored by WithRequestID, and whether
// the context carries one.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	return "", false
}

// NewRequestIDHandler wraps next. When a record is handled with a context
// that carries a non-empty request id, and the record has no request_id
// attribute of its own, the handler adds request_id to a clone of the record
// before delegating. See README, Part 4.
func NewRequestIDHandler(next slog.Handler) slog.Handler {
	return nil
}
