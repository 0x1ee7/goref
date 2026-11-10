package shoplog

import (
	"context"
	"errors"
	"log/slog"
	"slices"
)

// fanout delivers each record to every child that is enabled for its level.
type fanout struct {
	hs []slog.Handler
}

// NewFanout returns a handler that sends each record to all hs. Each enabled
// child receives its own Clone of the record, so one child cannot mutate what
// another sees. Errors from all children are joined with errors.Join, and no
// child is skipped because an earlier one failed. With no children nothing
// is enabled. See README, Part 4.
func NewFanout(hs ...slog.Handler) slog.Handler {
	return &fanout{hs: slices.Clone(hs)}
}

// Enabled reports whether any child is enabled for lv.
func (f *fanout) Enabled(ctx context.Context, lv slog.Level) bool {
	for _, h := range f.hs {
		if h.Enabled(ctx, lv) {
			return true
		}
	}
	return false
}

// Handle forwards r to each enabled child and joins their errors.
func (f *fanout) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.hs {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// WithAttrs applies the attributes to every child.
func (f *fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(f.hs))
	for i, h := range f.hs {
		out[i] = h.WithAttrs(attrs)
	}
	return &fanout{hs: out}
}

// WithGroup opens the group on every child.
func (f *fanout) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(f.hs))
	for i, h := range f.hs {
		out[i] = h.WithGroup(name)
	}
	return &fanout{hs: out}
}
