package ctxflow_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/15_context_and_cancellation"
)

// pages serves a fixed sequence of pages keyed by cursor and records every request.
func pages(seq map[string]ctxflow.Page[int], requested *[]string) func(context.Context, string) (ctxflow.Page[int], error) {
	return func(_ context.Context, cursor string) (ctxflow.Page[int], error) {
		*requested = append(*requested, cursor)
		p, ok := seq[cursor]
		if !ok {
			return ctxflow.Page[int]{}, errors.New("unknown cursor " + cursor)
		}
		return p, nil
	}
}

func TestPart4Paginate(t *testing.T) {
	t.Run("walks every page in order and stops at empty Next", func(t *testing.T) {
		var requested []string
		seq := map[string]ctxflow.Page[int]{
			"":   {Items: []int{1, 2}, Next: "p2"},
			"p2": {Items: []int{3}, Next: "p3"},
			"p3": {Items: []int{4, 5}},
		}
		var got []int
		for v, err := range ctxflow.Paginate(t.Context(), pages(seq, &requested)) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, v)
		}
		if diff := cmp.Diff([]int{1, 2, 3, 4, 5}, got); diff != "" {
			t.Errorf("items mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{"", "p2", "p3"}, requested); diff != "" {
			t.Errorf("cursors mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("an empty page with a cursor is followed", func(t *testing.T) {
		var requested []string
		seq := map[string]ctxflow.Page[int]{
			"":   {Next: "p2"},
			"p2": {Items: []int{7}},
		}
		var got []int
		for v, err := range ctxflow.Paginate(t.Context(), pages(seq, &requested)) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = append(got, v)
		}
		if diff := cmp.Diff([]int{7}, got); diff != "" {
			t.Errorf("items mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("pages are fetched lazily and breaking stops further fetches", func(t *testing.T) {
		var calls atomic.Int32
		fetch := func(_ context.Context, cursor string) (ctxflow.Page[int], error) {
			calls.Add(1)
			return ctxflow.Page[int]{Items: []int{1, 2, 3}, Next: "more"}, nil
		}
		for v := range ctxflow.Paginate(t.Context(), fetch) {
			_ = v
			break
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("fetch called %d times, want 1 after breaking on the first item", n)
		}
	})

	t.Run("a fetch error follows the items already yielded and ends the sequence", func(t *testing.T) {
		errUpstream := errors.New("upstream 503")
		var calls atomic.Int32
		fetch := func(_ context.Context, cursor string) (ctxflow.Page[int], error) {
			calls.Add(1)
			if cursor == "" {
				return ctxflow.Page[int]{Items: []int{1}, Next: "p2"}, nil
			}
			return ctxflow.Page[int]{}, errUpstream
		}
		var items []int
		var gotErr error
		for v, err := range ctxflow.Paginate(t.Context(), fetch) {
			if err != nil {
				gotErr = err
				continue
			}
			items = append(items, v)
		}
		if diff := cmp.Diff([]int{1}, items); diff != "" {
			t.Errorf("items mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(gotErr, errUpstream) {
			t.Errorf("error = %v, want errUpstream", gotErr)
		}
		if n := calls.Load(); n != 2 {
			t.Errorf("fetch called %d times, want 2 (no fetch after the error)", n)
		}
	})

	t.Run("cancellation before the next page yields the cause and stops fetching", func(t *testing.T) {
		errQuota := errors.New("quota exhausted")
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)
		var calls atomic.Int32
		fetch := func(_ context.Context, cursor string) (ctxflow.Page[int], error) {
			calls.Add(1)
			cancel(errQuota)
			return ctxflow.Page[int]{Items: []int{9}, Next: "more"}, nil
		}
		var items []int
		var gotErr error
		for v, err := range ctxflow.Paginate(ctx, fetch) {
			if err != nil {
				gotErr = err
				break
			}
			items = append(items, v)
		}
		if diff := cmp.Diff([]int{9}, items); diff != "" {
			t.Errorf("items mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(gotErr, errQuota) {
			t.Errorf("error = %v, want errQuota", gotErr)
		}
		if n := calls.Load(); n != 1 {
			t.Errorf("fetch called %d times, want 1", n)
		}
	})
}
