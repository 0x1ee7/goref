package funcs

import "context"

// Event is a message delivered to handlers.
type Event struct {
	Kind    string
	Payload string
}

// Handler consumes events.
type Handler interface {
	Handle(ctx context.Context, e Event) error
}

// HandlerFunc adapts an ordinary function to Handler, as net/http does.
type HandlerFunc func(ctx context.Context, e Event) error

// Handle calls f(ctx, e).
func (f HandlerFunc) Handle(ctx context.Context, e Event) error { return nil }

// Fanout delivers each event to every handler in order, never stopping early.
// It returns errors.Join of the handler errors, or nil if none failed.
func Fanout(hs ...Handler) Handler {
	return HandlerFunc(func(ctx context.Context, e Event) error { return nil })
}

// Only forwards events whose Kind equals kind to h. Other events are dropped
// and return nil.
func Only(kind string, h Handler) Handler {
	return HandlerFunc(func(ctx context.Context, e Event) error { return nil })
}

// BindMethod binds recv to a method expression such as (*Recorder).Record and
// returns a Handler that calls it.
func BindMethod[R any](recv R, m func(R, context.Context, Event) error) Handler {
	return HandlerFunc(func(ctx context.Context, e Event) error { return nil })
}

// Recorder is a Handler that keeps the events it receives. The zero value is
// ready to use and it is safe for concurrent use.
type Recorder struct{}

// Record appends e to the recorder.
func (r *Recorder) Record(ctx context.Context, e Event) error { return nil }

// Events returns a copy of the recorded events, in record order.
func (r *Recorder) Events() []Event { return nil }
