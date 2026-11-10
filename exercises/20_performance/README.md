# 20 — Performance: allocation discipline and data layout

## Scenario

You own the ingestion path of an order-analytics service. It reads access logs
at tens of thousands of lines a second, exports CSV and query-string
fragments for downstream systems, writes tab-separated batches to a sink, and
computes per-customer revenue nightly. A profile shows the GC running hot:
every log line allocates, every CSV field is built with `fmt`, and the revenue
loop walks a slice of wide structs for one column it needs.

Your job is to make those paths allocate as little as the language allows,
and to prove it with tests that count allocations rather than measure time.
Timings are noisy on shared CI runners. Allocation counts from
`testing.AllocsPerRun` are deterministic, so every budget in this exercise is
an exact count or an upper bound you can rely on.

Each Part has two kinds of test. A **correctness precheck** runs first and
must pass before any budget is measured. A stub returns zero values, so a
budget of `0` would pass against it. The precheck makes sure you cannot pass
by doing nothing.

Package `perf`, directory `exercises/20_performance/`.

## Part 1 — Zero-allocation log line parser

`ParseLine(line []byte) (Line, error)` parses one logfmt-style access-log line:

```
level=info route=/api/orders status=200 dur_ms=12
```

Grammar:

- A line is a sequence of tokens separated by one or more spaces (`' '`).
  Leading and trailing spaces are allowed.
- A single trailing `"\n"`, or `"\r\n"`, is not part of the last value.
- A token is `key=value`. The key is the bytes before the first `=`, and it
  must be non-empty. A token with no `=`, or an empty key, is a syntax error.
- The value is everything after the first `=`. It may be empty. It may contain
  further `=` characters.
- Unknown keys are valid and ignored, including with empty values
  (`request_id=`).
- A repeated key takes the last value.
- Required keys: `level`, `route`, `status`, `dur_ms`.
- `level` and `route` must be non-empty.
- `status` and `dur_ms` must be non-negative decimal integers that fit in
  `int`. No sign, no underscores, no empty value, no overflow.

Result: `Line{Level, Route []byte; Status, DurMS int}`. `Level` and `Route` are
**views into the input**, not copies. They are valid only while the input is
unchanged.

Errors (wrap with `%w`; match with `errors.Is`):

| Condition | Error |
|---|---|
| token without `=`, or empty key | `ErrSyntax` |
| a required key is absent (including an empty line) | `ErrMissingField` |
| required value empty, or number malformed or out of range | `ErrBadValue` |

Budget: a successful parse of a line allocates **0** times.

## Part 2 — Exact-size CSV and query-string building

Two encoders, both producing exact output.

**`CSVRecord(fields []string) string`** encodes one RFC 4180 record with no
trailing newline. A field is quoted only if it contains `,`, `"`, `\r`, or
`\n`. Embedded `"` is doubled. Spaces alone do not force quoting. Empty fields
are empty text, so `["", ""]` encodes as `,` and `[""]` encodes as the empty
string. `CSVRecord` makes exactly **one** allocation for non-empty input.

**`AppendCSVRecord(dst []byte, fields []string) []byte`** appends the same
encoding to `dst`. It grows `dst` at most once, and not at all when `dst`
already has enough spare capacity.

**`QueryString(params []Param) string`** encodes `Param{Key, Value}` pairs as
`application/x-www-form-urlencoded`, in the order given, joined with `&`. The
escaping matches `url.QueryEscape` exactly: unreserved bytes (`A-Z a-z 0-9 - _ . ~`)
are kept, space becomes `+`, and everything else becomes `%XX` with upper-case
hex. `QueryString` makes exactly **one** allocation for non-empty input.

Budgets: `CSVRecord` = 1; `AppendCSVRecord(nil, …)` = 1; `AppendCSVRecord` into
a buffer with enough capacity = 0; `QueryString` = 1.

Hint: `fmt`, `strings.Join` plus `Sprintf`, and `url.Values.Encode` each add
allocations you can avoid. Compute the exact length first, then fill one buffer.

## Part 3 — Preallocation

Generic helpers with exact allocation behaviour.

- **`Map[T, U any](xs []T, f func(T) U) []U`**: `f` is called once per element,
  in order. The result has `len(xs)` elements, allocated once.
- **`Concat[T any](chunks ...[]T) []T`**: the elements of every chunk, in
  order. The result never aliases an input chunk. It returns `nil` when the
  total length is zero. For a non-empty total it allocates once.
- **`Batch[T any](xs []T, n int) [][]T`**: splits `xs` into consecutive chunks
  of at most `n` elements. Only the last chunk may be shorter. Chunks are
  **views** into `xs`, with capacity capped at their length, so
  `append(chunk, v)` reallocates instead of overwriting the next chunk.
  `Batch` panics if `n < 1`. It allocates once for the outer slice.
- **`Dedupe[T comparable](xs []T) []T`**: the first occurrence of each value,
  in input order. The input is unchanged and the result never aliases it.

Budgets: `Map` = 1; `Concat` = 1; `Batch` = 1 (the outer slice).

## Part 4 — Batch writer with `sync.Pool`

**`WriteRecords(w io.Writer, recs []Record) (int64, error)`** writes one
`"<ID>\t<Name>\n"` line per record and returns the bytes written.

- Encode the batch into a scratch `*bytes.Buffer` borrowed from a `sync.Pool`,
  then write it with **one** `w.Write` call.
- Validate every name before encoding anything. A name containing `\t`, `\r`,
  or `\n` returns `ErrBadName`. Nothing is written and `n` is `0`.
- An empty batch writes nothing and makes no `Write` call. It returns `(0, nil)`.
- A writer error is returned, and `n` is what the writer reported.
- Do not keep huge buffers in the pool. A buffer whose capacity exceeds 64 KiB
  should be dropped, so one large batch does not pin memory for the life of
  the process. This is a review criterion, not a test.
- Must be safe for concurrent use: separate calls share the pool but not the
  buffers.

Budget: **0** steady-state allocations per call (checked with
`AllocsPerRun`, which averages with integer division. A pool miss after a GC
does not fail the budget. A per-call allocation does).

## Part 5 — Struct layout and fast formatting

`Sample` is one measurement:

```go
type Sample struct {
	OK      bool
	ID      uint64
	Status  uint16
	Latency int64 // nanoseconds
	Retries uint8
}
```

The fields are declared in a naive order. Reorder them so the struct occupies
**24** bytes (`unsafe.Sizeof`). Think about alignment: fields are padded to
their own alignment, and the struct is padded to its largest alignment.

**`AppendSample(dst []byte, s Sample) []byte`** appends this line:

```
id=<ID> latency=<Latency> status=<Status> retries=<Retries> ok=<true|false>\n
```

It must not use `fmt`. Each interface argument boxes its value on the heap,
so `fmt.Appendf` costs allocations per call. Use `strconv.Append*`.

Budget: **0** allocations into a buffer with enough capacity.

## Part 6 — AoS vs SoA revenue aggregation

Two layouts of the same orders.

```go
type Order struct {        // array of structs (AoS)
	Customer  uint32       // dense index in [0, customers)
	Cents     int64
	Cancelled bool
}

type Columns struct {      // struct of arrays (SoA)
	Customer  []uint32
	Cents     []int64
	Cancelled []bool
}
```

- **`ToColumns(orders []Order) Columns`**: converts AoS to SoA. The result does
  not alias `orders`. One allocation per column, three for non-empty input.
- **`RevenueAoS(orders []Order, customers int) ([]int64, error)`**: for each
  customer index in `[0, customers)`, the sum of `Cents` over that customer's
  non-cancelled orders. The result has length `customers`.
- **`RevenueSoA(c Columns, customers int) ([]int64, error)`**: the same result
  and errors, computed over the SoA layout.

Rules, for both:

- Every order's `Customer` must be in range, **including cancelled orders**.
  An out-of-range index returns `ErrCustomerRange`.
- A negative `customers` returns `ErrCustomerRange`.
- `RevenueSoA` returns `ErrRagged` if the three columns differ in length.

Budgets: `RevenueAoS` = 1 (the result); `RevenueSoA` = 1; `ToColumns` = 3.

The benchmark (`go test -bench Part6 -benchmem`) shows the layout effect. The
column loop touches fewer bytes per order, but it depends on your hardware.
Measure before you trust it.

## Concepts exercised

- Zero-allocation parsing: views instead of copies, byte-level scanning,
  avoiding `string(b)` conversions and `strconv` on byte slices.
- Escape analysis and what makes a value escape (`go build -gcflags=-m`).
- `unsafe.String` for a one-allocation `[]byte` → `string` handoff, and why it
  is safe when the buffer is never mutated afterward.
- Exact-size preallocation; `slices.Grow`; `make` with capacity.
- Slice three-index expressions (`s[i:j:k]`) and capacity capping.
- `sync.Pool` discipline: `Reset` on get, bounded retention, `AvailableBuffer`.
- Struct field ordering, `unsafe.Sizeof`, and padding.
- AoS vs SoA, cache locality, and the cost of boxing into `any`/`fmt`.
- Testing allocations deterministically with `testing.AllocsPerRun`, and why
  every budget needs a correctness precheck.
- `b.Loop()` and `b.ReportAllocs()` for manual benchmarks.

## Run

From the repository root:

```
go test ./exercises/20_performance/
go test ./exercises/20_performance/ -run Part2 -v
go run ./tools/solutions ./exercises/20_performance/
go test ./exercises/20_performance/ -run '^$' -bench . -benchmem
go build -gcflags=-m ./exercises/20_performance/
```

`-race` is the best check for the pool and the concurrent `WriteRecords` test.
It needs cgo, so run it on Linux or macOS: `go test -race ./exercises/20_performance/`.

Allocation budgets are checked by the normal `go test` run. You do not need
`-bench` to catch a regression.

## Stretch (untested)

- Replace `WriteRecords`' pool with a caller-supplied `*bytes.Buffer` and
  compare allocation counts. Which API is easier to misuse?
- Make `ParseLine` return a `Line` that owns its data, using one pooled arena
  per call. Measure what that costs compared with views.
- Add a `Sample` batch encoder that emits `\n`-separated records and reports
  the `bytes.Buffer` growth pattern with `-gcflags=-m`.
- Compare `RevenueSoA` against a version that sorts orders by customer first.
  When does the sort pay for itself?
- Write a deterministic test that a `Batch` result stays valid after the input
  slice is reallocated. What does it actually observe?
