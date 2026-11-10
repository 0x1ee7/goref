# 19 · HTTP services

Scenario: you own the inventory service for a small warehouse. Other teams call
it over HTTP: a storefront lists and reserves stock, a back-office tool edits
items with optimistic concurrency, an ops dashboard streams events, and a
batch job pulls the whole catalogue through a Go client. You build the server
side (routing, validation, middleware, pagination, streaming), the client, and
the stream, using only the standard library plus what `go.mod` already provides.

The package is `httpsvc`. Everything lives in this directory; tests are
black-box (`package httpsvc_test`) and drive the handlers through `httptest`
and real loopback servers.

## Exported surface

```go
// api.go
const MaxBodyBytes = 1 << 20
type Item struct{ ID, Name string; Quantity int }   // json: id, name, quantity
type ItemInput struct{ Name string; Quantity int }   // json: name, quantity
type Problem struct {                                // RFC 9457 problem details
    Type, Title string; Status int; Detail string
    Errors map[string]string                         // json: errors (field -> reason)
}
type FieldErrors map[string]string                   // error; field -> reason
func NewInventoryHandler() http.Handler

// middleware.go
type Middleware func(http.Handler) http.Handler
func Chain(h http.Handler, mws ...Middleware) http.Handler // mws[0] outermost
func RequestID(next http.Handler) http.Handler
func RequestIDFrom(ctx context.Context) string
func AccessLog(logger *slog.Logger) Middleware
func Recover(logger *slog.Logger) Middleware
func CORS(origins ...string) Middleware
func BearerAuth(token string) Middleware

// pagination.go
const DefaultLimit, MaxLimit = 20, 100
type ItemPage struct{ Items []Item; NextCursor string } // json: items, next_cursor
type ListQuery struct{ Limit int; Cursor, Search string; MinQuantity int }
func ParseListQuery(v url.Values) (ListQuery, error)
func (q ListQuery) Values() url.Values

// client.go
type Option func(*Client)
func WithHTTPClient(hc *http.Client) Option
func WithToken(token string) Option
func WithRetries(n int) Option
func WithBackoff(base time.Duration) Option
type Client struct{ /* engineer-owned */ }
func NewClient(baseURL string, opts ...Option) (*Client, error)
func (c *Client) CreateItem(ctx context.Context, name string, qty int) (Item, error)
func (c *Client) GetItem(ctx context.Context, id string) (Item, string, error) // item, ETag
func (c *Client) UpdateItem(ctx context.Context, id, etag, name string, qty int) (Item, string, error)
func (c *Client) DeleteItem(ctx context.Context, id string) error
func (c *Client) ListItems(ctx context.Context, q ListQuery) (ItemPage, error)
func (c *Client) AllItems(ctx context.Context, q ListQuery) iter.Seq2[Item, error]
type APIError struct{ StatusCode int; Problem Problem }
func (e *APIError) Error() string
type AuthTransport struct{ Base http.RoundTripper; Token string }
type MetricsTransport struct{ Base http.RoundTripper; Observe func(method string, status int, elapsed time.Duration, err error) }
func ParseRetryAfter(value string, now time.Time) (time.Duration, bool)

// stream.go
type Event struct{ Seq int64; Type, Message string }  // json: seq, type, message
type Broker struct{ /* engineer-owned */ }
func NewBroker() *Broker
func (b *Broker) Publish(eventType, message string)
func (b *Broker) Close()
func (b *Broker) Subscribers() int
func (b *Broker) Handler() http.Handler
func StreamEvents(ctx context.Context, client *http.Client, target string, after int64) iter.Seq2[Event, error]
```

Rules that apply to every Part:

- JSON bodies are `application/json`. Errors are `application/problem+json`
  with the `Problem` shape above; `Type` is `about:blank` and `Title` is
  `http.StatusText(status)`.
- Handlers never panic on bad input. Unknown paths are the mux's business.
- Store state is in memory and guarded by a mutex; tests create a fresh handler per case.

## Part 1 · REST API on ServeMux

Routes use Go 1.22 method and wildcard patterns:
`GET /items`, `POST /items`, `GET /items/{id}`, `PUT /items/{id}`, `DELETE /items/{id}`.

- `POST /items` returns `201`, `Location: /items/{id}`, `ETag: "1"`, and the item.
  IDs are increasing decimal strings starting at `"1"`.
- `GET /items/{id}` returns `200` with the item and its current `ETag`. Missing → `404`.
- `PUT /items/{id}` replaces name and quantity. Order of checks:
  1. Content-Type not JSON → `415`; body larger than `MaxBodyBytes` → `413`;
     malformed or unknown-field JSON, or trailing data → `400`.
  2. Missing `If-Match` → `428`.
  3. Validation failure → `422` with `Errors`.
  4. Unknown id → `404`.
  5. `If-Match` does not match the current ETag → `412`. `*` matches any existing
     item. The comparison is strong: `W/"n"` never matches.
  6. Renaming onto another item's name → `409`.

  Success returns `200`, the item, and a bumped ETag (`"2"`, …).
- `DELETE /items/{id}` returns `204`, or `404` if missing.
- Validation (`422`, keys in `Errors`): `name` is required after trimming and
  at most 64 runes. `quantity` must be ≥ 0. Names are stored trimmed.
- Duplicate names are `409`. Comparison is case-insensitive after trimming.
- Create and update share the same body reader. Use `http.MaxBytesReader`, and
  `DisallowUnknownFields`.
- A method the mux does not route returns `405` with an `Allow` header
  (generated by the mux, so you do not write it).

Edge cases the tests pin down: an empty body is `400`, not `422`; a `POST` with
an invalid body and a duplicate name reports `422` first; `PUT` with a stale
ETag and an invalid body reports `422`, because validation precedes precondition.

## Part 2 · Middleware

`Middleware` is `func(http.Handler) http.Handler`. `Chain(h, a, b)` runs `a`
outermost, so a request enters `a`, then `b`, then `h`.

- `RequestID`: keep a valid incoming `X-Request-ID` (1–128 bytes, printable
  ASCII, no spaces). Otherwise generate 32 lowercase hex characters. Echo the
  value on the response and store it in the request context
  (`RequestIDFrom`). A missing value returns `""`.
- `AccessLog`: one record per request, with `method`, `path` (no query string),
  `status`, `bytes`, `duration`, and `request_id`. Level is `ERROR` for 5xx,
  `WARN` for 4xx, `INFO` otherwise. The response wrapper must keep
  `http.Flusher` working (`Flush`, `FlushError`, and `Unwrap` for
  `http.ResponseController`).
- `Recover`: a panic becomes a `500` problem if no status has been written.
  Otherwise the response is left as is. Log the panic value and
  `debug.Stack()` at `ERROR`. Re-panic `http.ErrAbortHandler` untouched so the
  server can abort the connection.
- `CORS(origins...)`: `*` allows any origin. Allowed responses set
  `Access-Control-Allow-Origin`, `Vary: Origin`, and
  `Access-Control-Expose-Headers: ETag, Location, X-Request-ID`. A preflight is
  `OPTIONS` with `Access-Control-Request-Method`. An allowed preflight returns
  `204` with `Access-Control-Allow-Methods`. A disallowed preflight returns
  `403`. Requests without `Origin` pass through untouched.
- `BearerAuth(token)`: accept `Authorization: Bearer <token>`. The scheme is
  case-insensitive. Compare SHA-256 digests with `subtle.ConstantTimeCompare`.
  An empty configured token denies everything. Failures return `401` with
  `WWW-Authenticate: Bearer`.

Recommended order: `RequestID → AccessLog → Recover → CORS → BearerAuth → handler`.
A preflight must succeed without credentials, which is why CORS sits outside auth.

## Part 3 · Pagination and filtering on `GET /items`

Query parameters: `limit` (1–100, default 20), `cursor`, `q` (case-insensitive
substring of the name), `min_qty` (≥ 0).

- Use keyset pagination. The cursor is opaque: base64url (no padding) of
  `seq:<n>`, where `<n>` is the creation sequence of the last item returned.
- `ParseListQuery` validates everything. Errors are `FieldErrors` with keys
  `limit`, `min_qty`, or `cursor`. `GET /items` maps them to `400`.
- The response always has `items`, never `null`. `next_cursor` is present only
  when another matching item exists. Use a look-ahead of one item.
- Deletes and inserts between pages must not cause duplicates or skips.
- `ListQuery.Values()` is the inverse of `ParseListQuery` for valid queries.

## Part 4 · HTTP client

`NewClient(baseURL, opts...)` accepts an absolute `http`/`https` URL and keeps
any base path (`https://host/api/v1` → `/api/v1/items`). The default
`http.Client` has a 10 s timeout. Defaults: 3 retries, backoff `base << attempt`
with `base = 100ms`.

- Idempotent methods (`GET`, `HEAD`, `OPTIONS`, `PUT`, `DELETE`) retry on `5xx`
  and transport errors. `POST` never retries on those. Any method retries on
  `429`. Retry only when the budget allows.
- `Retry-After` (seconds or HTTP-date, parsed by `ParseRetryAfter`) overrides
  the computed backoff. Context cancellation during a wait returns the context
  error (`errors.Is` works).
- Every response body is drained (bounded) and closed, so keep-alive
  connections are reused.
- A non-2xx response becomes `*APIError` with the decoded `Problem`.
- `UpdateItem` with an empty `etag` fails before any request is sent.
- `WithToken` sends a bearer token. `AuthTransport` clones the request, so the
  caller's request is never mutated. `MetricsTransport` observes every attempt,
  including transport failures (status `0`).
- `AllItems` follows `next_cursor` lazily and stops when the consumer breaks.
  A failure yields one `(zero, err)` pair and ends the sequence.

## Part 5 · NDJSON event streaming

`Broker` records events in memory and fans them out to subscribers.

- `Publish` assigns `Seq` = 1, 2, 3, …, appends to history, and never blocks.
  A subscriber whose 16-slot buffer is full is dropped: its channel is closed
  and the stream ends.
- `Handler()` serves `GET ?after=N` (`N` ≥ 0, default 0, invalid → `400`). It
  writes `Content-Type: application/x-ndjson`, flushes the headers, then replays
  history with `Seq > N`, then streams live events. One JSON object per line.
  It ends on client disconnect or `Close`.
- Snapshot history and subscribe under one lock, so no event is lost or
  duplicated between replay and live.
- `Close` ends every stream and makes later `Publish` calls no-ops.
- `StreamEvents` GETs the handler URL with `after`, decodes events lazily, and
  yields `(Event, nil)` per line. A clean end yields nothing more. Non-200
  responses, decode errors, and context cancellation yield one `(Event{}, err)`
  and end. Breaking out closes the body.

## Concepts exercised

- `ServeMux` 1.22 patterns, `PathValue`, automatic `405`/`Allow`
- `MaxBytesReader`, `DisallowUnknownFields`, problem-details errors
- Conditional requests: `ETag`, `If-Match`, `428`/`412`, strong vs weak comparison
- Middleware composition, `Chain` order, `ResponseController`, `http.ErrAbortHandler`
- `slog` structured logs and `LogAttrs` levels
- Opaque keyset cursors and look-ahead pagination
- `http.Client` retries, idempotency rules, `Retry-After`, context-aware sleeps
- Drain-and-close for connection reuse, `RoundTripper` wrapping, `Clone`
- `iter.Seq2` pull-style iteration with early break
- Flushing, NDJSON, and fan-out with bounded, non-blocking subscriber buffers

## Run

```sh
go test ./exercises/19_http_services/                       # everything (red at first)
go test ./exercises/19_http_services/ -run Part2 -v         # one part
go test ./exercises/19_http_services/ -count=3              # repeat for the timing tests
go run ./tools/solutions ./exercises/19_http_services/      # grade the reference answer
```

Run the suite with `-race` on Linux or macOS (the Windows toolchain here has no
cgo). Streaming, retries, and the broker are concurrent code, and `-race` is what
catches the mistakes.

## Stretch (untested)

- Add `If-None-Match` on `GET /items/{id}` and return `304`.
- Replace the `Broker` history with a bounded ring buffer and expose `Last-Event-ID`
  with SSE framing (`text/event-stream`) instead of NDJSON.
- Add `Retry-After` support on the server side for `429` with a token bucket.
- Make the client's retry jitter deterministic under `testing/synctest`.
- Serve the handler over HTTP/2 with `http.Server.Protocols` and compare connection reuse.
