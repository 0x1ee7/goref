package orders

import (
	"context"
	"database/sql"
	"fmt"
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
	// One LEFT JOIN returns every order with its lines; grouping happens in Go.
	// Rows arrive ordered by order, so each order's lines are contiguous.
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.customer_id, o.placed_at, o.status,
		       oi.line_no, oi.sku, oi.quantity, oi.unit_price_cents
		FROM orders o
		LEFT JOIN order_items oi ON oi.order_id = o.id
		WHERE o.customer_id = ?
		ORDER BY o.placed_at, o.id, oi.line_no`, customerID)
	if err != nil {
		return nil, fmt.Errorf("orders: load customer orders: %w", err)
	}
	defer rows.Close()

	out := []OrderWithItems{}
	for rows.Next() {
		var (
			o                  Order
			placed             int64
			status             string
			lineNo, qty, price sql.NullInt64
			sku                sql.NullString
		)
		if err := rows.Scan(&o.ID, &o.CustomerID, &placed, &status,
			&lineNo, &sku, &qty, &price); err != nil {
			return nil, fmt.Errorf("orders: scan order: %w", err)
		}
		if len(out) == 0 || out[len(out)-1].Order.ID != o.ID {
			o.PlacedAt = time.UnixMicro(placed).UTC()
			o.Status = Status(status)
			out = append(out, OrderWithItems{Order: o})
		}
		if lineNo.Valid {
			last := &out[len(out)-1]
			last.Items = append(last.Items, OrderItem{
				OrderID:   last.Order.ID,
				LineNo:    int(lineNo.Int64),
				SKU:       sku.String,
				Quantity:  int(qty.Int64),
				UnitPrice: Money(price.Int64),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders: iterate orders: %w", err)
	}
	return out, nil
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
	return queryAll(ctx, db, func(s rowScanner) (ProductSales, error) {
		var p ProductSales
		err := s.Scan(&p.SKU, &p.UnitsSold, &p.OrderCount)
		return p, err
	}, `
		SELECT oi.sku, SUM(oi.quantity), COUNT(DISTINCT oi.order_id)
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.status <> 'cancelled'
		GROUP BY oi.sku
		ORDER BY SUM(oi.quantity) DESC, oi.sku`)
}

// RevenueByCustomer returns each customer's non-cancelled spend, sorted by
// CustomerID. Customers without non-cancelled orders are omitted. Totals are
// computed in integer cents; no floating point is involved.
func RevenueByCustomer(ctx context.Context, db DBTX) ([]CustomerRevenue, error) {
	return queryAll(ctx, db, func(s rowScanner) (CustomerRevenue, error) {
		var c CustomerRevenue
		var total int64
		err := s.Scan(&c.CustomerID, &c.Name, &c.OrderCount, &total)
		c.Total = Money(total)
		return c, err
	}, `
		SELECT c.id, c.name, COUNT(DISTINCT o.id), SUM(oi.quantity * oi.unit_price_cents)
		FROM customers c
		JOIN orders o ON o.customer_id = c.id AND o.status <> 'cancelled'
		JOIN order_items oi ON oi.order_id = o.id
		GROUP BY c.id, c.name
		ORDER BY c.id`)
}

// RankCustomersByOrders ranks every customer, including those with zero orders,
// by non-cancelled order count, descending, using RANK() in SQL. The result is
// sorted by Rank, then Name, then CustomerID.
func RankCustomersByOrders(ctx context.Context, db DBTX) ([]RankedCustomer, error) {
	return queryAll(ctx, db, func(s rowScanner) (RankedCustomer, error) {
		var c RankedCustomer
		err := s.Scan(&c.CustomerID, &c.Name, &c.OrderCount, &c.Rank)
		return c, err
	}, `
		SELECT c.id, c.name, COUNT(o.id) AS n,
		       RANK() OVER (ORDER BY COUNT(o.id) DESC) AS rnk
		FROM customers c
		LEFT JOIN orders o ON o.customer_id = c.id AND o.status <> 'cancelled'
		GROUP BY c.id, c.name
		ORDER BY rnk, c.name, c.id`)
}

// CustomerRunningUnits returns one row per non-cancelled order of the customer,
// ordered by (PlacedAt, OrderID), with the order's units and the running total
// computed by a window function.
func CustomerRunningUnits(ctx context.Context, db DBTX, customerID int64) ([]RunningUnits, error) {
	return queryAll(ctx, db, func(s rowScanner) (RunningUnits, error) {
		var u RunningUnits
		var placed int64
		err := s.Scan(&u.OrderID, &placed, &u.Units, &u.RunningUnits)
		u.PlacedAt = time.UnixMicro(placed).UTC()
		return u, err
	}, `
		WITH per_order AS (
			SELECT o.id AS order_id, o.placed_at, SUM(oi.quantity) AS units
			FROM orders o
			JOIN order_items oi ON oi.order_id = o.id
			WHERE o.customer_id = ? AND o.status <> 'cancelled'
			GROUP BY o.id, o.placed_at
		)
		SELECT order_id, placed_at, units,
		       SUM(units) OVER (ORDER BY placed_at, order_id
		                        ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW)
		FROM per_order
		ORDER BY placed_at, order_id`, customerID)
}
