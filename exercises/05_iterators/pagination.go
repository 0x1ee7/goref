package iterators

import (
	"context"
	"errors"
	"iter"
)

// ErrCursorLoop is returned (wrapped) when the server hands back a cursor that
// was already requested in this iteration.
var ErrCursorLoop = errors.New("pagination cursor loop")

// Page is one page of results from a paginated API.
type Page[T any] struct {
	Items []T
	// NextCursor is the cursor for the following page. The empty string means
	// there is no further page. An empty Items slice with a non-empty
	// NextCursor is legal.
	NextCursor string
}

// FetchFunc fetches one page. The empty cursor requests the first page. The
// function is injected so tests never touch the network.
type FetchFunc[T any] func(ctx context.Context, cursor string) (Page[T], error)

// Pages lazily yields each page. fetch is called for the first page on the
// first advance and for each subsequent page only when the consumer asks for
// it. A fetch error is yielded as (Page[T]{}, err) wrapping the cursor, and
// iteration ends. Before requesting a cursor, Pages checks that ctx is still
// live and that the cursor was not requested before (else ErrCursorLoop).
func Pages[T any](ctx context.Context, fetch FetchFunc[T]) iter.Seq2[Page[T], error] {
	return func(yield func(Page[T], error) bool) {}
}

// Paginate lazily yields every item of every page, in order. It is built on
// Pages. Errors follow Pages: yielded once as (zero, err), then iteration ends.
func Paginate[T any](ctx context.Context, fetch FetchFunc[T]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {}
}
