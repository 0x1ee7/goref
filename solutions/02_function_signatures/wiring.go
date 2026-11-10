package signatures

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNilDependency reports a nil collaborator passed to NewReminders.
var ErrNilDependency = errors.New("nil dependency")

// Clock is the time source. Consumers depend on this one-method interface, so
// tests can substitute a fixed time.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts an ordinary function to Clock, the way http.HandlerFunc
// adapts a function to http.Handler.
type ClockFunc func() time.Time

// Now calls f.
func (f ClockFunc) Now() time.Time { return f() }

// Notifier delivers a reminder message.
type Notifier interface {
	Notify(ctx context.Context, msg string) error
}

// NotifierFunc adapts an ordinary function to Notifier.
type NotifierFunc func(ctx context.Context, msg string) error

// Notify calls f.
func (f NotifierFunc) Notify(ctx context.Context, msg string) error { return f(ctx, msg) }

// Reminders sends a notification once a due time has passed.
type Reminders struct {
	clock    Clock
	notifier Notifier
}

// NewReminders returns an error wrapping ErrNilDependency, naming the parameter,
// when either dependency is nil.
func NewReminders(clock Clock, notifier Notifier) (*Reminders, error) {
	if clock == nil {
		return nil, fmt.Errorf("%w: clock", ErrNilDependency)
	}
	if notifier == nil {
		return nil, fmt.Errorf("%w: notifier", ErrNilDependency)
	}
	return &Reminders{clock: clock, notifier: notifier}, nil
}

// Check sends a reminder if due has passed according to the clock. It reports
// sent only when the notifier succeeded. A cancelled ctx is reported before the
// clock is read. Errors are wrapped with a "reminders: " prefix.
func (r *Reminders) Check(ctx context.Context, due time.Time) (sent bool, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("reminders: %w", err)
		}
	}()

	if err = ctx.Err(); err != nil {
		return false, err
	}
	if r.clock.Now().Before(due) {
		return false, nil
	}
	msg := "reminder due " + due.UTC().Format(time.RFC3339)
	if err = r.notifier.Notify(ctx, msg); err != nil {
		return false, err
	}
	return true, nil
}
