# 09 — Errors and cleanup

## Scenario

You own the support layer of a small config-driven service. It loads a JSON
config file, runs multi-step startup work, keeps a transactional key-value
store for runtime state, applies environment overrides for child processes,
writes files atomically, runs batches of background tasks, retries flaky
calls, and sits behind request and job boundaries that must never leak a panic.
Every one of those paths has to fail in a way the caller can classify, log,
and act on, and every one has to clean up after itself on every exit path,
including panics.

Write the package `support` in this directory. The stubs declare the full
exported surface; the tests describe the behaviour. Read the Parts in order.

## Conventions

- Match errors with `errors.Is` and `errors.AsType[T]` (Go 1.26+), never by
  comparing strings.
- Wrap with `%w`. Use `%v` only when you deliberately cut the chain.
- A function that returns an error returns a zero value for the other results
  (unless a Part says otherwise).
- Clean up with `defer`. Cleanup must run on success, on error, and on panic.
- Do not recover panics unless the Part says to.

---

## Part 1 — Typed config errors (`config.go`)

Define the error vocabulary and load a config file.

- Sentinels: `ErrConfig` (umbrella), `ErrConfigNotFound`,
  `ErrConfigUnreadable`, `ErrConfigInvalid`, `ErrSettingMissing`.
- `ConfigError{Kind, Path, Key, Err}` is the one error type. `Kind` is a
  sentinel. `Error()` must mention the Kind message, the `Path` or `Key` when
  set, and the cause when there is one. `Unwrap()` returns `Err`.
  `Is(target)` returns true for `ErrConfig` and for `Kind`.
- `LoadConfig(path) (map[string]any, error)` returns the top-level JSON object.
  - Missing file → `ErrConfigNotFound` (`os.IsNotExist`/`errors.Is(fs.ErrNotExist)`).
  - Any other read failure (a directory, for example) → `ErrConfigUnreadable`.
  - Invalid UTF-8, malformed JSON → `ErrConfigInvalid`. Malformed JSON keeps
    the `*json.SyntaxError` as `Err`.
  - Root that is not an object → `ErrConfigInvalid` with `Err == nil`.
  - `Path` is set on every error from `LoadConfig`.

Concepts: sentinel errors, custom error type with `Is`, `Unwrap`, `%w` chains,
`fs.ErrNotExist`, classifying I/O failures versus data failures.

## Part 2 — Lookups and inspecting chains (`config.go`)

- `Setting[T any](cfg, key, convert) (T, error)` reads a dotted key such as
  `db.pool.size`.
  - A missing key, or a non-object on the way to the key, → `ErrSettingMissing`
    with `Key` set to the full dotted key.
  - `convert == nil`: the value must already be a `T`. A mismatch →
    `ErrConfigInvalid`.
  - `convert != nil`: its error → `ErrConfigInvalid` wrapping the converter's
    error with `%w` (so its cause still matches). If the converter already
    returns an error matching `ErrConfig`, return that error unchanged.
  - On any error the value is the zero `T`.
- `Describe(err) []string` lists the error and every error reachable through
  `Unwrap() error` and `Unwrap() []error`. Format: `"%T: %s"` with the type and
  `Error()`. Order is depth-first pre-order. Each distinct error appears once,
  so a shared error or a cycle terminates. `Describe(nil)` is empty.

Concepts: `errors.Is` through custom types, the two `Unwrap` shapes, generics
with a converter, zero values on failure, a diagnostics helper that must
terminate on cyclic chains.

## Part 3 — Named steps and traces (`steps.go`)

- `StepError{Step, Err}`: `Error()` names the step and the cause, `Unwrap()`
  returns `Err`.
- `RunStep[T any](name, trace, step) (T, error)` brackets a step with trace
  events.
  - Before the step: `start <name>`. On success: `ok <name>`. On error:
    `failed <name>`, and the error is returned as `*StepError` with the zero
    `T`.
  - `end <name>` is always traced last, from a `defer`, including on panic.
  - A panic from `step` is not converted. It propagates.

Concepts: `defer` for guaranteed trace ordering, wrapping with a named context,
panics passing through a generic helper, the difference between "error" and
"panic" as failure channels.

## Part 4 — Transactional store with savepoints (`tx.go`)

A small in-memory store with one open transaction at a time.

- Sentinels: `ErrKeyNotFound`, `ErrTxActive`, `ErrTxDone`,
  `ErrRollbackSavepoint`.
- `NewStore(initial)` copies `initial`. A nil map is allowed. `Get`,
  `Snapshot` (returns a copy), `Begin() (*Tx, error)` (`ErrTxActive` while
  another transaction is open).
- `Tx` is not safe for concurrent use. `Get` sees the transaction's own
  uncommitted writes. `Set`. `Delete` on a key that is not visible →
  `ErrKeyNotFound`. `Commit` and `Rollback` finish the transaction. Once it is
  finished, every operation returns an error matching `ErrTxDone`.
- `Savepoint(fn) error` undoes `fn`'s writes if `fn` fails and keeps the
  transaction open.
  - A non-nil error from `fn` is undone and returned.
  - `ErrRollbackSavepoint` is undone and swallowed: `Savepoint` returns nil.
  - A panic is undone and propagates.
- `Update(s, fn) error` runs `fn` in a new transaction. It commits when `fn`
  returns nil. It rolls back and returns `fn`'s error unchanged when `fn`
  fails, and rolls back and re-panics when `fn` panics.

Concepts: defer-based commit/rollback, idempotent cleanup, savepoints as nested
undo, the "finished" state machine, panics inside transactions, copy on
construction and on read.

## Part 5 — Cleanup that composes (`cleanup.go`)

- `WithEnv(changes map[string]*string, fn) error` applies environment changes,
  runs `fn`, and restores the previous state. A nil value unsets the variable.
  Changes apply in sorted key order. If one fails, the changes already applied
  are undone, and `fn` is not called. Restoration runs when `fn` panics, and the
  panic propagates. A restore error is joined to `fn`'s error only if it is
  non-nil.
- `OpenAll(paths, open) ([]io.ReadCloser, error)` opens paths in order. If one
  open fails, the readers already opened are closed in reverse order. The
  result is nil. The error wraps the open failure with `%w`. Close failures are
  joined with it, so both stay matchable.
- `RemoveFiles(paths) (int, error)` deletes each path and returns how many it
  removed. A missing path is skipped. Any other failure stops the loop and
  returns the count so far.
- `WriteFileAtomic(path, data) error` writes to a temp file in the same
  directory, syncs and closes it, then renames it over `path`. Close errors are
  reported. On any failure the temp file is removed and `path` is unchanged.

Concepts: restoring global state with `defer`, reverse-order teardown, joining
close errors with `errors.Join`, the named-result `Close` pattern, atomic file
replacement, partial-failure reporting.

## Part 6 — Batches and retries (`batch.go`, `retry.go`)

Batch (`batch.go`):

- `ErrDuplicateTask`. `Task[T]{Name, Run}`.
- `TaskError{Name, Err}` (`Error()` renders `task "<name>": <err>`).
- `BatchError{Total, Failures}`. `Error()` renders `"<n> of <total> tasks
  failed"`. `Unwrap() []error` returns the failures so `errors.Is` and
  `errors.As` reach every task.
- `RunBatch[T](tasks) (map[string]T, error)` checks duplicate names before any
  task runs. Otherwise it runs every task, keeps going after failures, and
  returns a nil map plus a `*BatchError` if any task failed. Panics are not
  recovered.
- `Triage(b) (transient, permanent *BatchError)` splits the failures with
  `IsTransient`. Both sides keep the original `Total`. An empty side is nil.
  `Triage(nil)` returns `nil, nil`.

Retry (`retry.go`):

- `IsTransient(err) bool`. Context cancellation and deadline errors are never
  transient. The first error in the chain that implements `Transient() bool`
  decides. Otherwise `Timeout() bool == true` is transient. Otherwise the errnos
  `EAGAIN, EBUSY, EINTR, ECONNABORTED, ECONNREFUSED, ECONNRESET` are transient.
  `nil` is not transient.
- `RetryExhausted{Attempts, Err}`. `Error()` renders `gave up after N attempts:
  <err>`. `Unwrap()` returns `Err`.
- `RetryConfig{Attempts, BaseDelay, Sleep, Classify}`. `Sleep` nil means
  `time.Sleep`. `Classify` nil means `IsTransient`.
- `Retry[T](cfg, fn) (T, error)`. Failed attempts sleep `BaseDelay`, then
  double. There is no sleep after the last attempt. A permanent error is
  returned unchanged at once. A transient error on the last attempt is returned
  as `*RetryExhausted`. `Attempts < 1` is an error and `fn` is not called. On
  failure the value is the zero `T`.

Concepts: aggregating errors without losing them, `Unwrap() []error`, typed
nil pitfalls, classifying errors through interfaces and errnos, bounded retry
with backoff, injecting the clock for tests.

## Part 7 — Boundaries: panics, must, and context causes (`boundary.go`)

- `PanicError{Value, Stack}`. `Unwrap()` returns `Value` when it is an error.
- `Guard(fn) error` converts a panic from `fn` into `*PanicError` with the
  goroutine stack. A plain error from `fn` is returned unchanged.
- `Must[T](v, err) T` returns `v`, or panics with `err`. It is for wiring that
  cannot fail at runtime.
- `RunJob(ctx, job) error` runs `job`. If `job` fails after `ctx` is done and
  `context.Cause(ctx)` is not already matched by the job's error, the result
  wraps both: `fmt.Errorf("run job: %w (cause: %w)", err, cause)`. Otherwise
  the job's error is returned unchanged. Panics are not recovered.

Concepts: recovering panics at a boundary and keeping the stack, `Unwrap` on a
panic value, the `Must` idiom and its limits, `context.WithCancelCause` and
`context.Cause`, multi-`%w` wrapping, and why a boundary must not swallow
panics.

---

## Concepts exercised

- `errors.New`, sentinel values, `errors.Is`, `errors.AsType`, `errors.Join`
- `%w` wrapping (single and multiple), `Unwrap() error`, `Unwrap() []error`
- custom error types with `Is`, typed error values, zero values on failure
- `defer` for cleanup on every exit path, named results, reverse-order teardown
- `panic`, `recover`, re-panicking, converting panics at boundaries
- `fs.ErrNotExist`, `os.ReadFile`, atomic file replacement, `os.Setenv`
- `context.WithCancelCause`, `context.Cause`, `t.Context()`
- generics for converters, tasks and retried operations
- `sync.Mutex` around shared state

## Run

From the repository root:

```sh
go test ./exercises/09_errors_and_cleanup/
go test ./exercises/09_errors_and_cleanup/ -run Part2 -v
go run ./tools/solutions ./exercises/09_errors_and_cleanup/
```

Use the Go toolchain for this repository (Go 1.27). On Linux and macOS, also run
`go test -race ./exercises/09_errors_and_cleanup/`. The race detector needs
cgo, so it does not work on every Windows setup.

## Stretch (untested)

- Replace `Store`'s single-transaction rule with a read-only snapshot
  transaction that never blocks writers, and explain the isolation you get.
- Add `ErrorCode` to `ConfigError` and a `slog.LogValuer` that logs the chain
  without leaking secrets from `Err`.
- Make `RunBatch` bounded-parallel with `errgroup.SetLimit` and collect failures
  the same way. Decide whether one failure should cancel the rest.
- Replace the errno list in `IsTransient` with a `net.Error`-based check and
  discuss where each approach gives false positives.
- Make `WriteFileAtomic` durable: `fsync` the directory after the rename, and
  explain why Windows needs a different approach.
