package interfaces

import "context"

// Event is something worth telling people about.
type Event struct {
	Kind    string
	Message string
}

// Notifier delivers an Event to one channel (email, Slack, webhook...).
type Notifier interface {
	Notify(ctx context.Context, ev Event) error
}

// NotifierFunc adapts an ordinary function to Notifier (Part 5).
type NotifierFunc func(ctx context.Context, ev Event) error

// Notify calls f.
func (f NotifierFunc) Notify(ctx context.Context, ev Event) error { return nil }

// Option configures a Dispatcher.
type Option func(*Dispatcher)

// WithConcurrency limits how many notifiers run at once. n <= 0 means unlimited.
func WithConcurrency(n int) Option { return func(*Dispatcher) {} }

// Dispatcher fans an Event out to every notifier (Part 5).
type Dispatcher struct{}

// NewDispatcher returns a Dispatcher over a copy of notifiers.
func NewDispatcher(notifiers []Notifier, opts ...Option) *Dispatcher { return &Dispatcher{} }

// Dispatch delivers ev to every notifier and waits for all of them. Failures are
// wrapped with the notifier's position and joined in registration order.
func (d *Dispatcher) Dispatch(ctx context.Context, ev Event) error { return nil }
