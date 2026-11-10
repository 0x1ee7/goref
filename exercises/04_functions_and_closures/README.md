# 04 · Functions and closures

You are building the function-level toolkit a platform team ships with its Go
services: closures and pipelines, a deferred-cleanup stack, panic boundaries,
HTTP middleware, retry and timing wrappers, function types that implement
interfaces, a command registry, and a TTL memoiser.

Everything lives in package `funcs` in the files named below. The tests are
black-box (`package funcs_test`) and the stubs compile but return zero values,
so every test fails until you implement the spec. Run them with the toolchain
from the repo root:

```
go test ./exercises/04_functions_and_closures/
```

## Part 1: closures and first-class functions (`closures.go`)

- `NewIDGenerator(prefix string) func() string`: each generator has its own
  counter starting at 1. IDs are `prefix + "-" + n` in decimal. The returned
  function is safe for concurrent use and never repeats an ID.
- `Compose[T any](steps ...func(T) (T, error)) func(T) (T, error)`: applies the
  steps left to right, so `Compose(f, g)(x)` is `g(f(x))`. The first failing
  step stops the chain. Its error is returned wrapped as `step N: %w`, with N
  1-based, together with the zero `T`. With no steps the result is the identity.

## Part 2: defer, named results and cleanup (`cleanup.go`)

- `Cleanup`: the zero value is ready to use. `Defer(fn func() error)` registers
  a function. `Run() error` runs them in LIFO order. Every function runs even if
  earlier ones fail, and the errors are joined with `errors.Join` in the order
  they occurred. A second `Run` runs nothing and returns nil.
- `Annotate(err *error, format string, args ...any)`: meant to be deferred
  against a named error result. If `*err` is non-nil it becomes
  `fmt.Errorf(format+": %w", args..., *err)`, so `errors.Is` still works. A nil
  error is left alone. The arguments are evaluated when the `defer` statement
  runs, not when the function returns.
- `WriteFileAtomic(path string, data []byte) (err error)`: writes a temporary
  file in the same directory, `Sync`s it, and closes it, checking the `Close`
  error because a failed close on a written file is a data-loss bug.
  It then renames the temporary file over `path`. Any failure is returned with
  the prefix `write <path>: ` and the temporary file is removed. On success no
  temporary file is left behind.
- `ForEachFile(paths []string, fn func(path string, f *os.File) error) error`:
  opens each path in order, calls `fn`, and closes the file before moving on.
  Do not defer the close to the end of the function. Deferred calls inside a
  loop keep every file open until the function returns. Failures do not stop the
  loop. Open errors are wrapped as `open <path>: %w`, `fn` errors as
  `<path>: %w`, and close errors as `close <path>: %w`. All errors are joined,
  and the result is nil when nothing failed. Every file is closed when
  `ForEachFile` returns.

## Part 3: panic and recover at boundaries (`safe.go`)

- `PanicError{Value any, Stack []byte}`: `Error()` returns `"panic: <value>"`
  (`%v` formatting). `Unwrap()` returns `Value` when it is an `error`, and nil
  otherwise.
- `Protect(fn func() error) (err error)`: returns `fn`'s error unchanged. If
  `fn` panics, the panic is recovered and returned as a `*PanicError`. The stack
  is captured in the deferred function with `debug.Stack`.
  Since Go 1.21 `panic(nil)` also produces a non-nil recovered value
  (`*runtime.PanicNilError`), so `recover() != nil` is the correct test.
- `SafeGo(report func(error), fn func())`: runs `fn` in a new goroutine. If it
  panics, the panic is recovered and `report` is called with a `*PanicError`
  from that goroutine, and the process keeps running. If `fn` returns normally,
  `report` is not called. `report` must be non-nil.

## Part 4: middleware chains (`middleware.go`)

- `type Middleware func(http.Handler) http.Handler`.
- `Chain(mws ...Middleware) Middleware`: the first argument is the outermost.
  `Chain(a, b)(h)` behaves as `a(b(h))`. A request enters `a`, then `b`, then
  `h`, and the response unwinds through `b` and then `a`. With no middlewares
  the handler is returned unchanged.
- `RequestID(gen func() string) Middleware`: if the request carries an
  `X-Request-Id` header, that value is used and `gen` is not called. Otherwise
  `gen()` is called. The id is set on the response `X-Request-Id` header and
  stored in the request context. `RequestIDFrom(ctx)` returns it, or `""`.
- `Recover(logf func(format string, args ...any)) Middleware`: when the wrapped
  handler panics, it logs with `logf("panic serving %s %s: %v\n%s", method,
  path, value, stack)`, where the stack comes from `debug.Stack`. If response
  headers have not been written yet it responds `500` with the body
  `http.StatusText(500)`. If they have, it must not write a second status. A
  panic value of `http.ErrAbortHandler` is re-panicked without logging or
  responding, because `net/http` uses it to abort deliberately.
- `Instrument(now func() time.Time, record func(Sample)) Middleware`: calls
  `now()` before and after the wrapped handler and calls `record` once with a
  `Sample{Method, Path, Status, Duration}`. `Path` is `r.URL.Path`. `Status` is
  the status that was actually written, and 200 if nothing was written
  explicitly. If the handler panics, `record` is not called and the panic
  propagates.
- Ordering matters. `Chain(Instrument(...), Recover(...))` lets `Instrument`
  see the `500` that `Recover` writes. Reversing the order would hide the panic
  from the status recorder.

## Part 5: retry and timing wrappers (`retry.go`)

- `ErrInvalidAttempts`: returned by `Retry` when `attempts < 1`.
- `type Backoff func(attempt int) time.Duration`: `attempt` is 1-based and is the
  attempt that just failed.
- `type Sleeper func(ctx context.Context, d time.Duration) error`: an injected
  wait that returns non-nil if it was cut short. Production code uses a timer
  with `ctx.Done()`. Tests inject a fake.
- `Permanent(err error) error`: marks `err` as not retryable. `Retry` returns
  it immediately. The result still matches `err` with `errors.Is` and
  `errors.As`. `Permanent(nil)` is nil.
- `Retry(ctx, attempts int, backoff Backoff, sleep Sleeper, fn func(context.Context) error) error`:
  - `attempts < 1` returns `ErrInvalidAttempts` unwrapped and never calls `fn`.
  - `fn(ctx)` is called. A nil result returns nil immediately.
  - A `Permanent` error is returned immediately, with no sleep.
  - After a retryable failure on attempt `i < attempts`, `sleep(ctx, backoff(i))`
    is called before the next attempt. It is called even when the delay is `0`.
    It is never called after a success or after the last attempt. If `sleep`
    returns an error, `Retry` returns `errors.Join(lastErr, sleepErr)`.
  - Before every attempt after the first, a done `ctx` returns
    `errors.Join(lastErr, ctx.Err())` without calling `fn`.
  - When every attempt fails the result is
    `fmt.Errorf("retry: gave up after %d attempts: %w", attempts, lastErr)`.
- `Timed[In, Out any](name string, clock func() time.Time, logf func(format string, args ...any), fn func(In) (Out, error)) func(In) (Out, error)`:
  each call reads `clock()` before and after `fn`, then logs exactly once with
  `logf("%s took %v err=%v", name, elapsed, err)`. Results are returned
  unchanged. Panics propagate without a log line.

## Part 6: function types as interfaces and command registry (`handlers.go`, `commands.go`)

- `Event{Kind, Payload string}`. `Handler` is an interface with
  `Handle(ctx, Event) error`. `HandlerFunc` is a func type with a `Handle`
  method, the same pattern as `http.HandlerFunc`, so a plain function or a
  method value satisfies `Handler` through a conversion.
- `Fanout(hs ...Handler) Handler`: delivers each event to every handler in
  order and never stops early. It returns `errors.Join` of the errors, or nil.
- `Only(kind string, h Handler) Handler`: forwards events whose `Kind` equals
  `kind`. Other events are dropped and return nil.
- `BindMethod[R any](recv R, m func(R, context.Context, Event) error) Handler`:
  takes a method expression such as `(*Recorder).Record`, whose receiver is an
  ordinary first parameter, and returns a `Handler` bound to `recv`. Contrast
  this with a method value such as `rec.Record`, which is already bound and can
  be converted directly with `HandlerFunc(rec.Record)`.
- `Recorder`: the zero value is ready to use and is safe for concurrent use.
  `Record(ctx, Event) error` appends the event. `Events() []Event` returns a
  copy in record order, so the caller cannot mutate the recorder through it.
- `type Command func(ctx context.Context, args []string) error`.
- `Registry`: the zero value is ready to use, and each instance is independent.
  - `Register(name string, cmd Command) error`: an empty name or nil `cmd`
    returns an error wrapping `ErrInvalidCommand`. A duplicate name returns one
    wrapping `ErrDuplicateCommand`.
  - `Run(ctx, name string, args []string) error`: an unknown name returns an
    error wrapping `ErrUnknownCommand`. Otherwise the command is called with
    `args` and its error is returned unchanged.
  - `Names() []string`: sorted.

## Part 7: generic memoisation with TTL and stats (`memo.go`)

- `NewMemo[K comparable, V any](ttl time.Duration, clock func() time.Time, load func(ctx context.Context, key K) (V, error)) *Memo[K, V]`.
  A `ttl <= 0` disables expiry. An entry stored at `t` is fresh while
  `clock() < t + ttl`, so it is already expired at exactly `t + ttl`.
- `Get(ctx, key) (V, error)`:
  - A fresh entry is returned without calling `load` (a hit).
  - A missing or expired entry calls `load(ctx, key)`, run in the calling
    goroutine. The load receives the `ctx` of the `Get` that started it.
  - Concurrent `Get`s for the same key during one load wait for that single
    load and share its result and error. This is the single-flight pattern.
    Only one `load` runs per key at a time.
  - A waiter whose `ctx` ends stops waiting and returns `ctx.Err()`. The load
    continues for the others.
  - Errors are never cached, so the next `Get` loads again. A failed `Get`
    returns the zero `V` and the error.
- `Forget(key K)`: drops the stored value so the next `Get` loads again. An
  in-flight load is not affected.
- `Stats() MemoStats`: `Hits` counts `Get`s answered from a fresh entry. `Misses`
  counts all other `Get`s, including waiters that joined an in-flight load.
  `Loads` counts `load` invocations, and `Errors` counts the ones that failed.
  Each `Get` is counted exactly once as a hit or a miss.

## Concepts exercised

- Closures capture variables, not values. Go 1.22 made loop variables
  per-iteration, but a closure over a variable that is reassigned later still
  sees the new value.
- `defer` evaluates its arguments immediately. It runs LIFO, and it can modify
  named results. `Close` errors on written files must be checked.
- `recover` works only in the deferred function that runs during the panic.
  Panics do not cross goroutines, and `panic(nil)` is a real panic since Go 1.21.
- Middleware as `func(http.Handler) http.Handler`, with ordering that decides
  what each layer can observe.
- Injected clocks, sleepers and id generators keep time-based code deterministic.
- Function types implementing interfaces (`HandlerFunc`), method expressions
  versus method values, and generics with `comparable` keys and `any` values.
- `errors.Join`, `%w` wrapping, sentinel errors and `errors.Is` for every
  contract above.

## Run

```
go test ./exercises/04_functions_and_closures/                       # all tests (fail until implemented)
go test ./exercises/04_functions_and_closures/ -run Part2 -v         # one part
go run ./tools/solutions ./exercises/04_functions_and_closures/      # grade the reference solution
```

On Linux and macOS, also run the suite with `-race`. The Windows toolchain
here has no cgo, so `-race` is unavailable.

## Stretch

- Make `Retry` honour a `Retry-After` value carried by the error type, with a
  `Backoff` that receives the error.
- Give `Timed` a `context.Context` argument and log the span via `slog` with
  attributes instead of a formatted string.
- Let `Recover` report the panic to an error hook and also count panics in
  `Instrument` output.
- Replace the `Memo` in-flight map with `golang.org/x/sync/singleflight` and
  compare the cancellation behaviour you get.
