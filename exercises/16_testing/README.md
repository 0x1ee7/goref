# 16 — Testing a billing package (inverted)

## Scenario

A billing team ships `billing/billing.go`: line items, invoices with tiered
discounts and tax, due dates, a payment service with retries and receipts, and a
CSV export. The package is done and is not yours to change. Your job is the
test suite that protects it.

You write `suite/suite_test.go` (package `suite_test`, black-box, stdlib only).
The grader does not trust your suite. It:

1. runs your suite against the real `billing.go` (it must pass and be fast), and
2. applies 20 deliberate bugs to a copy of `billing.go`, one at a time, and runs
   your suite against each copy. A suite that still passes a mutant is missing a
   test.

This is mutation testing. Your tests are graded by what they catch, not by how
many lines they cover.

## Mutant categories

The grader's mutants cover these kinds of bug. The exact edits are in
`grader_test.go`; read them only after you have written your own tests.

- rounding direction and off-by-one at a tier boundary
- a discount tier with the wrong rate, and a boundary that is exclusive
- tax computed on the wrong base, and a rejected 100% tax rate that is accepted
- validation bounds that are too loose (zero quantity, zero total)
- UTC conversion dropped, and a due-day comparison that flips
- backoff without a cap, a doubling step one attempt early
- retrying errors that must not be retried
- wrong log level, a receipt failure that fails the charge, and an off-by-one attempt count
- an error cause dropped from a wrapped error, and a cancellation that is ignored
- an idempotency key that is not the invoice ID
- a temp file that is never removed, and an iterator error that is swallowed

## Parts

### Part 1 — Line items, invoices, totals

Table-driven tests of `NewLineItem`, `NewInvoice`, and `Totals`. Cover invalid
inputs and their sentinel errors (`errors.Is`), the invoice owning a copy of its
lines, and the arithmetic. Add invariants that must hold for any valid invoice
(total equals subtotal minus discount plus tax) and a fuzz target that
generates line items.

Concepts: `t.Run` tables, `errors.Is` on sentinels, invariants, `testing.F` fuzzing.

### Part 2 — Discount tiers and rounding

Pin down the discount tiers at and around their boundaries and the rounding rule
of `roundDiv`. Then check the discount is monotone in the subtotal. Part 2's
grader test runs every mutant against your suite. A mutant that survives means a
gap in your tests.

Concepts: boundary-value analysis, property-style checks over a range, reading
the spec from the package doc comments.

### Part 3 — Dates and the clock

`DueDate`, `IssueDueDate`, and `IsOverdue` take time through the `Clock`
interface. Use a fixed clock, never `time.Now`. Test the UTC calendar-day rule
(an invoice is overdue only strictly after its due day), negative net days, and
that `IssueDueDate` uses the injected clock.

Concepts: injecting time through an interface, boundary days, UTC versus local.

### Part 4 — Payment service: retries and cancellation

`PaymentService.Charge` retries transient gateway errors with exponential backoff.
Test it with a scripted fake gateway and a fake `Sleeper` that records waits and
never sleeps. Cover: retry then success, exhaustion, `MaxAttempts == 1`, declines
and unknown errors that must not be retried, a context cancelled during backoff,
and the delay schedule. Also test that the default `RealSleeper` honours
cancellation.

Concepts: test doubles through interfaces, `testing/synctest`-free determinism,
`t.Context()`, asserting on wrapped errors.

### Part 5 — Logging and receipts

The service logs through `*slog.Logger`. Capture logs with a buffer-backed
handler and assert on levels and messages, not on timestamps. Check the receipt
content and that it is sent exactly once after retries, that a receipt failure
does not fail the charge, and that a decline is logged once.

Concepts: `slog.NewTextHandler` into a `bytes.Buffer`, checking levels, testing
side effects through a fake `Notifier`.

### Part 6 — CSV export

`ExportInvoicesCSV` writes atomically: a temp file, sync, and rename. Test the
header and row values with `encoding/csv`. Check that a successful export
replaces the old file, that an iterator error leaves the previous file intact
and removes the temp file, and that a missing directory is an error. Use
`t.TempDir`.

Concepts: `iter.Seq2` as test input, filesystem effects in `t.TempDir`,
atomic replacement.

## Run

```sh
go test ./exercises/16_testing/ -v -count=1           # grades your suite
go test ./exercises/16_testing/ -run Part2 -v          # just the mutation gate
go run ./tools/solutions ./exercises/16_testing/       # reference suite, must pass
```

Note: `go run ./tools/solutions ./exercises/16_testing/...` (the recursive form)
hides `billing/billing.go` from the overlay, because there is no solution copy.
Use the non-recursive path above.

The grader runs many `go test` processes. Expect it to take about 15 seconds.
Run `go test` with `-race` on Linux or macOS. It is not available on this
Windows setup.

## Stretch (untested)

- Add a 21st mutant that only your own tests could catch, then check that your suite kills it.
- Make the suite stop depending on the wall clock entirely (the grader already
  does not), and prove it with `testing/synctest`.
- Replace the hand-written fakes with a generated one (`go generate`) and compare readability.
- Measure coverage with `go test -coverprofile`. Then argue why a 100% coverage
  suite can still let the mutants through.
