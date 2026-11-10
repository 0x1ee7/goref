# 08 · Interfaces in depth

A blob-storage, byte-pipeline and notification toolkit built the way a senior
Go codebase builds them: small consumer-owned interfaces, optional capabilities
discovered by type assertion, decorators that keep those capabilities alive, a
fan-out dispatcher, and the nil-interface trap that bites everyone once.

Implement the stubs in `store.go`, `decorate.go`, `iopipe.go`, `notify.go`,
`trap.go`, `queue.go`, `registry.go` and `numeric.go`. The `*_test.go` files
are the full spec alongside this README. Every test fails until you implement
the API below. Keep the exported surface exactly as declared; you may add
unexported helpers and fields.

Every part is test-driven: a stub returns zero values, so each subtest also
asserts a positive effect (a counter moved, a byte was written, a sanity call
succeeded) so that a stub cannot pass by accident.

## Part 1: Implicit satisfaction, small interfaces, consumer-side contracts

Implement in `store.go`:

- `ErrNotFound`, `ErrInvalidKey` sentinels.
- `BlobStore` (`Get`, `Put`, `Delete`) is the contract the application depends
  on. No implementation declares that it implements it; satisfaction is implicit.
- `BlobGetter` is the read-only view that `Transfer` needs. The consumer owns
  the interface, so `Transfer` accepts any reader, not only a `BlobStore`.
- `MemoryStore`: map-backed; the **zero value is usable**; `Put` copies in and
  `Get` copies out, so neither side can mutate stored bytes.
- `FileStore`: one file per key under a root directory, implemented with
  `os.Root` (Go 1.24+) so keys cannot escape it. `NewFileStore` creates the
  directory. `Close` releases the root handle, and the implementation is an
  `io.Closer`.
- Key rules (both stores): non-empty; no leading `/`; no `\`; every
  `/`-separated segment is non-empty and is not `.` or `..`. Violations wrap
  `ErrInvalidKey`.
- Every operation honours `ctx` (return `ctx.Err()` wrapped if it is done).
- `Get`/`Delete` of a missing key wrap `ErrNotFound`.
- Add the compile-time assertions `var _ BlobStore = (*MemoryStore)(nil)` and
  the equivalents for `FileStore` in `store.go`.
- `Transfer(ctx, dst, src BlobGetter, keys...) (int, error)`: copy keys in order;
  stop at the first failure and wrap it with the key; return how many were copied.

Concepts: implicit `implements`, "accept interfaces, return structs", consumer-
side interfaces, `var _ I = (*T)(nil)`, value copies at the boundary, `os.Root`.

## Part 2: Optional capabilities via type assertion

Implement in `store.go`:

- `Lister` (`List(ctx, prefix) ([]string, error)`, sorted) is implemented by
  `MemoryStore` and `FileStore`. `Sizer` (`Size`) is implemented by `MemoryStore`
  only. `FileStore` must **not** implement `Sizer`.
- `ListKeys(ctx, s, prefix)`: delegates to `Lister` if `s` implements it;
  otherwise returns an error wrapping `errors.ErrUnsupported`.
- `SizeOf(ctx, s, key)`: uses `Sizer` if available; otherwise falls back to
  `Get` and `len`. The fallback preserves `ErrNotFound`.
- `Capabilities(s) []string`: type switch over the optional interfaces,
  returning `"list"`, `"size"`, `"close"` in that order (only those held).
- `CloseStore(s) error`: calls `Close` when `s` is an `io.Closer`, otherwise
  returns nil.

Concepts: `x.(Iface)` vs type switch, the comma-ok form, capability discovery,
`errors.ErrUnsupported`, the difference between "not capable" and "failed".

## Part 3: Decorators that preserve capabilities

Implement in `decorate.go`:

- `CacheStore` wraps a `BlobStore` with a bounded cache (`maxEntries >= 1`,
  otherwise `NewCacheStore` returns an error). It is write-through (`Put` stores
  a copy in the cache after the inner write succeeds), read-through (a miss
  fetches from the inner store, and only successful reads are cached), and
  invalidating (`Delete` evicts). Eviction is FIFO by insertion. `Stats()`
  reports `Hits`, `Misses` and `Evictions`. It is safe for concurrent use.
- `CacheStore` deliberately does **not** implement `Lister`: a cached listing
  would go stale. Explain this trade-off in a doc comment.
- `Metrics` counts `Gets`, `Puts`, `Deletes`, `Lists`, `Sizes` and `Errors`.
  `Errors` counts failures other than `ErrNotFound`. The zero value is usable and
  it is safe for concurrent use (`sync/atomic`).
- `Instrument(inner, m) BlobStore`: wraps `inner` and counts every call. The
  result implements `Lister` iff `inner` does, `Sizer` iff `inner` does, and
  `io.Closer` iff `inner` does. That is 8 possible combinations; find a clean way
  to produce exactly the right one.

Concepts: decorators by embedding and by composition, the cost of hiding
optional interfaces, struct-embedding of interface fields, `atomic.Int64`,
concurrency-safe caches.

## Part 4: io.Writer pipelines

Implement in `iopipe.go`:

- `CountingWriter` counts bytes the underlying writer **reports as accepted**
  (`n`), not `len(p)`. `Count() int64`.
- `PrefixWriter` writes its prefix at the start of every line. It keeps state
  across `Write` calls, so a line may be split between writes, and the prefix is
  written once per line start. It is lazy: no prefix is emitted until data
  arrives for the next line, so a trailing newline yields no trailing prefix. An
  empty line still gets the prefix. `Write` returns the number of **input** bytes
  consumed (not the bytes emitted). On an underlying error, return the input
  consumed so far.
- `HashTee` writes to `w` and digests exactly the bytes `w` accepted, for each
  named algorithm: `"md5"`, `"sha1"`, `"sha256"`. An unknown name is an error.
  `Sums()` returns lowercase hex per name.
- `LimitWriter` passes through at most `limit` bytes in total. The write that
  crosses the limit writes the allowed prefix and returns `ErrLimitExceeded`. Later
  non-empty writes return `0, ErrLimitExceeded`; empty writes return `0, nil`.

Concepts: `io.Writer` contract (`n` vs `len(p)`, returning errors with partial
counts), stateful writers, decorator ordering (what a hash "sees"), `io.Copy`
and `WriterTo`/`ReaderFrom` fast paths, `io.Discard`, `io.MultiWriter`.

## Part 5: Notifiers and a fan-out dispatcher

Implement in `notify.go`:

- `Notifier` interface; `NotifierFunc` adapter (the `http.HandlerFunc` pattern).
- `NewDispatcher(notifiers, opts...)` copies the slice, so later caller mutations
  do not affect it.
- `WithConcurrency(n)` option caps in-flight notifiers; `n <= 0` means unlimited
  (the default). Use `golang.org/x/sync/semaphore` with `Acquire(ctx, 1)`.
- `Dispatch(ctx, ev)`: runs every notifier and waits for all of them (no early
  exit). Each failure is wrapped with the notifier's index and the failures are
  joined with `errors.Join` in **registration order**. Returns a true `nil` when
  every notifier succeeds, including when there are none. If `ctx` is done
  before a notifier gets its slot, that notifier's error is `ctx.Err()`.

Concepts: functional options, adapter types, `errors.Join`, bounded fan-out
with `semaphore`, `sync.WaitGroup.Go`, deterministic error ordering under
concurrency, `testing/synctest` for deterministic timing.

## Part 6: The nil-interface trap, `fmt.Stringer`, `sort.Interface`, `heap.Interface`

Implement in `trap.go`:

- `Severity` enum with `String()`: `"info"`, `"warning"`, `"error"`, and
  `"Severity(N)"` for anything else.
- `ErrorList` collects `FieldError`s via `Add(sev, field, reason)`. `Len`,
  `Errors` (a copy) and `Error` are **nil-safe** on a nil `*ErrorList`.
- `ValidateUser(u User) error`: rules, in field order:
  - `Name` empty: `Name: is required`
  - `Age` outside `0..150`: `Age: must be between 0 and 150`
  - `Email` empty: `Email: is required`; no `@`: `Email: must contain '@'`
  - Invalid input returns a `*ErrorList`. **Valid input must return a true
    untyped `nil`**. Do not return a typed nil `*ErrorList` as `error`; this
    is the trap.
- `IsNilValue(v any) bool`: true for an untyped nil and for a nil pointer, map,
  slice, func, chan or interface value (via `reflect`). False otherwise.

Implement in `queue.go`:

- `ByPriority []Job` implements `sort.Interface`: higher `Priority` first, ties
  broken by `Name` ascending. The result must be a strict weak order.
- `Queue` is a max-priority queue built on `container/heap`. Equal priorities
  pop FIFO (keep an insertion sequence number). The zero value is usable.
  `Pop` on empty returns `(Job{}, false)`.

Concepts: the typed-nil gotcha (interface = type word + value word), nil-safe
receivers, `fmt.Stringer` and how fmt uses it, `sort.Interface` vs
`slices.SortFunc`, `heap.Interface` (pointer receivers on `Push`/`Pop`),
stability via sequence numbers.

## Part 7: Plugin registry with factories, and generics versus interfaces

Implement in `registry.go`:

- `Exporter` interface: `Export(w io.Writer, rows [][]string) error`; `rows[0]`
  is the header.
- `Factory func(options map[string]string) (Exporter, error)`.
- `Registry` (zero value usable, safe for concurrent use):
  - `Register(name, f)`: empty name, nil factory or duplicate name are errors.
    A duplicate wraps `ErrDuplicateExporter`.
  - `New(name, options)`: unknown name wraps `ErrUnknownExporter`; a factory
    error is wrapped with the name.
  - `Names()`: sorted.
- `Builtins()` returns a registry with `"csv"` and `"jsonl"` registered.
- `NewCSVExporter`: option `delimiter` must be exactly one rune (default `,`);
  unknown option keys are errors. Uses `encoding/csv`.
- `NewJSONLinesExporter`: one JSON object per data row, keys in **header order**
  (not sorted, which `encoding/json` would do for maps). A row whose length
  differs from the header is an error.

Implement in `numeric.go`:

- `Number` type-set constraint (`~int | ~int32 | ~int64 | ~float64`).
- `Sum[T Number]`, `Max[T cmp.Ordered]` (returns `(T, bool)`), and
  `Join[T fmt.Stringer]`. Each shows a different constraint kind: a type set
  (`~` admits named types), an ordering constraint, and a method-set constraint.

Concepts: `Registry` as explicit state rather than a global `init()` side effect,
factories versus constructors, `encoding/csv`, ordered JSON without maps, when
generics beat interfaces (no boxing, type-set arithmetic) and when interfaces
beat generics (runtime polymorphism, heterogeneous collections).

## Run

```bash
go test ./exercises/08_interfaces/                      # all parts
go test ./exercises/08_interfaces/ -run Part2 -v       # one part
go run ./tools/solutions ./exercises/08_interfaces/    # reference check
```

On Linux or macOS, also run the suite with `-race`; the notifier and cache
tests exercise concurrency.

## Stretch (untested)

- Make `Instrument` preserve every capability without enumerating 8 types
  (for example, a capability bag or code generation). Discuss the trade-off.
- Give `FileStore.Put` atomic replacement: write to a temp name, then
  `root.Rename`.
- Add a `Watcher` optional interface that streams key changes, and extend
  `Capabilities` to report it.
- Replace `IsNilValue` with a `go vet`-style analyzer over `return` statements.
