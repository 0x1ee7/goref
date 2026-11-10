package typed

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
)

// Handler handles one event of type E. A returned error does not stop the
// other handlers of the same publish.
type Handler[E any] func(ctx context.Context, event E) error

// registration is one Subscribe call. The id distinguishes two registrations
// of the same function.
type registration struct {
	id uint64
	h  any // always a Handler[E] for the E that keys its list
}

// Bus routes events to handlers. Routing is keyed by the exact type argument E
// given to Subscribe and Publish. The zero value is ready to use and the Bus is
// safe for concurrent use.
type Bus struct {
	mu     sync.Mutex
	nextID uint64
	subs   map[reflect.Type][]registration
}

// Subscribe registers h for events of type E and returns a function that removes
// exactly this registration. The returned function is idempotent and may be
// called from inside a handler.
func Subscribe[E any](b *Bus, h Handler[E]) (cancel func()) {
	key := reflect.TypeFor[E]()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	if b.subs == nil {
		b.subs = make(map[reflect.Type][]registration)
	}
	b.subs[key] = append(b.subs[key], registration{id: id, h: h})
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.subs[key] = slices.DeleteFunc(b.subs[key], func(r registration) bool {
			return r.id == id
		})
	}
}

// Publish delivers event to every handler of type E that was registered when
// Publish started. Handlers added during dispatch miss this event; handlers
// cancelled during dispatch are skipped. It returns the number of handlers
// invoked and the errors.Join of their errors (nil if none failed).
//
// Before each handler, Publish checks ctx. If ctx is done, dispatch stops and
// ctx.Err() is included in the joined error.
func Publish[E any](ctx context.Context, b *Bus, event E) (int, error) {
	key := reflect.TypeFor[E]()
	b.mu.Lock()
	snapshot := slices.Clone(b.subs[key])
	b.mu.Unlock()

	var (
		invoked int
		errs    []error
	)
	for _, r := range snapshot {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		if !b.registered(key, r.id) {
			continue
		}
		invoked++
		if err := r.h.(Handler[E])(ctx, event); err != nil {
			errs = append(errs, err)
		}
	}
	return invoked, errors.Join(errs...)
}

// registered reports whether the registration id is still subscribed under key.
func (b *Bus) registered(key reflect.Type, id uint64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.ContainsFunc(b.subs[key], func(r registration) bool {
		return r.id == id
	})
}
