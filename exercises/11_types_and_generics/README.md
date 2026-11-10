# 11 — Types and Generics

## Scenario

You are building the typed core of an order service: domain IDs that must
never be confused with each other, an order lifecycle enum that serializes
safely, a handful of generic helpers, collections used by the request path,
an in-memory repository shared between goroutines, an event bus that routes by
type, and a unit-aware distance type for a shipping API.

Everything lives in package `typed` (directory `exercises/11_types_and_generics`).
Implement the stubs in the `.go` files; the tests are the executable spec.

## Part 1 — Defined types and text marshalling (`ids.go`)

`UserID` and `OrderID` are distinct defined types over `int64`. Valid IDs are
positive; zero means "unset".

- `ParseUserID("usr_42")`, `ParseOrderID("ord_7")`. Malformed text, the wrong
  prefix (`usr_7` is not an `OrderID`), or a non-positive number wrap
  `ErrInvalidID`.
- `String()` always returns `usr_<n>` / `ord_<n>`, even for zero or negative
  values. `%v` uses it; `%d` does not.
- `MarshalText` rejects non-positive IDs with `ErrInvalidID`. `UnmarshalText`
  leaves the receiver unchanged on error.
- Because `encoding/json` honours `TextMarshaler` for values and for map keys,
  IDs serialize as strings: `{"owner":"usr_42","order":"ord_7"}`.

## Part 2 — Enums with a safe zero value (`enums.go`)

`OrderStatus` is an `int` enum: `OrderStatusUnspecified` (the zero value),
`Pending`, `Paid`, `Shipped`, `Cancelled`.

- `Unspecified` is never valid. `Valid()` is false for it, and `MarshalText`
  rejects it (`ErrInvalidOrderStatus`), so an unset field cannot slip out as a
  real status.
- `ParseOrderStatus` trims whitespace and is case-insensitive. `"unspecified"`
  and unknown names are errors.
- `String()` gives the lowercase name. `OrderStatus(99)` renders as
  `OrderStatus(99)`.
- `CanTransition(next)`: Pending → {Paid, Cancelled}; Paid → {Shipped,
  Cancelled}; nothing else. Invalid states on either side never transition.
- `AllOrderStatuses()` is an `iter.Seq` over the valid states in declaration order.

## Part 3 — Generic functions and constraints (`constraints.go`)

- `Number` is a type set `~int | ~int32 | ~int64 | ~float64`. The `~` makes
  `type Cents int64` work with `Sum`.
- `Sum[T Number]` returns the zero value for empty input.
- `Max[T cmp.Ordered]` returns `ErrEmpty` for empty input.
- `Clamp[T cmp.Ordered]` returns `ErrBounds` when `lo > hi`.
- `GroupBy` keeps input order inside each group and never returns nil.
- `SortedKeys` returns a non-nil empty slice for a nil or empty map.
- `ParsePort` accepts `1..65535` after trimming. Everything else wraps
  `ErrInvalidPort`.
- `Setting[T]` reads configuration from a `map[string]string`:
  - `Required(key, parse)`: absent or blank yields `ErrMissing` (the message
    names the key).
  - `WithDefault(key, def, parse)`: falls back to `def` when absent or blank. A
    present but unparseable value is an error and is **not** replaced by the
    default.
  - The raw value is trimmed before `parse` runs. Parse errors are wrapped with
    the key and keep their chain, so `errors.Is` still works.

## Part 4 — Generic collections (`collections.go`)

- `Set[T comparable]`: the zero value is usable. `Add` and `Remove` have
  pointer receivers; the reads take values. `NewSet`, `Has`, `Len`, `All`
  (an `iter.Seq`), `Clone`, `Union`, `Intersect`, `Difference`, `Equal`.
  `Clone` returns an independent copy.
- `OrderedMap[K comparable, V any]`: the zero value is usable. It keeps
  insertion order. Overwriting a key keeps its position; deleting and
  re-adding moves it to the end. `Keys()` yields `iter.Seq[K]`, `All()` yields
  `iter.Seq2[K, V]`.
- `type Index[V any] = OrderedMap[string, V]` is a **generic type alias**: it
  is identical to `OrderedMap[string, V]`, not a new named type.

## Part 5 — A concurrent generic repository (`repository.go`)

`Repository[K comparable, T Keyed[K]]` stores entities that implement
`Key() K`. The constraint `Keyed[K]` is an interface, so `T` can be a value
type (`user`) or a pointer type (`*order`).

- `Create` returns `ErrExists` for a duplicate key and leaves the stored entity
  untouched.
- `Get` and `Replace` and `Delete` return `ErrNotFound` for a missing key.
  `Get` then returns the zero `T`.
- `Replace` keeps the entity's position. `Find` returns matches in insertion
  order and never returns nil.
- `Find` returns copies of the stored values. A pointer `T` is shared, so
  callers can see mutations made through their own pointer.
- The repository is safe for concurrent use. It must not be copied after
  first use.

## Part 6 — A type-routed event bus (`events.go`)

`Handler[E any] func(ctx, event E) error`, `Bus` (zero value usable), and the
generic functions `Subscribe[E]` and `Publish[E]`.

- Routing is keyed on the **exact** type argument. A `Subscribe[any]` handler
  does not receive events published as `userCreated`, and `Publish[any]` does
  not reach concrete subscribers.
- Delivery order is subscription order. A handler error does not stop the
  others; `Publish` returns the invoked count and `errors.Join` of the errors.
- `Subscribe` returns a cancel function that removes exactly its own
  registration. Cancel is idempotent and may be called from inside a handler.
- `Publish` takes a snapshot. Handlers added during dispatch miss the current
  event. Handlers cancelled during dispatch are skipped.
- `Publish` checks `ctx` before each handler. If the context is done, it stops
  and includes `ctx.Err()` in the joined error.

## Part 7 — Phantom types and units (`units.go`)

`Distance[U Unit] float64` carries its unit in the type. `U` appears nowhere in
the underlying type, so `Distance[Meters]` and `Distance[Feet]` are different
types with the same representation.

- `Meters` (`m`, 1) and `Feet` (`ft`, 0.3048). `Symbol()` and `MetersPerUnit()`
  describe each unit.
- `Add` takes only the same unit.
- `String()` gives `12.5 m`, using the shortest exact float64 form.
  `0.1 + 0.2` renders as `0.30000000000000004 m`.
- `Convert[To, From](d)` re-expresses a distance in another unit.
- `ParseDistance[U](s)` takes `<number>[ ]<symbol>` with surrounding whitespace
  ignored. A symbol other than `U.Symbol()` returns `ErrUnitMismatch`. A
  missing symbol, a bad number, or a non-finite number (`NaN`, `inf`) returns
  `ErrInvalidDistance`.
- `MarshalText` / `UnmarshalText` use the `String` form. A JSON field
  `Distance[Feet]` becomes `"3 ft"`. `UnmarshalText` leaves the value unchanged
  on error.

## Concepts exercised

- Defined types vs aliases; the zero value as a design tool
- `TextMarshaler` / `TextUnmarshaler` and JSON map keys
- `fmt.Stringer` and verb-dependent formatting
- Type constraints: `~T` type sets, `cmp.Ordered`, interface constraints
- Generic types with pointer-receiver mutators and value-receiver reads
- Generic type aliases
- `iter.Seq` / `iter.Seq2` range-over-func iterators and early break
- Generic repositories and `sync.RWMutex` with `wg.Go`
- Routing by `reflect.TypeFor[E]()`; snapshot semantics; `errors.Join`
- Phantom type parameters and conversion across units
- Error wrapping: `%w`, multiple `%w`, `errors.Is` through `json` and generics

## Run

From the repository root:

```sh
go test ./exercises/11_types_and_generics/
go test ./exercises/11_types_and_generics/ -run Part2 -v
go run ./tools/solutions ./exercises/11_types_and_generics/
```

On Linux or macOS, also run the concurrency-sensitive parts with the race
detector:

```sh
go test -race ./exercises/11_types_and_generics/ -run 'Part5|Part6' -count=3
```

`-race` needs cgo and is not available on every Windows toolchain.

## Stretch (not tested)

- Compile-time error: `typed.Distance[typed.Meters](1).Add(typed.Distance[typed.Feet](1))`
  must not compile. Write the reason in a comment.
- Interface conformance checks, for example `var _ encoding.TextMarshaler = typed.UserID(0)`.
- Replace the unit marker structs with a `Unit` interface that carries the
  symbol as a type parameter constant. Discuss why Go has no const type
  parameters and what that costs.
- Add a `Set.Filter` that returns `iter.Seq`. Decide whether it should be lazy
  and what happens if the set changes during iteration.
- `OrderedMap.Delete` is O(n) because it scans `keys`. Replace it with a
  linked list and measure the difference with `b.Loop()`.
