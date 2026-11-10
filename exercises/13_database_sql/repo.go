package orders

import (
	"context"
	"database/sql"
	"time"
)

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
	return ""
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
	return Customer{}, errNotImplemented
}

// Get returns the customer with the given ID, or ErrNotFound.
func (r *CustomerRepo) Get(ctx context.Context, id int64) (Customer, error) {
	return Customer{}, errNotImplemented
}

// GetByEmail returns the customer whose email matches case-insensitively, or
// ErrNotFound.
func (r *CustomerRepo) GetByEmail(ctx context.Context, email string) (Customer, error) {
	return Customer{}, errNotImplemented
}

// SearchByName returns customers whose name contains term, case-insensitively,
// ordered by ID. term is matched literally: '%' and '_' are not wildcards. An
// empty term returns ErrInvalidInput.
func (r *CustomerRepo) SearchByName(ctx context.Context, term string) ([]Customer, error) {
	return nil, errNotImplemented
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
	return errNotImplemented
}

// Get returns the product with the given SKU, or ErrNotFound.
func (r *ProductRepo) Get(ctx context.Context, sku string) (Product, error) {
	return Product{}, errNotImplemented
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

// Place creates an order with the given lines in one transaction.
//
// It returns ErrInvalidInput when lines is empty or any quantity is below 1; a
// *UnknownSKUError when any SKU is missing from the catalogue; and ErrNotFound
// when customerID does not exist. Unit prices are copied from the catalogue at
// placement time. Line numbers start at 1 in input order. On any error nothing
// is written.
func (r *OrderRepo) Place(ctx context.Context, customerID int64, placedAt time.Time, lines []Line) (Order, error) {
	return Order{}, errNotImplemented
}

// Get returns the order with the given ID, or ErrNotFound.
func (r *OrderRepo) Get(ctx context.Context, id int64) (Order, error) {
	return Order{}, errNotImplemented
}

// SetStatus changes an order's status. A status outside the four Status
// constants returns ErrInvalidInput; an unknown order returns ErrNotFound.
func (r *OrderRepo) SetStatus(ctx context.Context, id int64, status Status) error {
	return errNotImplemented
}

// Items returns the lines of an order ordered by LineNo. An order with no lines
// or an unknown ID yields an empty slice, not an error.
func (r *OrderRepo) Items(ctx context.Context, orderID int64) ([]OrderItem, error) {
	return nil, errNotImplemented
}
