package orders_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"sync/atomic"
	"testing"

	orders "goref/exercises/13_database_sql"

	"github.com/google/go-cmp/cmp"
	"modernc.org/sqlite"
)

// stmtCount counts statements executed through countingDriver.
var stmtCount atomic.Int64

func init() {
	sql.Register("goref-counting-sqlite", countingDriver{base: &sqlite.Driver{}})
}

type countingDriver struct{ base driver.Driver }

func (d countingDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &countingConn{Conn: c}, nil
}

type countingConn struct{ driver.Conn }

// QueryContext counts only when the base driver can serve the call directly;
// otherwise database/sql falls back to Prepare, which is counted by countingStmt.
func (c *countingConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	qc, ok := c.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	stmtCount.Add(1)
	return qc.QueryContext(ctx, q, args)
}

func (c *countingConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	ec, ok := c.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	stmtCount.Add(1)
	return ec.ExecContext(ctx, q, args)
}

func (c *countingConn) Prepare(q string) (driver.Stmt, error) {
	s, err := c.Conn.Prepare(q)
	if err != nil {
		return nil, err
	}
	return &countingStmt{Stmt: s}, nil
}

type countingStmt struct{ driver.Stmt }

func (s *countingStmt) Query(args []driver.Value) (driver.Rows, error) {
	stmtCount.Add(1)
	return s.Stmt.Query(args) //nolint:staticcheck // driver.Stmt fallback path
}

func (s *countingStmt) Exec(args []driver.Value) (driver.Result, error) {
	stmtCount.Add(1)
	return s.Stmt.Exec(args) //nolint:staticcheck // driver.Stmt fallback path
}

// openCounting is migratedDB on the counting driver, with a single pooled connection.
func openCounting(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("goref-counting-sqlite", orders.DSN(filepath.Join(t.TempDir(), "count.db")))
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := orders.Migrate(t.Context(), db, orders.Migrations); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

type reportFixture struct {
	ann, bob, cy, dee  orders.Customer
	o1, o2, o3, o4, o5 orders.Order
}

// seedReports builds the data set documented in the Part 7 README:
//
//	ann: o1 (2x BOOK-1, 1x MUG-7, 01-01), o3 (1x MUG-7, 01-03 12:00, cancelled), o4 (1x BOOK-1, 01-03 08:00)
//	bob: o2 (3x CABLE-2, 01-02)
//	cy:  no orders
//	dee: o5 (1x CABLE-2, 01-04)
func seedReports(t *testing.T, db *sql.DB) reportFixture {
	t.Helper()
	seedCatalogue(t, db)
	var f reportFixture
	f.ann = addCustomer(t, db, "ann@example.com", "Ann")
	f.bob = addCustomer(t, db, "bob@example.com", "Bob")
	f.cy = addCustomer(t, db, "cy@example.com", "Cy")
	f.dee = addCustomer(t, db, "dee@example.com", "Dee")
	f.o1 = placeAt(t, db, f.ann.ID, "2026-01-01T10:00:00Z",
		orders.Line{SKU: "BOOK-1", Quantity: 2}, orders.Line{SKU: "MUG-7", Quantity: 1})
	f.o2 = placeAt(t, db, f.bob.ID, "2026-01-02T09:00:00Z", orders.Line{SKU: "CABLE-2", Quantity: 3})
	f.o3 = placeAt(t, db, f.ann.ID, "2026-01-03T12:00:00Z", orders.Line{SKU: "MUG-7", Quantity: 1})
	if err := orders.NewOrderRepo(db).SetStatus(t.Context(), f.o3.ID, orders.StatusCancelled); err != nil {
		t.Fatalf("cancel o3: %v", err)
	}
	f.o4 = placeAt(t, db, f.ann.ID, "2026-01-03T08:00:00Z", orders.Line{SKU: "BOOK-1", Quantity: 1})
	f.o5 = placeAt(t, db, f.dee.ID, "2026-01-04T09:00:00Z", orders.Line{SKU: "CABLE-2", Quantity: 1})
	return f
}

func TestPart7Reports(t *testing.T) {
	t.Run("units sold by product", func(t *testing.T) {
		db := migratedDB(t)
		seedReports(t, db)
		got, err := orders.UnitsSoldByProduct(t.Context(), db)
		if err != nil {
			t.Fatalf("UnitsSoldByProduct: %v", err)
		}
		want := []orders.ProductSales{
			{SKU: "CABLE-2", UnitsSold: 4, OrderCount: 2},
			{SKU: "BOOK-1", UnitsSold: 3, OrderCount: 2},
			{SKU: "MUG-7", UnitsSold: 1, OrderCount: 1},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("revenue by customer is exact and ordered by id", func(t *testing.T) {
		db := migratedDB(t)
		f := seedReports(t, db)
		got, err := orders.RevenueByCustomer(t.Context(), db)
		if err != nil {
			t.Fatalf("RevenueByCustomer: %v", err)
		}
		want := []orders.CustomerRevenue{
			{CustomerID: f.ann.ID, Name: "Ann", OrderCount: 2, Total: 2*4999 + 1250 + 4999},
			{CustomerID: f.bob.ID, Name: "Bob", OrderCount: 1, Total: 3 * 899},
			{CustomerID: f.dee.ID, Name: "Dee", OrderCount: 1, Total: 899},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("rank customers with RANK() ties and gaps", func(t *testing.T) {
		db := migratedDB(t)
		f := seedReports(t, db)
		got, err := orders.RankCustomersByOrders(t.Context(), db)
		if err != nil {
			t.Fatalf("RankCustomersByOrders: %v", err)
		}
		want := []orders.RankedCustomer{
			{CustomerID: f.ann.ID, Name: "Ann", OrderCount: 2, Rank: 1},
			{CustomerID: f.bob.ID, Name: "Bob", OrderCount: 1, Rank: 2},
			{CustomerID: f.dee.ID, Name: "Dee", OrderCount: 1, Rank: 2},
			{CustomerID: f.cy.ID, Name: "Cy", OrderCount: 0, Rank: 4},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("customer running units", func(t *testing.T) {
		db := migratedDB(t)
		f := seedReports(t, db)
		ann, err := orders.CustomerRunningUnits(t.Context(), db, f.ann.ID)
		if err != nil {
			t.Fatalf("CustomerRunningUnits(ann): %v", err)
		}
		wantAnn := []orders.RunningUnits{
			{OrderID: f.o1.ID, PlacedAt: f.o1.PlacedAt, Units: 3, RunningUnits: 3},
			{OrderID: f.o4.ID, PlacedAt: f.o4.PlacedAt, Units: 1, RunningUnits: 4},
		}
		if diff := cmp.Diff(wantAnn, ann); diff != "" {
			t.Errorf("ann mismatch (-want +got):\n%s", diff)
		}
		bob, err := orders.CustomerRunningUnits(t.Context(), db, f.bob.ID)
		if err != nil {
			t.Fatalf("CustomerRunningUnits(bob): %v", err)
		}
		wantBob := []orders.RunningUnits{{OrderID: f.o2.ID, PlacedAt: f.o2.PlacedAt, Units: 3, RunningUnits: 3}}
		if diff := cmp.Diff(wantBob, bob); diff != "" {
			t.Errorf("bob mismatch (-want +got):\n%s", diff)
		}
		cy, err := orders.CustomerRunningUnits(t.Context(), db, f.cy.ID)
		if err != nil || len(cy) != 0 {
			t.Errorf("CustomerRunningUnits(cy) = %v, %v; want empty, nil", cy, err)
		}
	})
}

func TestPart7LoadCustomerOrders(t *testing.T) {
	t.Run("orders with lines in (placed_at, id) order, cancelled included", func(t *testing.T) {
		db := migratedDB(t)
		f := seedReports(t, db)
		got, err := orders.NewOrderRepo(db).LoadCustomerOrders(t.Context(), f.ann.ID)
		if err != nil {
			t.Fatalf("LoadCustomerOrders: %v", err)
		}
		want := []orders.OrderWithItems{
			{
				Order: f.o1,
				Items: []orders.OrderItem{
					{OrderID: f.o1.ID, LineNo: 1, SKU: "BOOK-1", Quantity: 2, UnitPrice: 4999},
					{OrderID: f.o1.ID, LineNo: 2, SKU: "MUG-7", Quantity: 1, UnitPrice: 1250},
				},
			},
			{
				Order: f.o4,
				Items: []orders.OrderItem{{OrderID: f.o4.ID, LineNo: 1, SKU: "BOOK-1", Quantity: 1, UnitPrice: 4999}},
			},
			{
				Order: withStatus(f.o3, orders.StatusCancelled),
				Items: []orders.OrderItem{{OrderID: f.o3.ID, LineNo: 1, SKU: "MUG-7", Quantity: 1, UnitPrice: 1250}},
			},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("customer without orders gets an empty slice", func(t *testing.T) {
		db := migratedDB(t)
		f := seedReports(t, db)
		got, err := orders.NewOrderRepo(db).LoadCustomerOrders(t.Context(), f.cy.ID)
		if err != nil || len(got) != 0 {
			t.Errorf("LoadCustomerOrders(cy) = %v, %v; want empty, nil", got, err)
		}
	})

	t.Run("statement count does not grow with the order count", func(t *testing.T) {
		db := openCounting(t)
		seedCatalogue(t, db)
		small := addCustomer(t, db, "small@example.com", "Small")
		big := addCustomer(t, db, "big@example.com", "Big")
		for range 3 {
			placeAt(t, db, small.ID, "2026-02-01T10:00:00Z", orders.Line{SKU: "MUG-7", Quantity: 1})
		}
		for range 40 {
			placeAt(t, db, big.ID, "2026-02-01T10:00:00Z",
				orders.Line{SKU: "BOOK-1", Quantity: 1}, orders.Line{SKU: "MUG-7", Quantity: 1})
		}
		repo := orders.NewOrderRepo(db)
		count := func(id int64) (statements int64, orderCount int) {
			stmtCount.Store(0)
			got, err := repo.LoadCustomerOrders(t.Context(), id)
			if err != nil {
				t.Fatalf("LoadCustomerOrders(%d): %v", id, err)
			}
			return stmtCount.Load(), len(got)
		}
		smallN, smallOrders := count(small.ID)
		bigN, bigOrders := count(big.ID)
		if smallOrders != 3 || bigOrders != 40 {
			t.Fatalf("order counts = %d, %d; want 3, 40", smallOrders, bigOrders)
		}
		if bigN > 2 {
			t.Errorf("LoadCustomerOrders with 40 orders ran %d statements, want at most 2", bigN)
		}
		if smallN != bigN {
			t.Errorf("statements with 3 orders = %d, with 40 = %d; want equal (no N+1)", smallN, bigN)
		}
	})
}

func withStatus(o orders.Order, s orders.Status) orders.Order {
	o.Status = s
	return o
}
