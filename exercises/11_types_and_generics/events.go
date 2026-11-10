package typed

import "context"

// Handler handles one event of type E. A returned error does not stop the
// other handlers of the same publish.
type Handler[E any] func(ctx context.Context, event E) error

// Bus routes events to handlers. Routing is keyed by the exact type argument E
// given to Subscribe and Publish. The zero value is ready to use and the Bus is
// safe for concurrent use.
type Bus struct{}

// Subscribe registers h for events of type E and returns a function that removes
// exactly this registration. The returned function is idempotent and may be
// called from inside a handler.
func Subscribe[E any](b *Bus, h Handler[E]) (cancel func()) {
	return func() {}
}

// Publish delivers event to every handler of type E that was registered when
// Publish started. Handlers added during dispatch miss this event; handlers
// cancelled during dispatch are skipped. It returns the number of handlers
// invoked and the errors.Join of their errors (nil if none failed).
//
// Before each handler, Publish checks ctx. If ctx is done, dispatch stops and
// ctx.Err() is included in the joined error.
func Publish[E any](ctx context.Context, b *Bus, event E) (int, error) {
	return 0, nil
}
