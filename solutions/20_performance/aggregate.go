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
	n := len(orders)
	c := Columns{
		Customer:  make([]uint32, n),
		Cents:     make([]int64, n),
		Cancelled: make([]bool, n),
	}
	for i, o := range orders {
		c.Customer[i] = o.Customer
		c.Cents[i] = o.Cents
		c.Cancelled[i] = o.Cancelled
	}
	return c
}

// RevenueAoS returns, for each customer index in [0, customers), the sum of Cents
// over that customer's non-cancelled orders. The result has length customers.
// Every order's Customer must be in range, cancelled or not. It returns
// ErrCustomerRange if any is not, or if customers is negative. It allocates once
// on success.
//
// The range check runs before the cancelled check, so a bad index is reported even
// on an order whose value is never added.
func RevenueAoS(orders []Order, customers int) ([]int64, error) {
	if customers < 0 {
		return nil, ErrCustomerRange
	}
	out := make([]int64, customers)
	for _, o := range orders {
		// Casting both sides to uint64 makes one comparison cover negative
		// values as well: a negative int wraps to a huge unsigned value.
		if uint64(o.Customer) >= uint64(customers) {
			return nil, ErrCustomerRange
		}
		if !o.Cancelled {
			out[o.Customer] += o.Cents
		}
	}
	return out, nil
}

// RevenueSoA is RevenueAoS over the struct-of-arrays layout, with the same
// results and errors. It also returns ErrRagged if the columns have different
// lengths. It allocates once on success.
//
// The loop reads each column sequentially. Cancelled and Cents are only touched
// for the same index, so a column that is not needed is never pulled into cache.
func RevenueSoA(c Columns, customers int) ([]int64, error) {
	n := len(c.Cents)
	if len(c.Customer) != n || len(c.Cancelled) != n {
		return nil, ErrRagged
	}
	if customers < 0 {
		return nil, ErrCustomerRange
	}
	out := make([]int64, customers)
	for i, id := range c.Customer {
		if uint64(id) >= uint64(customers) {
			return nil, ErrCustomerRange
		}
		if !c.Cancelled[i] {
			out[id] += c.Cents[i]
		}
	}
	return out, nil
}
