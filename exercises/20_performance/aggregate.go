package perf

import "errors"

// Sentinel errors returned by the revenue aggregations.
var (
	// ErrCustomerRange reports a customer index outside [0, customers), or a negative customers count.
	ErrCustomerRange = errors.New("perf: customer index out of range")
	// ErrRagged reports Columns whose slices have different lengths.
	ErrRagged = errors.New("perf: ragged columns")
)

// Order is one row in the array-of-structs layout.
type Order struct {
	Customer  uint32 // dense index in [0, customers)
	Cents     int64
	Cancelled bool
}

// Columns is the struct-of-arrays layout: index i of every slice describes the
// same order. A valid Columns has all three slices of equal length.
type Columns struct {
	Customer  []uint32
	Cents     []int64
	Cancelled []bool
}

// ToColumns converts orders to the struct-of-arrays layout. The result does not
// alias orders. It allocates once per column (three allocations for a non-empty input).
func ToColumns(orders []Order) Columns {
	return Columns{}
}

// RevenueAoS returns, for each customer index in [0, customers), the sum of Cents
// over that customer's non-cancelled orders. The result has length customers.
// Every order's Customer must be in range, cancelled or not. It returns
// ErrCustomerRange if any is not, or if customers is negative. It allocates once
// on success.
func RevenueAoS(orders []Order, customers int) ([]int64, error) {
	return nil, nil
}

// RevenueSoA is RevenueAoS over the struct-of-arrays layout, with the same
// results and errors. It also returns ErrRagged if the columns have different
// lengths. It allocates once on success.
func RevenueSoA(c Columns, customers int) ([]int64, error) {
	return nil, nil
}
