package ctxflow

import (
	"context"
	"time"
)

// Broker fans every published message out to all current subscribers. Each
// subscriber has its own buffered channel.
type Broker[M any] struct{}

// NewBroker returns a broker whose publishers wait at most patience for a full
// subscriber buffer to make room before dropping the message for that
// subscriber. patience < 0 wraps ErrInvalidArgument.
func NewBroker[M any](patience time.Duration) (*Broker[M], error) {
	return nil, nil
}

// Subscribe registers a subscriber with a channel buffer of buffer messages.
// buffer < 0 wraps ErrInvalidArgument; a closed broker returns ErrClosed.
func (b *Broker[M]) Subscribe(buffer int) (*Subscription[M], error) {
	return nil, nil
}

// Publish delivers msg to every current subscriber in subscription order.
//
// A subscriber with room in its buffer receives immediately. A full subscriber
// is waited for until the publish's patience budget runs out, and then the
// message is dropped for that subscriber and counted in Dropped. Publish returns
// context.Cause(ctx) if ctx ends while it waits; subscribers after that point do
// not receive the message. Publish on a closed broker returns ErrClosed.
func (b *Broker[M]) Publish(ctx context.Context, msg M) error {
	return nil
}

// Close unsubscribes everyone and makes later Subscribe and Publish calls fail
// with ErrClosed. Messages already buffered stay readable before each channel
// is closed. Close is idempotent.
func (b *Broker[M]) Close() {}

// Subscription is one subscriber's view of a broker. Receive from C; the channel
// is closed when the subscription ends.
type Subscription[M any] struct {
	C <-chan M
}

// Unsubscribe ends the subscription and closes C. It is safe to call more than
// once and concurrently with Publish: it never causes a send on a closed channel.
func (s *Subscription[M]) Unsubscribe() {}

// Dropped returns how many messages were dropped for this subscriber because
// its buffer stayed full past the publisher's patience.
func (s *Subscription[M]) Dropped() int { return 0 }
