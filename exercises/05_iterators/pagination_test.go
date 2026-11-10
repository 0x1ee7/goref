package iterators_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	iterators "goref/exercises/05_iterators"
)

// fakeAPI serves canned pages keyed by cursor and records every requested cursor.
type fakeAPI struct {
	pages map[string]iterators.Page[int]
	errs  map[string]error
	// hook, if set, runs after a page has been served (used to cancel a context).
	hook  func(cursor string)
	calls []string
}

func (f *fakeAPI) fetch(_ context.Context, cursor string) (iterators.Page[int], error) {
	f.calls = append(f.calls, cursor)
	if err := f.errs[cursor]; err != nil {
		return iterators.Page[int]{}, err
	}
	p, ok := f.pages[cursor]
	if !ok {
		return iterators.Page[int]{}, fmt.Errorf("unknown cursor %q", cursor)
	}
	if f.hook != nil {
		f.hook(cursor)
	}
	return p, nil
}

// threePages: first page has items, middle page is empty but not last, last page has items.
func threePages() *fakeAPI {
	return &fakeAPI{pages: map[string]iterators.Page[int]{
		"":   {Items: []int{1, 2}, NextCursor: "c1"},
		"c1": {NextCursor: "c2"},
		"c2": {Items: []int{3}},
	}}
}

func TestPart5Pages(t *testing.T) {
	t.Run("yields every page including empty ones", func(t *testing.T) {
		api := threePages()
		pages, err := drain(t, iterators.Pages(t.Context(), api.fetch))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []iterators.Page[int]{
			{Items: []int{1, 2}, NextCursor: "c1"},
			{NextCursor: "c2"},
			{Items: []int{3}},
		}
		if diff := cmp.Diff(want, pages); diff != "" {
			t.Errorf("pages mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{"", "c1", "c2"}, api.calls); diff != "" {
			t.Errorf("requested cursors mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fetches only on demand", func(t *testing.T) {
		api := threePages()
		for p, err := range iterators.Pages(t.Context(), api.fetch) {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(p.Items) > 0 {
				break
			}
		}
		if diff := cmp.Diff([]string{""}, api.calls); diff != "" {
			t.Errorf("requested cursors mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fetch error is yielded once and ends iteration", func(t *testing.T) {
		api := threePages()
		api.errs = map[string]error{"c1": errBoom}
		pages, err := drain(t, iterators.Pages(t.Context(), api.fetch))
		if diff := cmp.Diff([]iterators.Page[int]{{Items: []int{1, 2}, NextCursor: "c1"}}, pages); diff != "" {
			t.Errorf("pages mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, errBoom) {
			t.Errorf("err = %v, want errBoom", err)
		}
		if diff := cmp.Diff([]string{"", "c1"}, api.calls); diff != "" {
			t.Errorf("requested cursors mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPart5Paginate(t *testing.T) {
	t.Run("yields all items across pages", func(t *testing.T) {
		items, err := drain(t, iterators.Paginate(t.Context(), threePages().fetch))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if diff := cmp.Diff([]int{1, 2, 3}, items); diff != "" {
			t.Errorf("items mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("first item costs one page fetch", func(t *testing.T) {
		api := threePages()
		for v := range iterators.Paginate(t.Context(), api.fetch) {
			if v != 1 {
				t.Fatalf("first item = %d, want 1", v)
			}
			break
		}
		if diff := cmp.Diff([]string{""}, api.calls); diff != "" {
			t.Errorf("requested cursors mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("break after a page's last item does not fetch the next page", func(t *testing.T) {
		api := threePages()
		var got []int
		for v := range iterators.Paginate(t.Context(), api.fetch) {
			got = append(got, v)
			if v == 2 {
				break
			}
		}
		if diff := cmp.Diff([]int{1, 2}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{""}, api.calls); diff != "" {
			t.Errorf("requested cursors mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("next page is fetched when its item is requested", func(t *testing.T) {
		api := threePages()
		var got []int
		for v := range iterators.Paginate(t.Context(), api.fetch) {
			got = append(got, v)
			if v == 3 {
				break
			}
		}
		if diff := cmp.Diff([]int{1, 2, 3}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{"", "c1", "c2"}, api.calls); diff != "" {
			t.Errorf("requested cursors mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("fetch error surfaces after the items before it", func(t *testing.T) {
		api := threePages()
		api.errs = map[string]error{"c1": errBoom}
		items, err := drain(t, iterators.Paginate(t.Context(), api.fetch))
		if diff := cmp.Diff([]int{1, 2}, items); diff != "" {
			t.Errorf("items mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, errBoom) {
			t.Errorf("err = %v, want errBoom", err)
		}
	})

	t.Run("cursor loop is detected before it is requested", func(t *testing.T) {
		api := &fakeAPI{pages: map[string]iterators.Page[int]{
			"":  {Items: []int{1}, NextCursor: "a"},
			"a": {Items: []int{2}, NextCursor: "b"},
			"b": {Items: []int{3}, NextCursor: "a"},
		}}
		items, err := drain(t, iterators.Paginate(t.Context(), api.fetch))
		if diff := cmp.Diff([]int{1, 2, 3}, items); diff != "" {
			t.Errorf("items mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, iterators.ErrCursorLoop) {
			t.Errorf("err = %v, want ErrCursorLoop", err)
		}
		if diff := cmp.Diff([]string{"", "a", "b"}, api.calls); diff != "" {
			t.Errorf("requested cursors mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("cancelled context stops before the next fetch", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		api := threePages()
		api.hook = func(cursor string) {
			if cursor == "" {
				cancel()
			}
		}
		items, err := drain(t, iterators.Paginate(ctx, api.fetch))
		if diff := cmp.Diff([]int{1, 2}, items); diff != "" {
			t.Errorf("items mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
		if diff := cmp.Diff([]string{""}, api.calls); diff != "" {
			t.Errorf("requested cursors mismatch (-want +got):\n%s", diff)
		}
	})
}
