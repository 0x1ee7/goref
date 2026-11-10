package slicesmaps

import (
	"errors"
	"iter"
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

// TopK aggregates sales per SKU and returns the k best sellers, units
// descending with ties broken by SKU ascending. It ranges over sales exactly
// once. A negative Units value yields ErrInvalidValue.
func TopK(sales iter.Seq[Sale], k int) ([]ProductTotal, error) {
	return nil, errors.New("not implemented")
}
