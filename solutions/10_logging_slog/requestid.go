package shoplog

import (
	"context"
	"log/slog"
)

type requestIDKey struct{}

// WithRequestID returns a context carrying id for NewRequestIDHandler and
// RequestIDFromContext.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the id stored by WithRequestID, and whether the
// context carries one.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey{}).(string)
	return id, ok
}

// requestIDHandler adds request_id from the context to records that do not
// already set one. It remembers whether an explicit request_id was attached
// with With at the top level, so the attribute is never duplicated.
type requestIDHandler struct {
	next      slog.Handler
	groupOpen bool
	explicit  bool // a top-level request_id was attached via WithAttrs
}

// NewRequestIDHandler wraps next so that every record logged with a context
// carrying a non-empty request id gets a request_id attribute. An explicit
// request_id on the record or from With wins. Inside an open group the
// attribute lands in that group. See README, Part 4.
func NewRequestIDHandler(next slog.Handler) slog.Handler {
	return &requestIDHandler{next: next}
}

// Enabled delegates to the wrapped handler.
func (h *requestIDHandler) Enabled(ctx context.Context, lv slog.Level) bool {
	return h.next.Enabled(ctx, lv)
}

// Handle clones the record before adding the attribute, so the caller's
// record is never modified.
func (h *requestIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if id, ok := RequestIDFromContext(ctx); ok && id != "" && !h.explicit && !hasAttr(r, "request_id") {
		r = r.Clone()
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs forwards the attributes and records an explicit top-level request_id.
func (h *requestIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := *h
	nh.next = h.next.WithAttrs(attrs)
	if !h.groupOpen {
		for _, a := range attrs {
			if a.Key == "request_id" {
				nh.explicit = true
			}
		}
	}
	return &nh
}

// WithGroup forwards the group and marks subsequent attributes as nested.
func (h *requestIDHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := *h
	nh.next = h.next.WithGroup(name)
	nh.groupOpen = true
	return &nh
}

// hasAttr reports whether the record carries a top-level attribute named key.
func hasAttr(r slog.Record, key string) bool {
	found := false
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			found = true
			return false
		}
		return true
	})
	return found
}
