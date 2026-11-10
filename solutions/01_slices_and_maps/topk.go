package slicesmaps

import (
	"cmp"
	"container/heap"
	"fmt"
	"iter"
	"slices"
)

// Sale is one sales line from the order pipeline.
type Sale struct {
	SKU   string
	Units int
}

// ProductTotal is the aggregated units sold for one SKU.
type ProductTotal struct {
	SKU   string
	Units int
}

// worse reports whether a ranks below b: fewer units, or equal units with a
// larger SKU.
func worse(a, b ProductTotal) bool {
	if a.Units != b.Units {
		return a.Units < b.Units
	}
	return a.SKU > b.SKU
}

// worstFirst is a heap.Interface whose top element is the worst candidate, so
// a bounded top-K can evict it cheaply.
type worstFirst []ProductTotal

func (h worstFirst) Len() int           { return len(h) }
func (h worstFirst) Less(i, j int) bool { return worse(h[i], h[j]) }
func (h worstFirst) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *worstFirst) Push(x any) { *h = append(*h, x.(ProductTotal)) }

func (h *worstFirst) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// TopK aggregates sales per SKU and returns the k best sellers, units
// descending with ties broken by SKU ascending. It ranges over sales exactly
// once. A negative Units value yields ErrInvalidValue. k <= 0 yields nil.
func TopK(sales iter.Seq[Sale], k int) ([]ProductTotal, error) {
	totals := make(map[string]int)
	for s := range sales {
		if s.Units < 0 {
			return nil, fmt.Errorf("%w: sku %q has negative units %d", ErrInvalidValue, s.SKU, s.Units)
		}
		totals[s.SKU] += s.Units
	}
	if k <= 0 {
		return nil, nil
	}

	// Keep at most k candidates; the heap top is the worst of them.
	h := make(worstFirst, 0, k)
	for sku, units := range totals {
		t := ProductTotal{SKU: sku, Units: units}
		switch {
		case len(h) < k:
			heap.Push(&h, t)
		case worse(h[0], t):
			h[0] = t
			heap.Fix(&h, 0)
		}
	}

	slices.SortFunc(h, func(a, b ProductTotal) int {
		return cmp.Or(cmp.Compare(b.Units, a.Units), cmp.Compare(a.SKU, b.SKU))
	})
	return h, nil
}
