# 06 · Functional programming with generics

## Scenario

You own a small set of service modules that lean on functional style: a
generic collection toolkit, a cart reducer for a checkout API, an
event-sourced account, a config and port parser, an input validator for
signup payloads, and a nightly sales ETL that produces a revenue report.
Every module is built from small, pure functions that are composed together.

Go is not Python. There are no decorators, default arguments, or keyword
arguments, and the type system only gives you generics, so the idiomatic
answer is often a plain loop, a `(T, error)` pair, or a named type with
methods. This exercise asks you to use functional techniques where they pay
off and to write ordinary Go where they do not.

## Parts

### Part 1 · Generic combinators

File: `combinators.go`

- `Map[T, U any](xs []T, f func(T) U) []U`: never nil; empty input gives an
  empty slice.
- `Filter[T any](xs []T, keep func(T) bool) []T`: input order; a fresh
  slice that never aliases `xs`; never nil.
- `Reduce[T, A any](xs []T, init A, f func(A, T) A) A`: left fold.
- `TryReduce[T, A any](xs []T, init A, f func(A, T) (A, error)) (A, error)`:
  stops at the first error, returns the zero `A`, and does not call `f` again.
- `GroupBy[T any, K comparable](xs []T, key func(T) K) map[K][]T`: keeps
  input order inside each bucket; never nil.
- `MapSeq`, `FilterSeq`, `ReduceSeq`, `Enumerate` over `iter.Seq`. The lazy
  forms pull nothing until ranged over, and a `break` in the consumer stops
  upstream too. `Enumerate` yields `iter.Seq2[int, T]` with zero-based indexes.

Edge cases: nil and empty slices; `TryReduce` error short-circuit; `MapSeq`
laziness and early break; `GroupBy` on an empty input.

### Part 2 · Composition and currying

File: `compose.go`

- `Compose[A, B, C any](f func(B) C, g func(A) B) func(A) C`: right to left,
  `f(g(x))`.
- `Pipe[T any](v T, stages ...func(T) T) T`: left to right over one type.
  No stages returns `v` unchanged.
- `Curry2`, `Curry3`: turn multi-argument functions into chains of
  one-argument calls. Each partial application captures its own argument.
- `Flip[A, B, R any](f func(A, B) R) func(B, A) R`.
- `Bind[A, B, R any](f func(A, B) R, a A) func(B) R`: the closure analogue of
  `functools.partial`. Go has no default or keyword arguments, so binding the
  leading argument is the practical substitute.

Edge cases: order of composition; partial applications stay independent of
each other.

### Part 3 · Immutable cart reducer

File: `cart.go`

Sentinel errors: `ErrUnknownAction`, `ErrInvalidQuantity`, `ErrInvalidPrice`,
`ErrUnknownSKU`, `ErrUnknownCoupon`. Use `errors.Is`.

- Types: `LineItem{SKU, Name, UnitPriceCents, Quantity, Labels}`,
  `Cart{Items, Coupon}`. The zero `Cart` is an empty cart with no coupon.
- Actions are plain data: `AddItem` (build with `NewAddItem(sku, name,
  price)`, which sets quantity 1), `RemoveItem{SKU}`,
  `SetQuantity{SKU, Quantity}`, `ApplyCoupon{Code}`, `ClearCart{}`.
- `CouponPercent(code string) (int, bool)`: the known coupons and their
  percent off.
- `CartReducer(state Cart, action any) (Cart, error)`: returns a new cart.
  It never modifies `state` or `action`, and the result shares no memory with
  either. On error, the returned cart is ignored.
- `CartSubtotal(cart) int64` and `CartTotal(cart) int64`. The discount is
  floored to whole cents. An unknown coupon in state grants no discount.
- `RunActions(state Cart, actions []any) (Cart, error)`: folds with the
  reducer and stops at the first error. The input slice is not modified.

Edge cases: adding an existing SKU increases its quantity; `SetQuantity` 0
removes the line; a negative price or quantity is rejected; `Labels` is a map,
so a naive struct copy shares it and the reducer must deep-copy it; unknown
action types return `ErrUnknownAction`.

### Part 4 · Event-sourced account and snapshots

File: `account.go`

Sentinel errors: `ErrAlreadyOpened`, `ErrNotOpened`, `ErrNonPositiveAmount`,
`ErrAccountFrozen`, `ErrInsufficientFunds`, `ErrEmptyLog`, `ErrUnknownEvent`.

- Events: `Opened{AccountID, Owner}`, `Deposited{AmountCents}`,
  `Withdrawn{AmountCents}`, `Frozen{}`, `Unfrozen{}`.
- `Apply(acc *Account, event any) (*Account, error)`: `acc` is nil before the
  account exists. It returns a fresh `*Account` and leaves `acc` unchanged, so
  an earlier snapshot stays valid.
- `Replay(events []any) (*Account, error)`: an empty log is `ErrEmptyLog`.
- `ReplayFrom(snapshot *Account, events []any) (*Account, error)`: resumes from a
  snapshot. The result is always a copy, even with no events. Error indexes
  count from `events[0]`.
- `Snapshots(events []any) ([]*Account, error)`: one entry per event. The last
  entry equals `Replay(events)`. Every entry is a separate value.
- `DecideWithdraw(acc *Account, amountCents int64) (Withdrawn, error)`: the
  command half of "decide, then evolve". It validates against the current
  state and returns the event to record without changing `acc`.

Edge cases: operations on a frozen account; withdrawing more than the
balance; `Frozen` twice is harmless; `Unfrozen` restores `StatusOpen`; events
before `Opened`; snapshot aliasing.

### Part 5 · Result, ports, and config parsing

Files: `result.go`, `config.go`

`Result[T]` is a secondary helper. `(T, error)` remains the primary Go style.

- `Ok[T](v)`, `Err[T](err)`. `Err(nil)` is indistinguishable from `Ok(zero)`,
  so never pass nil.
- Methods: `Unwrap() (T, error)`, `IsOK() bool`, `OrElse(fallback T) T`.
  Methods cannot take type parameters, so the transforms are functions.
- `MapResult[T, U]`, `BindResult[T, U]`: skip the step after an error.
- `Lift[T, U](f func(T) (U, error)) func(T) Result[U]`.
- `CollectResults[T](rs iter.Seq[Result[T]]) ([]T, error)`: stops consuming at
  the first failure. The success slice is never nil.
- `ParsePort(raw string) (int, error)`: 1..65535, whitespace trimmed.
- `ParsePorts(raw string) ([]int, error)`: comma separated. It fails on the
  first bad entry and never parses the later ones.
- `ParseBool(raw string) (bool, error)`: case-insensitive `true|yes|1|on` and
  `false|no|0|off`.
- `ParseConfig(text string) (Config, error)` with `Config{Host, Port, Debug,
  Workers}`. Lines are `key = value`; blank and `#` lines are ignored; line
  numbers count physical lines. Defaults: `Workers` 4, `Debug` false. Error
  text is part of the spec and is listed in the doc comment of `config.go`.

Edge cases: `ParsePort("0")` and `"65536"`; a duplicate key; the lexically
first unknown key is reported; `workers` outside 1..64.

### Part 6 · Validator combinators

File: `validate.go`

Input is untrusted and JSON-shaped, so values are `any` (objects are
`map[string]any`, arrays are `[]any`). A `Rule` is `func(value any) []error`.
A nil result means valid. Rules never panic on bad input.

- `IsType[T]()`, `IsList()`, `MinLength(n)` (counts runes), `FullMatch(pattern)`
  (wraps the pattern in `^(?:…)$`, panics on an invalid pattern),
  `InRange(lo, hi float64)` (ints and floats; other types pass).
- `AllOf(rules...)`: runs all rules and joins their errors, so a form reports
  every problem.
- `AnyOf(rules...)`: stops at the first passing rule. If none pass, returns all
  errors. Panics when given no rules.
- `Field(name, rule)` prefixes errors with `name: `. A missing key yields
  `name: missing`, and `errors.Is(err, ErrMissing)` holds. `OptionalField`
  treats a missing key as valid. `Optional(rule)` treats `null` as valid.
- `Each(rule)`: applies to every `[]any` element and prefixes `[i] `.
- `Validate(rule, value) error`: joins errors with `errors.Join`, or returns nil.
- `ValidateSignup(payload any) []error`: a non-object is reported first, and the
  field checks are skipped. Field order: username, age, email, then optional
  tags.

Gotcha to note: `ValidateSignup` checks `age` with `IsType[int]()`. Numbers
decoded from JSON into `any` arrive as `float64`, so a JSON `42` fails with
`age must be an int, not a float`. This is deliberate. A caller who wants JSON
semantics should decode with `UseNumber` or convert first, and the README of
the validator should say so. The test suite checks this behaviour.

Edge cases: `AnyOf` short-circuits; `Each` on a non-list; `null` through
`Optional`; `FullMatch` must match the whole string; `InRange` ignores bools.

### Part 7 · ETL and sales report

File: `etl.go`

- `ParseCents(raw string) (int64, error)`: non-negative dollars to cents
  without floats. `"12.5"` is 1250, `"12"` is 1200, `"0.01"` is 1. Rejects a
  sign, an exponent, `".5"`, `"12."`, and more than two decimals.
- `Row` is `map[string]string`; a missing column reads as `""`.
- `ParseRow(row Row) (Sale, error)`: trims fields, lowercases the region,
  rejects negative units. `Sale{Region, Product, Units, UnitPriceCents, SoldOn}`.
- `CleanSales(rows iter.Seq[Row]) ([]Sale, error)`: drops rows with a blank
  region or zero units, parses the rest in order, and reads the sequence once.
  It returns the first bad row with a 1-based position in the error. The
  result is never nil.
- `RevenueCents(s Sale) int64`.
- `TotalsBy(sales, key func(Sale) string) map[string]int64`.
- `Ranked(totals map[string]int64) []Total`: revenue descending, then key
  ascending. Map order is random, so the tie-break is what makes the output
  deterministic.
- `SortSales(sales) []Sale`: a stable copy ordered by date, region, product.
  The input is not modified.
- `DistinctRegions(sales) []string`: sorted, unique, upper case.
- `FormatCents(cents int64, currency string) (string, error)`: `1234567` is
  `"$12,345.67"`. A negative amount is an error.
- `BuildReport(rows iter.Seq[Row], currency string) ([]string, error)`: one
  `"region: $amount"` line per region, ranked by revenue. Each step passes its
  value to the next, and a failing step ends the pipeline.

Edge cases: a blank region is dropped, but a bad units value is an error with
its row number; ties in revenue; thousands separators; zero-sale regions.

## Concepts exercised

- Generic functions with type parameters and constraints (`any`,
  `comparable`), and inference at call sites.
- Pure functions, value semantics, and defensive copies of slices and maps.
- Function values, closures, and currying by hand.
- `iter.Seq` and `iter.Seq2`: lazy pipelines, early `break`, single-pass input.
- Sentinel errors, `errors.Is`, `errors.Join`, and wrapped error positions.
- Discriminated unions via `any` and type switches with a `default` error.
- Event sourcing and snapshots through the fold (`Apply`, `Replay`).
- Trade-offs between `Result[T]` and `(T, error)`.
- Validating untrusted, JSON-shaped input with composable rules.
- Integer money (cents), `time.Time` parsing, and deterministic ordering.

## Run

From the repository root:

```sh
go test ./exercises/06_functional_generics/
go test ./exercises/06_functional_generics/ -run Part2 -v
go run ./tools/solutions ./exercises/06_functional_generics/
```

On Linux and macOS, also run the suite with the race detector:

```sh
go test -race ./exercises/06_functional_generics/
```

(The race detector needs cgo, so it does not run on this Windows setup.)

## Stretch ideas (untested)

- Add a `Memoize[K comparable, V any]` wrapper with `sync.Mutex` and a
  `singleflight` variant from `golang.org/x/sync`. Explain why the cache must
  not hold a lock during `f`.
- Rewrite `Pipe` as a typed builder that composes stages of different types.
  Note what Go's type system refuses and why.
- Replace `Snapshots` with an `iter.Seq2[int, *Account]` and compare memory
  behaviour for large logs.
- Port `ValidateSignup` to a `Rule` built from a schema struct and reflection.
  Decide whether the reflection cost is worth the code it removes.
- Add a `ParseConfig` variant that reports every error with
  `errors.Join` instead of the first one.
