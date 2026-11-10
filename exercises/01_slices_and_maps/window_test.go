package slicesmaps_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"goref/exercises/01_slices_and_maps"
)

func TestPart1Head(t *testing.T) {
	base := []int{1, 2, 3, 4, 5}
	tests := []struct {
		name string
		n    int
		want []int
	}{
		{"prefix", 2, []int{1, 2}},
		{"exact length", 5, []int{1, 2, 3, 4, 5}},
		{"zero is empty", 0, nil},
		{"negative clamps to empty", -3, nil},
		{"past end clamps to all", 99, []int{1, 2, 3, 4, 5}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slicesmaps.Head(base, tc.n)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Fatalf("Head(base, %d) mismatch (-want +got):\n%s", tc.n, diff)
			}
			if len(tc.want) == 0 {
				// Positive control: an empty expectation must not pass against a stub that returns nothing.
				if got := slicesmaps.Head(base, 2); len(got) != 2 {
					t.Fatalf("control: Head(base, 2) returned %d elements, want 2", len(got))
				}
			}
		})
	}
}

func TestPart1HeadAppendIsSafe(t *testing.T) {
	base := []int{1, 2, 3, 4, 5}
	head := slicesmaps.Head(base, 2)
	if cap(head) != 2 {
		t.Fatalf("cap(Head(base, 2)) = %d, want 2", cap(head))
	}
	head = append(head, 99)
	if want := []int{1, 2, 3, 4, 5}; !slices.Equal(base, want) {
		t.Fatalf("append to Head result overwrote base: got %v, want %v", base, want)
	}
	if len(head) != 3 || head[2] != 99 {
		t.Fatalf("appended Head result = %v, want [1 2 99]", head)
	}
}

func TestPart1Batches(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	tests := []struct {
		name  string
		items []int
		size  int
		want  [][]int
	}{
		{"uneven tail", items, 3, [][]int{{1, 2, 3}, {4, 5, 6}, {7}}},
		{"exact multiple", []int{1, 2, 3, 4}, 2, [][]int{{1, 2}, {3, 4}}},
		{"size larger than input", items, 10, [][]int{items}},
		{"nil input yields nothing", nil, 3, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(slicesmaps.Batches(tc.items, tc.size))
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Fatalf("Batches(%v, %d) mismatch (-want +got):\n%s", tc.items, tc.size, diff)
			}
			if len(tc.items) == 0 {
				// Positive control for the empty case.
				if got := slices.Collect(slicesmaps.Batches([]int{1, 2}, 1)); len(got) != 2 {
					t.Fatalf("control: Batches([1 2], 1) yielded %d batches, want 2", len(got))
				}
			}
		})
	}
}

func TestPart1BatchesEarlyBreak(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	seen := 0
	for range slicesmaps.Batches(items, 2) {
		seen++
		if seen == 2 {
			break
		}
	}
	if seen != 2 {
		t.Fatalf("loop body ran %d times before break, want 2", seen)
	}
}

func TestPart1BatchesNonOverlapping(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6}
	batches := slices.Collect(slicesmaps.Batches(items, 2))
	if len(batches) != 3 {
		t.Fatalf("got %d batches, want 3", len(batches))
	}
	for i, b := range batches {
		if cap(b) != len(b) {
			t.Fatalf("batch %d has cap %d, want cap == len == %d", i, cap(b), len(b))
		}
	}
	batches[0] = append(batches[0], 99)
	if want := []int{3, 4}; !slices.Equal(batches[1], want) {
		t.Fatalf("appending to batch 0 clobbered batch 1: got %v, want %v", batches[1], want)
	}
}

func TestPart1BatchesPanicOnNonPositiveSize(t *testing.T) {
	for _, size := range []int{0, -1} {
		t.Run(fmt.Sprintf("size=%d", size), func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("Batches(items, %d) did not panic", size)
				}
			}()
			for range slicesmaps.Batches([]int{1, 2, 3}, size) {
			}
		})
	}
}

func TestPart1FilterInPlace(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		keep func(int) bool
		want []int
	}{
		{"keeps evens in order", []int{1, 2, 3, 4, 5, 6}, isEven, []int{2, 4, 6}},
		{"keep all", []int{1, 2, 3}, func(int) bool { return true }, []int{1, 2, 3}},
		{"keep none", []int{1, 2, 3}, func(int) bool { return false }, nil},
		{"empty input", nil, isEven, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := slices.Clone(tc.in)
			got := slicesmaps.FilterInPlace(in, tc.keep)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Fatalf("FilterInPlace mismatch (-want +got):\n%s", diff)
			}
			if len(tc.want) == 0 {
				// Positive control for the empty-result cases.
				if got := slicesmaps.FilterInPlace([]int{2}, isEven); len(got) != 1 {
					t.Fatalf("control: FilterInPlace([2], isEven) returned %d elements, want 1", len(got))
				}
			}
		})
	}
}

func TestPart1FilterInPlaceZeroesTail(t *testing.T) {
	t.Run("vacated tail is zeroed", func(t *testing.T) {
		s := []string{"a", "b", "c", "d"}
		got := slicesmaps.FilterInPlace(s, func(v string) bool { return v == "b" || v == "d" })
		if !slices.Equal(got, []string{"b", "d"}) {
			t.Fatalf("FilterInPlace = %q, want [b d]", got)
		}
		if s[2] != "" || s[3] != "" {
			t.Fatalf("vacated tail not zeroed: s = %q", s)
		}
	})
	t.Run("keep none zeroes everything", func(t *testing.T) {
		s := []string{"x", "y"}
		slicesmaps.FilterInPlace(s, func(string) bool { return false })
		if s[0] != "" || s[1] != "" {
			t.Fatalf("keep-none did not zero the slice: s = %q", s)
		}
	})
}

func TestPart1FilterInPlaceReusesBackingArray(t *testing.T) {
	t.Run("result aliases input", func(t *testing.T) {
		s := []int{1, 2, 3, 4}
		got := slicesmaps.FilterInPlace(s, isEven)
		if len(got) != 2 {
			t.Fatalf("FilterInPlace returned %d elements, want 2", len(got))
		}
		if &got[0] != &s[0] {
			t.Fatal("result does not alias the input backing array")
		}
	})
	t.Run("no allocation", func(t *testing.T) {
		src := []int{1, 2, 3, 4, 5, 6, 7, 8}
		buf := make([]int, len(src))
		keepEven := func(n int) bool { return n%2 == 0 }
		if got := slicesmaps.FilterInPlace(append(buf[:0], src...), keepEven); len(got) != 4 {
			t.Fatalf("control: FilterInPlace returned %d elements, want 4", len(got))
		}
		var sink []int
		allocs := testing.AllocsPerRun(100, func() {
			copy(buf, src)
			sink = slicesmaps.FilterInPlace(buf, keepEven)
		})
		_ = sink
		if allocs != 0 {
			t.Fatalf("FilterInPlace allocated %v times per run, want 0", allocs)
		}
	})
}
