package ctxflow

import (
	"context"
	"iter"
)

// Page is one response of a cursor-paginated upstream. Next is empty on the last page.
type Page[T any] struct {
	Items []T
	Next  string
}

// Paginate returns a lazy sequence over every item of a cursor-paginated upstream.
// The first request uses cursor "". Pages are fetched only when an item of that
// page is needed, and an empty page with a non-empty Next is followed.
//
// A fetch error is yielded as (zero, err) after the items already produced, and
// the sequence then stops. Before each fetch the context is checked; if it is
// done, (zero, context.Cause(ctx)) is yielded and the sequence stops. Breaking
// out of the loop stops all further fetches.
func Paginate[T any](ctx context.Context, fetch func(ctx context.Context, cursor string) (Page[T], error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {}
}
