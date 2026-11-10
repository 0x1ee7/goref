package slicesmaps

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
)

// Set is a set of comparable values backed by a map. The zero value is an
// empty set ready for use by Add.
type Set[T comparable] map[T]struct{}

// NewSet returns a set holding items, with duplicates removed.
func NewSet[T comparable](items ...T) Set[T] {
	s := make(Set[T], len(items))
	for _, v := range items {
		s[v] = struct{}{}
	}
	return s
}

// Add inserts v. It allocates the underlying map if the set is zero, which is
// why it has a pointer receiver.
func (s *Set[T]) Add(v T) {
	if *s == nil {
		*s = make(Set[T])
	}
	(*s)[v] = struct{}{}
}

// Has reports whether v is in s.
func (s Set[T]) Has(v T) bool {
	_, ok := s[v]
	return ok
}

// Len returns the number of elements in s.
func (s Set[T]) Len() int {
	return len(s)
}

// Difference returns a new set with the elements of a that are not in b. Neither
// input is modified.
func Difference[T comparable](a, b Set[T]) Set[T] {
	out := make(Set[T])
	for v := range a {
		if !b.Has(v) {
			out[v] = struct{}{}
		}
	}
	return out
}

// Sorted returns the elements of s in ascending order.
func Sorted[T cmp.Ordered](s Set[T]) []T {
	return slices.Sorted(maps.Keys(s))
}

// Reconciliation compares a system of record with a physical count.
type Reconciliation struct {
	Missing    []string       // expected but not counted, sorted
	Unexpected []string       // counted but not expected, sorted
	Mismatched map[string]int // counted quantity minus expected quantity
}

// IsClean reports whether the count matches the record exactly.
func (r Reconciliation) IsClean() bool {
	return len(r.Missing) == 0 && len(r.Unexpected) == 0 && len(r.Mismatched) == 0
}

// Reconcile compares expected and counted quantities by SKU. A SKU counted at 0
// against an expected 3 is a mismatch of -3, not a missing SKU. Negative
// quantities yield ErrInvalidValue.
func Reconcile(expected, counted map[string]int) (Reconciliation, error) {
	if err := checkNonNegative("expected", expected); err != nil {
		return Reconciliation{}, err
	}
	if err := checkNonNegative("counted", counted); err != nil {
		return Reconciliation{}, err
	}

	var r Reconciliation
	for sku, want := range expected {
		got, ok := counted[sku]
		if !ok {
			r.Missing = append(r.Missing, sku)
			continue
		}
		if got != want {
			if r.Mismatched == nil {
				r.Mismatched = make(map[string]int)
			}
			r.Mismatched[sku] = got - want
		}
	}
	for sku := range counted {
		if _, ok := expected[sku]; !ok {
			r.Unexpected = append(r.Unexpected, sku)
		}
	}
	slices.Sort(r.Missing)
	slices.Sort(r.Unexpected)
	return r, nil
}

// checkNonNegative returns ErrInvalidValue for the first negative quantity in m.
func checkNonNegative(label string, m map[string]int) error {
	for sku, q := range m {
		if q < 0 {
			return fmt.Errorf("%w: %s quantity for %q is %d", ErrInvalidValue, label, sku, q)
		}
	}
	return nil
}

// BinKey identifies a stock bin as (warehouse, sku). Arrays are comparable and
// make valid map keys.
type BinKey [2]string

// StockLine is one line of a stock ledger.
type StockLine struct {
	Warehouse string
	SKU       string
	Qty       int
}

// Level is the aggregated stock in one bin.
type Level struct {
	Units int // sum of Qty over contributing lines
	Lines int // number of contributing lines
}

// StockByBin aggregates lines per bin. A line with Qty 0 still creates its bin.
// Struct values cannot be updated through the map, so each update reads,
// modifies and writes back.
func StockByBin(lines []StockLine) map[BinKey]Level {
	levels := make(map[BinKey]Level)
	for _, l := range lines {
		k := BinKey{l.Warehouse, l.SKU}
		lv := levels[k]
		lv.Units += l.Qty
		lv.Lines++
		levels[k] = lv
	}
	return levels
}

// SortedBins returns the keys of levels ordered by warehouse, then SKU.
func SortedBins(levels map[BinKey]Level) []BinKey {
	keys := slices.Collect(maps.Keys(levels))
	slices.SortFunc(keys, func(a, b BinKey) int {
		return cmp.Or(cmp.Compare(a[0], b[0]), cmp.Compare(a[1], b[1]))
	})
	return keys
}
