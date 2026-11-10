# goref — Go refresher, test-driven

Real-world Go exercises for an experienced engineer. The Go sibling of `pyref`:
same philosophy (one topic per directory, a spec, failing tests, a reference
answer), but every exercise is designed around how production Go is written —
error values, small interfaces, zero values, `context`, iterators, generics
where they pay off — and adds Go-specific topics (HTTP services, performance).

## Setup

Requires **Go 1.27+** (`go.mod` says `go 1.27`).

```powershell
go version                      # must be >= 1.27
go test ./exercises/...         # everything (all red at first)
```

## Workflow

```powershell
go test ./exercises/10_logging_slog/                     # one topic
go test ./exercises/10_logging_slog/ -run Part2 -v       # one part (tests are TestPart<N>...)
go run ./tools/solutions ./exercises/10_logging_slog/    # grade the reference answer instead of yours
go vet ./exercises/10_logging_slog/
```

1. Read `exercises/<topic>/README.md` — it, together with the tests, is the spec.
2. The `.go` files next to the tests are yours. They declare the exported API
   (Go needs it to compile) with bodies that return zero values; implement them.
   Add unexported helpers, fields and files freely.
3. Iterate until green, then compare with `solutions/<topic>/`.

`-race` needs cgo; run `go test -race` on Linux/macOS (or Windows with a C
toolchain) for the concurrency topics.

## How the harness works

- Tests only use the exported API, mostly as black-box `package x_test`.
- Stubs never panic or block, so one unimplemented function fails only its
  own tests instead of killing the test binary.
- `go run ./tools/solutions [packages] [go test flags]` runs `go test -overlay`:
  every non-test `.go` file of an exercise is swapped for the file of the same
  name in `solutions/<topic>/` (yours stay untouched on disk). Tests that read
  non-Go files you own (Dockerfile, SQL) locate them via `internal/exercise`.

With the stubs every test fails; with `go run ./tools/solutions` every test passes.

> `solutions/` is the answer key. Don't open it until your own version is green.

## Exercises

| # | Topic | What you build | Tests |
|---|---|---|---|
| 01 | [Slices and maps](exercises/01_slices_and_maps) | Slice views: Head, Batches, FilterInPlace · Money and volume tiers · TTL + LRU cache · Sliding-window rate limiter · Streaming top-K with container/heap · Layered configuration with map[string]any · Sets, reconciliation and array keys | 199 |
| 02 | [Function and Method Signatures](exercises/02_function_signatures) | Select, identifiers, and copied input · Options: immutable chaining and functional options · Optional values and PATCH semantics · Interface wiring and adapters · Variadic payloads and an event bus · Structured cache keys and a generic cache | 147 |
| 03 | [Packages and scope](exercises/03_packages_and_scope) | Encapsulation, internal/, and iota · Closures and loop variables · Shadowing: := and the swallowed error · A registry without package state · Package-level hooks and scoped overrides · A whitelist-only formula evaluator with go/parser | 199 |
| 04 | [Functions and closures](exercises/04_functions_and_closures) | closures and first-class functions · defer, named results and cleanup · panic and recover at boundaries · middleware chains · retry and timing wrappers · function types as interfaces and command registry · generic memoisation with TTL and stats | 90 |
| 05 | [Iterators](exercises/05_iterators) | Stateless adapters · Pull-style consumers · Streaming text · Paginated APIs · Resource-owning generators · K-way merge · Channel bridge | 111 |
| 06 | [Functional programming with generics](exercises/06_functional_generics) | Generic combinators · Composition and currying · Immutable cart reducer · Event-sourced account and snapshots · Result, ports, and config parsing · Validator combinators · ETL and sales report | 237 |
| 07 | [Structs and methods](exercises/07_structs_and_methods) | Money: a value type with integer minor units · Product and Cart: immutable values and a zero-value cart · Accounts: embedding, interfaces and one mutex · Employee: constructors, loose input and a non-comparable struct · Version: precedence, bumping and text encoding | 302 |
| 08 | [Interfaces in depth](exercises/08_interfaces) | Implicit satisfaction, small interfaces, consumer-side contracts · Optional capabilities via type assertion · Decorators that preserve capabilities · io.Writer pipelines · Notifiers and a fan-out dispatcher · The nil-interface trap, fmt.Stringer, sort.Interface, heap.Interface · Plugin registry with factories, and generics versus interfaces | 134 |
| 09 | [Errors and cleanup](exercises/09_errors_and_cleanup) | Typed config errors · Lookups and inspecting chains · Named steps and traces · Transactional store with savepoints · Cleanup that composes · Batches and retries · Boundaries: panics, must, and context causes | 109 |
| 10 | [Logging with log/slog](exercises/10_logging_slog) | Handlers, levels and normalised keys · Values that describe themselves · A logfmt handler written from scratch · Request ids and fan-out · Per-component levels and sampling · HTTP access log · Managing the default logger | 91 |
| 11 | [Types and Generics](exercises/11_types_and_generics) | Defined types and text marshalling · Enums with a safe zero value · Generic functions and constraints · Generic collections · A concurrent generic repository · A type-routed event bus · Phantom types and units | 113 |
| 12 | [Files & Encoding](exercises/12_files_and_encoding) | Tree scan, ignore rules, and manifest paths · Directory scan, symlinks, and hashing · Diff, path safety, and apply · Atomic writes, strict JSON, and digest encoding · Streaming a JSON array with iterators · CSV report with awkward values · Gzip streams and a versioned binary snapshot | 141 |
| 13 | [Database access with database/sql and SQLite](exercises/13_database_sql) | Connection, DSN and Money · Versioned migrations · Transactions that cannot leak · Repositories over DBTX · Bulk import with bounded memory · Keyset and offset pagination · Reports and the N+1 trap | 87 |
| 14 | [Goroutines and sync (linkwatch)](exercises/14_goroutines_and_sync) | Bounded link checking · Bounded downloader · Shared state · Cache with single-flight loading · Bounded queue and pipeline · CPU-bound parallel map · Periodic scheduler | 130 |
| 15 | [Context and Cancellation](exercises/15_context_and_cancellation) | Aggregate with per-call budgets · Fan-out and hedging · Component lifecycle and cleanup · Paginated iteration · Bounded resource pool · Worker pool with graceful shutdown · Pub/sub broker with slow-consumer policy · Retry with capped backoff · Token-bucket rate limiter · Circuit breaker | 106 |
| 16 | [Testing a billing package (inverted)](exercises/16_testing) | Line items, invoices, totals · Discount tiers and rounding · Dates and the clock · Payment service: retries and cancellation · Logging and receipts · CSV export | 23 |
| 17 | [Time](exercises/17_time) | instants, zones, and the DST policy · parsing heterogeneous timestamps into UTC · calendar versus elapsed arithmetic, Duration overflow, humanize · calendars, billing dates, business days · recurring meetings with an explicit DST policy · booking slots across time zones · injected clocks, a monotonic stopwatch, and timers | 184 |
| 18 | [Deployment](exercises/18_deployment) | Configuration from the environment · HTTP server · Graceful shutdown · Command-line interface · Build metadata · Container image | 96 |
| 19 | [HTTP services](exercises/19_http_services) | REST API on ServeMux · Middleware · Pagination and filtering on GET /items · HTTP client · NDJSON event streaming | 155 |
| 20 | [Performance: allocation discipline and data layout](exercises/20_performance) | Zero-allocation log line parser · Exact-size CSV and query-string building · Preallocation · Batch writer with sync.Pool · Struct layout and fast formatting · AoS vs SoA revenue aggregation | 113 |

Counts are test + subtest cases. Topic notes:

- **16 Testing** is inverted: you write `suite/suite_test.go` for the provided
  `billing` package; the grader runs it against 20 mutants in temp modules
  (~15 s). Run it as `./exercises/16_testing/` (not `/...`).
- **18 Deployment** — you also own `Dockerfile` and `.dockerignore`; one test runs
  a real `go build -ldflags -X`.
- **12 Files** — two symlink tests skip on Windows without symlink privilege.
- `solutions/` has its own `go.mod` so `go build ./...` and gopls ignore it; its
  files only compile through the overlay.

