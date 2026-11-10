package signatures

import (
	"context"
	"errors"
	"slices"
	"sync"
)

// ErrNilHandler reports a nil Handler passed to Subscribe.
var ErrNilHandler = errors.New("nil handler")

// Handler receives a published payload. Each handler gets its own copy of the
// payload slice, so one handler cannot change what another sees.
type Handler func(ctx context.Context, payload ...any) error

type subscription struct {
	id uint64
	h  Handler
}

// Bus is an in-process publish/subscribe hub. The zero value is ready to use
// and safe for concurrent use.
type Bus struct {
	mu     sync.Mutex
	nextID uint64
	subs   map[string][]subscription
}

// Subscribe registers h for topic and returns a cancel function. Cancel is
// idempotent and removes only this registration, even if the same Handler was
// subscribed more than once.
func (b *Bus) Subscribe(topic string, h Handler) (cancel func(), err error) {
	if h == nil {
		return nil, ErrNilHandler
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.subs == nil {
		b.subs = make(map[string][]subscription)
	}
	b.nextID++
	id := b.nextID
	b.subs[topic] = append(b.subs[topic], subscription{id: id, h: h})
	return func() { b.unsubscribe(topic, id) }, nil
}

func (b *Bus) unsubscribe(topic string, id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := slices.DeleteFunc(slices.Clone(b.subs[topic]), func(s subscription) bool {
		return s.id == id
	})
	if len(remaining) == 0 {
		delete(b.subs, topic)
		return
	}
	b.subs[topic] = remaining
}

// Publish delivers payload to a snapshot of the topic's subscribers, taken
// before any handler runs. Handlers subscribed during the publish wait for the
// next one, and handlers cancelled during it still run. Lock is not held while
// handlers run, so they may subscribe, cancel, or publish. All handlers run
// even after one fails, and errors are joined. If ctx is done, the remaining
// handlers are skipped and ctx.Err() is included.
func (b *Bus) Publish(ctx context.Context, topic string, payload ...any) error {
	b.mu.Lock()
	snapshot := slices.Clone(b.subs[topic])
	b.mu.Unlock()

	var errs []error
	for _, s := range snapshot {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if err := s.h(ctx, slices.Clone(payload)...); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
