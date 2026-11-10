package slicesmaps_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"goref/exercises/01_slices_and_maps"
)

func TestPart7Set(t *testing.T) {
	t.Run("zero value Add", func(t *testing.T) {
		var s slicesmaps.Set[string]
		s.Add("a")
		if !s.Has("a") {
			t.Fatal("Has(a) false after Add on zero Set")
		}
		if s.Len() != 1 {
			t.Fatalf("Len() = %d, want 1", s.Len())
		}
	})
	t.Run("NewSet deduplicates", func(t *testing.T) {
		if got := slicesmaps.NewSet("a", "b", "a").Len(); got != 2 {
			t.Fatalf("NewSet(a, b, a).Len() = %d, want 2", got)
		}
	})
	t.Run("Has reports membership", func(t *testing.T) {
		s := slicesmaps.NewSet("a")
		if !s.Has("a") {
			t.Fatal("Has(a) = false, want true")
		}
		if s.Has("b") {
			t.Fatal("Has(b) = true, want false")
		}
	})
	t.Run("Difference leaves inputs unchanged", func(t *testing.T) {
		a := slicesmaps.NewSet(1, 2, 3)
		b := slicesmaps.NewSet(2, 4)
		got := slicesmaps.Difference(a, b)
		if diff := cmp.Diff([]int{1, 3}, slicesmaps.Sorted(got)); diff != "" {
			t.Fatalf("Difference mismatch (-want +got):\n%s", diff)
		}
		if a.Len() != 3 || b.Len() != 2 {
			t.Fatalf("inputs changed: len(a)=%d len(b)=%d, want 3 and 2", a.Len(), b.Len())
		}
	})
	t.Run("Sorted is ascending", func(t *testing.T) {
		got := slicesmaps.Sorted(slicesmaps.NewSet("pear", "apple", "fig"))
		want := []string{"apple", "fig", "pear"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Sorted mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("Sorted of empty set", func(t *testing.T) {
		if got := slicesmaps.Sorted(slicesmaps.NewSet[string]()); len(got) != 0 {
			t.Fatalf("Sorted(empty) = %v, want empty", got)
		}
		// Positive control: a non-empty set must sort to one element.
		if got := slicesmaps.Sorted(slicesmaps.NewSet("x")); len(got) != 1 {
			t.Fatalf("control: Sorted({x}) = %v, want one element", got)
		}
	})
}

func TestPart7Reconcile(t *testing.T) {
	tests := []struct {
		name     string
		expected map[string]int
		counted  map[string]int
		want     slicesmaps.Reconciliation
		clean    bool
	}{
		{
			name:     "clean count",
			expected: map[string]int{"A": 3, "B": 1},
			counted:  map[string]int{"A": 3, "B": 1},
			want:     slicesmaps.Reconciliation{},
			clean:    true,
		},
		{
			name:     "empty on both sides is clean",
			expected: nil,
			counted:  nil,
			want:     slicesmaps.Reconciliation{},
			clean:    true,
		},
		{
			name:     "mixed differences",
			expected: map[string]int{"A": 3, "B": 2, "C": 5},
			counted:  map[string]int{"A": 3, "B": 1, "D": 4},
			want: slicesmaps.Reconciliation{
				Missing:    []string{"C"},
				Unexpected: []string{"D"},
				Mismatched: map[string]int{"B": -1},
			},
			clean: false,
		},
		{
			name:     "zero count against expected three is a mismatch",
			expected: map[string]int{"A": 3},
			counted:  map[string]int{"A": 0},
			want:     slicesmaps.Reconciliation{Mismatched: map[string]int{"A": -3}},
			clean:    false,
		},
		{
			name:     "only unexpected",
			expected: map[string]int{},
			counted:  map[string]int{"X": 1},
			want:     slicesmaps.Reconciliation{Unexpected: []string{"X"}},
			clean:    false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := slicesmaps.Reconcile(tc.expected, tc.counted)
			if err != nil {
				t.Fatalf("Reconcile: %v", err)
			}
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Fatalf("Reconcile mismatch (-want +got):\n%s", diff)
			}
			if got.IsClean() != tc.clean {
				t.Fatalf("IsClean() = %v, want %v", got.IsClean(), tc.clean)
			}
		})
	}
}

func TestPart7ReconcileRejectsNegative(t *testing.T) {
	t.Run("negative expected", func(t *testing.T) {
		if _, err := slicesmaps.Reconcile(map[string]int{"A": -1}, map[string]int{"A": 1}); !errors.Is(err, slicesmaps.ErrInvalidValue) {
			t.Fatalf("Reconcile error = %v, want ErrInvalidValue", err)
		}
	})
	t.Run("negative counted", func(t *testing.T) {
		if _, err := slicesmaps.Reconcile(map[string]int{"A": 1}, map[string]int{"A": -2}); !errors.Is(err, slicesmaps.ErrInvalidValue) {
			t.Fatalf("Reconcile error = %v, want ErrInvalidValue", err)
		}
	})
}

func TestPart7IsClean(t *testing.T) {
	t.Run("zero value is clean and any difference is not", func(t *testing.T) {
		if !(slicesmaps.Reconciliation{}).IsClean() {
			t.Fatal("zero Reconciliation is not clean")
		}
		if (slicesmaps.Reconciliation{Missing: []string{"x"}}).IsClean() {
			t.Fatal("Missing alone reported clean")
		}
		if (slicesmaps.Reconciliation{Unexpected: []string{"x"}}).IsClean() {
			t.Fatal("Unexpected alone reported clean")
		}
		if (slicesmaps.Reconciliation{Mismatched: map[string]int{"x": 1}}).IsClean() {
			t.Fatal("Mismatched alone reported clean")
		}
	})
}

func TestPart7StockByBin(t *testing.T) {
	lines := []slicesmaps.StockLine{
		{Warehouse: "east", SKU: "A1", Qty: 5},
		{Warehouse: "west", SKU: "A1", Qty: 0},
		{Warehouse: "east", SKU: "A1", Qty: 3},
		{Warehouse: "east", SKU: "B2", Qty: 2},
	}
	eastA1 := slicesmaps.BinKey{"east", "A1"}
	westA1 := slicesmaps.BinKey{"west", "A1"}
	eastB2 := slicesmaps.BinKey{"east", "B2"}

	t.Run("sums quantities and counts lines", func(t *testing.T) {
		got := slicesmaps.StockByBin(lines)
		want := map[slicesmaps.BinKey]slicesmaps.Level{
			eastA1: {Units: 8, Lines: 2},
			westA1: {Units: 0, Lines: 1},
			eastB2: {Units: 2, Lines: 1},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("StockByBin mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("struct values in the map are copies", func(t *testing.T) {
		levels := slicesmaps.StockByBin(lines)
		v, ok := levels[eastA1]
		if !ok {
			t.Fatal("bin east/A1 missing")
		}
		v.Units = 999
		if levels[eastA1].Units != 8 {
			t.Fatalf("modifying a copy changed the map: Units = %d, want 8", levels[eastA1].Units)
		}
	})
	t.Run("sorted bins order by warehouse then sku", func(t *testing.T) {
		levels := slicesmaps.StockByBin(lines)
		want := []slicesmaps.BinKey{eastA1, eastB2, westA1}
		if diff := cmp.Diff(want, slicesmaps.SortedBins(levels)); diff != "" {
			t.Fatalf("SortedBins mismatch (-want +got):\n%s", diff)
		}
		// Positive control: a single-entry map must sort to that entry.
		single := map[slicesmaps.BinKey]slicesmaps.Level{{"x", "y"}: {Units: 1, Lines: 1}}
		if got := slicesmaps.SortedBins(single); len(got) != 1 || got[0] != (slicesmaps.BinKey{"x", "y"}) {
			t.Fatalf("control: SortedBins(single) = %v", got)
		}
	})
}
