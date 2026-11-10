package ctxflow

import (
	"context"
	"iter"
)

// Page is one response from a cursor-paginated upstream. An empty Next means
// the last page.
type Page[T any] struct {
	Items []T
	Next  string
}

// Paginate returns a lazy sequence over every item of every page. Each page is
// fetched only when the consumer asks for it. The sequence yields a non-nil
// error at most once, as its final element. The error is the fetch error or
// the context cause, and it comes after the items already yielded.
func Paginate[T any](ctx context.Context, fetch func(ctx context.Context, cursor string) (Page[T], error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		cursor := ""
		for {
			if err := ctx.Err(); err != nil {
				yield(zero, context.Cause(ctx))
				return
			}
			page, err := fetch(ctx, cursor)
			if err != nil {
				yield(zero, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			if page.Next == "" {
				return
			}
			cursor = page.Next
		}
	}
}
