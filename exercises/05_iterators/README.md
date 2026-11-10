# 05 · Iterators

Go 1.23 gave the language range-over-func. An iterator is a plain function
`func(yield func(V) bool)`, named by `iter.Seq[V]` and `iter.Seq2[K, V]`.
This exercise builds the production patterns on top of them: lazy adapters,
resource-owning sources, paginated API clients, k-way merges, and the bridge
to channels.

Scenarios: a lazy log pipeline, a paginated API client, rolling windows over
metrics, a resource-owning reader that yields batches, a k-way merge of
sorted event streams, and a channel bridge for code that already speaks
channels.

## Rules of the road

- An iterator must stop when `yield` returns false. Ignoring that is the
  most common bug; every adapter here is tested against it.
- Pull-based code (`iter.Pull`) must call `stop` exactly once, including on
  paths that never drain the sequence.
- Lazy means lazy: a consumer that breaks after two items must cause at most
  the upstream work for those items, not the whole source.
- Error-producing sequences use `iter.Seq2[T, error]`. Yield the error as the
  final pair and stop.

## Part 1 · Stateless adapters

Implement in `adapters.go`:

- `Map[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U]`
- `Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T]`
- `Take[T any](seq iter.Seq[T], n int) iter.Seq[T]`. `n <= 0` yields nothing.
  Stop right after the n-th element; do not pull element n+1.
- `Chunk[T any](seq iter.Seq[T], size int) iter.Seq[[]T]`. Yield full chunks
  and a final short chunk. `size < 1` panics at construction (not at first
  iteration).
- `Window[T any](seq iter.Seq[T], size int) iter.Seq[[]T]`. Sliding window
  of the last `size` items. Each yielded slice is a fresh copy, so the
  consumer may keep it. `size < 1` panics at construction.
- `MovingAverage(seq iter.Seq[float64], n int) iter.Seq[float64]`. Average of
  the last `n` values, emitted once the window is full. Use a ring buffer and
  a running sum, O(1) per item. `n < 1` panics at construction.

Part 2 (same file):

- `Concat[T any](seqs ...iter.Seq[T]) iter.Seq[T]`. Lazy; later sequences
  are not started until earlier ones are exhausted.
- `Enumerate[T any](seq iter.Seq[T]) iter.Seq2[int, T]`. Indices start at 0.
- `SortedEntries[K cmp.Ordered, V any](m map[K]V) iter.Seq2[K, V]`.
  Deterministic key order. Sorting happens when iteration starts, not when
  the function is called.

Concepts: `iter.Seq`, `iter.Seq2`, early break, construction-time validation
versus iteration-time behaviour, `slices.Clone`, `slices.Sorted(maps.Keys)`.

## Part 3 · Pull-style consumers

Implement in `zip.go`:

- `Zip[A, B any](a iter.Seq[A], b iter.Seq[B]) iter.Seq2[A, B]`. Stops at
  the shorter sequence. Only pull `b` after `a` has produced a value.
  Release both pulled iterators on every exit path.
- `type Peekable[T any]` with `NewPeekable[T any](seq iter.Seq[T]) *Peekable[T]`.
  Methods:
  - `Peek() (T, bool)`: look at the next value without consuming it.
  - `Next() (T, bool)`: consume and return the next value.
  - `Close()`: stop the underlying iterator. Idempotent. After `Close`,
    `Peek` and `Next` return `(zero, false)`.

Concepts: `iter.Pull`, `iter.Pull2`, the `stop` contract, deferred cleanup,
idempotent `Close`, zero values as "no value".

## Part 4 · Streaming text

Implement in `lines.go`:

- `var ErrUnknownLevel` (sentinel).
- `type Entry struct { Lineno int; Level string; Message string }`.
- `Tokens(r io.Reader, split bufio.SplitFunc, maxTokenSize int) iter.Seq2[string, error]`.
  Wraps `bufio.Scanner`. A `maxTokenSize > 0` sets the scanner buffer limit.
  Scanner errors are yielded as the final pair.
- `Lines(r io.Reader, maxLineBytes int) iter.Seq2[string, error]`. `Tokens`
  with line splitting.
- `Entries(lines iter.Seq2[string, error]) iter.Seq2[Entry, error]`. Parse
  `LEVEL message` lines. `Lineno` counts physical lines, including blank
  lines. Blank lines are skipped. An unknown level yields
  `fmt.Errorf("line %d: %w", n, ErrUnknownLevel)` and stops.
  Upstream errors pass through.

Known levels: `DEBUG`, `INFO`, `WARN`, `ERROR`.

Concepts: `bufio.Scanner` limits, `SplitFunc`, lazy file reads, error
values with `%w` and `errors.Is`, error as final yield.

## Part 5 · Paginated APIs

Implement in `pagination.go`:

- `var ErrCursorLoop` (sentinel).
- `type Page[T any] struct { Items []T; NextCursor string }`.
- `type FetchFunc[T any] func(ctx context.Context, cursor string) (Page[T], error)`.
- `Pages[T any](ctx context.Context, fetch FetchFunc[T]) iter.Seq2[Page[T], error]`.
  The first request uses cursor `""`. An empty `NextCursor` ends the stream.
  - Check `ctx` before each fetch. Yield `ctx.Err()` if cancelled.
  - A fetch error is wrapped with the cursor and yielded as the final pair.
  - If a cursor repeats, yield `fmt.Errorf("cursor %q: %w", c, ErrCursorLoop)`.
- `Paginate[T any](ctx, fetch) iter.Seq2[T, error]`. Flatten pages into
  items. Built on `Pages`. Stops immediately when the consumer breaks.

Concepts: context cancellation, cursor loops, flattening nested sequences,
composing iterators, wrapping with `%w`.

## Part 6 · Resource-owning generators

Implement in `resource.go`:

- `var ErrBatchSize` (sentinel).
- `ReadBatches(open func() (io.ReadCloser, error), size int) iter.Seq2[[]string, error]`.
  Reads newline-separated records from the opened source and yields batches
  of up to `size` lines.
  - `size < 1` yields `ErrBatchSize` before calling `open`.
  - An open error is wrapped `"open source: %w"`.
  - The source is closed exactly once, however iteration ends: full read,
    break, or error.
  - A read error is wrapped `"read source: %w"`. The partial batch is
    discarded.
  - A close error is yielded as `"close source: %w"` after a complete read.
    On break or error, close errors are ignored.

Concepts: `defer` inside iterators, cleanup on early break, separating read
errors from close errors, zero-allocation batch reuse versus copying (the
contract is that each batch is owned by the consumer).

## Part 7 · K-way merge

Implement in `merge.go`:

- `Merge[T any](cmp func(a, b T) int, streams ...iter.Seq[T]) iter.Seq[T]`.
  Each input stream is sorted by `cmp`. Output is the sorted union.
  - Ties go to the earlier stream (stable across streams).
  - Each stream is pulled only as needed, and all are stopped when the merge
    ends, including on break.
  - Zero streams yield nothing.

Concepts: `iter.Pull` for many sources, heap-free min selection for small
k (a linear scan is fine), stability, deterministic tie-breaking, releasing
N coroutines correctly.

## Part 8 · Channel bridge

Implement in `channels.go`:

- `FromChannel[T any](ctx context.Context, ch <-chan T) iter.Seq2[T, error]`.
  Yields values from `ch` until it closes (`nil` error). If `ctx` is
  cancelled first, yield `ctx.Err()` and stop.
- `ToChannel[T any](ctx context.Context, seq iter.Seq[T]) <-chan T`. Runs
  `seq` in a goroutine and sends each value. The goroutine closes the
  channel when `seq` ends or `ctx` is cancelled. A cancelled context must
  stop the sequence (the `yield` returns false), and no goroutine may be left
  blocked on a send.

Concepts: bridging push and channel models, `select` with `ctx.Done()`,
goroutine ownership, closing channels from the producer side only, the cost
of goroutines compared with the zero-cost `yield` loop.

## Concepts exercised

- `iter.Seq` / `iter.Seq2`, range-over-func, early break, the `yield` contract
- `iter.Pull` / `iter.Pull2` and the mandatory `stop`
- Construction-time validation versus iteration-time behaviour
- Lazy evaluation and bounded upstream work
- `context` cancellation inside iterators
- Sentinel errors, `%w` wrapping, `errors.Is`, error as final yield
- `bufio.Scanner`, `io.ReadCloser` ownership, close-once semantics
- Generics with `cmp.Ordered` and `func(a, b T) int`
- Channel bridging and goroutine lifetime

## Run

```sh
go test ./exercises/05_iterators/
go test ./exercises/05_iterators/ -run Part2 -v
go run ./tools/solutions ./exercises/05_iterators/
```

On Linux and macOS, also run the tests with `-race`. Iterators and pulls are
easy to get wrong concurrently, and the race detector is the best check.

## Stretch (untested)

- Add `Tee` that duplicates one sequence to two consumers without buffering
  the whole stream.
- Make `Merge` use a heap (`container/heap`) and compare the cost at k=2,
  k=16, and k=256.
- Write a `Batch` adapter that flushes on either size or elapsed time,
  using `time.Ticker` and a goroutine (decide how the ticker is stopped).
- Replace `Pages` with a `FetchFunc` that retries with exponential backoff
  respecting `ctx`.
