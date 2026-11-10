package ctxflow

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Broker fans each published message out to every subscriber. A subscriber
// whose buffer is full gets the message dropped once the publish's patience
// budget runs out, so one slow consumer cannot stall the others.
type Broker[M any] struct {
	patience time.Duration

	mu     sync.RWMutex
	closed bool
	subs   map[*Subscription[M]]struct{}
}

// Subscription is one consumer's view of a Broker. C is closed when the
// subscription ends, after any buffered messages have been read.
type Subscription[M any] struct {
	C <-chan M

	broker  *Broker[M]
	ch      chan M
	done    chan struct{}
	once    sync.Once
	// mu guards gone and ch's close: deliveries hold RLock, stop holds Lock.
	mu      sync.RWMutex
	gone    bool
	dropped atomic.Int64
}

// NewBroker returns a broker whose publishes wait at most patience for a full
// subscriber before dropping the message for it. patience must not be negative.
func NewBroker[M any](patience time.Duration) (*Broker[M], error) {
	if patience < 0 {
		return nil, fmt.Errorf("%w: patience must be >= 0, got %v", ErrInvalidArgument, patience)
	}
	return &Broker[M]{patience: patience, subs: make(map[*Subscription[M]]struct{})}, nil
}

// Subscribe registers a subscriber with a channel buffer of the given size.
func (b *Broker[M]) Subscribe(buffer int) (*Subscription[M], error) {
	if buffer < 0 {
		return nil, fmt.Errorf("%w: buffer must be >= 0, got %d", ErrInvalidArgument, buffer)
	}
	ch := make(chan M, buffer)
	s := &Subscription[M]{C: ch, broker: b, ch: ch, done: make(chan struct{})}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ErrClosed
	}
	b.subs[s] = struct{}{}
	return s, nil
}

// Publish delivers msg to every current subscriber. The patience budget is
// shared by all subscribers for this one publish. It returns
// context.Cause(ctx) if ctx ends, and ErrClosed after Close.
func (b *Broker[M]) Publish(ctx context.Context, msg M) error {
	if err := ctx.Err(); err != nil {
		return context.Cause(ctx)
	}
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrClosed
	}
	subs := slices.Collect(maps.Keys(b.subs))
	b.mu.RUnlock()

	pctx, cancel := context.WithTimeout(ctx, b.patience)
	defer cancel()
	for _, s := range subs {
		if err := s.deliver(ctx, pctx, msg); err != nil {
			return err
		}
	}
	return nil
}

// Close ends every subscription. Buffered messages stay readable before each
// channel closes. Later Publish and Subscribe calls return ErrClosed.
func (b *Broker[M]) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	subs := slices.Collect(maps.Keys(b.subs))
	b.subs = nil
	b.mu.Unlock()
	for _, s := range subs {
		s.stop()
	}
}

// C-free delivery path. A non-blocking try goes first, so a message that fits
// in the buffer is never dropped just because the budget is already spent.
func (s *Subscription[M]) deliver(ctx, pctx context.Context, msg M) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.gone {
		return nil
	}
	select {
	case s.ch <- msg:
		return nil
	default:
	}
	select {
	case s.ch <- msg:
	case <-s.done:
	case <-pctx.Done():
		if err := ctx.Err(); err != nil {
			return context.Cause(ctx)
		}
		s.dropped.Add(1)
	}
	return nil
}

// Unsubscribe ends this subscription. It is safe to call more than once and
// concurrently with Publish.
func (s *Subscription[M]) Unsubscribe() {
	s.broker.mu.Lock()
	delete(s.broker.subs, s)
	s.broker.mu.Unlock()
	s.stop()
}

// stop unblocks any in-flight delivery, then closes the channel once no send
// can be running.
func (s *Subscription[M]) stop() {
	s.once.Do(func() {
		close(s.done)
		s.mu.Lock()
		s.gone = true
		close(s.ch)
		s.mu.Unlock()
	})
}

// Dropped reports how many messages were dropped for this subscriber because
// its buffer stayed full past the publish's patience budget.
func (s *Subscription[M]) Dropped() int { return int(s.dropped.Load()) }

var _ = errors.New
