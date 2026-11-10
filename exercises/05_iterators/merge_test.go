package iterators_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	iterators "goref/exercises/05_iterators"
)

// rec is a time-ordered record; ts stands in for a timestamp.
type rec struct {
	ts  int
	src string
}

func byTS(a, b rec) int { return a.ts - b.ts }

func intCmp(a, b int) int { return a - b }

// stride yields start, start+step, start+2*step, ... forever, counting production.
func stride(start, step int, pulled *int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for v := start; ; v += step {
			*pulled++
			if !yield(v) {
				return
			}
		}
	}
}

func TestPart7Merge(t *testing.T) {
	t.Run("three sorted int streams", func(t *testing.T) {
		got := slices.Collect(iterators.Merge(intCmp,
			slices.Values([]int{1, 4, 7}),
			slices.Values([]int{2, 5}),
			slices.Values([]int{3, 6, 8}),
		))
		if diff := cmp.Diff([]int{1, 2, 3, 4, 5, 6, 7, 8}, got); diff != "" {
			t.Errorf("Merge mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("ties favour the earlier stream, and keep stream order", func(t *testing.T) {
		a := slices.Values([]rec{{1, "a1"}, {2, "a2"}})
		b := slices.Values([]rec{{1, "b1"}, {3, "b3"}})
		got := slices.Collect(iterators.Merge(byTS, a, b))
		want := []rec{{1, "a1"}, {1, "b1"}, {2, "a2"}, {3, "b3"}}
		if diff := cmp.Diff(want, got, cmp.AllowUnexported(rec{})); diff != "" {
			t.Errorf("Merge mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("empty streams are harmless; zero streams yield nothing", func(t *testing.T) {
		got := slices.Collect(iterators.Merge(intCmp, slices.Values([]int{}), slices.Values([]int{1, 2})))
		if diff := cmp.Diff([]int{1, 2}, got); diff != "" {
			t.Errorf("Merge with an empty stream mismatch (-want +got):\n%s", diff)
		}
		if none := slices.Collect(iterators.Merge[int](intCmp)); len(none) != 0 {
			t.Errorf("Merge with zero streams = %v, want nothing", none)
		}
	})

	t.Run("infinite streams with early break", func(t *testing.T) {
		var pe, po int
		var got []int
		for v := range iterators.Merge(intCmp, stride(0, 2, &pe), stride(1, 2, &po)) {
			got = append(got, v)
			if len(got) == 5 {
				break
			}
		}
		if diff := cmp.Diff([]int{0, 1, 2, 3, 4}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		// Evens consumed: 0,2,4 -> at most one element ahead, so at most 4 pulled.
		// Odds consumed: 1,3 -> at most 3 pulled.
		if pe > 4 || po > 3 || pe < 3 || po < 2 {
			t.Errorf("pulled (evens, odds) = (%d, %d); want at most one element read ahead of each stream", pe, po)
		}
	})

	t.Run("break stops every stream", func(t *testing.T) {
		var ca, cb bool
		var got []int
		for v := range iterators.Merge(intCmp, withCleanup(10, &ca), withCleanup(10, &cb)) {
			got = append(got, v)
			if len(got) == 2 {
				break
			}
		}
		if diff := cmp.Diff([]int{1, 1}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if !ca || !cb {
			t.Errorf("streams cleaned up = (%t, %t), want (true, true)", ca, cb)
		}
	})
}
