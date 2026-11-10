package iterators_test

import (
	"iter"
	"math"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	iterators "goref/exercises/05_iterators"
)

// withCleanup yields 1..n and sets *cleaned when its body is left, either by
// running to the end or by an early stop from the consumer.
func withCleanup(n int, cleaned *bool) iter.Seq[int] {
	return func(yield func(int) bool) {
		defer func() { *cleaned = true }()
		for i := 1; i <= n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func TestPart3Zip(t *testing.T) {
	letters := func(s ...string) iter.Seq[string] { return slices.Values(s) }
	tests := []struct {
		name string
		a    iter.Seq[int]
		b    iter.Seq[string]
		want []pair[int, string]
	}{
		{"equal lengths", ints(3), letters("a", "b", "c"), []pair[int, string]{{1, "a"}, {2, "b"}, {3, "c"}}},
		{"a shorter", ints(2), letters("a", "b", "c"), []pair[int, string]{{1, "a"}, {2, "b"}}},
		{"b shorter", ints(5), letters("a", "b"), []pair[int, string]{{1, "a"}, {2, "b"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := collectPairs(iterators.Zip(tc.a, tc.b))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Zip mismatch (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("b is not pulled once a is exhausted", func(t *testing.T) {
		var pa, pb int
		got := collectPairs(iterators.Zip(counted(2, &pa), counted(5, &pb)))
		if diff := cmp.Diff([]pair[int, int]{{1, 1}, {2, 2}}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if pa != 2 || pb != 2 {
			t.Errorf("pulled (a, b) = (%d, %d), want (2, 2)", pa, pb)
		}
	})

	t.Run("break stops both sources", func(t *testing.T) {
		var ca, cb bool
		var got []pair[int, int]
		for x, y := range iterators.Zip(withCleanup(10, &ca), withCleanup(10, &cb)) {
			got = append(got, pair[int, int]{x, y})
			if len(got) == 2 {
				break
			}
		}
		if diff := cmp.Diff([]pair[int, int]{{1, 1}, {2, 2}}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if !ca || !cb {
			t.Errorf("sources cleaned up = (%t, %t), want (true, true)", ca, cb)
		}
	})

	t.Run("lazy over infinite sources", func(t *testing.T) {
		var pa, pb int
		var got []pair[int, int]
		for x, y := range iterators.Zip(counted(math.MaxInt, &pa), counted(math.MaxInt, &pb)) {
			got = append(got, pair[int, int]{x, y})
			if len(got) == 3 {
				break
			}
		}
		if len(got) != 3 || got[2] != (pair[int, int]{3, 3}) {
			t.Errorf("got %v, want three pairs ending in (3,3)", got)
		}
		if pa != 3 || pb != 3 {
			t.Errorf("pulled (a, b) = (%d, %d), want (3, 3)", pa, pb)
		}
	})
}

func TestPart3Peekable(t *testing.T) {
	t.Run("peek does not consume", func(t *testing.T) {
		p := iterators.NewPeekable(ints(3))
		steps := []struct {
			op   string
			want int
			ok   bool
		}{
			{"peek", 1, true},
			{"peek", 1, true},
			{"next", 1, true},
			{"peek", 2, true},
			{"next", 2, true},
			{"next", 3, true},
			{"next", 0, false},
			{"peek", 0, false},
			{"next", 0, false},
		}
		for i, s := range steps {
			var got int
			var ok bool
			switch s.op {
			case "peek":
				got, ok = p.Peek()
			case "next":
				got, ok = p.Next()
			}
			if got != s.want || ok != s.ok {
				t.Fatalf("step %d (%s): got (%d, %t), want (%d, %t)", i, s.op, got, ok, s.want, s.ok)
			}
		}
	})

	t.Run("lazy: nothing pulled until needed", func(t *testing.T) {
		var pulled int
		p := iterators.NewPeekable(counted(10, &pulled))
		if v, ok := p.Peek(); !ok || v != 1 {
			t.Fatalf("Peek() = (%d, %t), want (1, true)", v, ok)
		}
		if pulled != 1 {
			t.Errorf("after Peek pulled = %d, want 1", pulled)
		}
		p.Peek()
		if pulled != 1 {
			t.Errorf("after second Peek pulled = %d, want still 1", pulled)
		}
		if v, _ := p.Next(); v != 1 || pulled != 1 {
			t.Errorf("Next() = %d with pulled=%d, want 1 with pulled=1", v, pulled)
		}
		if v, _ := p.Next(); v != 2 || pulled != 2 {
			t.Errorf("Next() = %d with pulled=%d, want 2 with pulled=2", v, pulled)
		}
		p.Close()
	})

	t.Run("close stops the source and is idempotent", func(t *testing.T) {
		var cleaned bool
		p := iterators.NewPeekable(withCleanup(10, &cleaned))
		if v, ok := p.Peek(); !ok || v != 1 {
			t.Fatalf("Peek() = (%d, %t), want (1, true)", v, ok)
		}
		p.Close()
		if !cleaned {
			t.Error("Close did not stop the underlying iterator")
		}
		if v, ok := p.Next(); ok {
			t.Errorf("Next after Close = (%d, true), want (0, false)", v)
		}
		p.Close()
	})
}
