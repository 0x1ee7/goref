package iterators_test

import (
	"errors"
	"iter"
	"math"
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	iterators "goref/exercises/05_iterators"
)

// errBoom is the sentinel injected by fakes; tests check it with errors.Is.
var errBoom = errors.New("boom")

// errReader fails every Read with err.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// pair is a key/value (or left/right) pair for comparing Seq2 output.
type pair[K, V any] struct {
	Key K
	Val V
}

// ints yields 1..n.
func ints(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 1; i <= n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// counted yields 1..n and counts every element it produces in *pulled, so a
// test can observe read-ahead. Pass math.MaxInt for an unbounded source.
func counted(n int, pulled *int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 1; i <= n; i++ {
			*pulled++
			if !yield(i) {
				return
			}
		}
	}
}

// unboundedFloats yields 1.0, 2.0, 3.0, ... and counts production in *pulled.
func unboundedFloats(pulled *int) iter.Seq[float64] {
	return func(yield func(float64) bool) {
		for i := 1; ; i++ {
			*pulled++
			if !yield(float64(i)) {
				return
			}
		}
	}
}

// collectPairs gathers every pair yielded by seq.
func collectPairs[K, V any](seq iter.Seq2[K, V]) []pair[K, V] {
	var out []pair[K, V]
	for k, v := range seq {
		out = append(out, pair[K, V]{k, v})
	}
	return out
}

// drain collects the values of a Seq2 whose second element is an error. It
// records the first error and fails the test if anything is yielded after it.
func drain[T any](t *testing.T, seq iter.Seq2[T, error]) ([]T, error) {
	t.Helper()
	var vals []T
	var err error
	for v, e := range seq {
		if err != nil {
			t.Errorf("value %v yielded after error %v", v, err)
			continue
		}
		if e != nil {
			err = e
			continue
		}
		vals = append(vals, v)
	}
	return vals, err
}

// mustNotPanic turns a panic in the deferring test body into a test failure.
// Use as: defer mustNotPanic(t).
func mustNotPanic(t *testing.T) {
	t.Helper()
	if r := recover(); r != nil {
		t.Fatalf("unexpected panic: %v", r)
	}
}

// assertPanics fails the test unless f panics.
func assertPanics(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: expected panic", what)
		}
	}()
	f()
}

func TestPart1Map(t *testing.T) {
	tests := []struct {
		name string
		in   iter.Seq[int]
		want []string
	}{
		{"three", ints(3), []string{"1", "2", "3"}},
		{"one", ints(1), []string{"1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(iterators.Map(tc.in, strconv.Itoa))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Map mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("lazy: stops reading source on break", func(t *testing.T) {
		var pulled int
		var got []string
		for s := range iterators.Map(counted(math.MaxInt, &pulled), strconv.Itoa) {
			got = append(got, s)
			if len(got) == 2 {
				break
			}
		}
		if diff := cmp.Diff([]string{"1", "2"}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if pulled != 2 {
			t.Errorf("source produced %d elements, want 2", pulled)
		}
	})
}

func TestPart1Filter(t *testing.T) {
	even := func(i int) bool { return i%2 == 0 }
	odd := func(i int) bool { return i%2 == 1 }
	tests := []struct {
		name string
		in   iter.Seq[int]
		keep func(int) bool
		want []int
	}{
		{"evens", ints(6), even, []int{2, 4, 6}},
		{"odds", ints(5), odd, []int{1, 3, 5}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(iterators.Filter(tc.in, tc.keep))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Filter mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("lazy: reads only up to the last needed match", func(t *testing.T) {
		var pulled int
		var got []int
		for v := range iterators.Filter(counted(math.MaxInt, &pulled), even) {
			got = append(got, v)
			if len(got) == 2 {
				break
			}
		}
		if diff := cmp.Diff([]int{2, 4}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if pulled != 4 {
			t.Errorf("source produced %d elements, want 4", pulled)
		}
	})
}

func TestPart1Take(t *testing.T) {
	tests := []struct {
		name string
		in   iter.Seq[int]
		n    int
		want []int
	}{
		{"fewer than source", ints(5), 3, []int{1, 2, 3}},
		{"more than source", ints(2), 5, []int{1, 2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(iterators.Take(tc.in, tc.n))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Take mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("never pulls the element after the n-th", func(t *testing.T) {
		var pulled int
		got := slices.Collect(iterators.Take(counted(math.MaxInt, &pulled), 4))
		if diff := cmp.Diff([]int{1, 2, 3, 4}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if pulled != 4 {
			t.Errorf("source produced %d elements, want exactly 4", pulled)
		}
	})
}

func TestPart1TakeNonPositive(t *testing.T) {
	if diff := cmp.Diff([]int{1, 2}, slices.Collect(iterators.Take(ints(3), 2))); diff != "" {
		t.Fatalf("Take(ints(3), 2) mismatch (-want +got):\n%s", diff)
	}
	for _, n := range []int{0, -3} {
		var pulled int
		got := slices.Collect(iterators.Take(counted(math.MaxInt, &pulled), n))
		if len(got) != 0 {
			t.Errorf("Take(n=%d) yielded %v, want nothing", n, got)
		}
		if pulled != 0 {
			t.Errorf("Take(n=%d) ranged over source (%d elements), want 0", n, pulled)
		}
	}
}

func TestPart1Chunk(t *testing.T) {
	tests := []struct {
		name string
		in   iter.Seq[int]
		size int
		want [][]int
	}{
		{"short tail", ints(5), 2, [][]int{{1, 2}, {3, 4}, {5}}},
		{"exact", ints(4), 2, [][]int{{1, 2}, {3, 4}}},
		{"size larger than input", ints(3), 5, [][]int{{1, 2, 3}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(iterators.Chunk(tc.in, tc.size))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Chunk mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("lazy: a chunk is yielded as soon as it is full", func(t *testing.T) {
		var pulled int
		var got [][]int
		for c := range iterators.Chunk(counted(math.MaxInt, &pulled), 3) {
			got = append(got, c)
			break
		}
		if diff := cmp.Diff([][]int{{1, 2, 3}}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if pulled != 3 {
			t.Errorf("source produced %d elements, want 3", pulled)
		}
	})
}

func TestPart1ChunkPanics(t *testing.T) {
	if diff := cmp.Diff([][]int{{1}, {2}}, slices.Collect(iterators.Chunk(ints(2), 1))); diff != "" {
		t.Fatalf("Chunk(ints(2), 1) mismatch (-want +got):\n%s", diff)
	}
	for _, size := range []int{0, -1} {
		assertPanics(t, "Chunk size "+strconv.Itoa(size), func() { iterators.Chunk(ints(3), size) })
	}
}

func TestPart1Window(t *testing.T) {
	tests := []struct {
		name string
		in   iter.Seq[int]
		size int
		want [][]int
	}{
		{"sliding size 3", ints(4), 3, [][]int{{1, 2, 3}, {2, 3, 4}}},
		{"size 1", ints(3), 1, [][]int{{1}, {2}, {3}}},
		{"no partial windows", ints(5), 4, [][]int{{1, 2, 3, 4}, {2, 3, 4, 5}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Windows are collected first and compared afterwards, so a
			// solution that reuses one buffer shows up as corrupted windows.
			got := slices.Collect(iterators.Window(tc.in, tc.size))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Window mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("lazy", func(t *testing.T) {
		var pulled int
		var got [][]int
		for w := range iterators.Window(counted(math.MaxInt, &pulled), 2) {
			got = append(got, w)
			if len(got) == 2 {
				break
			}
		}
		if diff := cmp.Diff([][]int{{1, 2}, {2, 3}}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if pulled != 3 {
			t.Errorf("source produced %d elements, want 3", pulled)
		}
	})
	assertPanicsWindow(t)
}

func assertPanicsWindow(t *testing.T) {
	t.Helper()
	if diff := cmp.Diff([][]int{{1}}, slices.Collect(iterators.Window(ints(1), 1))); diff != "" {
		t.Fatalf("Window(ints(1), 1) mismatch (-want +got):\n%s", diff)
	}
	assertPanics(t, "Window size 0", func() { iterators.Window(ints(3), 0) })
}

func TestPart1MovingAverage(t *testing.T) {
	floats := func(n int) iter.Seq[float64] {
		return func(yield func(float64) bool) {
			for i := 1; i <= n; i++ {
				if !yield(float64(i)) {
					return
				}
			}
		}
	}
	tests := []struct {
		name string
		n    int
		want []float64
	}{
		{"window 2", 2, []float64{1.5, 2.5, 3.5, 4.5}},
		{"window 3", 3, []float64{2, 3, 4}},
		{"window equals length", 5, []float64{3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(iterators.MovingAverage(floats(5), tc.n))
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateApprox(0, 1e-9)); diff != "" {
				t.Errorf("MovingAverage mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("lazy", func(t *testing.T) {
		var pulled int
		var got []float64
		for v := range iterators.MovingAverage(unboundedFloats(&pulled), 3) {
			got = append(got, v)
			break
		}
		if diff := cmp.Diff([]float64{2}, got, cmpopts.EquateApprox(0, 1e-9)); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if pulled != 3 {
			t.Errorf("source produced %d values, want 3", pulled)
		}
	})
	t.Run("window < 1 panics", func(t *testing.T) {
		if diff := cmp.Diff([]float64{1}, slices.Collect(iterators.MovingAverage(floats(1), 1)), cmpopts.EquateApprox(0, 1e-9)); diff != "" {
			t.Fatalf("MovingAverage window 1 mismatch (-want +got):\n%s", diff)
		}
		assertPanics(t, "MovingAverage n=0", func() { iterators.MovingAverage(floats(3), 0) })
	})
}

func TestPart2Concat(t *testing.T) {
	tests := []struct {
		name string
		seqs []iter.Seq[int]
		want []int
	}{
		{"two", []iter.Seq[int]{ints(2), ints(3)}, []int{1, 2, 1, 2, 3}},
		{"three", []iter.Seq[int]{ints(1), ints(1), ints(2)}, []int{1, 1, 1, 2}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := slices.Collect(iterators.Concat(tc.seqs...))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Concat mismatch (-want +got):\n%s", diff)
			}
		})
	}
	t.Run("lazy: later sequences are not ranged over after a break", func(t *testing.T) {
		var p1, p2 int
		var got []int
		for v := range iterators.Concat(counted(2, &p1), counted(3, &p2)) {
			got = append(got, v)
			if len(got) == 1 {
				break
			}
		}
		if diff := cmp.Diff([]int{1}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if p1 != 1 || p2 != 0 {
			t.Errorf("pulled (first, second) = (%d, %d), want (1, 0)", p1, p2)
		}
	})
	t.Run("lazy: break inside second sequence", func(t *testing.T) {
		var p1, p2 int
		var got []int
		for v := range iterators.Concat(counted(2, &p1), counted(3, &p2)) {
			got = append(got, v)
			if len(got) == 3 {
				break
			}
		}
		if diff := cmp.Diff([]int{1, 2, 1}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if p1 != 2 || p2 != 1 {
			t.Errorf("pulled (first, second) = (%d, %d), want (2, 1)", p1, p2)
		}
	})
}

func TestPart2Enumerate(t *testing.T) {
	want := []pair[int, string]{{0, "a"}, {1, "b"}, {2, "c"}}
	got := collectPairs(iterators.Enumerate(slices.Values([]string{"a", "b", "c"})))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Enumerate mismatch (-want +got):\n%s", diff)
	}
	t.Run("lazy", func(t *testing.T) {
		var pulled int
		var got []pair[int, int]
		for i, v := range iterators.Enumerate(counted(math.MaxInt, &pulled)) {
			got = append(got, pair[int, int]{i, v})
			if len(got) == 2 {
				break
			}
		}
		if diff := cmp.Diff([]pair[int, int]{{0, 1}, {1, 2}}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
		if pulled != 2 {
			t.Errorf("source produced %d elements, want 2", pulled)
		}
	})
}

func TestPart2SortedEntries(t *testing.T) {
	m := map[string]int{"b": 2, "c": 3, "a": 1}
	want := []pair[string, int]{{"a", 1}, {"b", 2}, {"c", 3}}
	if diff := cmp.Diff(want, collectPairs(iterators.SortedEntries(m))); diff != "" {
		t.Errorf("SortedEntries mismatch (-want +got):\n%s", diff)
	}
	t.Run("int keys with break", func(t *testing.T) {
		m := map[int]string{3: "c", 1: "a", 2: "b"}
		var got []pair[int, string]
		for k, v := range iterators.SortedEntries(m) {
			got = append(got, pair[int, string]{k, v})
			if len(got) == 2 {
				break
			}
		}
		if diff := cmp.Diff([]pair[int, string]{{1, "a"}, {2, "b"}}, got); diff != "" {
			t.Errorf("got mismatch (-want +got):\n%s", diff)
		}
	})
}

// TestPart2BreakContract breaks every adapter after two visible items and
// checks the source was read no further than the adapter needed.
func TestPart2BreakContract(t *testing.T) {
	even := func(i int) bool { return i%2 == 0 }
	tests := []struct {
		name       string
		run        func(p *int) int // returns the number of items the consumer saw
		wantPulled int
	}{
		{"Map", func(p *int) (n int) {
			for range iterators.Map(counted(math.MaxInt, p), strconv.Itoa) {
				if n++; n == 2 {
					break
				}
			}
			return n
		}, 2},
		{"Filter", func(p *int) (n int) {
			for range iterators.Filter(counted(math.MaxInt, p), even) {
				if n++; n == 2 {
					break
				}
			}
			return n
		}, 4},
		{"Take", func(p *int) (n int) {
			for range iterators.Take(counted(math.MaxInt, p), 10) {
				if n++; n == 2 {
					break
				}
			}
			return n
		}, 2},
		{"Chunk", func(p *int) (n int) {
			for range iterators.Chunk(counted(math.MaxInt, p), 3) {
				if n++; n == 2 {
					break
				}
			}
			return n
		}, 6},
		{"Window", func(p *int) (n int) {
			for range iterators.Window(counted(math.MaxInt, p), 2) {
				if n++; n == 2 {
					break
				}
			}
			return n
		}, 3},
		{"MovingAverage", func(p *int) (n int) {
			for range iterators.MovingAverage(unboundedFloats(p), 2) {
				if n++; n == 2 {
					break
				}
			}
			return n
		}, 3},
		{"Concat", func(p *int) (n int) {
			for range iterators.Concat(counted(math.MaxInt, p), ints(5)) {
				if n++; n == 2 {
					break
				}
			}
			return n
		}, 2},
		{"Enumerate", func(p *int) (n int) {
			for range iterators.Enumerate(counted(math.MaxInt, p)) {
				if n++; n == 2 {
					break
				}
			}
			return n
		}, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer mustNotPanic(t)
			var pulled int
			if n := tc.run(&pulled); n != 2 {
				t.Fatalf("consumer saw %d items, want 2", n)
			}
			if pulled != tc.wantPulled {
				t.Errorf("source produced %d elements, want %d", pulled, tc.wantPulled)
			}
		})
	}
}
