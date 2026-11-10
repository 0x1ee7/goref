# 03 — Packages and scope

## Scenario

You maintain `scope`, a small library used by a billing service. It exposes
account ledgers, a text-window parser for business hours, a plugin registry,
a token-expiry helper, and a spreadsheet-style formula evaluator. Each piece
exercises a package-level rule that a senior Go engineer must get right:
what leaks out of a package, what stays in `internal/`, when a closure shares
state, where `:=` silently shadows an error, why package-level mutable state is
a trap, and how to scope a test hook.

The Python sibling (`pyref/exercises/03_namespaces_and_scope`) covers LEGB
lookup and `global`/`nonlocal`. Go has no `global`; scope here is about
identifier visibility (exported vs unexported), package boundaries, closure
capture, and shadowing.

Run everything from the repository root. Use Go 1.27 or newer.

## Part 1 — Encapsulation, `internal/`, and `iota`

`Account` has unexported fields. Its state changes only through methods, and
`History()` returns a copy so callers cannot reach into the ledger.

`internal/checksum` (under `exercises/03_packages_and_scope/internal/`) computes
Luhn check digits. Only packages rooted in this directory may import it.

API (`account.go`, `internal/checksum/checksum.go`):

- `Kind` with `KindChecking`, `KindSavings`, `KindLoan` via `iota + 1`. The zero
  value is not a valid kind. `String()` gives `checking`, `savings`, `loan`, or
  `Kind(n)`. `ParseKind(s)` is its inverse; anything else returns
  `ErrInvalidArgument`.
- `NewAccount(kind, seq)`: `seq` must be in `1..99999999`. The number is `seq`
  zero-padded to 8 digits plus one Luhn check digit (9 digits). For example
  `seq = 1` gives `000000018`.
- `Deposit`, `Withdraw` (return the new balance). Non-positive amounts return
  `ErrInvalidArgument`. Overdraft returns `ErrInsufficientFunds`. On error the
  balance and history are unchanged.
- `History() []Entry` returns a copy. `Entry{Delta, Balance}`.
- `checksum.Digit`, `Append`, `Valid`: payload must be decimal digits.
  `Valid` needs at least two digits.

Concepts: exported vs unexported fields, defensive copies, `internal/`,
`iota` with a deliberately invalid zero value, sentinel errors with `%w`.

## Part 2 — Closures and loop variables

`closures.go`:

- `NewCounter(start, step)` returns `func() int`. Each call returns the current
  value and then advances. Step 0 is `ErrInvalidArgument`. Counters are
  independent.
- `NewIDGenerator(prefix, width)` returns `func() string` yielding
  `<prefix>-<n>`, starting at 1 and zero-padded to `width` (never truncated).
  Empty prefix or `width < 1` is `ErrInvalidArgument`. The closure must be safe
  for concurrent calls.
- `LoopCallbacks(labels)`: since Go 1.22 each iteration has its own loop
  variable, so each callback returns `clicked <its label>`.
- `SharedCallbacks(labels)`: the classic bug, written on purpose. The variable
  is declared outside the loop, so every callback returns `clicked <last label>`.
- `NewAdders(offsets)`: `adder(v)` returns `v + offset`.

Concepts: closure capture by reference, per-iteration loop variables (Go 1.22),
shared captured state, goroutine-safe closures with `sync/atomic`.

## Part 3 — Shadowing: `:=` and the swallowed error

`window.go` parses `HH:MM-HH:MM` (for example `09:00-17:30`) into `Window`
(offsets from midnight). Rules are in the doc comment of `ParseWindow`.
Implement it correctly.

Here is a version that a colleague shipped. Study it, then write yours. It
compiles and passes the happy path. It is wrong:

```go
func ParseWindow(s string) (w Window, err error) {
	start, end, ok := strings.Cut(s, "-")
	if !ok {
		return Window{}, fmt.Errorf("%w: %q", ErrInvalidWindow, s)
	}
	if t, err := parseClock(start); err == nil { // err is a NEW variable here
		w.Start = t
	}
	if t, err := parseClock(end); err == nil {
		w.End = t
	}
	return w, err // the named result: still nil
}
```

Why does `"09:00-25:00"` return no error? Which `err` does the `return` read?
Fix it. Errors must surface and carry `ErrInvalidWindow`. The message must include the input.

`Window.String()` formats the window back as `HH:MM-HH:MM`.

Concepts: `:=` in `if`/`switch`/`for` headers creates new variables; named
results plus shadowing; errors as values that must be checked; a bare `return`
hides the mistake; `strings.Cut`; `time.Duration` for time-of-day.

## Part 4 — A registry without package state

`registry.go` holds `Plugin` (`func(string) (string, error)`) and a `Registry`
that maps names to plugins.

- `var r Registry` must be usable. `NewRegistry()` returns `*Registry`.
- `Register(name, p)`: empty name or nil plugin is `ErrInvalidArgument`;
  duplicates are `ErrDuplicate`; a failed call changes nothing.
- `Lookup(name)` returns `ErrNotFound` for unknown names.
- `Names()` returns sorted names. It returns a copy.
- Concurrent `Register` and `Lookup` must be safe.

The package also has a default registry: `Default()`, plus package-level
`Register`, `Lookup`, `Names`. These functions are a convenience. The point of
this part is that `NewRegistry()` instances are isolated from the default and
from each other.

Concepts: zero-value-useful types, instance state vs package state, `sync.RWMutex`,
sentinel errors, `slices`/`maps`, copy-on-return for mutable state.

## Part 5 — Package-level hooks and scoped overrides

`clock.go` reads time through a package-level clock so tests can freeze it.

- `SetClock(now) (restore func())` replaces the package clock. `restore`
  reinstates the clock in place when `SetClock` was called. Calling `restore`
  twice is harmless. A nil `now` selects `time.Now`.
- `Now()` reads the package clock. `ExpiresAt(ttl)` returns `Now().Add(ttl)`;
  a non-positive `ttl` is `ErrInvalidArgument`.
- `NewIssuer(now)` injects its own clock. A nil `now` means the package clock.
  `Issuer.ExpiresAt(ttl)` is the method version.

Concepts: `defer` for scoped state, `func` values, the trade-off between
package hooks and explicit dependencies, `t.Cleanup`, restoring state after a
panic.

## Part 6 — A whitelist-only formula evaluator with `go/parser`

`formula.go` evaluates spreadsheet-like formulas such as `A1 + B2*2` or
`ROUND(A1 / 3)`. Parse with `go/parser` and walk the AST. Accept only the
whitelisted grammar. Reject everything else.

Grammar:

- Literals: Go `INT` and `FLOAT` syntax, including `_` separators, `0x`, `0b`,
  `0o` and leading `0` octal (`010` is 8). `STRING`, `CHAR` and imaginary
  literals are forbidden.
- Cell names: a bare identifier looked up in `cells`.
- Unary `+` and `-`. Binary `+`, `-`, `*`, `/`, `%`. Parentheses.
- Calls: `NAME(args...)` where `NAME` is in the merged function set.
- Everything else is forbidden: selectors, indexing, comparisons, logic,
  bitwise operators, `!`, `^`, `*x`, `&x`, `...`, function literals,
  composite literals, and any call whose target is not a plain identifier.

Semantics:

- All values are `float64`. `/` is IEEE division. `%` is `math.Mod`, which
  takes the sign of the dividend, so `-7 % 3` is `-1`.
- Division or modulo by zero is `ErrEval`. A non-finite result (Inf or NaN) is
  `ErrEval`.
- A name in `cells` that is also a function name (including the defaults) is
  `ErrConflict`.
- Errors: `ErrParse` for anything `go/parser` rejects (including empty input and
  out-of-range literals). `ErrForbidden` for valid-Go-but-disallowed syntax,
  including `**`: it parses as `*` followed by a dereference, so it is not a
  power operator here. `ErrUnknownName` for unknown cells, functions, or `true`.
  `ErrEval` for runtime and function errors. Function errors wrap both
  `ErrEval` and the original error.

Gotcha to design around: Go has no floor-division operator. `//` starts a
comment, and `go/parser` drops comments silently. `1 // 2` would parse as `1`.
Detect comments before parsing (for example with `go/scanner` in
`scanner.ScanComments` mode) and reject them. This is also why `//` is not an
operator in this language.

`DefaultFunctions()` returns a fresh map each call: `ABS`, `MIN`, `MAX`, `ROUND`
(`math.Round`, half away from zero). `Evaluate` merges `funcs` over the
defaults, so a caller can override `ABS`.

Concepts: `go/ast` and `go/parser`, walking node types with type switches,
whitelists over blacklists, `go/scanner` for comments, Go literal syntax,
IEEE 754 edge cases, `errors.Is` across multiple `%w`, `maps.Copy`.

## Run

```sh
go test ./exercises/03_packages_and_scope/
go test ./exercises/03_packages_and_scope/ -run Part2 -v
go run ./tools/solutions ./exercises/03_packages_and_scope/
```

With `go test`, the stub bodies return zero values. Every test should fail
until you implement the part. Run the tests on Linux or macOS with `-race`
for a real data-race check. Windows without cgo cannot run `-race`.

## Stretch (untested)

- Make `SetClock` lock-free with `atomic.Pointer`, and prove it with a race test.
- Replace `SharedCallbacks` with a version that uses `iter.Seq` to stream labels.
- Add `Registry.Unregister`, then decide what happens to a `Lookup` in flight.
- Make the formula evaluator report the column of the first forbidden node, using
  `go/token`.
- Compare `slog`-style global logger scoping with explicit `*Logger` injection.
