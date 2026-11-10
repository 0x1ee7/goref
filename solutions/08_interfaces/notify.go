package interfaces

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"golang.org/x/sync/semaphore"
)

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
func (f NotifierFunc) Notify(ctx context.Context, ev Event) error { return f(ctx, ev) }

// Option configures a Dispatcher.
type Option func(*Dispatcher)

// WithConcurrency limits how many notifiers run at once. n <= 0 means unlimited.
func WithConcurrency(n int) Option {
	return func(d *Dispatcher) { d.limit = n }
}

// Dispatcher fans an Event out to every notifier (Part 5).
type Dispatcher struct {
	notifiers []Notifier
	limit     int
}

// NewDispatcher returns a Dispatcher over a copy of notifiers.
func NewDispatcher(notifiers []Notifier, opts ...Option) *Dispatcher {
	d := &Dispatcher{notifiers: slices.Clone(notifiers)}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Dispatch delivers ev to every notifier and waits for all of them. Failures are
// wrapped with the notifier's position and joined in registration order. A
// notifier that cannot get a concurrency slot because ctx ended is reported as
// a failure and is not called.
func (d *Dispatcher) Dispatch(ctx context.Context, ev Event) error {
	var sem *semaphore.Weighted
	if d.limit > 0 {
		sem = semaphore.NewWeighted(int64(d.limit))
	}

	errs := make([]error, len(d.notifiers))
	var wg sync.WaitGroup
	for i, n := range d.notifiers {
		wg.Go(func() {
			if sem != nil {
				if err := sem.Acquire(ctx, 1); err != nil {
					errs[i] = fmt.Errorf("notifier %d: %w", i, err)
					return
				}
				defer sem.Release(1)
			}
			if err := n.Notify(ctx, ev); err != nil {
				errs[i] = fmt.Errorf("notifier %d: %w", i, err)
			}
		})
	}
	wg.Wait()
	return errors.Join(errs...)
}
