package slicesmaps_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/01_slices_and_maps"
)

func sale(sku string, units int) slicesmaps.Sale {
	return slicesmaps.Sale{SKU: sku, Units: units}
}

func total(sku string, units int) slicesmaps.ProductTotal {
	return slicesmaps.ProductTotal{SKU: sku, Units: units}
}

func TestPart5TopK(t *testing.T) {
	t.Run("ranks by units with sku tie-break", func(t *testing.T) {
		sales := []slicesmaps.Sale{sale("A", 5), sale("B", 3), sale("C", 5), sale("D", 1)}
		got, err := slicesmaps.TopK(slices.Values(sales), 2)
		if err != nil {
			t.Fatalf("TopK: %v", err)
		}
		want := []slicesmaps.ProductTotal{total("A", 5), total("C", 5)}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("TopK mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("aggregates lines per sku", func(t *testing.T) {
		sales := []slicesmaps.Sale{sale("X", 2), sale("Y", 4), sale("X", 3), sale("Y", 1), sale("Z", 6)}
		got, err := slicesmaps.TopK(slices.Values(sales), 3)
		if err != nil {
			t.Fatalf("TopK: %v", err)
		}
		want := []slicesmaps.ProductTotal{total("Z", 6), total("X", 5), total("Y", 5)}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("TopK mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("k larger than distinct skus returns all ranked", func(t *testing.T) {
		sales := []slicesmaps.Sale{sale("X", 1), sale("Y", 2)}
		got, err := slicesmaps.TopK(slices.Values(sales), 10)
		if err != nil {
			t.Fatalf("TopK: %v", err)
		}
		want := []slicesmaps.ProductTotal{total("Y", 2), total("X", 1)}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("TopK mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("non-positive k is empty", func(t *testing.T) {
		sales := []slicesmaps.Sale{sale("X", 1), sale("Y", 2)}
		for _, k := range []int{0, -1} {
			got, err := slicesmaps.TopK(slices.Values(sales), k)
			if err != nil {
				t.Fatalf("TopK(k=%d): %v", k, err)
			}
			if len(got) != 0 {
				t.Fatalf("TopK(k=%d) returned %d totals, want 0", k, len(got))
			}
		}
		// Positive control: the same input with k=1 must return one total.
		got, err := slicesmaps.TopK(slices.Values(sales), 1)
		if err != nil || len(got) != 1 {
			t.Fatalf("control: TopK(k=1) = %v, %v; want one total", got, err)
		}
	})
	t.Run("zero units are ranked last", func(t *testing.T) {
		sales := []slicesmaps.Sale{sale("A", 0), sale("B", 0), sale("C", 1)}
		got, err := slicesmaps.TopK(slices.Values(sales), 3)
		if err != nil {
			t.Fatalf("TopK: %v", err)
		}
		want := []slicesmaps.ProductTotal{total("C", 1), total("A", 0), total("B", 0)}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("TopK mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("negative units are rejected", func(t *testing.T) {
		sales := []slicesmaps.Sale{sale("A", 2), sale("B", -1)}
		if _, err := slicesmaps.TopK(slices.Values(sales), 2); !errors.Is(err, slicesmaps.ErrInvalidValue) {
			t.Fatalf("TopK error = %v, want ErrInvalidValue", err)
		}
	})
	t.Run("ranges the sequence exactly once", func(t *testing.T) {
		sales := []slicesmaps.Sale{sale("A", 1), sale("B", 2), sale("C", 3)}
		calls := 0
		seq := func(yield func(slicesmaps.Sale) bool) {
			calls++
			for _, s := range sales {
				if !yield(s) {
					return
				}
			}
		}
		got, err := slicesmaps.TopK(seq, 2)
		if err != nil {
			t.Fatalf("TopK: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("TopK returned %d totals, want 2", len(got))
		}
		if calls != 1 {
			t.Fatalf("sequence ranged %d times, want 1", calls)
		}
	})
	t.Run("matches brute force on generated stream", func(t *testing.T) {
		var sales []slicesmaps.Sale
		for i := range 2000 {
			sales = append(sales, sale(fmt.Sprintf("sku-%02d", (i*7)%37), (i*13)%11))
		}
		totals := map[string]int{}
		for _, s := range sales {
			totals[s.SKU] += s.Units
		}
		want := make([]slicesmaps.ProductTotal, 0, len(totals))
		for sku, u := range totals {
			want = append(want, total(sku, u))
		}
		slices.SortFunc(want, func(a, b slicesmaps.ProductTotal) int {
			if a.Units != b.Units {
				return b.Units - a.Units
			}
			return strings.Compare(a.SKU, b.SKU)
		})
		want = want[:5]
		got, err := slicesmaps.TopK(slices.Values(sales), 5)
		if err != nil {
			t.Fatalf("TopK: %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("TopK mismatch vs brute force (-want +got):\n%s", diff)
		}
	})
}
