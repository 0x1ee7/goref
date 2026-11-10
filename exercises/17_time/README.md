# 17 · Time

A scheduling and billing backend. Timestamps arrive from logs, clients, and
payment providers in mixed formats. Meetings recur in local time across DST.
Bookings span time zones. Invoices bill on month ends. Business-day SLAs skip
weekends and holidays. Every "now" is injected, and nothing depends on the
machine's local zone.

Implement the package in the stub files (`zone.go`, `parse.go`, `arith.go`,
`humanize.go`, `calendar.go`, `meeting.go`, `slots.go`, `clock.go`, `poll.go`).
The tests and this README are the spec. Errors wrap the sentinels in
`errors.go` with `%w`, so tests use `errors.Is`.

Two rules apply throughout:
- Compare instants with `Equal`, `Before`, or `After`. Use `==` only after
  `Canonical`. Merge and subtract in UTC or with `Sub`, never on wall fields.
- Calendar dates are `time.Time` values at UTC midnight. Calendar functions read
  only the wall-clock date of their argument, in that argument's own zone.

## Part 1: instants, zones, and the DST policy

- `Canonical(t time.Time) time.Time`: removes the monotonic reading and converts
  to UTC. The result represents the same instant, is `==`-comparable, and is
  safe as a map key. `Canonical(t).Equal(t)` always holds.
- `LoadZone(name string) (*time.Location, error)`: loads an IANA zone. `""` and
  `"Local"` are rejected because they depend on the machine. Unknown names and
  path traversal (`"../Europe/Berlin"`) wrap `ErrUnknownZone`. The package
  imports `time/tzdata`, so this works on Windows without a system zoneinfo.
- `type GapPolicy`: `GapShiftForward` (the zero value) or `GapSkip`.
- `type OverlapPolicy`: `OverlapEarlier` (the zero value), `OverlapLater`, or
  `OverlapSkip`.
- `Localize(wall time.Time, loc *time.Location, gap GapPolicy, overlap OverlapPolicy) (time.Time, error)`:
  finds the instant whose wall clock in `loc` equals `wall`'s fields. The zone of
  `wall` is ignored.
  - A normal time returns its single instant.
  - A gap time (02:00–02:59 on the spring-forward day in Berlin) returns the
    instant the clock jumps to under `GapShiftForward`. 02:30 becomes 03:30 CEST.
    Under `GapSkip` it returns `ErrNonexistent`.
  - An overlap time (02:00–02:59 on the fall-back day) returns the earlier or
    later occurrence, or `ErrAmbiguous` under `OverlapSkip`.
  - A nil `loc` or an unknown policy value returns `ErrInvalid`.

Go's `time.Date` also resolves gaps and overlaps, but its documentation only
promises that the result is "correct in one of the two zones involved" and does
not say which. Test `TestPart1TimeDateVersusPolicy` checks both behaviours. Use
an explicit policy whenever the answer matters.

## Part 2: parsing heterogeneous timestamps into UTC

- `ParseTimestamp(s string) (time.Time, error)`: returns UTC. Accepted forms:
  - RFC 3339 / ISO 8601 with a mandatory offset or `Z`. Fractional seconds are
    allowed.
  - RFC 1123Z (RFC 2822): `"Sun, 29 Mar 2026 12:00:00 +0200"`. The weekday must
    match the date. Go's parser does not check it, so this package does.
  - A decimal integer epoch, passed to `FromEpoch`.
  - Surrounding whitespace is trimmed. Naive, date-only, fractional-epoch, and
    other malformed input wraps `ErrInvalidTimestamp`. An offset of `-0000` or
    `-00:00` means the offset is unknown and wraps `ErrUnknownOffset`.
- `FromEpoch(n int64) time.Time`: a magnitude of at least `1e11` means
  milliseconds. Smaller values are seconds. The result is UTC.
- `EpochMilli(t time.Time) int64`: whole milliseconds since the epoch, rounded
  toward the past. `Duration.Milliseconds` truncates toward zero instead.

## Part 3: calendar versus elapsed arithmetic, Duration overflow, humanize

- `AddCalendarDays(t time.Time, days int) time.Time`: keeps the wall-clock time
  and zone, and resolves the result with `GapShiftForward` and `OverlapEarlier`.
  `t.Add(24*time.Hour)` moves by elapsed time instead. Across DST the two
  differ by an hour.
- `ElapsedBetween(start, end time.Time) (time.Duration, error)`: `end - start`.
  Spans beyond about 292 years return `ErrOverflow`, where `Time.Sub` would
  saturate silently.
- `SafeAdd(a, b time.Duration) (time.Duration, error)`: returns `ErrOverflow`
  instead of wrapping.
- `Humanize(when, now time.Time) string`: `now` is injected.
  - Under 45 s: `"just now"`.
  - Under 1 h: minutes, at least 1: `"1 minute ago"`, `"in 59 minutes"`.
  - Under 24 h: hours. Under 30 days: days.
  - Otherwise `"on YYYY-MM-DD"`, using the date of `when` in its own zone.
  - Units are singular for 1.

## Part 4: calendars, billing dates, business days

- `DaysInMonth(year int, month time.Month) (int, error)`: leap-year aware.
  Months outside 1..12 return `ErrInvalid`.
- `AddMonths(d time.Time, months int) time.Time`: clamps the day to the target
  month's length. `time.Time.AddDate` instead normalizes Jan 31 + 1 month to
  March 3. Negative values are allowed.
- `BillingDates(anchor time.Time, count int) ([]time.Time, error)`: each date is
  computed from the anchor, so clamping never drifts. Anchor Jan 31 yields Jan 31,
  Feb 28, Mar 31, Apr 30. A negative count returns `ErrInvalid`.
- `BusinessCalendar`, built by `NewBusinessCalendar(holidays []time.Time, weekend ...time.Weekday) (*BusinessCalendar, error)`:
  - The default weekend is Saturday and Sunday.
  - A weekday outside Sunday..Saturday is `ErrInvalid`. A weekend covering all
    seven days is also `ErrInvalid`, because `AddBusinessDays` would never
    terminate.
  - `IsBusinessDay(d) bool`: not a weekend day, not a holiday.
  - `AddBusinessDays(d, n) time.Time`: steps `n` business days forward, or back
    if `n` is negative. Each step lands on a business day, so a forward step from
    a weekend goes to the next business day. The result is UTC midnight, even
    when `n == 0`.
  - `BusinessDaysBetween(start, end) int`: counts the business days in
    `(start, end]`. If `end < start`, returns the negated count of `(end, start]`.

## Part 5: recurring meetings with an explicit DST policy

- `RecurrenceRule{Start, Weekday, Hour, Minute, Location, Gap, Overlap}`. The
  zero `Gap` and `Overlap` values are `GapShiftForward` and `OverlapEarlier`.
- `(RecurrenceRule) Occurrences(count int) ([]time.Time, error)`:
  - Returns the next `count` occurrences on `Weekday` at `Hour:Minute` in
    `Location`, on or after `Start`'s date, in ascending order.
  - Occurrences dropped by `GapSkip` or `OverlapSkip` do not count toward
    `count`.
  - Invalid input returns `ErrInvalid`: negative count, nil `Location`, weekday
    outside 0..6, hour or minute out of range, unknown policy.

## Part 6: booking slots across time zones

- `TimeSlot{Start, End time.Time}`: half-open `[Start, End)`.
  `NewTimeSlot` returns `ErrInvalid` unless `End` is strictly after `Start`.
- `(TimeSlot) Overlaps(o TimeSlot) bool`: true when the slots share an instant.
  Touching slots (`s.End == o.Start`) do not overlap. Compare by instant, so
  slots in different zones work.
- `MergeSlots(slots []TimeSlot) ([]TimeSlot, error)`: validates every slot,
  returns them sorted by start with overlapping or touching slots combined, in
  UTC. The input slice is not modified, even though sorting is the natural
  implementation. An invalid literal returns `ErrInvalid`.
- `TotalBooked(slots []TimeSlot) (time.Duration, error)`: covered time after
  merging. Overlapped time counts once. Overflow returns `ErrOverflow`.

## Part 7: injected clocks, a monotonic stopwatch, and timers

- `Clock` interface: `Now() time.Time`. `SystemClock{}` reads the real clock,
  including its monotonic reading.
- `Stopwatch{Clock Clock}`: the zero value is ready to use and reads
  `SystemClock` when `Clock` is nil.
  - `Start() error`: `ErrAlreadyRunning` if already running.
  - `Stop() (time.Duration, error)`: returns the time since `Start`.
    `ErrNotRunning` if not running.
  - `Elapsed() time.Duration`: the live reading while running, the value from
    the last `Stop` after stopping, and 0 before the first `Start`.
- `Poll(ctx context.Context, interval time.Duration, fn func(ctx context.Context, tick time.Time) error) error`:
  - Calls `fn` on each tick, never concurrently with itself.
  - Ticks that arrive during a slow `fn` are dropped rather than queued. This is
    Go 1.23 `Ticker` behaviour, and it prevents bursts.
  - An `fn` error stops the loop and is returned unchanged.
  - When `ctx` is done, returns `ctx.Err()`.
  - A non-positive interval or a nil `fn` returns `ErrInvalid` instead of
    panicking. `time.NewTicker` panics on a non-positive interval.

Timer tests use `testing/synctest`, so time advances deterministically with no
real sleeping.

## Concepts exercised

- `time.Time`: wall clock, monotonic reading, `Round(0)`, `Equal` vs `==`,
  `Canonical` for keys
- `time.Location`, `LoadLocation`, `time/tzdata` on Windows
- DST gaps and overlaps, `time.Date` normalization, explicit policies
- `AddDate` normalization versus clamped month arithmetic
- `Duration` overflow, `Sub` saturation, `Milliseconds` truncation
- Reference layouts, RFC 3339 and RFC 1123Z parsing, epoch seconds versus milliseconds
- Injected `Clock`, zero-value-ready types, sentinel errors with `%w`
- Half-open intervals, interval merging across zones without mutating input
- Go 1.23 `Timer`/`Ticker` semantics, `testing/synctest`, context-driven loops

## Run

```
go test ./exercises/17_time/                          # your implementation
go test ./exercises/17_time/ -run Part2 -v            # one Part
go run ./tools/solutions ./exercises/17_time/         # reference implementation
```

On Linux or macOS, add `-race` to the test commands. Timer and poll tests run
inside `synctest` bubbles, so they stay deterministic and fast.

## Stretch (untested)

- Make `RecurrenceRule` accept a list of zones, so a second meeting in Tokyo
  can sit in a Berlin DST gap.
- Implement `FreeWindows(slots, dayStart, dayEnd)` over merged bookings.
- Load holiday calendars from an ICS feed, with a `BusinessCalendar` constructor
  that takes a `io.Reader`.
- Compare `time` with `github.com/golang-module/carbon` or `tzdata` releases for
  historical DST edge cases.
