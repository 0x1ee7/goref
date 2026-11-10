package orders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"modernc.org/sqlite"
)

// sqliteConstraintUnique is SQLITE_CONSTRAINT_UNIQUE (extended result code).
const sqliteConstraintUnique = 2067

// DBTX is the query surface shared by *sql.DB and *sql.Tx, so a repository can
// run either standalone or inside a caller's transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Status is the lifecycle state of an order.
type Status string

// Order statuses. The database enforces this set with a CHECK constraint.
const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusShipped   Status = "shipped"
	StatusCancelled Status = "cancelled"
)

// Customer is a stored customer. Email comparisons are case-insensitive.
// CreatedAt is UTC with microsecond precision.
type Customer struct {
	ID        int64
	Email     string
	Name      string
	Phone     sql.Null[string] // optional
	CreatedAt time.Time
}

// NewCustomer is the input to CustomerRepo.Add.
type NewCustomer struct {
	Email     string
	Name      string
	Phone     sql.Null[string]
	CreatedAt time.Time
}

// Product is a catalogue entry. Price is in cents.
type Product struct {
	SKU   string
	Name  string
	Price Money
}

// Order is an order header. PlacedAt is UTC with microsecond precision.
type Order struct {
	ID         int64
	CustomerID int64
	PlacedAt   time.Time
	Status     Status
}

// Line is one requested item in Place.
type Line struct {
	SKU      string
	Quantity int
}

// OrderItem is a stored order line. UnitPrice is the price snapshotted from the
// catalogue when the order was placed.
type OrderItem struct {
	OrderID   int64
	LineNo    int // 1-based, in input order
	SKU       string
	Quantity  int
	UnitPrice Money
}

// UnknownSKUError is returned by OrderRepo.Place when some requested SKUs are
// not in the catalogue. SKUs is sorted and de-duplicated.
type UnknownSKUError struct {
	SKUs []string
}

func (e *UnknownSKUError) Error() string {
	return fmt.Sprintf("orders: unknown SKUs: %s", strings.Join(e.SKUs, ", "))
}

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// queryAll runs query and maps every row through scan.
func queryAll[T any](ctx context.Context, db DBTX, scan func(rowScanner) (T, error), query string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("orders: query: %w", err)
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("orders: scan: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("orders: iterate: %w", err)
	}
	return out, nil
}

const customerCols = `id, email, name, phone, created_at`

func scanCustomer(s rowScanner) (Customer, error) {
	var c Customer
	var created int64
	if err := s.Scan(&c.ID, &c.Email, &c.Name, &c.Phone, &created); err != nil {
		return Customer{}, err
	}
	c.CreatedAt = time.UnixMicro(created).UTC()
	return c, nil
}

// CustomerRepo reads and writes customers through any DBTX.
type CustomerRepo struct {
	db DBTX
}

// NewCustomerRepo returns a repository that runs its queries on db.
func NewCustomerRepo(db DBTX) *CustomerRepo {
	return &CustomerRepo{db: db}
}

// Add inserts a customer and returns the stored row with its generated ID.
// It returns an error wrapping ErrDuplicateEmail when the email is taken.
func (r *CustomerRepo) Add(ctx context.Context, c NewCustomer) (Customer, error) {
	if strings.TrimSpace(c.Email) == "" {
		return Customer{}, fmt.Errorf("%w: empty email", ErrInvalidInput)
	}
	created := time.UnixMicro(c.CreatedAt.UnixMicro()).UTC()
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO customers (email, name, phone, created_at) VALUES (?, ?, ?, ?) RETURNING id`,
		c.Email, c.Name, c.Phone, created.UnixMicro(),
	).Scan(&id)
	if err != nil {
		var se *sqlite.Error
		if errors.As(err, &se) && se.Code() == sqliteConstraintUnique {
			return Customer{}, fmt.Errorf("%w: %s", ErrDuplicateEmail, c.Email)
		}
		return Customer{}, fmt.Errorf("orders: add customer: %w", err)
	}
	return Customer{ID: id, Email: c.Email, Name: c.Name, Phone: c.Phone, CreatedAt: created}, nil
}

// Get returns the customer with the given ID, or ErrNotFound.
func (r *CustomerRepo) Get(ctx context.Context, id int64) (Customer, error) {
	return r.queryOne(ctx, `SELECT `+customerCols+` FROM customers WHERE id = ?`, id)
}

// GetByEmail returns the customer whose email matches case-insensitively, or
// ErrNotFound.
func (r *CustomerRepo) GetByEmail(ctx context.Context, email string) (Customer, error) {
	return r.queryOne(ctx, `SELECT `+customerCols+` FROM customers WHERE email = ?`, email)
}

func (r *CustomerRepo) queryOne(ctx context.Context, query string, args ...any) (Customer, error) {
	c, err := scanCustomer(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Customer{}, ErrNotFound
	}
	if err != nil {
		return Customer{}, fmt.Errorf("orders: get customer: %w", err)
	}
	return c, nil
}

// likeEscaper makes a search term literal inside a LIKE pattern.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// SearchByName returns customers whose name contains term, case-insensitively,
// ordered by ID. term is matched literally: '%' and '_' are not wildcards. An
// empty term returns ErrInvalidInput.
func (r *CustomerRepo) SearchByName(ctx context.Context, term string) ([]Customer, error) {
	if term == "" {
		return nil, fmt.Errorf("%w: empty search term", ErrInvalidInput)
	}
	pattern := "%" + likeEscaper.Replace(term) + "%"
	return queryAll(ctx, r.db, scanCustomer,
		`SELECT `+customerCols+` FROM customers WHERE name LIKE ? ESCAPE '\' ORDER BY id`, pattern)
}

// ProductRepo reads and writes catalogue entries through any DBTX.
type ProductRepo struct {
	db DBTX
}

// NewProductRepo returns a repository that runs its queries on db.
func NewProductRepo(db DBTX) *ProductRepo {
	return &ProductRepo{db: db}
}

// Upsert inserts the product or, if the SKU exists, replaces its name and
// price. A negative price returns ErrInvalidInput and writes nothing.
func (r *ProductRepo) Upsert(ctx context.Context, p Product) error {
	if p.Price < 0 {
		return fmt.Errorf("%w: product %s has negative price %d", ErrInvalidInput, p.SKU, p.Price)
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO products (sku, name, price_cents) VALUES (?, ?, ?)
		 ON CONFLICT (sku) DO UPDATE SET name = excluded.name, price_cents = excluded.price_cents`,
		p.SKU, p.Name, int64(p.Price))
	if err != nil {
		return fmt.Errorf("orders: upsert product %s: %w", p.SKU, err)
	}
	return nil
}

// Get returns the product with the given SKU, or ErrNotFound.
func (r *ProductRepo) Get(ctx context.Context, sku string) (Product, error) {
	var p Product
	var price int64
	err := r.db.QueryRowContext(ctx, `SELECT sku, name, price_cents FROM products WHERE sku = ?`, sku).
		Scan(&p.SKU, &p.Name, &price)
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	if err != nil {
		return Product{}, fmt.Errorf("orders: get product %s: %w", sku, err)
	}
	p.Price = Money(price)
	return p, nil
}

// OrderRepo places and reads orders. It takes a *sql.DB because Place owns its
// transaction; callers must not invoke it from inside their own transaction.
type OrderRepo struct {
	db *sql.DB
}

// NewOrderRepo returns a repository bound to db.
func NewOrderRepo(db *sql.DB) *OrderRepo {
	return &OrderRepo{db: db}
}

const orderCols = `id, customer_id, placed_at, status`

func scanOrder(s rowScanner) (Order, error) {
	var o Order
	var placed int64
	var status string
	if err := s.Scan(&o.ID, &o.CustomerID, &placed, &status); err != nil {
		return Order{}, err
	}
	o.PlacedAt = time.UnixMicro(placed).UTC()
	o.Status = Status(status)
	return o, nil
}

func scanItem(s rowScanner) (OrderItem, error) {
	var it OrderItem
	var price int64
	if err := s.Scan(&it.OrderID, &it.LineNo, &it.SKU, &it.Quantity, &price); err != nil {
		return OrderItem{}, err
	}
	it.UnitPrice = Money(price)
	return it, nil
}

// Place creates an order with the given lines in one transaction.
//
// It returns ErrInvalidInput when lines is empty or any quantity is below 1; a
// *UnknownSKUError when any SKU is missing from the catalogue; and ErrNotFound
// when customerID does not exist. Unit prices are copied from the catalogue at
// placement time. Line numbers start at 1 in input order. On any error nothing
// is written.
func (r *OrderRepo) Place(ctx context.Context, customerID int64, placedAt time.Time, lines []Line) (Order, error) {
	if len(lines) == 0 {
		return Order{}, fmt.Errorf("%w: order has no lines", ErrInvalidInput)
	}
	for _, l := range lines {
		if l.Quantity < 1 {
			return Order{}, fmt.Errorf("%w: quantity %d for %s", ErrInvalidInput, l.Quantity, l.SKU)
		}
	}
	placed := time.UnixMicro(placedAt.UnixMicro()).UTC()

	var order Order
	err := WithTx(ctx, r.db, func(tx *sql.Tx) error {
		var one int
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM customers WHERE id = ?`, customerID).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: customer %d", ErrNotFound, customerID)
		}
		if err != nil {
			return fmt.Errorf("orders: check customer: %w", err)
		}

		prices := make(map[string]int64, len(lines))
		unknown := make(map[string]struct{})
		for _, l := range lines {
			if _, ok := prices[l.SKU]; ok {
				continue
			}
			if _, ok := unknown[l.SKU]; ok {
				continue
			}
			var p int64
			err := tx.QueryRowContext(ctx, `SELECT price_cents FROM products WHERE sku = ?`, l.SKU).Scan(&p)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				unknown[l.SKU] = struct{}{}
			case err != nil:
				return fmt.Errorf("orders: price of %s: %w", l.SKU, err)
			default:
				prices[l.SKU] = p
			}
		}
		if len(unknown) > 0 {
			return &UnknownSKUError{SKUs: slices.Sorted(maps.Keys(unknown))}
		}

		res, err := tx.ExecContext(ctx,
			`INSERT INTO orders (customer_id, placed_at, status) VALUES (?, ?, ?)`,
			customerID, placed.UnixMicro(), string(StatusPending))
		if err != nil {
			return fmt.Errorf("orders: insert order: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("orders: order id: %w", err)
		}
		for i, l := range lines {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO order_items (order_id, line_no, sku, quantity, unit_price_cents) VALUES (?, ?, ?, ?, ?)`,
				id, i+1, l.SKU, l.Quantity, prices[l.SKU])
			if err != nil {
				return fmt.Errorf("orders: insert line %d: %w", i+1, err)
			}
		}
		order = Order{ID: id, CustomerID: customerID, PlacedAt: placed, Status: StatusPending}
		return nil
	})
	if err != nil {
		return Order{}, err
	}
	return order, nil
}

// Get returns the order with the given ID, or ErrNotFound.
func (r *OrderRepo) Get(ctx context.Context, id int64) (Order, error) {
	o, err := scanOrder(r.db.QueryRowContext(ctx, `SELECT `+orderCols+` FROM orders WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Order{}, ErrNotFound
	}
	if err != nil {
		return Order{}, fmt.Errorf("orders: get order %d: %w", id, err)
	}
	return o, nil
}

// SetStatus changes an order's status. A status outside the four Status
// constants returns ErrInvalidInput; an unknown order returns ErrNotFound.
func (r *OrderRepo) SetStatus(ctx context.Context, id int64, status Status) error {
	switch status {
	case StatusPending, StatusPaid, StatusShipped, StatusCancelled:
	default:
		return fmt.Errorf("%w: status %q", ErrInvalidInput, status)
	}
	res, err := r.db.ExecContext(ctx, `UPDATE orders SET status = ? WHERE id = ?`, string(status), id)
	if err != nil {
		return fmt.Errorf("orders: set status of order %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("orders: rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: order %d", ErrNotFound, id)
	}
	return nil
}

// Items returns the lines of an order ordered by LineNo. An order with no lines
// or an unknown ID yields an empty slice, not an error.
func (r *OrderRepo) Items(ctx context.Context, orderID int64) ([]OrderItem, error) {
	return queryAll(ctx, r.db, scanItem,
		`SELECT order_id, line_no, sku, quantity, unit_price_cents FROM order_items
		 WHERE order_id = ? ORDER BY line_no`, orderID)
}
