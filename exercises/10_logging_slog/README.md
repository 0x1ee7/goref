# 10 · Logging with log/slog

## Scenario

You own the logging layer of a shop HTTP service. Every request, background
job and payment call must produce structured logs that operations can filter
by request id, that never leak card or session secrets, that can be turned up
for one noisy component without flooding the rest, and that survive a retry
storm without drowning the disk. You build that toolkit on `log/slog`:
handlers, redaction through `slog.LogValuer`, a logfmt encoder, context-scoped
request ids, fan-out, per-component levels, sampling, an HTTP access log, and
default-logger management.

The package is `shoplog`. All code lives in `exercises/10_logging_slog/`.
Stub files declare the exported surface; the tests in `shoplog_test.go` are the
executable spec, and this README is the prose spec.

## Part 1 · Handlers, levels and normalised keys

```go
const (
    FormatJSON Format = "json"
    FormatText Format = "text"
)
var ErrUnknownFormat = errors.New(...)
var ErrUnknownLevel  = errors.New(...)

func NewHandler(w io.Writer, format Format, level slog.Leveler, addSource bool) (slog.Handler, error)
func ParseLevel(s string) (slog.Level, error)
```

- `NewHandler` returns a `slog.JSONHandler` or `slog.TextHandler` wired with a
  `ReplaceAttr` that normalises only the top-level attributes
  (`len(groups) == 0`):
  - `time` is converted to UTC.
  - `level` is lowercase (`info`, `error+2`).
  - `msg` is renamed `message`.
  - `source`, when present, becomes `file` with the value `base.go:LINE`
    (`filepath.Base` of the file, plus the line).
- Keys inside groups are never renamed. A user key `msg` in group `g` stays `g.msg`.
- A nil level means INFO.
- An unknown format returns an error wrapping `ErrUnknownFormat`.
- `ParseLevel` trims space and ignores case. `WARN` is an alias of `WARNING`.
  `error+2` is accepted. The empty string and unknown names return an error
  wrapping `ErrUnknownLevel`.

Edge cases: a record with a fixed non-UTC time must render in UTC; a nil
level must behave as INFO, not as "everything".

## Part 2 · Values that describe themselves

```go
const RedactedValue = "[REDACTED]"
type Secret string        // LogValue and String both return RedactedValue
type LazyString func() string
type Credentials struct{ User, Password string }
func RequestAttr(r *http.Request) slog.Attr
```

- `Secret` redacts in records and under `fmt` (`%v`, `%s`).
- `LazyString` calls its function once per emitted record. A record that is
  filtered out by level must never call it. A nil `LazyString` logs `""`.
- `Credentials` logs as the group `{user, password}`. The password is always
  `RedactedValue`, even when empty, because the presence of a password is itself
  sensitive.
- `RequestAttr` returns the group `http` with `method`, `path` and `user_agent`.
  The query string is omitted because it carries tokens. A nil request returns
  the zero `slog.Attr`.

## Part 3 · A logfmt handler written from scratch

```go
func NewLogfmtHandler(w io.Writer, level slog.Leveler) slog.Handler
```

Implement `slog.Handler` yourself. Each record is one line:

```
level=INFO msg="hello world" svc=api http.method=GET
```

- No timestamp is written.
- Keys are `level`, `msg`, then the attributes in call order.
- Groups flatten with dotted keys (`http.status`). An empty group name inlines.
  An empty group renders nothing.
- `WithAttrs` pre-renders its output under the group prefix in effect at that time.
- Values are resolved first, so `LogValuer` works. Durations use `String()`.
  Errors use their message.
- Times are RFC 3339 with nanoseconds, in UTC.
- A value is quoted (`strconv.AppendQuote`) when it is empty or contains space,
  `=`, `"`, a control character or DEL.
- A line is written with one `Write` call under a mutex shared by all handlers
  derived from the same root. Lines never interleave.
- A write error is returned wrapped (`%w`).
- A nil level means INFO.

## Part 4 · Request ids and fan-out

```go
func WithRequestID(ctx context.Context, id string) context.Context
func RequestIDFromContext(ctx context.Context) (string, bool)
func NewRequestIDHandler(next slog.Handler) slog.Handler
func NewFanout(hs ...slog.Handler) slog.Handler
```

- `NewRequestIDHandler` adds `request_id` from the context to a record. It does
  nothing when the context has no id or the id is empty.
- An explicit `request_id` wins. That covers a record attribute and a top-level
  `With` attribute. It is never duplicated.
- Inside an open group the attribute lands in that group.
- It works on a `Clone` of the record. The caller's record is not modified.
- `Enabled` delegates to `next`.
- `NewFanout` sends each record to every child that is enabled for its level.
  Each child gets its own `Clone`. Errors are joined with `errors.Join`, and
  one failing child does not stop the others.
- `With` and `WithGroup` go to every child.
- Zero children means nothing is enabled and `Handle` returns nil.

## Part 5 · Per-component levels and sampling

```go
func NewComponentLevels(next slog.Handler, def slog.Level, levels map[string]slog.Level) slog.Handler
var ErrInvalidSampling = ...
func NewSampler(next slog.Handler, first, thereafter int) (slog.Handler, error)
```

Component levels:

- A record's component is the `component` string attribute. It can come from a
  top-level `With` or from the record itself, and the record wins. Inside an open
  group it is ignored.
- The threshold is `levels[component]` when present, otherwise `def`. A record is
  dropped when its level is below that threshold. Dropped records are never
  forwarded.
- `Enabled` is a cheap prefilter: `lv >= floor && next.Enabled(ctx, lv)`, where
  `floor` is the lowest of `def` and every value in `levels`. `next` keeps
  its own level.
- The `levels` map is copied at construction.

Sampling:

- Occurrence `n` of a message text is kept when `n <= first` or
  `(n-first) % thereafter == 0`. Records at WARN or above are always kept.
- Counters are per message text. They are shared by every handler derived from
  the returned one.
- `first` must be at least 0 and `thereafter` at least 1. Otherwise the error
  wraps `ErrInvalidSampling`.
- `Enabled` does not count. Counting happens in `Handle`.

## Part 6 · HTTP access log

```go
const RequestIDHeader = "X-Request-Id"
func AccessLog(logger *slog.Logger, next http.Handler) http.Handler
```

For each request:

- Take the inbound `X-Request-Id` when it is 1 to 128 bytes of printable ASCII
  (0x20 to 0x7E). Otherwise generate 16 lowercase hex characters from 8 random
  bytes.
- Echo the id in the response header and store it in the request context.
  `RequestIDFromContext` must work inside the handler.
- Emit exactly one record, msg `request`, with `request_id`, `RequestAttr(r)`,
  `status`, `bytes` and `duration_ms` (float milliseconds).
- Level is ERROR for 5xx, WARN for 4xx and INFO otherwise.
- `statusWriter` records the first final status. An implicit 200 comes from a
  body written before any header. Informational 1xx responses (except 101) are
  forwarded and not recorded.
- `statusWriter` implements `Unwrap()` so `http.ResponseController` reaches the
  underlying writer.
- A nil logger means `slog.Default()`, read at request time.

## Part 7 · Managing the default logger

```go
func UseDefault(l *slog.Logger) func()
```

- `UseDefault` installs `l` with `slog.SetDefault`. The returned function
  restores the previous default.
- The restore function is idempotent. Only its first call acts, so a stale call
  cannot undo a newer installation.
- Nested installs unwind in order.
- `log` package output goes through the installed handler as well.

## Concepts exercised

- `slog.Handler` contract: `Enabled`, `Handle`, `WithAttrs`, `WithGroup`, group
  semantics, and `Record.Clone` before mutation.
- `ReplaceAttr` and `HandlerOptions`; `slog.Leveler` and `Level` arithmetic.
- `slog.LogValuer` and `Value.Resolve` for redaction and lazy values.
- A correct line-atomic writer with a mutex shared across derived handlers.
- Context-scoped values with unexported key types.
- Fan-out with `errors.Join`, sampling counters under a mutex, and copying
  constructor input (`maps.Clone`, `slices.Clone`).
- `net/http` middleware: `statusWriter`, `Unwrap` and `ResponseController`,
  and first-status-wins semantics.
- `sync.Once` for an idempotent restore; default logger lifecycle.

## Run

```sh
go test ./exercises/10_logging_slog/
go test ./exercises/10_logging_slog/ -run Part2 -v
go run ./tools/solutions ./exercises/10_logging_slog/
```

On Linux and macOS, also run the tests with `-race`. This matters most for
Part 3 and Part 5.

## Stretch (untested)

- Add a `context.Context` deadline to the logfmt write path and measure what a
  blocked writer does to request latency. Propose a drop-or-block policy.
- Make the sampler token-bucket based (rate per second) instead of
  occurrence based. Compare fairness under bursts.
- Emit OpenTelemetry trace ids from the context through the same handler
  chain, without touching call sites.
- Benchmark the logfmt handler against `slog.TextHandler` with `b.Loop()`, and
  see how much the pre-rendered `WithAttrs` buys you.
