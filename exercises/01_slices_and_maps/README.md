# 01 · Slices and maps

You are building the internal `opstoolkit`-style helpers for a small API gateway
and its billing/inventory side jobs. Everything is standard library (plus
`go-cmp` in the tests only). Each part is a component you would actually ship;
the tests encode the production edge cases: aliasing of backing arrays, clock
boundaries, ties, nil maps, and shared nested state.

Put your code in the stub files already in this directory. Public names,
signatures and behaviour below are the full spec. Where a test says "error",
it means a sentinel from `errors.go` (wrapped is fine; test with `errors.Is`).

---

## Part 1 — Slice views: `Head`, `Batches`, `FilterInPlace` (`window.go`)

Three ways to hand out part of a slice without surprising the caller.

```go
func Head[T any](s []T, n int) []T
func Batches[T any](items []T, size int) iter.Seq[[]T]
func FilterInPlace[T any](s []T, keep func(T) bool) []T
```

- `Head`: `n` is clamped to `[0, len(s)]`. The result is a **view** of `s` (it
  shares the backing array), but its capacity equals its length, written as a
  full slice expression `s[:n:n]`. Appending to the result must reallocate, so
  it can never overwrite `s[n:]`. Plain `s[:n]` does overwrite it; that is the
  gotcha this part is about.
- `Batches`: yields consecutive sub-slices of at most `size` elements; the last
  may be shorter. Batches are views into `items` (no copy), each with capacity
  limited to its length, so appending to one batch cannot clobber the next. An
  empty or nil `items` yields nothing. Stopping the loop early (`break`) must
  stop the iteration. `size < 1` panics (like `slices.Chunk`).
- `FilterInPlace`: keeps the elements for which `keep` is true, preserving
  order, and **reuses `s`'s backing array**: no allocation. The vacated slots at
  the end of `s` are zeroed so the garbage collector can reclaim what they
  referenced. The returned slice aliases `s`.

## Part 2 — Money and volume tiers (`pricing.go`)

Money is never a float. `Money` is an integer count of minor units (cents).

```go
type Money int64
func (m Money) String() string
type Tier struct { MinQty int; Unit Money }
type PriceTiers struct{ /* yours */ }
func NewPriceTiers(tiers []Tier) (*PriceTiers, error)
func (p *PriceTiers) UnitPrice(qty int) (Money, error)
func (p *PriceTiers) LineTotal(qty int) (Money, error)
func SLATier(responseMinutes float64) (string, error)
```

- `Money.String()`: two decimals, `1234` → `"12.34"`, `5` → `"0.05"`, `-5` → `"-0.05"`.
- `NewPriceTiers`: tiers may come in any order. Sort a **copy**; the caller's
  slice must not be reordered. Errors: no tiers → `ErrNoTiers`; the lowest tier
  does not start at quantity 1 (includes `MinQty` 0 or negative), a duplicate
  `MinQty`, or a price `<= 0` → `ErrInvalidTier`.
- `UnitPrice(qty)`: the tier with the largest `MinQty <= qty` applies. A quantity
  exactly on a boundary belongs to the new tier. `qty < 1` → `ErrInvalidValue`.
  Binary search (`slices.BinarySearchFunc`), not a linear scan.
- `LineTotal(qty)`: `UnitPrice(qty) * qty`.
- `SLATier(minutes)`: `<= 15` → `"critical"`, `<= 60` → `"high"`, `<= 240` →
  `"normal"`, otherwise `"low"` (including `+Inf`). Upper bounds are inclusive.
  Negative or NaN → `ErrInvalidValue`. Bands are a sorted table searched with
  `slices.BinarySearchFunc`.

## Part 3 — TTL + LRU cache (`cache.go`)

Gateway response cache: bounded size, least-recently-used eviction, and every
entry expires `ttl` after it was **written**. Time comes from an injected clock
(`now func() time.Time`, `nil` means `time.Now`).

```go
func NewTTLCache[K comparable, V any](maxsize int, ttl time.Duration, now func() time.Time) (*TTLCache[K, V], error)
func (c *TTLCache[K, V]) Get(key K) (V, bool)
func (c *TTLCache[K, V]) Put(key K, value V)
func (c *TTLCache[K, V]) Contains(key K) bool
func (c *TTLCache[K, V]) Keys() []K
func (c *TTLCache[K, V]) Len() int
func (c *TTLCache[K, V]) Hits() int
func (c *TTLCache[K, V]) Misses() int
```

- `maxsize < 1` or `ttl <= 0` → `ErrInvalidConfig`.
- An entry written at `t` expires at `t + ttl`. It is expired once `now >= expiresAt`
  (the instant itself is already a miss). Use `Before`, not `==`.
- `Get`: a live entry is a hit and becomes most recently used. Missing or expired
  is a miss (returns `false`); an expired entry is removed. Reading never extends a TTL.
- `Put`: insert or replace. Replacing refreshes TTL and recency. Expired entries
  are purged first, then least-recently-used entries are evicted until `Len() <= maxsize`.
  Expired entries never count toward capacity.
- `Contains`: true iff the key is live. Does **not** change recency or counters.
- `Keys`: live keys, least recently used first. Does not change recency.
- `Len`: live entries only.
- `Hits`/`Misses`: count `Get` outcomes only.
- Not safe for concurrent use.

## Part 4 — Sliding-window rate limiter (`limiter.go`)

Per-client limiter: at most `limit` requests in any trailing `window`. Each client
gets a **fixed-size ring buffer** of `limit` timestamps, so memory per client is
bounded no matter how long it runs.

```go
func NewSlidingWindowLimiter(limit int, window time.Duration, now func() time.Time) (*SlidingWindowLimiter, error)
func (l *SlidingWindowLimiter) Allow(client string) bool
func (l *SlidingWindowLimiter) Remaining(client string) int
func (l *SlidingWindowLimiter) RetryAfter(client string) time.Duration
func (l *SlidingWindowLimiter) Prune() int
```

- `limit < 1` or `window <= 0` → `ErrInvalidConfig`.
- An event at `t` counts at `now` iff `now - t < window`. An event exactly `window` old has left.
- `Allow`: if fewer than `limit` live events, record `now` and return `true`;
  otherwise `false` and record **nothing**.
- `Remaining`: `limit - live`. A client never seen has `limit`.
- `RetryAfter`: `0` if `Remaining > 0`; otherwise the time until the oldest live
  event leaves the window.
- `Prune`: drop state for clients with no live events; return how many were dropped.
- Queries (`Remaining`, `RetryAfter`) must not create state for unknown clients.
- Not safe for concurrent use.

## Part 5 — Streaming top-K with `container/heap` (`topk.go`)

Sales-line stream from the order pipeline. The input is a one-shot
`iter.Seq[Sale]` with far more lines than distinct SKUs.

```go
type Sale struct { SKU string; Units int }
type ProductTotal struct { SKU string; Units int }
func TopK(sales iter.Seq[Sale], k int) ([]ProductTotal, error)
```

- Units are summed per SKU. Negative units → `ErrInvalidValue` (wrapped with the SKU).
  Zero is valid; a SKU whose total is 0 is still ranked.
- Returns at most `k` totals, units **descending**, ties by SKU **ascending**.
- `k <= 0` → an empty result (nil or empty; check `len`).
- `k` bigger than the distinct SKU count → all of them, ranked.
- The sequence is ranged over exactly once.
- Keep only `k` candidates in a bounded `container/heap` (the worst candidate
  sits at the top) instead of sorting everything. Sort the final `k` with
  `slices.SortFunc` and `cmp.Or` for the multi-key order.

## Part 6 — Layered configuration with `map[string]any` (`config.go`)

Precedence, highest first: **CLI > env > file > defaults**.

```go
func DeepMerge(base, override map[string]any) map[string]any
func ParseEnv(environ map[string]string, prefix string) (map[string]any, error)

type Layers struct { Defaults, File, Env, CLI map[string]any }
type Config struct{ /* yours */ }
func NewConfig(layers Layers) *Config
func (c *Config) Get(path string) (any, bool)
func (c *Config) Source(path string) (string, bool)
func (c *Config) AsMap() map[string]any
```

- `DeepMerge`: where both values are `map[string]any`, merge recursively.
  Otherwise the override wins: lists (`[]any`) are **replaced**, not concatenated;
  `nil` overrides; a mapping may replace a scalar and vice versa.
  Neither input is mutated, and the result shares **no** maps or slices with
  either input, nested or not.
- `ParseEnv(environ, prefix)`: only keys starting with `prefix + "_"` count. The
  rest is split on `__` into lowercased path segments:
  `APP_DB__POOL__SIZE=10` → `{"db": {"pool": {"size": 10}}}`.
  Values: `true`/`false` (any case) → `bool`; an optional `-` followed by ASCII digits →
  `int` (if it fits in `int`; otherwise it stays a string); anything else is a
  string, including `"+5"`, `"1.5"` and `""`.
  Errors → `ErrInvalidEnv`: an empty path segment (`APP_`, `APP_DB____HOST`), a
  key that is both a value and a branch (`APP_DB=1` with `APP_DB__HOST=x`), or two
  keys that collide after lowercasing (`APP_HOST` with `APP_host`).
- `NewConfig`: each layer is **snapshotted** (deep-copied) at construction; later
  mutation of the inputs does not affect the config. `nil` layers are empty.
- `Get(path)`: dotted lookup (`"db.pool.size"`). The highest-precedence layer
  containing the **full** path wins. A `nil` value still counts as present
  (`(nil, true)`). A path running through a non-mapping is absent in that layer.
- `Source(path)`: `"cli"`, `"env"`, `"file"` or `"defaults"`; `false` if absent.
- `AsMap()`: the fully merged effective configuration, as a fresh map that the
  caller may mutate without affecting the `Config`.

## Part 7 — Sets, reconciliation and array keys (`inventory.go`)

Warehouse count vs. system of record, and stock keyed by `[warehouse, sku]`.

```go
type Set[T comparable] map[T]struct{}
func NewSet[T comparable](items ...T) Set[T]
func (s *Set[T]) Add(v T)
func (s Set[T]) Has(v T) bool
func (s Set[T]) Len() int
func Difference[T comparable](a, b Set[T]) Set[T]
func Sorted[T cmp.Ordered](s Set[T]) []T

type Reconciliation struct {
	Missing    []string       // sorted
	Unexpected []string       // sorted
	Mismatched map[string]int // counted - expected
}
func (r Reconciliation) IsClean() bool
func Reconcile(expected, counted map[string]int) (Reconciliation, error)

type BinKey [2]string // (warehouse, sku): arrays are comparable map keys
type StockLine struct { Warehouse, SKU string; Qty int }
type Level struct { Units, Lines int }
func StockByBin(lines []StockLine) map[BinKey]Level
func SortedBins(levels map[BinKey]Level) []BinKey
```

- `Set`: the zero value is usable: `Add` on a zero `Set` must not panic (it
  allocates the map). `NewSet` deduplicates. `Difference` is `a \ b` and leaves
  both inputs unchanged. `Sorted` returns ascending order.
- `Reconcile`:
  - `Missing`: SKUs in `expected` absent from `counted`.
  - `Unexpected`: SKUs in `counted` absent from `expected`.
  - `Mismatched`: SKUs in both with different quantities, mapped to `counted - expected`.
    A count of `0` against expected `3` is a mismatch of `-3`, **not** a missing SKU.
  - `IsClean` is true iff all three are empty.
  - Negative quantity → `ErrInvalidValue`.
- `StockByBin`: sums `Qty` per `BinKey` and counts contributing lines. Values are
  structs stored in a map: you cannot assign to a field through `m[k]`, so
  read, modify, write back. Lines with `Qty` 0 still create the bin.
- `SortedBins`: by warehouse, then SKU.

---

## Concepts exercised

- Slice headers: len vs cap, full slice expression `s[a:b:c]`, aliasing after `append`
- In-place filtering with `clear` on the vacated tail; `testing.AllocsPerRun`
- `iter.Seq` and early termination; `slices.Collect`, `slices.Values`, `slices.Sorted`
- `slices.Clone` before sorting a caller's input; `slices.SortFunc` with `cmp.Or`
- `slices.BinarySearchFunc` boundary semantics (`found` vs insertion point)
- `Money` as integer minor units; no floats for currency
- Generic `map` types with `container/list` for an LRU; `Before` for expiry boundaries
- Ring buffers for bounded per-key state; pruning maps during `range`
- `container/heap` as a bounded top-K; `Less` as "worse than"
- Deep copies of `map[string]any`/`[]any`; `strings.Split`, `strconv`
- Zero-value-usable generic sets (`*Set` receiver lazily allocating the map)
- Array (`[2]string`) map keys; struct values in maps are copies

## Run

From the repository root:

```
go test ./exercises/01_slices_and_maps/
go test ./exercises/01_slices_and_maps/ -run Part2 -v
go run ./tools/solutions ./exercises/01_slices_and_maps/
```

Use `-race` on Linux or macOS to check your concurrency-free code is still
race-free under the harness; the Windows toolchain here has no cgo, so `-race`
is unavailable.

## Stretch (untested)

- Make `TTLCache` expiry O(log n) with a second index ordered by expiry time.
- Compare `SlidingWindowLimiter` memory against a fixed-bucket counter with 1M clients.
- Add `Config.Watch(path)` that reports when a layer change alters the effective value.
- Give `DeepMerge` cycle detection for self-referencing maps and see what breaks.
