# 15 — Context and Cancellation

## Scenario

You own a small aggregation service. It fans requests out to flaky upstream
APIs, runs a worker pool behind them, paginates through large listings,
publishes events to subscribers, and protects the upstreams with retries,
rate limits and a circuit breaker. Every one of those pieces must respect
deadlines, cancel cleanly, shut down without leaking goroutines, and report
*why* it stopped. The package is `ctxflow`.

Work through the Parts in order. Each Part has its own file pair:
`<name>.go` (your code) and `<name>_test.go` (the spec). The README and tests
are the spec; the stubs compile but do nothing.

Two package-level sentinels are shared across Parts:

- `ErrInvalidArgument` — wrapped by every constructor or call given a
  configuration that cannot work (non-positive budgets, negative sizes).
- `ErrClosed` — returned when a pool, broker or subscription is used after
  it has been closed or shut down.

General rule: when a context ends, return `context.Cause(ctx)`, not
`ctx.Err()`, so callers can see the reason set with `WithCancelCause` /
`WithTimeoutCause`.

## Part 1 — Aggregate with per-call budgets (`aggregate.go`)

```go
type Source[T any] struct {
    Name  string
    Fetch func(ctx context.Context) (T, error)
}
type Result[T any] struct {
    Name  string
    Value T
    Err   error
}
var ErrCallTimeout error

func WithSource(ctx context.Context, name string) context.Context
func SourceFrom(ctx context.Context) (string, bool)
func Aggregate[T any](ctx context.Context, perCall time.Duration,
    sources []Source[T]) ([]Result[T], error)
```

- `Aggregate` calls every source concurrently. Each call gets its own
  `perCall` budget (`WithTimeoutCause(..., ErrCallTimeout)`).
- Results come back in input order. One failing source does not fail the
  call.
- A source that exceeded its budget reports `ErrCallTimeout`. A source's own
  errors, including its own `DeadlineExceeded`, are reported unchanged.
- The source's context carries its name; `SourceFrom` reads it back.
- If the parent `ctx` ends, the sources are cancelled, the partial results
  are still returned, and the error is `context.Cause(ctx)`.
- `perCall <= 0` → `ErrInvalidArgument`, and no source runs.

Concepts: `WithTimeoutCause`, cause attribution, `context.WithValue` with an
unexported key, fan-out with a `WaitGroup` writing to disjoint indices,
`wg.Go`.

## Part 2 — Fan-out and hedging (`fanout.go`)

```go
func FetchAll[T any](ctx context.Context, sources []Source[T]) ([]T, error)
func Hedge[T any](ctx context.Context, delay time.Duration, attempts int,
    fetch func(ctx context.Context, attempt int) (T, error)) (T, error)
```

`FetchAll`:
- Runs all sources concurrently; values are returned in source order.
  An empty input gives an empty, non-nil slice and `nil`.
- The first error cancels the siblings, and `FetchAll` returns only after
  every goroutine has returned.
- If the parent ctx ended, return its cause, not the siblings'
  `context.Canceled`.

`Hedge`:
- Attempt 0 starts immediately. Attempt n+1 starts when `delay` passes with
  no result, or immediately when attempt n fails.
- The first success wins. Losers are cancelled, and `Hedge` waits for them
  before returning, so none outlive the call.
- All attempts failing → `errors.Join` of the errors in attempt order.
- Parent ctx ends → `context.Cause(ctx)`.
- `attempts < 1` or `delay < 0` → `ErrInvalidArgument`.

Concepts: `errgroup.WithContext`, why waiting for losers matters, timers
that are reset after each launch, `select` over result channels, buffered
result channels so losers never block.

## Part 3 — Component lifecycle and cleanup (`lifecycle.go`)

```go
type Component struct{ /* unexported */ }
func Start(parent context.Context, run func(ctx context.Context) error) *Component
func (c *Component) Stop(cause error)
func (c *Component) Wait() error
func (c *Component) Cause() error

func RunWithCleanup(ctx context.Context, cleanupTimeout time.Duration,
    body, cleanup func(ctx context.Context) error) error
func CloseOnDone(ctx context.Context, c io.Closer,
    onErr func(error)) (stop func() bool)
```

`Component`:
- `Start` runs `run` in a goroutine under a cancellable child of `parent`.
- `Stop(cause)` cancels with that cause. Only the first cause is kept.
- `Wait` blocks until `run` returns and reports its error.
- `Cause` reports why the context ended. It is `nil` until the component
  finishes, and `nil` if it was never cancelled. A clean return with no
  cancellation has `Cause() == nil`.

`RunWithCleanup`:
- If `ctx` is already done, `body` is skipped and its error is
  `context.Cause(ctx)`.
- `cleanup` always runs. It gets a context that ignores `ctx`'s
  cancellation (`context.WithoutCancel`) but is bounded by `cleanupTimeout`.
- The result joins the body and cleanup errors.
- `cleanupTimeout <= 0` → `ErrInvalidArgument`, and nothing runs.

`CloseOnDone`:
- Closes `c` when `ctx` ends, via `context.AfterFunc`. `onErr`, if non-nil,
  receives a `Close` error.
- The returned `stop` reports `true` if it prevented the close, and `false`
  if the close already started or ran.

Concepts: `WithCancelCause`, `Cause` vs `Err`, `WithoutCancel` for cleanup,
bounded cleanup, `AfterFunc` and its stop semantics, goroutine ownership.

## Part 4a — Paginated iteration (`paginate.go`)

```go
type Page[T any] struct {
    Items []T
    Next  string
}
func Paginate[T any](ctx context.Context,
    fetch func(ctx context.Context, cursor string) (Page[T], error)) iter.Seq2[T, error]
```

- A lazy `iter.Seq2`. A page is fetched only when the consumer needs it, so
  `break` stops further fetches.
- An empty `Next` ends the walk. An empty page that still has a `Next` is
  followed.
- `ctx` is checked before each fetch. If it has ended, the sequence yields
  `context.Cause(ctx)` as its final element.
- A fetch error is yielded after the items already produced, and the
  sequence ends. Nothing more is fetched.

Concepts: `iter.Seq2`, lazy pull vs eager slices, yield-false handling,
errors as the final yielded element.

## Part 4b — Bounded resource pool (`resourcepool.go`)

```go
type ResourcePool[R any] struct{ /* unexported */ }
func NewResourcePool[R any](size int,
    factory func(ctx context.Context) (R, error),
    closeFn func(R) error) (*ResourcePool[R], error)
func (p *ResourcePool[R]) Acquire(ctx context.Context) (R, error)
func (p *ResourcePool[R]) Release(r R)
func (p *ResourcePool[R]) Close() error
```

- At most `size` resources exist at once (idle or leased). Idle resources
  are reused before `factory` is called again.
- `Acquire` waits for a free slot. A waiter whose `ctx` ends returns
  `context.Cause(ctx)` and must not consume a slot. A factory error frees
  its slot and is returned.
- `Release` returns a resource to the idle list and wakes one waiter.
- `Close` closes all idle resources and wakes every blocked waiter with
  `ErrClosed`. Resources leased at `Close` are closed when released, and
  any `closeFn` error from that path is discarded (`Release` has no error
  return). A second `Close` returns `nil`.
- `Acquire` on a closed pool → `ErrClosed`.
- `size < 1` → `ErrInvalidArgument`.

Concepts: a channel as a counting semaphore, a cancelled waiter not leaking
a slot, mutex plus semaphore, closing a pool with leased resources in
flight.

## Part 5 — Worker pool with graceful shutdown (`workerpool.go`)

```go
type WorkerPool[J any] struct{ /* unexported */ }
func NewWorkerPool[J any](workers, queue int,
    handle func(ctx context.Context, job J) error) (*WorkerPool[J], error)
func (p *WorkerPool[J]) Submit(ctx context.Context, job J) error
func (p *WorkerPool[J]) Shutdown(ctx context.Context) (abandoned int, err error)
func (p *WorkerPool[J]) Succeeded() int
func (p *WorkerPool[J]) Failed() int
```

- `workers` goroutines consume a queue with capacity `queue`.
- `Submit`'s `ctx` governs admission only: it waits for queue space and
  returns `context.Cause(ctx)` if it ends first. After `Shutdown` has begun,
  `Submit` returns `ErrClosed`.
- The handler's `ctx` belongs to the pool, not to the submitter. The
  submitter's cancellation never reaches a running job.
- `Shutdown` stops admission, then waits for the queue to drain and
  in-flight handlers to return.
- If `Shutdown`'s `ctx` ends first, the handler context is cancelled with
  that cause. Queued jobs that have not started are counted as abandoned,
  and `Shutdown` still waits for in-flight handlers before returning
  `(abandoned, context.Cause(ctx))`.
- A second `Shutdown` returns `(0, ErrClosed)`.
- `workers < 1` or `queue < 0` → `ErrInvalidArgument`.
- `Failed` includes jobs cancelled mid-flight.

Concepts: a channel-based queue with a bounded buffer, separating
admission context from work context, drain versus abort shutdown, the
`closing`-then-close pattern to avoid send-on-closed-channel panics, atomic
counters.

## Part 6 — Pub/sub broker with slow-consumer policy (`broker.go`)

```go
type Broker[M any] struct{ /* unexported */ }
func NewBroker[M any](patience time.Duration) (*Broker[M], error)
func (b *Broker[M]) Subscribe(buffer int) (*Subscription[M], error)
func (b *Broker[M]) Publish(ctx context.Context, msg M) error
func (b *Broker[M]) Close()

type Subscription[M any] struct {
    C <-chan M
    // unexported
}
func (s *Subscription[M]) Unsubscribe()
func (s *Subscription[M]) Dropped() int
```

- Each `Publish` delivers `msg` to every current subscriber.
- Delivery to a subscriber whose buffer has room never waits. Otherwise
  `Publish` waits for room, but only up to one shared `patience` budget per
  `Publish`. A subscriber still full when the budget runs out gets the
  message dropped, and `Dropped` counts it. One slow consumer must not stall
  the others.
- `Publish` returns `context.Cause(ctx)` if `ctx` ends during the wait, and
  `ErrClosed` after `Close`. An already-done `ctx` fails before delivery.
- `Unsubscribe` ends one subscription. It is idempotent and safe to call
  while a `Publish` is blocked on that subscriber. The channel `C` closes,
  and nothing is ever sent on a closed channel.
- `Close` ends every subscription. Messages already buffered stay readable
  before each `C` closes. Later `Publish` and `Subscribe` return
  `ErrClosed`.
- `patience < 0` or `buffer < 0` → `ErrInvalidArgument`.

Concepts: a shared patience deadline across fan-out targets, a
non-blocking try before a blocking send, closing a channel safely while
senders may be in flight, `sync.Once` teardown, `maps` and `slices` helpers.

## Part 7a — Retry with capped backoff (`retry.go`)

```go
type Retrier struct {
    Attempts  int
    Base      time.Duration
    Max       time.Duration
    Retryable func(error) bool
    Jitter    func(nominal time.Duration) time.Duration
}
func (r Retrier) Do(ctx context.Context, op func(ctx context.Context) error) error
```

- Before retry n (n ≥ 1) wait `Jitter(nominal)`, where
  `nominal = min(Max, Base·2^(n-1))`. With `Base` 10 ms and `Max` 35 ms
  the nominals are 10, 20, 35, 35 ms.
- Full jitter when `Jitter` is nil: a uniform random wait in `[0, nominal)`.
  A zero nominal waits zero.
- No wait after the final attempt. `Attempts < 1` is treated as 1.
- `Retryable == nil` retries every error. A non-retryable error returns
  immediately.
- A `ctx` that is already done before an attempt returns
  `context.Cause(ctx)` without calling `op`.
- If `ctx` ends during a wait, return `errors.Join(lastErr, context.Cause(ctx))`.
- The backoff must not overflow for large attempt counts.

Concepts: value receiver for an immutable config, capped exponential
backoff, injectable jitter for deterministic tests, context-aware sleep,
`errors.Join`.

## Part 7b — Token-bucket rate limiter (`ratelimit.go`)

```go
type TokenBucket struct{ /* unexported */ }
func NewTokenBucket(rate float64, burst int,
    now func() time.Time) (*TokenBucket, error)
func (tb *TokenBucket) Allow() bool
func (tb *TokenBucket) Wait(ctx context.Context) error
```

- The bucket starts full with `burst` tokens and refills at `rate` tokens
  per second, capped at `burst`.
- `Allow` takes a token if one is available and never blocks.
- `Wait` blocks until a token is available, then takes it. If `ctx` ends
  first, it returns `context.Cause(ctx)`.
- `Wait` sleeps on the real timer, so tests run it under `synctest` with
  `now` set to `time.Now`.
- `rate <= 0`, `burst < 1` or nil `now` → `ErrInvalidArgument`.

Concepts: lazy refill from an injected clock, a mutex around arithmetic,
computing the sleep from the deficit, context-aware waiting, float
tolerance.

## Part 7c — Circuit breaker (`breaker.go`)

```go
var ErrOpen error

type BreakerState int
const (
    StateClosed BreakerState = iota
    StateOpen
    StateHalfOpen
)
func (s BreakerState) String() string

type Breaker struct{ /* unexported */ }
func NewBreaker(threshold int, cooldown time.Duration,
    now func() time.Time) (*Breaker, error)
func (b *Breaker) State() BreakerState
func (b *Breaker) Do(ctx context.Context, op func(ctx context.Context) error) error
```

- `threshold` consecutive failures open the breaker. While open, `Do`
  returns `ErrOpen` without calling `op`.
- After `cooldown`, the breaker is half-open and admits exactly one probe.
  Calls that arrive while the probe is in flight get `ErrOpen`.
- A successful probe closes the breaker. A failed probe reopens it with a
  fresh cooldown.
- An error caused by the caller's own `ctx` ending is neutral. It neither
  counts as a failure nor resets the count. A neutral probe leaves the
  breaker half-open.
- A `ctx` that is already done returns `context.Cause(ctx)` without calling
  `op`.
- `State` moves open to half-open lazily, from the clock.
- `String` returns distinct, non-empty names.
- `threshold < 1` or `cooldown <= 0` or nil `now` → `ErrInvalidArgument`.

Concepts: a state machine behind one mutex, the single-probe rule, what
counts as a failure (caller-caused errors are neutral), lazy state
transitions from an injected clock, `iota` enums with `String`.

## Concepts exercised

- `context`: `WithCancel`, `WithCancelCause`, `WithTimeout`,
  `WithTimeoutCause`, `WithoutCancel`, `AfterFunc`, `WithValue`,
  `Cause` vs `Err`
- Goroutine ownership: no goroutine outlives the call that started it
- Concurrency: channels as semaphores and queues, `select` with cancellation
  arms, `sync.WaitGroup.Go`, `sync.Once`, `sync.Mutex`/`RWMutex`, atomics
- `errgroup`, `errors.Join`, sentinel and wrapped errors
- `iter.Seq2` lazy sequences
- Testing time with `testing/synctest` and deterministic clocks and jitter
  functions

## Run

```sh
go test ./exercises/15_context_and_cancellation/
go test ./exercises/15_context_and_cancellation/ -run Part2 -v
go run ./tools/solutions ./exercises/15_context_and_cancellation/
```

Run with `-race` on Linux or macOS. The race detector is not available on
Windows without cgo.

## Stretch (untested)

- Make `Aggregate` return partial results with a per-source error type that
  implements `Is`, so callers can match on the source name.
- Add `Hedge` backoff: double `delay` after each losing attempt.
- Give `WorkerPool` a `Stats()` snapshot that is consistent across the four
  counters, using a single mutex.
- Replace the `Breaker`'s single mutex with `atomic.Uint32` state and measure
  the contention difference.
- Add `Broker.Publish` batching: deliver a slice of messages under one
  patience budget.
