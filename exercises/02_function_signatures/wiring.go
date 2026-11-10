package signatures

import (
	"context"
	"errors"
	"time"
)

// ErrNilDependency is wrapped by NewReminders when a dependency is nil. The
// message names the parameter.
var ErrNilDependency = errors.New("nil dependency")

// Clock is the time source a Reminders uses. It is a one-method interface, so
// any type with a Now method satisfies it.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts an ordinary function to Clock.
type ClockFunc func() time.Time

// Now calls f.
func (f ClockFunc) Now() time.Time { return time.Time{} }

// Notifier delivers a message. It is the only side effect Reminders has.
type Notifier interface {
	Notify(ctx context.Context, msg string) error
}

// NotifierFunc adapts an ordinary function to Notifier. A method value such as
// rec.Notify converts with NotifierFunc(rec.Notify).
type NotifierFunc func(ctx context.Context, msg string) error

// Notify calls f.
func (f NotifierFunc) Notify(ctx context.Context, msg string) error { return nil }

// Reminders sends a notification once a due time has passed.
type Reminders struct{}

// NewReminders wires the dependencies. A nil clock or notifier returns an
// error wrapping ErrNilDependency whose message names the parameter.
func NewReminders(clock Clock, notifier Notifier) (*Reminders, error) {
	return nil, nil
}

// Check notifies once now is at or after due. The message is exactly
// "reminder due " followed by due in UTC, RFC3339. It returns sent=true only
// when the notification was delivered. A cancelled ctx returns before the
// clock is read. Errors are wrapped with "reminders: ". The named results
// exist for the deferred wrap.
func (r *Reminders) Check(ctx context.Context, due time.Time) (sent bool, err error) {
	return false, nil
}
