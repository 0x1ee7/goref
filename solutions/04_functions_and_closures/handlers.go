package funcs

import (
	"context"
	"errors"
	"slices"
	"sync"
)

// Event is a named message with a payload.
type Event struct {
	Kind    string
	Payload string
}

// Handler processes events.
type Handler interface {
	Handle(ctx context.Context, e Event) error
}

// HandlerFunc adapts an ordinary function to the Handler interface.
type HandlerFunc func(ctx context.Context, e Event) error

// Handle calls f(ctx, e).
func (f HandlerFunc) Handle(ctx context.Context, e Event) error {
	return f(ctx, e)
}

// Fanout returns a Handler that delivers each event to every handler in order.
// It always calls all of them and joins their errors, or returns nil if none
// failed.
func Fanout(hs ...Handler) Handler {
	return HandlerFunc(func(ctx context.Context, e Event) error {
		var errs []error
		for _, h := range hs {
			if err := h.Handle(ctx, e); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	})
}

// Only returns a Handler that forwards events whose Kind equals kind and drops
// all others with a nil error.
func Only(kind string, h Handler) Handler {
	return HandlerFunc(func(ctx context.Context, e Event) error {
		if e.Kind != kind {
			return nil
		}
		return h.Handle(ctx, e)
	})
}

// BindMethod binds recv to a method expression, producing a Handler that calls
// m(recv, ctx, e).
func BindMethod[R any](recv R, m func(R, context.Context, Event) error) Handler {
	return HandlerFunc(func(ctx context.Context, e Event) error {
		return m(recv, ctx, e)
	})
}

// Recorder keeps every event it is given. The zero value is ready to use and
// safe for concurrent use.
type Recorder struct {
	mu     sync.Mutex
	events []Event
}

// Record appends e. It never fails.
func (r *Recorder) Record(_ context.Context, e Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return nil
}

// Events returns a copy of the recorded events in arrival order.
func (r *Recorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}
