# 02 — Function and Method Signatures

## Scenario

You are building a small internal service toolkit that other teams import: a
SQL query builder for a reporting API, an HTTP API client, a PATCH endpoint for
user profiles, a reminder job that depends on a clock and a notifier, an
in-process event bus, and a cache keyed by structured values. Every one of
these is a public API, so its signatures are the contract. Callers should be
able to use it correctly without reading the source, and misuse should fail
loudly and early.

Write the stubs in `query.go`, `client.go`, `patch.go`, `wiring.go`, `bus.go`,
and `cachekey.go`. The tests in `*_test.go` and the Parts below are the spec.

## Part 1 — Select, identifiers, and copied input

API (`query.go`):

```go
var ErrInvalidIdentifier error
var ErrInvalidLimit error

func Select(table string, cols ...string) Query
func (Query) Where(conds ...Cond) Query
func (Query) OrderBy(terms ...string) Query
func (Query) Limit(n int) Query
func (Query) Build() (sql string, args []any, err error)
func Eq(col string, v any) Cond
```

Behaviour:

- `Select` with no columns renders `SELECT * FROM t`.
- Table and column names must match `^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`.
  Anything else is an `ErrInvalidIdentifier`. Identifiers are never quoted or
  escaped, so the builder refuses them rather than trying to make them safe.
- `Select` copies its column slice. Later mutation by the caller has no effect.
- Values go into `args`, never into the SQL text. `Eq(col, nil)` renders
  `col IS NULL` and binds nothing.
- Errors are sticky. The first invalid input is remembered, every later method
  keeps it, and `Build` returns it with empty SQL.

Edge cases the tests check: an invalid table or column fails `Build` with
`errors.Is(err, ErrInvalidIdentifier)`. Mutating the slice you passed to
`Select` does not change the query.

## Part 2 — Options: immutable chaining and functional options

### 2a. Immutable builder

`Query` values are immutable. Branching a base query yields independent
queries. `Where` with no conditions does nothing. `Limit` is last-wins.
`Limit(-1)` returns `ErrInvalidLimit`.

### 2b. Functional options for the client (`client.go`)

API:

```go
const DefaultTimeout = 30 * time.Second
const DefaultUserAgent = "goref-client/1"

var ErrInvalidBaseURL error
var ErrInvalidOption error

type Config struct {
    BaseURL   string
    Timeout   time.Duration
    Retries   int
    UserAgent string
    Headers   http.Header
}

type Option func(*Config) error

func WithTimeout(d time.Duration) Option
func WithRetries(n int) Option
func WithUserAgent(ua string) Option
func WithHeader(name, value string) Option

type Client struct{ /* unexported */ }
func NewClient(baseURL string, opts ...Option) (*Client, error)
func (*Client) Config() Config
```

Behaviour:

- The base URL must be an absolute `http` or `https` URL with a host and no
  query or fragment. A trailing `/` is trimmed. Otherwise the error wraps
  `ErrInvalidBaseURL`.
- Defaults: timeout `DefaultTimeout`, user agent `DefaultUserAgent`, zero
  retries, no headers.
- Options run in order, and the first failing option aborts construction.
  Its error is returned and no client is returned. A nil option is an error
  wrapping `ErrInvalidOption`.
- Option rules: timeout must be positive. Retries must be `>= 0` (zero is
  valid). User agent must not be blank. Header names must be RFC 7230 tokens.
  Header values must not contain CR or LF. Header names are canonicalized, and
  a later `WithHeader` for the same name replaces the value.
- `Config()` returns a deep copy. Changing its `Headers` does not affect the
  client.

## Part 3 — Optional values and PATCH semantics (`patch.go`)

API:

```go
var ErrInvalidEmail error
var ErrInvalidNickname error

type Optional[T any] struct{ /* unexported */ }
func Some[T any](v T) Optional[T]
func Null[T any]() Optional[T]
func (Optional[T]) IsSet() bool
func (Optional[T]) IsNull() bool
func (Optional[T]) Get() (T, bool)
func (Optional[T]) IsZero() bool
func (Optional[T]) MarshalJSON() ([]byte, error)
func (*Optional[T]) UnmarshalJSON([]byte) error

type User struct {
    ID       int
    Email    string
    Nickname *string
}

type UserPatch struct {
    Email    Optional[string] `json:"email,omitzero"`
    Nickname Optional[string] `json:"nickname,omitzero"`
}

func ApplyPatch(u User, p UserPatch) (User, error)
```

Behaviour:

- The zero `Optional` is unset. `Some(zero)` is set, and `Null` is set and null.
  `IsZero` reports unset only, so `omitzero` keeps `Some("")` and `Null` on the
  wire.
- JSON: a missing field is unset. A JSON `null` is `Null`. Any other value is
  `Some`.
- `ApplyPatch` never mutates its inputs. It returns a new `User`, and the
  Nickname pointer is never shared with the input.
- `Email`: null is an error. The value is trimmed and lowercased, then must
  match `^[^@\s]+@[^@\s]+\.[^@\s]+$`. Otherwise the error wraps
  `ErrInvalidEmail`. An unset email keeps the old value.
- `Nickname`: null clears it. A value is trimmed. It must not be blank and must
  be at most 32 runes (count runes, not bytes). Otherwise the error wraps
  `ErrInvalidNickname`. An unset nickname is untouched.

## Part 4 — Interface wiring and adapters (`wiring.go`)

API:

```go
var ErrNilDependency error

type Clock interface{ Now() time.Time }
type ClockFunc func() time.Time
func (ClockFunc) Now() time.Time

type Notifier interface{ Notify(ctx context.Context, msg string) error }
type NotifierFunc func(ctx context.Context, msg string) error
func (NotifierFunc) Notify(ctx context.Context, msg string) error

type Reminders struct{ /* unexported */ }
func NewReminders(clock Clock, notifier Notifier) (*Reminders, error)
func (*Reminders) Check(ctx context.Context, due time.Time) (sent bool, err error)
```

Behaviour:

- Each interface has one method, and the function types implement it, the same
  way `http.HandlerFunc` does. Consumers depend on the interface, not the
  concrete type.
- `NewReminders` rejects a nil interface and names the parameter, for example
  `nil dependency: clock`, wrapping `ErrNilDependency`.
- `Check` returns `sent == true` only after a successful `Notify`.
- If `ctx` is already cancelled, `Check` returns `context.Canceled`, or the
  context's error, before it reads the clock.
- If `now < due`, nothing is sent and the result is `(false, nil)`. If
  `now >= due`, the message is `"reminder due " + due.UTC().Format(time.RFC3339)`.
- Every error from `Check` is wrapped with the prefix `"reminders: "`, and the
  original error is still reachable with `errors.Is`.
- A method value such as `r.Check` works as an ordinary function value.

## Part 5 — Variadic payloads and an event bus (`bus.go`)

API:

```go
var ErrNilHandler error

type Handler func(ctx context.Context, payload ...any) error

type Bus struct{ /* unexported; zero value usable */ }
func (*Bus) Subscribe(topic string, h Handler) (cancel func(), err error)
func (*Bus) Publish(ctx context.Context, topic string, payload ...any) error
```

Behaviour:

- Subscribing a nil handler returns `ErrNilHandler`.
- Each handler gets its own copy of the payload slice, so one handler cannot
  change what another sees.
- `Publish` takes a snapshot of the subscribers before any handler runs.
  Handlers subscribed during the publish wait for the next one. Handlers
  cancelled during it still run.
- No lock is held while handlers run. A handler may subscribe, cancel, or
  publish without deadlocking. The tests guard this with a timeout.
- `Publish` runs every handler even after one fails. Errors are combined with
  `errors.Join`, and `errors.Is` finds each one.
- If `ctx` is done, the remaining handlers are skipped and `ctx.Err()` is part
  of the result.
- `cancel` removes only its own registration. It is idempotent, and the same
  handler can be subscribed twice and cancelled separately.

## Part 6 — Structured cache keys and a generic cache (`cachekey.go`)

API:

```go
var ErrUnsupportedKeyPart error
var ErrInvalidNamespace error

func Key(namespace string, parts ...any) (string, error)

type Cache[V any] struct{ /* unexported; zero value usable */ }
func (*Cache[V]) Get(key string) (V, bool)
func (*Cache[V]) Set(key string, v V)
func (*Cache[V]) Remember(ctx context.Context, key string,
    load func(context.Context) (V, error)) (V, error)
```

Behaviour:

- Key format is `namespace:part:part`. The namespace must be non-empty and must
  not contain `:`.
- Supported parts: strings, bools, `time.Time` (UTC, RFC 3339 with
  nanoseconds), and every integer kind, including named types, selected by
  `reflect.Kind`. Anything else, including nil, floats, slices, and structs, is
  an error wrapping `ErrUnsupportedKeyPart`.
- Parts must not be able to forge a boundary. A `:` inside a part is escaped,
  and so is `%`, so `("a:b","c")` and `("a","b:c")` produce different keys.
- `Remember` returns a cached value on a hit. On a miss it calls `load` with the
  caller's context, stores a successful result, and returns it. Errors are
  returned and not cached.
- `Remember` has no single-flight. Concurrent misses may each call `load`.
- The zero `Cache` is usable and safe for concurrent use.

## Concepts exercised

- Parameter design: variadic arguments, copying slices, and reference or value
  semantics of slices, maps, and pointers.
- Immutable value builders and sticky errors, compared with a mutating builder.
- The functional options pattern, with validation owned by each option and
  deep-copied configuration.
- Zero values, `omitzero`, and the three-state `Optional[T]` for PATCH.
- Small consumer-side interfaces, function adapters, and method values.
- Error values, `%w` wrapping, `errors.Is`, `errors.Join`, and error prefixes.
- Context propagation and cancellation checks.
- Generics: `Cache[V]`, `Optional[T]`, and generic constructors.
- Concurrency-safe zero values and no locks held during callbacks.

## Run

```sh
go test ./exercises/02_function_signatures/
go test ./exercises/02_function_signatures/ -run Part2 -v
go run ./tools/solutions ./exercises/02_function_signatures/
```

On Linux and macOS, also run the tests with `-race`:

```sh
go test -race ./exercises/02_function_signatures/
```

## Stretch (untested)

- Add a `Query.Join` that takes a typed `JoinSpec`. Decide whether it should be
  a method or a function, and justify the choice.
- Make `Optional[T]` round-trip through `database/sql` with `Scan` and `Value`.
- Replace `Cache.Remember` with a single-flight version using
  `golang.org/x/sync/singleflight`. Check that a failing `load` is shared by all
  waiters and still not cached.
- Turn `Bus` into a bounded, asynchronous dispatcher. Decide what the
  backpressure contract should be.
- Compare the functional-options style with an options struct plus a
  `Validate()` method, using a real API you have worked with.
