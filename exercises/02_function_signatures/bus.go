package signatures

import (
	"context"
	"errors"
)

// ErrNilHandler is returned by Subscribe for a nil Handler.
var ErrNilHandler = errors.New("nil handler")

// Handler receives a published payload. Each handler receives its own copy of
// the payload slice.
type Handler func(ctx context.Context, payload ...any) error

// Bus is an in-process publish/subscribe hub for a single process. The zero
// value is ready to use and is safe for concurrent use.
type Bus struct{}

// Subscribe registers h for topic and returns a cancel function that removes
// this registration. cancel is idempotent. A nil h returns ErrNilHandler. The
// same handler may be subscribed more than once, and each registration is
// removed separately.
func (b *Bus) Subscribe(topic string, h Handler) (cancel func(), err error) {
	return func() {}, nil
}

// Publish calls the handlers for topic in subscription order with payload.
// Publish works on a snapshot: a handler added during the call is not called
// in it, and a handler cancelled during the call still runs in it. Every
// handler runs even when an earlier one fails. Their errors are joined with
// errors.Join. If ctx is done, the remaining handlers are skipped and
// ctx.Err() is included in the result. With no subscribers Publish returns nil.
func (b *Bus) Publish(ctx context.Context, topic string, payload ...any) error {
	return nil
}
