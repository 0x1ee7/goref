package iterators

import (
	"context"
	"errors"
	"fmt"
	"iter"
)

// ErrCursorLoop reports that a page's NextCursor was already seen, which would
// otherwise make pagination run forever.
var ErrCursorLoop = errors.New("pagination cursor loop")

// Page is one response from a paginated API. An empty NextCursor marks the last page.
type Page[T any] struct {
	Items      []T
	NextCursor string
}

// FetchFunc fetches the page identified by cursor. The empty cursor requests the first page.
type FetchFunc[T any] func(ctx context.Context, cursor string) (Page[T], error)

// Pages lazily yields each page in order. A page is fetched only when the
// consumer asks for it. Before each fetch the context is checked. A fetch
// error is yielded once, wrapped with the cursor, and ends iteration.
func Pages[T any](ctx context.Context, fetch FetchFunc[T]) iter.Seq2[Page[T], error] {
	return func(yield func(Page[T], error) bool) {
		seen := map[string]bool{"": true}
		cursor := ""
		for {
			if err := ctx.Err(); err != nil {
				yield(Page[T]{}, fmt.Errorf("pagination: %w", err))
				return
			}
			page, err := fetch(ctx, cursor)
			if err != nil {
				yield(Page[T]{}, fmt.Errorf("fetch cursor %q: %w", cursor, err))
				return
			}
			if !yield(page, nil) {
				return
			}
			next := page.NextCursor
			if next == "" {
				return
			}
			if seen[next] {
				yield(Page[T]{}, fmt.Errorf("cursor %q: %w", next, ErrCursorLoop))
				return
			}
			seen[next] = true
			cursor = next
		}
	}
}

// Paginate lazily yields every item across all pages, built on Pages. Errors
// are yielded as (zero, err) and end iteration.
func Paginate[T any](ctx context.Context, fetch FetchFunc[T]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for page, err := range Pages(ctx, fetch) {
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
		}
	}
}
