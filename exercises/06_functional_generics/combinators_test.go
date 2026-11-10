package functional_test

import (
	"errors"
	"fmt"
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	fp "goref/exercises/06_functional_generics"
)

var errNegative = errors.New("negative")

func TestPart1Map(t *testing.T) {
	t.Run("doubles each element", func(t *testing.T) {
		got := fp.Map([]int{1, 2, 3}, func(x int) int { return x * 2 })
		if diff := cmp.Diff([]int{2, 4, 6}, got); diff != "" {
			t.Errorf("Map mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("may change element type", func(t *testing.T) {
		got := fp.Map([]string{"ab", "c"}, strings.ToUpper)
		if diff := cmp.Diff([]string{"AB", "C"}, got); diff != "" {
			t.Errorf("Map mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("empty input yields non-nil empty slice", func(t *testing.T) {
		got := fp.Map([]int{}, func(x int) int { return x })
		if got == nil {
			t.Fatal("Map(empty) returned nil; want a non-nil empty slice")
		}
		if len(got) != 0 {
			t.Errorf("Map(empty) len = %d, want 0", len(got))
		}
	})
}

func TestPart1Filter(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		want []int
	}{
		{"keeps evens in order", []int{1, 2, 3, 4, 5, 6}, []int{2, 4, 6}},
		{"nothing matches", []int{1, 3}, []int{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fp.Filter(tc.in, func(x int) bool { return x%2 == 0 })
			if got == nil {
				t.Fatal("Filter returned nil; want a non-nil slice")
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Filter mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("result does not alias input", func(t *testing.T) {
		in := []int{1, 2, 3, 4}
		got := fp.Filter(in, func(int) bool { return true })
		if len(got) != 4 {
			t.Fatalf("Filter(keep all) len = %d, want 4", len(got))
		}
		got[0] = 99
		if in[0] != 1 {
			t.Errorf("writing to the result changed the input: in[0] = %d, want 1", in[0])
		}
	})
}

func TestPart1Reduce(t *testing.T) {
	add := func(acc, x int) int { return acc + x }
	t.Run("sums left to right", func(t *testing.T) {
		if got := fp.Reduce([]int{1, 2, 3, 4}, 0, add); got != 10 {
			t.Errorf("Reduce(sum) = %d, want 10", got)
		}
	})
	t.Run("fold order is left to right", func(t *testing.T) {
		got := fp.Reduce([]int{1, 2, 3}, 0, func(acc, x int) int { return acc*10 + x })
		if got != 123 {
			t.Errorf("Reduce(digits) = %d, want 123", got)
		}
	})
	t.Run("empty input returns init", func(t *testing.T) {
		if got := fp.Reduce([]int{}, 7, add); got != 7 {
			t.Errorf("Reduce(empty, 7) = %d, want 7", got)
		}
	})
}

func TestPart1TryReduce(t *testing.T) {
	t.Run("success returns the folded value", func(t *testing.T) {
		got, err := fp.TryReduce([]int{1, 2, 3}, 0, func(acc, x int) (int, error) { return acc + x, nil })
		if err != nil || got != 6 {
			t.Errorf("TryReduce = (%d, %v), want (6, <nil>)", got, err)
		}
	})
	t.Run("stops at first error and returns zero value", func(t *testing.T) {
		calls := 0
		f := func(acc, x int) (int, error) {
			calls++
			if x < 0 {
				return 0, errNegative
			}
			return acc + x, nil
		}
		got, err := fp.TryReduce([]int{1, 2, -1, 4}, 0, f)
		if !errors.Is(err, errNegative) {
			t.Errorf("TryReduce err = %v, want errNegative", err)
		}
		if got != 0 {
			t.Errorf("TryReduce value = %d, want zero value 0", got)
		}
		if calls != 3 {
			t.Errorf("step function called %d times, want 3 (no calls after the failure)", calls)
		}
	})
}

func TestPart1GroupBy(t *testing.T) {
	t.Run("buckets keep input order", func(t *testing.T) {
		got := fp.GroupBy([]string{"go", "rust", "zig", "java"}, func(s string) int { return len(s) })
		want := map[int][]string{2: {"go"}, 3: {"zig"}, 4: {"rust", "java"}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("GroupBy mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("empty input yields non-nil empty map", func(t *testing.T) {
		got := fp.GroupBy([]string{}, func(s string) int { return len(s) })
		if got == nil {
			t.Fatal("GroupBy(empty) returned nil; want a non-nil empty map")
		}
		if len(got) != 0 {
			t.Errorf("GroupBy(empty) has %d keys, want 0", len(got))
		}
	})
}

func TestPart1Seq(t *testing.T) {
	t.Run("MapSeq and FilterSeq are lazy and stop early", func(t *testing.T) {
		pulled := 0
		var src iter.Seq[int] = func(yield func(int) bool) {
			for i := range 10 {
				pulled++
				if !yield(i) {
					return
				}
			}
		}
		pipeline := fp.MapSeq(
			fp.FilterSeq(src, func(x int) bool { return x%2 == 0 }),
			func(x int) int { return x * 10 },
		)
		var got []int
		for v := range pipeline {
			got = append(got, v)
			if len(got) == 2 {
				break
			}
		}
		if diff := cmp.Diff([]int{0, 20}, got); diff != "" {
			t.Errorf("pipeline output mismatch (-want +got):\n%s", diff)
		}
		if pulled != 3 {
			t.Errorf("source pulled %d times, want 3 (laziness and early stop)", pulled)
		}
	})
	t.Run("ReduceSeq consumes the sequence", func(t *testing.T) {
		square := func(x int) int { return x * x }
		got := fp.ReduceSeq(fp.MapSeq(slices.Values([]int{1, 2, 3}), square), 0, func(acc, x int) int { return acc + x })
		if got != 14 {
			t.Errorf("ReduceSeq = %d, want 14", got)
		}
	})
	t.Run("Enumerate pairs each element with its index", func(t *testing.T) {
		var got []string
		for i, s := range fp.Enumerate(slices.Values([]string{"a", "b"})) {
			got = append(got, fmt.Sprintf("%d:%s", i, s))
		}
		if diff := cmp.Diff([]string{"0:a", "1:b"}, got); diff != "" {
			t.Errorf("Enumerate mismatch (-want +got):\n%s", diff)
		}
	})
}
