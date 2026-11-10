# 13 — Database access with `database/sql` and SQLite

## Scenario

You are building the persistence layer of a small order-management backend:
customers place orders for catalogue products, orders move through a status
lifecycle, a nightly job bulk-imports the product catalogue, an API pages
through orders, and a reporting endpoint summarises sales. The store is a single
SQLite file, opened through `modernc.org/sqlite` (pure Go, no cgo, driver name
`"sqlite"`).

Everything lives in package `orders`. The stub files declare the full exported
surface; the tests in `*_test.go` are the executable specification. Read the
Parts in order: later Parts reuse the schema and helpers from earlier ones.

Money is an integer number of cents everywhere. Timestamps are stored as UTC
microseconds and come back as UTC.

## Part 1 — Connection, DSN and `Money`

Files: `db.go`

- `DSN(path string) string` builds the modernc connection string. Every pooled
  connection must get: foreign-key enforcement on, a 5000 ms busy timeout, WAL
  journal mode, and `BEGIN IMMEDIATE` for transactions (`_txlock=immediate`).
  Paths may contain spaces, `%`, `?` and `#`; they must reach SQLite intact.
  Use forward slashes.
- `Open(ctx, path) (*sql.DB, error)` opens the database and pings it under
  `ctx`. The pool is capped at **one** open connection, because SQLite has a
  single writer. A failed open closes the pool and returns a wrapped error. The
  caller owns the handle.
- `Money` is an `int64` count of cents. `String()` prints `"12.34"`,
  `"0.05"`, `"-0.05"`, with no separators, and must handle `math.MinInt64`
  without overflow.

Edge cases the tests cover: a path with spaces works; an unopenable path
(missing directory) fails; a cancelled context fails with `context.Canceled`
in the chain.

## Part 2 — Versioned migrations

Files: `migrate.go`

- `Migrations` is the schema history: v1 creates `customers`, `products`,
  `orders`, `order_items` (all `STRICT`, with `CHECK` constraints and a
  `COLLATE NOCASE` unique email); v2 adds the indexes used by pagination and
  reports.
- `Migrate(ctx, db, migrations) (int, error)` applies every migration above
  `PRAGMA user_version` and returns the resulting version. Each migration plus
  its version bump runs in one transaction, so a failure leaves schema and
  version consistent. Re-running is a no-op.
- Versions must be `1..n` in slice order. Anything else returns
  `ErrInvalidInput` **before** the database is touched.
- A database whose `user_version` is greater than `len(migrations)` returns
  `ErrSchemaTooNew`. Its version is still returned.
- Failures wrap `ErrMigration`, and the driver error stays reachable through
  `errors.As` (`*sqlite.Error`).

Edge cases: a broken statement in v2 rolls back v2 and leaves the version at 1;
a partially migrated database is brought current; `user_version` is readable
after migration.

## Part 3 — Transactions that cannot leak

Files: `tx.go`

- `WithTx(ctx, db, fn) error` runs `fn` in a transaction. It commits when `fn`
  returns nil and rolls back on error.
- A panic inside `fn` rolls back and **is re-raised**, so the caller still sees
  it. Nothing is swallowed.
- `fn`'s error is returned unchanged, so `errors.Is` and `errors.As` see the
  original error through the wrapper.
- A failing `Commit` is returned wrapped; it is not hidden.
- If the rollback itself fails (other than `sql.ErrTxDone`), the rollback error
  is joined with `fn`'s error, and `errors.Is(err, fnErr)` still holds.

Edge cases: a context that is already cancelled stops `WithTx` before `fn` runs;
a `WithTx` that returns nil commits data visible to other queries; a `WithTx`
that errors leaves no rows behind.

## Part 4 — Repositories over `DBTX`

Files: `repo.go`

- `DBTX` is the interface shared by `*sql.DB` and `*sql.Tx`. `CustomerRepo`
  and `ProductRepo` take a `DBTX`, so the same code runs standalone or inside a
  caller's transaction.
- `CustomerRepo`: `Add` (returns the stored row with its generated ID; a
  duplicate email, compared case-insensitively, returns `ErrDuplicateEmail`),
  `Get`, `GetByEmail`, `SearchByName` (case-insensitive substring; `%` and `_`
  are literal; an empty term is `ErrInvalidInput`). Missing rows return
  `ErrNotFound`.
- `ProductRepo`: `Upsert` (insert or replace name and price; a negative price is
  `ErrInvalidInput`), `Get`.
- `OrderRepo` takes `*sql.DB` because `Place` owns its transaction.
  - `Place(ctx, customerID, placedAt, lines)` creates an order in one
    transaction. Prices are snapshotted from the catalogue. Line numbers start at
    1. Empty lines or quantity below 1 is `ErrInvalidInput`. An unknown customer
    is `ErrNotFound`. Unknown SKUs return `*UnknownSKUError` with a sorted,
    de-duplicated `SKUs` list. On any error nothing is written.
  - `Get`, `SetStatus` (only the four `Status` constants; unknown ID is
    `ErrNotFound`), `Items` (ordered by line number; an order with no lines is an
    empty slice, not an error).

Edge cases: `SearchByName` treats `%` literally, so it finds no match rather
than everything; an unknown customer leaves no order behind after `Place`
fails.

## Part 5 — Bulk import with bounded memory

Files: `import.go`

- `ImportProducts(ctx, db, products iter.Seq[Product], batchSize int) (int, error)`
  upserts products in batches. Each batch is its own transaction (`WithTx` plus
  `ProductRepo.Upsert`).
- The input sequence is consumed lazily. Memory is bounded by `batchSize`, not by
  the number of rows.
- It returns the count of rows committed. On error, the failing batch is rolled
  back, iteration stops, and the count covers only earlier, committed batches.
- `batchSize < 1` is `ErrInvalidInput` before any write.

Edge cases: an empty sequence returns 0 and no error; re-importing an existing
SKU updates it; a bad row mid-stream keeps earlier batches committed and rolls
back only its own batch.

## Part 6 — Keyset and offset pagination

Files: `page.go`

- `Cursor{PlacedAt, ID}` is the position in the order list, ordered by
  `(placed_at, id)`. `Encode()` returns an opaque, URL-safe, unpadded token.
  `DecodeCursor` rejects malformed input with `ErrInvalidInput` and never
  panics, on any input.
- `PageOrders(ctx, afterToken, limit) (OrderPage, error)` is keyset pagination:
  `WHERE (placed_at, id) > (?, ?)`. An empty token starts at the beginning.
  `NextToken` is set when the page is full. Pages do not overlap, even when older
  rows are inserted between calls. `limit < 1` and malformed tokens are
  `ErrInvalidInput`.
- `PageOrdersOffset(ctx, offset, limit) ([]Order, error)` is offset pagination.
  It is the contrast case: it **shifts** when rows are inserted before the
  offset. Negative offset or `limit < 1` is `ErrInvalidInput`.

Edge cases: an exact-multiple final page yields one empty page; an empty table
returns an empty page with no token.

## Part 7 — Reports and the N+1 trap

Files: `report.go`

- `LoadCustomerOrders(ctx, customerID) ([]OrderWithItems, error)` returns every
  order (cancelled included) with its items, in `(placed_at, id)` order. It must
  issue **at most two statements** regardless of how many orders the customer
  has; the stub counts statements through a counting driver. A customer with no
  orders gets an empty slice.
- `UnitsSoldByProduct(ctx, db DBTX)` sums quantities per SKU over non-cancelled
  orders; sorted by units descending, then SKU.
- `RevenueByCustomer(ctx, db DBTX)` totals `quantity * unit_price` per customer,
  excluding cancelled orders. Totals are exact integer cents.
- `RankCustomersByOrders(ctx, db DBTX)` ranks **every** customer, including those
  with zero orders, with SQL `RANK()`. Ties share a rank and the next rank is
  skipped.
- `CustomerRunningUnits(ctx, db DBTX, customerID)` returns each non-cancelled
  order's units and a running total via a window function, ordered by
  `(placed_at, order_id)`.

Edge cases: a cancelled order is excluded from every aggregate but included in
`LoadCustomerOrders`; a customer with no orders appears in the ranking with
count 0; ties in `RANK()` skip the next rank.

## Concepts exercised

- `database/sql`: `Open`, pool sizing, `QueryRowContext` vs `QueryContext`,
  `Rows.Close`/`Rows.Err`, `sql.Null[T]`, `sql.ErrNoRows`, `sql.ErrTxDone`.
- Connection strings and SQLite pragmas (foreign keys, WAL, busy timeout,
  `BEGIN IMMEDIATE`) and why `MaxOpenConns(1)` fits SQLite.
- Schema-as-code migrations with `PRAGMA user_version`, transactional DDL, and
  `STRICT` tables with `CHECK` constraints.
- Transaction helpers: commit/rollback discipline, panic safety, `errors.Join`.
- Repositories over a small `DBTX` interface so one implementation serves both
  pool and transaction.
- Error design: sentinel errors plus a typed error (`UnknownSKUError`), `%w`
  wrapping, `errors.As` into `*sqlite.Error` for constraint codes.
- `iter.Seq` for streaming input and bounded-memory batching.
- Opaque, validated keyset cursors vs offset pagination.
- N+1 queries, joins, `RANK()` and window functions, exact integer money math.

## Run

From the repository root (Go 1.27):

```
go test ./exercises/13_database_sql/
go test ./exercises/13_database_sql/ -run Part2 -v
go run ./tools/solutions ./exercises/13_database_sql/
```

On Windows, put the Go 1.27 toolchain first on `PATH`:

```
go test ./exercises/13_database_sql/
```

Concurrency-sensitive tests are not part of this exercise, but race detection
is worth using on Linux or macOS:

```
go test -race ./exercises/13_database_sql/
```

`-race` needs cgo, so it does not run on a default Windows setup.

## Stretch (untested)

- Soft delete with a partial unique index (`WHERE deleted_at IS NULL`) so an
  email can be reused after deletion.
- Read `EXPLAIN QUERY PLAN` for the keyset query and confirm it uses
  `orders_customer_placed_idx`.
- Retry `SQLITE_BUSY` with jittered backoff, and note that `busy_timeout`
  already covers most cases.
- Savepoints (`SAVEPOINT`/`RELEASE`) so `Place` can nest inside a caller's
  transaction.
- Stream a large report with `iter.Seq2[T, error]` instead of building slices.
