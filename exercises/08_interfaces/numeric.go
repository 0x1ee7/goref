package interfaces

import (
	"cmp"
	"fmt"
)

// Number is a type-set constraint: any integer or float whose underlying type
// is one of these (Part 7).
type Number interface {
	~int | ~int32 | ~int64 | ~float64
}

// Sum returns the sum of xs; the empty sum is the zero value.
func Sum[T Number](xs []T) T {
	var zero T
	return zero
}

// Max returns the largest element of xs and true, or the zero value and false
// when xs is empty.
func Max[T cmp.Ordered](xs []T) (T, bool) {
	var zero T
	return zero, false
}

// Join renders items with their String methods, separated by sep.
func Join[T fmt.Stringer](items []T, sep string) string { return "" }
