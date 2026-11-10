package perf_test

import (
	"slices"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	perf "goref/exercises/20_performance"
)

func double(x int) int { return x * 2 }

func TestPart3Map(t *testing.T) {
	if got := perf.Map([]int{1, 2, 3}, double); !slices.Equal(got, []int{2, 4, 6}) {
		t.Errorf("Map(double) = %v; want [2 4 6]", got)
	}
	got := perf.Map([]int{10, 200}, strconv.Itoa)
	if diff := cmp.Diff([]string{"10", "200"}, got); diff != "" {
		t.Errorf("Map(strconv.Itoa) mismatch (-want +got):\n%s", diff)
	}
	var calls []int
	perf.Map([]int{5, 6, 7}, func(x int) int {
		calls = append(calls, x)
		return x
	})
	if diff := cmp.Diff([]int{5, 6, 7}, calls); diff != "" {
		t.Errorf("f call order mismatch (-want +got):\n%s", diff)
	}
	if got := perf.Map([]int(nil), double); len(got) != 0 {
		t.Errorf("Map(nil) has length %d; want 0", len(got))
	}
}

func TestPart3Concat(t *testing.T) {
	if got := perf.Concat([]int{1}); len(got) != 1 {
		t.Fatalf("precheck: Concat([1]) has length %d; want 1", len(got))
	}
	tests := []struct {
		name   string
		chunks [][]int
		want   []int
	}{
		{"no chunks", nil, nil},
		{"one chunk", [][]int{{1, 2}}, []int{1, 2}},
		{"order kept, empty chunks skipped", [][]int{{1}, nil, {}, {2, 3}}, []int{1, 2, 3}},
		{"all empty", [][]int{nil, {}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := perf.Concat(tt.chunks...)
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Concat(%v) mismatch (-want +got):\n%s", tt.chunks, diff)
			}
		})
	}
}

func TestPart3ConcatDoesNotAlias(t *testing.T) {
	a := []int{1, 2, 3}
	got := perf.Concat(a)
	if len(got) != 3 {
		t.Fatalf("Concat(a) has length %d; want 3", len(got))
	}
	got[0] = 99
	if a[0] != 1 {
		t.Errorf("mutating the result changed the input; Concat must return a fresh slice even for one chunk")
	}
}

func TestPart3Batch(t *testing.T) {
	xs := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	tests := []struct {
		name string
		n    int
		want [][]int
	}{
		{"uneven tail", 3, [][]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {9}}},
		{"exact multiple", 5, [][]int{{0, 1, 2, 3, 4}, {5, 6, 7, 8, 9}}},
		{"size larger than input", 100, [][]int{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}},
		{"size one", 1, [][]int{{0}, {1}, {2}, {3}, {4}, {5}, {6}, {7}, {8}, {9}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := perf.Batch(xs, tt.n)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Batch(xs, %d) mismatch (-want +got):\n%s", tt.n, diff)
			}
		})
	}
	if got := perf.Batch([]int(nil), 3); len(got) != 0 {
		t.Errorf("Batch(nil, 3) has length %d; want 0", len(got))
	}
}

func TestPart3BatchPanicsOnNonPositiveSize(t *testing.T) {
	for _, n := range []int{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Batch(xs, %d) did not panic; want panic like slices.Chunk", n)
				}
			}()
			perf.Batch([]int{1, 2}, n)
		}()
	}
}

func TestPart3BatchChunksAreCappedViews(t *testing.T) {
	xs := []int{0, 1, 2, 3, 4, 5}
	chunks := perf.Batch(xs, 3)
	if len(chunks) != 2 {
		t.Fatalf("Batch(xs, 3) returned %d chunks; want 2", len(chunks))
	}
	if &chunks[0][0] != &xs[0] {
		t.Fatal("chunk does not alias the input; Batch must return views, not copies")
	}
	if cap(chunks[0]) != 3 {
		t.Errorf("cap(chunks[0]) = %d; want 3 so that append reallocates instead of overwriting the next chunk", cap(chunks[0]))
	}
	_ = append(chunks[0], 99)
	if xs[3] != 3 {
		t.Errorf("appending to chunks[0] overwrote xs[3] = %d; want 3", xs[3])
	}
}

func TestPart3Dedupe(t *testing.T) {
	if got := perf.Dedupe([]string{"a", "a"}); len(got) != 1 {
		t.Fatalf("precheck: Dedupe([a a]) has length %d; want 1", len(got))
	}
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty", nil, nil},
		{"keeps first occurrences in order", []string{"b", "a", "b", "c", "a"}, []string{"b", "a", "c"}},
		{"no duplicates", []string{"x", "y"}, []string{"x", "y"}},
		{"all same", []string{"z", "z", "z"}, []string{"z"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := perf.Dedupe(tt.in)
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Dedupe(%q) mismatch (-want +got):\n%s", tt.in, diff)
			}
		})
	}
}

func TestPart3DedupeDoesNotModifyInput(t *testing.T) {
	in := []string{"a", "b", "a"}
	got := perf.Dedupe(in)
	if len(got) != 2 {
		t.Fatalf("Dedupe(%q) has length %d; want 2", in, len(got))
	}
	got[0] = "changed"
	if !slices.Equal(in, []string{"a", "b", "a"}) {
		t.Errorf("input after mutating the result = %q; want unchanged [a b a]", in)
	}
}

func TestPart3PreallocAllocs(t *testing.T) {
	xs := make([]int, 100)
	for i := range xs {
		xs[i] = i
	}
	chunks := [][]int{{1, 2, 3}, {4, 5}, {6}}
	batchIn := make([]int, 1000)

	// Correctness first: a stub returning nil passes an allocation budget of zero.
	if got := perf.Map(xs, double); len(got) != 100 || got[99] != 198 {
		t.Fatalf("Map precheck failed: len=%d", len(got))
	}
	if got := perf.Concat(chunks...); !slices.Equal(got, []int{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("Concat precheck = %v; want [1 2 3 4 5 6]", got)
	}
	if got := perf.Batch(batchIn, 10); len(got) != 100 {
		t.Fatalf("Batch precheck returned %d chunks; want 100", len(got))
	}

	if n := testing.AllocsPerRun(100, func() { sinkInts = perf.Map(xs, double) }); n != 1 {
		t.Errorf("Map allocated %v times per call; want exactly 1", n)
	}
	if n := testing.AllocsPerRun(100, func() { sinkInts = perf.Concat(chunks...) }); n != 1 {
		t.Errorf("Concat allocated %v times per call; want exactly 1", n)
	}
	if n := testing.AllocsPerRun(100, func() { sinkChunks = perf.Batch(batchIn, 10) }); n != 1 {
		t.Errorf("Batch allocated %v times per call; want exactly 1 (the outer slice)", n)
	}
}
