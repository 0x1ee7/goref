# 14 — Goroutines and sync (linkwatch)

## Scenario

You are building **linkwatch**, an internal service that checks the health of
links in documentation and runs background jobs against a shared service
registry. The work is concurrent, cancellable and must never leak goroutines.
Each Part adds one building block.

Package `goroutines` (import `goref/exercises/14_goroutines_and_sync`). Every
stub file is engineer-owned: fill in the bodies, keep the exported surface.

## Part 1 — Bounded link checking (`linkcheck.go`)

```go
type Fetcher func(ctx context.Context, url string) (int, error)
type LinkResult struct { URL string; Status int; Err error }
func (r LinkResult) OK() bool
func CheckLinks(ctx context.Context, urls []string, fetch Fetcher, workers int) ([]LinkResult, error)
var ErrInvalidWorkers
```

- One result per input, in input order. Duplicate URLs are fetched once; the result repeats in place.
- Per-URL errors go in `Err` (`Status` is 0 when `Err != nil`) and do not stop the run.
- `workers < 1` returns `ErrInvalidWorkers`. Empty input returns an empty result and no error.
- Cancellation: once `ctx` is done, no new fetch starts. Unfetched URLs get `Err = ctx.Err()` and `CheckLinks` returns `ctx.Err()`.
- Return only after every worker has exited.
- `OK()`: `Err == nil` and `200 <= Status < 400`.

Concepts: worker pool, channel fan-out, dedupe with a map, `ctx.Done` in a
`select`, `WaitGroup.Go`.

## Part 2 — Bounded downloader (`downloader.go`)

```go
type BodyFetcher func(ctx context.Context, url string) ([]byte, error)
type Downloader struct{ /* engineer-owned */ }
func NewDownloader(fetch BodyFetcher, maxConcurrent int) (*Downloader, error)
func (d *Downloader) Download(ctx context.Context, url string) ([]byte, error)
func (d *Downloader) DownloadAll(ctx context.Context, urls []string) ([][]byte, error)
func (d *Downloader) InFlight() int64
func (d *Downloader) PeakConcurrency() int64
var ErrInvalidConcurrency
```

- `maxConcurrent` is a limit shared by every caller of the same `Downloader`. Use a semaphore (`golang.org/x/sync/semaphore`).
- `Download` on an already-done context returns `ctx.Err()` without calling `fetch`. A wait for a slot that ends with cancellation returns `ctx.Err()`.
- `DownloadAll` returns bodies in input order. On the first failure it cancels the remaining work (`errgroup.WithContext`), waits for every started fetch to finish, and returns `(nil, err)`. `err` wraps the first failure and names its URL.
- `PeakConcurrency` is the high-water mark of `InFlight`, measured inside the slot. It never exceeds `maxConcurrent`.

Concepts: `semaphore.Weighted`, `errgroup` with `SetLimit`, atomic counters
and compare-and-swap high-water marks, error wrapping with `%w`.

## Part 3 — Shared state (`shared.go`)

```go
type Counter struct{ /* zero value usable */ }
func (c *Counter) Add(name string, delta int64) int64
func (c *Counter) Get(name string) int64
func (c *Counter) Snapshot() map[string]int64

type Account struct{ /* create with NewAccount */ }
func NewAccount(id string, balance int64) (*Account, error)
func (a *Account) ID() string
func (a *Account) Balance() int64
func (a *Account) Deposit(amount int64) error
func (a *Account) Withdraw(amount int64) error
func Transfer(src, dst *Account, amount int64) error

type Registry struct{ /* zero value usable */ }
func (r *Registry) Register(name, addr string)
func (r *Registry) Lookup(name string) (string, bool)
func (r *Registry) Names() []string

func PermanentOnce[T any](load func() (T, error)) func() (T, error)
type Lazy[T any]
func NewLazy[T any](load func() (T, error)) *Lazy[T]
func (l *Lazy[T]) Get() (T, error)

var ErrInvalidAmount, ErrInsufficientFunds, ErrSameAccount
```

- `Counter` and `Registry` are safe for concurrent use. `Snapshot` returns a copy.
- `Transfer` is atomic: no goroutine sees money in flight. On error neither account changes. Concurrent opposite-direction transfers must not deadlock, so lock acquisition needs one global order.
- `PermanentOnce` caches the result and the error forever (`sync.OnceValues`). Concurrent callers share the first outcome.
- `Lazy` caches only success. A failed load is returned to that caller, and the next `Get` retries.

Concepts: `sync.Mutex`, `sync.RWMutex`, lock ordering, `sync.Once` family,
the zero value as a usable type, copy hazards (`noCopy`).

## Part 4 — Cache with single-flight loading (`cache.go`)

```go
type LRU[K comparable, V any]
func NewLRU[K comparable, V any](capacity int) (*LRU[K, V], error)
func (c *LRU[K, V]) Get(key K) (V, bool)      // marks as most recently used
func (c *LRU[K, V]) Peek(key K) (V, bool)     // does not change recency
func (c *LRU[K, V]) Put(key K, value V)
func (c *LRU[K, V]) Len() int

type Loader[V any] func(ctx context.Context, key string) (V, error)
type Cache[V any]
func NewCache[V any](capacity int, load Loader[V]) (*Cache[V], error)
func (c *Cache[V]) GetOrLoad(ctx context.Context, key string) (V, error)
func (c *Cache[V]) Loads() int64
var ErrInvalidCapacity
```

- `GetOrLoad` coalesces concurrent misses for the same key: one `Loader` call, all callers share the result.
- A waiter whose `ctx` ends stops waiting and gets `ctx.Err()`. The load it joined keeps running, and its result is still cached. The loader therefore must not see the first caller's cancellation.
- Loader errors go to every waiter and are not cached.

Concepts: `container/list` plus a map for LRU, `singleflight.Group.DoChan`,
`context.WithoutCancel`, `atomic.Int64`.

## Part 5 — Bounded queue and pipeline (`queue.go`, `pipeline.go`)

```go
var ErrQueueClosed
type BoundedQueue[T any]
func NewBoundedQueue[T any](capacity int) (*BoundedQueue[T], error)
func (q *BoundedQueue[T]) Put(item T) error   // blocks when full
func (q *BoundedQueue[T]) Get() (T, bool)     // blocks when empty
func (q *BoundedQueue[T]) Close()
func (q *BoundedQueue[T]) Len() int

func Source[T any](ctx context.Context, g *errgroup.Group, items []T) <-chan T
func Map[In, Out any](ctx context.Context, g *errgroup.Group, in <-chan In, workers int, fn func(context.Context, In) (Out, error)) <-chan Out
func Collect[T any](ctx context.Context, in <-chan T) []T
```

- `Put` on a closed queue returns `ErrQueueClosed`. `Close` is idempotent and wakes every blocked `Put` and `Get`. After `Close`, `Get` drains the remaining items, then returns `(zero, false)`.
- `Source` and `Map` close their output channel on every exit path, including cancellation, so readers never hang. `Map` closes its output only after all of its workers have stopped sending.
- `workers < 1` in `Map` means 1. An `fn` error is returned by `g.Wait()` and cancels the shared context.

Concepts: `sync.Cond` for a bounded buffer, unbuffered channel pipelines,
`errgroup` cancellation, the rule that the sender closes the channel.

## Part 6 — CPU-bound parallel map (`parallel.go`)

```go
func ParallelMap[T, R any](ctx context.Context, workers int, items []T, fn func(context.Context, T) (R, error)) ([]R, error)
```

- Results are in input order. `workers <= 0` means `runtime.GOMAXPROCS(0)`. Empty input returns an empty slice without calling `fn`.
- The first `fn` error wins: workers stop taking items, the error is returned and the results are nil.
- If `ctx` ends before every item is processed, returns `(nil, ctx.Err())`.
- Returns only after every worker has exited.

Concepts: work distribution with an atomic index, `sync.Once` for the first
error, results written by index with no lock, `GOMAXPROCS` as the default.

## Part 7 — Periodic scheduler (`scheduler.go`)

```go
type Job func(ctx context.Context) error
type JobError struct{ Name string; Err error }   // Error() = "job <name>: <err>", Unwrap
type Scheduler struct{ /* zero value usable */ }
func (s *Scheduler) Add(name string, interval time.Duration, job Job) error
func (s *Scheduler) Start(ctx context.Context) error
func (s *Scheduler) Stop(ctx context.Context) error
func (s *Scheduler) Errors() []JobError
var ErrInvalidInterval, ErrNilJob, ErrDuplicateJob, ErrStarted, ErrStopped
```

- Each job runs immediately, then waits `interval` **after each run finishes**, then runs again. Each job gets its own goroutine.
- Failures are recorded as `JobError` in the order they happen. A run that fails with `context.Canceled` while the scheduler is stopping is not recorded.
- `Stop` cancels the job contexts, waits for every job goroutine, and returns nil. If `ctx` ends first it returns `ctx.Err()`, and later `Stop` calls keep waiting. `Stop` before `Start`, and repeated `Stop` calls, are no-ops.
- `Add` after `Start` returns `ErrStarted`. `Start` after `Stop` returns `ErrStopped`.

Concepts: one goroutine per job, `time.Timer` with `select` on `ctx.Done`,
`testing/synctest` for deterministic time, the difference between a ticker
and "sleep after the run".

## Run

```sh
go test ./exercises/14_goroutines_and_sync/
go test ./exercises/14_goroutines_and_sync/ -run Part2 -v
go run ./tools/solutions ./exercises/14_goroutines_and_sync/
```

Stubs return zero values, so every test fails until you implement the part.
Use `-race` on Linux or macOS; it is not available on Windows without cgo.

## Stretch (untested)

- Add a `Retry` helper for `Fetcher` that uses exponential backoff with jitter. Make it cancellable and test it with `synctest`.
- Replace `Cache`'s `singleflight` with a hand-written call map. Compare the lock contention under load.
- Give `Scheduler` a jitter option and a per-job timeout, and report timeouts as `JobError`.
- Make `BoundedQueue` lock-free with channels only. Explain what you lose (`Len`, `Close` semantics).
