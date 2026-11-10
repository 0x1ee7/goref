package orders

import (
	"context"
	"time"
)

// OrderWithItems is an order together with its lines.
type OrderWithItems struct {
	Order Order
	Items []OrderItem // ordered by LineNo
}

// LoadCustomerOrders returns every order of the customer, cancelled included,
// in (PlacedAt, ID) order, each with its items. A customer with no orders gets
// an empty slice.
//
// The number of statements must not grow with the number of orders (no N+1):
// at most two queries.
func (r *OrderRepo) LoadCustomerOrders(ctx context.Context, customerID int64) ([]OrderWithItems, error) {
	return nil, errNotImplemented
}

// ProductSales summarises non-cancelled sales of one SKU.
type ProductSales struct {
	SKU        string
	UnitsSold  int64
	OrderCount int64 // distinct non-cancelled orders containing the SKU
}

// CustomerRevenue summarises a customer's non-cancelled spend.
type CustomerRevenue struct {
	CustomerID int64
	Name       string
	OrderCount int64
	Total      Money // exact sum of quantity * unit price over non-cancelled lines
}

// RankedCustomer is a customer with a rank by non-cancelled order count.
type RankedCustomer struct {
	CustomerID int64
	Name       string
	OrderCount int64
	Rank       int64 // SQL RANK(): ties share a rank and the next rank is skipped
}

// RunningUnits is one non-cancelled order of a customer with a running total.
type RunningUnits struct {
	OrderID      int64
	PlacedAt     time.Time
	Units        int64 // total units in this order
	RunningUnits int64 // units in this and all earlier (PlacedAt, OrderID) orders
}

// UnitsSoldByProduct returns sales per SKU for non-cancelled orders, sorted by
// UnitsSold descending, then SKU ascending. SKUs without a non-cancelled sale
// are omitted.
func UnitsSoldByProduct(ctx context.Context, db DBTX) ([]ProductSales, error) {
	return nil, errNotImplemented
}

// RevenueByCustomer returns each customer's non-cancelled spend, sorted by
// CustomerID. Customers without non-cancelled orders are omitted. Totals are
// computed in integer cents; no floating point is involved.
func RevenueByCustomer(ctx context.Context, db DBTX) ([]CustomerRevenue, error) {
	return nil, errNotImplemented
}

// RankCustomersByOrders ranks every customer, including those with zero orders,
// by non-cancelled order count, descending, using RANK() in SQL. The result is
// sorted by Rank, then Name, then CustomerID.
func RankCustomersByOrders(ctx context.Context, db DBTX) ([]RankedCustomer, error) {
	return nil, errNotImplemented
}

// CustomerRunningUnits returns one row per non-cancelled order of the customer,
// ordered by (PlacedAt, OrderID), with the order's units and the running total
// computed by a window function.
func CustomerRunningUnits(ctx context.Context, db DBTX, customerID int64) ([]RunningUnits, error) {
	return nil, errNotImplemented
}
