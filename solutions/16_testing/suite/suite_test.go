// Package suite_test is the reference test suite for the billing package.
//
// It exercises billing through its exported API only, with hand-written fakes
// for every port (Gateway, Notifier, Sleeper, Clock) and a slog handler that
// writes to a buffer so log output can be asserted.
package suite_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"goref/exercises/16_testing/billing"
)

// frozen is the instant every test clock reads, unless a test says otherwise.
var frozen = time.Date(2026, 11, 10, 15, 4, 5, 0, time.UTC)

const basis = 10_000

// --- fakes -----------------------------------------------------------------

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

// fakeGateway returns the scripted errors in order (a nil entry succeeds), then
// succeeds with "tx-<n>". Every request is recorded.
type fakeGateway struct {
	script []error
	calls  []billing.ChargeRequest
}

func (g *fakeGateway) Charge(_ context.Context, req billing.ChargeRequest) (string, error) {
	g.calls = append(g.calls, req)
	n := len(g.calls)
	if n <= len(g.script) && g.script[n-1] != nil {
		return "", g.script[n-1]
	}
	return fmt.Sprintf("tx-%d", n), nil
}

type sentMail struct{ to, subject, body string }

type fakeNotifier struct {
	err  error
	sent []sentMail
}

func (n *fakeNotifier) Send(_ context.Context, to, subject, body string) error {
	n.sent = append(n.sent, sentMail{to, subject, body})
	return n.err
}

// fakeSleeper records each requested wait. If cancel is set it is called first,
// so a test can cancel the context during a backoff.
type fakeSleeper struct {
	waits  []time.Duration
	cancel context.CancelFunc
}

func (s *fakeSleeper) Sleep(ctx context.Context, d time.Duration) error {
	s.waits = append(s.waits, d)
	if s.cancel != nil {
		s.cancel()
	}
	return ctx.Err()
}

type rig struct {
	gw       *fakeGateway
	notifier *fakeNotifier
	sleeper  *fakeSleeper
	logs     *bytes.Buffer
	svc      *billing.PaymentService
}

func newRig(t testing.TB, now time.Time, policy billing.RetryPolicy, script ...error) rig {
	t.Helper()
	logs := &bytes.Buffer{}
	r := rig{gw: &fakeGateway{script: script}, notifier: &fakeNotifier{}, sleeper: &fakeSleeper{}, logs: logs}
	svc, err := billing.NewPaymentService(billing.Deps{
		Gateway:  r.gw,
		Notifier: r.notifier,
		Clock:    fixedClock{now},
		Sleeper:  r.sleeper,
		Logger:   slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}, policy)
	if err != nil {
		t.Fatalf("NewPaymentService: %v", err)
	}
	r.svc = svc
	return r
}

// --- helpers ---------------------------------------------------------------

func mustLine(t testing.TB, sku string, unit int64, qty int) billing.LineItem {
	t.Helper()
	l, err := billing.NewLineItem(sku, unit, qty)
	if err != nil {
		t.Fatalf("NewLineItem(%q, %d, %d): %v", sku, unit, qty, err)
	}
	return l
}

func validSpec(t testing.TB) billing.InvoiceSpec {
	t.Helper()
	return billing.InvoiceSpec{
		ID:            "INV-1",
		CustomerEmail: "ada@example.com",
		Currency:      "USD",
		Lines:         []billing.LineItem{mustLine(t, "SKU-1", 600, 1)},
		IssuedOn:      frozen,
	}
}

func mustInvoice(t testing.TB, spec billing.InvoiceSpec) billing.Invoice {
	t.Helper()
	inv, err := billing.NewInvoice(spec)
	if err != nil {
		t.Fatalf("NewInvoice: %v", err)
	}
	return inv
}

// invoiceWith returns a valid invoice with the given lines and tax rate.
func invoiceWith(t testing.TB, taxBPS int64, lines ...billing.LineItem) billing.Invoice {
	t.Helper()
	spec := validSpec(t)
	if len(lines) > 0 {
		spec.Lines = lines
	}
	spec.TaxBPS = taxBPS
	return mustInvoice(t, spec)
}

func checkErr(t testing.TB, err, sentinel error) {
	t.Helper()
	if sentinel == nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want errors.Is(%v)", err, sentinel)
	}
}

func checkTotals(t testing.TB, got, want billing.Totals) {
	t.Helper()
	if got != want {
		t.Errorf("Totals() = %+v, want %+v", got, want)
	}
}

// checkInvariants asserts the pricing rules that hold for any invoice: the
// discount rate is picked from the subtotal and both rounded amounts are within
// half a cent of their exact values.
func checkInvariants(t testing.TB, inv billing.Invoice, taxBPS int64) {
	t.Helper()
	tot := inv.Totals()
	var subtotal int64
	for _, l := range inv.Lines() {
		subtotal += l.TotalCents()
	}
	if tot.Subtotal != subtotal {
		t.Fatalf("Subtotal = %d, want %d", tot.Subtotal, subtotal)
	}
	rate := billing.DiscountRateBPS(subtotal)
	if abs(tot.Discount*basis-subtotal*rate) > basis/2 {
		t.Fatalf("Discount = %d for subtotal %d at %d bp: not the nearest cent", tot.Discount, subtotal, rate)
	}
	taxable := subtotal - tot.Discount
	if abs(tot.Tax*basis-taxable*taxBPS) > basis/2 {
		t.Fatalf("Tax = %d for taxable %d at %d bp: not the nearest cent", tot.Tax, taxable, taxBPS)
	}
	if tot.Total != taxable+tot.Tax {
		t.Fatalf("Total = %d, want taxable+tax = %d", tot.Total, taxable+tot.Tax)
	}
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func invoices(invs ...billing.Invoice) iter.Seq2[billing.Invoice, error] {
	return func(yield func(billing.Invoice, error) bool) {
		for _, inv := range invs {
			if !yield(inv, nil) {
				return
			}
		}
	}
}

func dirNames(t testing.TB, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	slices.Sort(names)
	return names
}

func readCSV(t testing.TB, path string) [][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return recs
}

// --- Part 1: invoices and totals -------------------------------------------

func TestPart1LineItemValidation(t *testing.T) {
	tests := []struct {
		name     string
		sku      string
		unit     int64
		qty      int
		wantErr  bool
		wantTotal int64
	}{
		{name: "ok", sku: "A", unit: 250, qty: 4, wantTotal: 1000},
		{name: "free item is allowed", sku: "A", unit: 0, qty: 1, wantTotal: 0},
		{name: "empty sku", sku: "", unit: 1, qty: 1, wantErr: true},
		{name: "negative price", sku: "A", unit: -1, qty: 1, wantErr: true},
		{name: "zero quantity", sku: "A", unit: 1, qty: 0, wantErr: true},
		{name: "negative quantity", sku: "A", unit: 1, qty: -2, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l, err := billing.NewLineItem(tt.sku, tt.unit, tt.qty)
			if tt.wantErr {
				checkErr(t, err, billing.ErrInvalidInvoice)
				return
			}
			checkErr(t, err, nil)
			if got := l.TotalCents(); got != tt.wantTotal {
				t.Errorf("TotalCents() = %d, want %d", got, tt.wantTotal)
			}
			if l.SKU() != tt.sku || l.UnitCents() != tt.unit || l.Quantity() != tt.qty {
				t.Errorf("accessors = (%q, %d, %d), want (%q, %d, %d)",
					l.SKU(), l.UnitCents(), l.Quantity(), tt.sku, tt.unit, tt.qty)
			}
		})
	}
}

func TestPart1InvoiceValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*billing.InvoiceSpec)
	}{
		{"empty id", func(s *billing.InvoiceSpec) { s.ID = "" }},
		{"email without @", func(s *billing.InvoiceSpec) { s.CustomerEmail = "ada.example.com" }},
		{"lower-case currency", func(s *billing.InvoiceSpec) { s.Currency = "usd" }},
		{"two-letter currency", func(s *billing.InvoiceSpec) { s.Currency = "US" }},
		{"four-letter currency", func(s *billing.InvoiceSpec) { s.Currency = "USDX" }},
		{"no lines", func(s *billing.InvoiceSpec) { s.Lines = nil }},
		{"negative tax", func(s *billing.InvoiceSpec) { s.TaxBPS = -1 }},
		{"tax of 100 percent", func(s *billing.InvoiceSpec) { s.TaxBPS = 10_000 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec(t)
			tt.mutate(&spec)
			_, err := billing.NewInvoice(spec)
			checkErr(t, err, billing.ErrInvalidInvoice)
		})
	}

	t.Run("tax just under 100 percent is allowed", func(t *testing.T) {
		spec := validSpec(t)
		spec.TaxBPS = 9_999
		mustInvoice(t, spec)
	})

	t.Run("invalid line is reported with the invoice id", func(t *testing.T) {
		spec := validSpec(t)
		spec.Lines = []billing.LineItem{{}} // zero LineItem has an empty SKU
		_, err := billing.NewInvoice(spec)
		checkErr(t, err, billing.ErrInvalidInvoice)
		if !strings.Contains(err.Error(), "INV-1") {
			t.Errorf("error %q does not name the invoice", err)
		}
	})
}

func TestPart1InvoiceOwnsItsLines(t *testing.T) {
	spec := validSpec(t)
	inv := mustInvoice(t, spec)

	// Mutating the caller's slice after construction must not change the invoice.
	spec.Lines[0] = mustLine(t, "OTHER", 9_999, 9)
	if got := inv.Lines()[0].SKU(); got != "SKU-1" {
		t.Errorf("after caller mutated spec.Lines, SKU = %q, want SKU-1", got)
	}

	// Mutating the slice returned by Lines() must not change the invoice either.
	lines := inv.Lines()
	lines[0] = mustLine(t, "OTHER", 9_999, 9)
	if got := inv.Lines()[0].SKU(); got != "SKU-1" {
		t.Errorf("after mutating Lines() result, SKU = %q, want SKU-1", got)
	}
}

func TestPart1IssuedOnIsUTCDay(t *testing.T) {
	est := time.FixedZone("EST", -5*60*60)
	spec := validSpec(t)
	spec.IssuedOn = time.Date(2026, 11, 10, 23, 30, 0, 0, est) // 2026-11-11 04:30 UTC
	inv := mustInvoice(t, spec)

	want := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	if got := inv.IssuedOn(); !got.Equal(want) || got.Location() != time.UTC {
		t.Errorf("IssuedOn() = %v, want %v in UTC", got, want)
	}
}

func TestPart1Totals(t *testing.T) {
	tests := []struct {
		name  string
		tax   int64
		lines []billing.LineItem
		want  billing.Totals
	}{
		{
			name:  "free line, no discount, no tax",
			lines: []billing.LineItem{mustLineT(0, 1)},
			want:  billing.Totals{Subtotal: 0, Discount: 0, Tax: 0, Total: 0},
		},
		{
			name:  "quantity multiplies the unit price",
			lines: []billing.LineItem{mustLineT(250, 4)},
			want:  billing.Totals{Subtotal: 1000, Total: 1000},
		},
		{
			name:  "two-percent tier with exact discount",
			lines: []billing.LineItem{mustLineT(10_000, 1)},
			want:  billing.Totals{Subtotal: 10_000, Discount: 200, Total: 9_800},
		},
		{
			name:  "tax is charged on the discounted amount",
			tax:   1_000,
			lines: []billing.LineItem{mustLineT(10_000, 1)},
			want:  billing.Totals{Subtotal: 10_000, Discount: 200, Tax: 980, Total: 10_780},
		},
		{
			name:  "discount tie 200.5 rounds up to 201",
			lines: []billing.LineItem{mustLineT(10_025, 1)},
			want:  billing.Totals{Subtotal: 10_025, Discount: 201, Total: 9_824},
		},
		{
			name:  "discount 200.48 rounds down to 200",
			lines: []billing.LineItem{mustLineT(10_024, 1)},
			want:  billing.Totals{Subtotal: 10_024, Discount: 200, Total: 9_824},
		},
		{
			name:  "tax tie 502.5 rounds up to 503",
			tax:   5_000,
			lines: []billing.LineItem{mustLineT(1_005, 1)},
			want:  billing.Totals{Subtotal: 1_005, Tax: 503, Total: 1_508},
		},
		{
			name:  "five-percent tier",
			lines: []billing.LineItem{mustLineT(60_000, 1)},
			want:  billing.Totals{Subtotal: 60_000, Discount: 3_000, Total: 57_000},
		},
		{
			name:  "ten-percent tier",
			lines: []billing.LineItem{mustLineT(100_000, 1)},
			want:  billing.Totals{Subtotal: 100_000, Discount: 10_000, Total: 90_000},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inv := invoiceWith(t, tt.tax, tt.lines...)
			checkTotals(t, inv.Totals(), tt.want)
		})
	}
}

// mustLineT builds a SKU-less test line; it fails the test through a fresh
// testing.TB-free path by panicking on invalid input, which tables never send.
func mustLineT(unit int64, qty int) billing.LineItem {
	l, err := billing.NewLineItem("SKU", unit, qty)
	if err != nil {
		panic(err)
	}
	return l
}

func TestPart1TotalsInvariants(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 500 {
		spec := validSpec(t)
		spec.Lines = nil
		for range 1 + rng.IntN(4) {
			spec.Lines = append(spec.Lines, mustLine(t, "SKU", int64(rng.IntN(50_000)), 1+rng.IntN(5)))
		}
		spec.TaxBPS = int64(rng.IntN(basis))
		checkInvariants(t, mustInvoice(t, spec), spec.TaxBPS)
	}
}

func FuzzPart1Totals(f *testing.F) {
	f.Add(int64(600), 1, uint16(0))
	f.Add(int64(10_025), 1, uint16(0))
	f.Add(int64(1_005), 1, uint16(5_000))
	f.Add(int64(100_000), 3, uint16(9_999))
	f.Fuzz(func(t *testing.T, unit int64, qty int, tax uint16) {
		if unit < 0 || unit > 1_000_000_000 || qty < 1 || qty > 1_000 || tax >= basis {
			t.Skip("outside the documented domain")
		}
		inv := invoiceWith(t, int64(tax), mustLine(t, "SKU", unit, qty))
		checkInvariants(t, inv, int64(tax))
	})
}

// --- Part 2: volume discount tiers -----------------------------------------

func TestPart2DiscountTiers(t *testing.T) {
	tests := []struct {
		subtotal int64
		want     int64
	}{
		{0, 0},
		{9_999, 0},
		{10_000, 200},
		{49_999, 200},
		{50_000, 500},
		{99_999, 500},
		{100_000, 1_000},
		{1_000_000_000, 1_000},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprint(tt.subtotal), func(t *testing.T) {
			if got := billing.DiscountRateBPS(tt.subtotal); got != tt.want {
				t.Errorf("DiscountRateBPS(%d) = %d, want %d", tt.subtotal, got, tt.want)
			}
		})
	}
}

func TestPart2DiscountIsMonotone(t *testing.T) {
	prev := int64(-1)
	for subtotal := int64(0); subtotal <= 200_000; subtotal += 37 {
		rate := billing.DiscountRateBPS(subtotal)
		if rate < prev {
			t.Fatalf("rate fell from %d to %d at subtotal %d", prev, rate, subtotal)
		}
		prev = rate
	}
}

// --- Part 3: dates ----------------------------------------------------------

func TestPart3DueDate(t *testing.T) {
	est := time.FixedZone("EST", -5*60*60)
	tests := []struct {
		name     string
		issuedOn time.Time
		netDays  int
		want     time.Time
	}{
		{"net 30 crosses a month", time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC), 30, time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)},
		{"net 0 is the issue day", time.Date(2026, 11, 10, 15, 0, 0, 0, time.UTC), 0, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)},
		{"leap day is counted", time.Date(2028, 2, 20, 0, 0, 0, 0, time.UTC), 10, time.Date(2028, 3, 1, 0, 0, 0, 0, time.UTC)},
		{"issue day is the UTC day", time.Date(2026, 11, 10, 23, 30, 0, 0, est), 0, time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := billing.DueDate(tt.issuedOn, tt.netDays)
			checkErr(t, err, nil)
			if !got.Equal(tt.want) {
				t.Errorf("DueDate = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPart3DueDateRejectsNegativeTerms(t *testing.T) {
	_, err := billing.DueDate(frozen, -1)
	checkErr(t, err, billing.ErrInvalidTerms)
}

func TestPart3IssueDueDateUsesClock(t *testing.T) {
	got, err := billing.IssueDueDate(fixedClock{frozen}, 14)
	checkErr(t, err, nil)
	if want := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("IssueDueDate = %v, want %v", got, want)
	}
}

func TestPart3IsOverdue(t *testing.T) {
	due := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"before the due day", time.Date(2026, 11, 9, 23, 59, 0, 0, time.UTC), false},
		{"on the due day, late", time.Date(2026, 11, 10, 23, 59, 0, 0, time.UTC), false},
		{"day after the due day", time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := billing.IsOverdue(fixedClock{tt.now}, due); got != tt.want {
				t.Errorf("IsOverdue = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPart3IsOverdueUsesUTCDay(t *testing.T) {
	// 23:30 at UTC-5 is already the next day in UTC, so the invoice is overdue.
	est := time.FixedZone("EST", -5*60*60)
	now := time.Date(2026, 11, 10, 23, 30, 0, 0, est)
	due := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	if !billing.IsOverdue(fixedClock{now}, due) {
		t.Errorf("IsOverdue at %v (UTC %v) against due %v = false, want true", now, now.UTC(), due)
	}
}

// --- Part 4: charging and retries ------------------------------------------

func TestPart4NewPaymentServiceValidation(t *testing.T) {
	good := func() billing.Deps {
		return billing.Deps{Gateway: &fakeGateway{}, Notifier: &fakeNotifier{}, Clock: fixedClock{frozen}, Sleeper: &fakeSleeper{}}
	}
	policy := billing.DefaultRetryPolicy()

	depTests := []struct {
		name   string
		mutate func(*billing.Deps)
	}{
		{"nil gateway", func(d *billing.Deps) { d.Gateway = nil }},
		{"nil notifier", func(d *billing.Deps) { d.Notifier = nil }},
		{"nil clock", func(d *billing.Deps) { d.Clock = nil }},
		{"nil sleeper", func(d *billing.Deps) { d.Sleeper = nil }},
	}
	for _, tt := range depTests {
		t.Run(tt.name, func(t *testing.T) {
			d := good()
			tt.mutate(&d)
			_, err := billing.NewPaymentService(d, policy)
			checkErr(t, err, billing.ErrInvalidConfig)
		})
	}

	policyTests := []struct {
		name string
		p    billing.RetryPolicy
	}{
		{"zero attempts", billing.RetryPolicy{MaxAttempts: 0, BaseDelay: time.Second, MaxDelay: time.Second}},
		{"negative base delay", billing.RetryPolicy{MaxAttempts: 1, BaseDelay: -1, MaxDelay: time.Second}},
		{"negative max delay", billing.RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, MaxDelay: -1}},
	}
	for _, tt := range policyTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := billing.NewPaymentService(good(), tt.p)
			checkErr(t, err, billing.ErrInvalidConfig)
		})
	}

	t.Run("nil logger falls back to the default", func(t *testing.T) {
		d := good()
		d.Logger = nil
		svc, err := billing.NewPaymentService(d, policy)
		checkErr(t, err, nil)
		if _, err := svc.Charge(context.Background(), invoiceWith(t, 0), "tok"); err != nil {
			t.Errorf("Charge with default logger: %v", err)
		}
	})
}

func TestPart4DefaultRetryPolicy(t *testing.T) {
	want := billing.RetryPolicy{MaxAttempts: 4, BaseDelay: time.Second, MaxDelay: 8 * time.Second}
	if got := billing.DefaultRetryPolicy(); got != want {
		t.Errorf("DefaultRetryPolicy() = %+v, want %+v", got, want)
	}
}

func TestPart4RetryDelay(t *testing.T) {
	tests := []struct {
		name    string
		p       billing.RetryPolicy
		attempt int
		want    time.Duration
	}{
		{"default first wait", billing.DefaultRetryPolicy(), 1, time.Second},
		{"default doubles", billing.DefaultRetryPolicy(), 2, 2 * time.Second},
		{"default doubles again", billing.DefaultRetryPolicy(), 3, 4 * time.Second},
		{"default reaches cap", billing.DefaultRetryPolicy(), 4, 8 * time.Second},
		{"default stays at cap", billing.DefaultRetryPolicy(), 5, 8 * time.Second},
		{"base above cap is capped", billing.RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Second, MaxDelay: 3 * time.Second}, 1, 3 * time.Second},
		{"zero base stays zero", billing.RetryPolicy{MaxAttempts: 9, BaseDelay: 0, MaxDelay: 5 * time.Second}, 5, 0},
		{"huge attempt does not overflow", billing.RetryPolicy{MaxAttempts: 1000, BaseDelay: time.Second, MaxDelay: time.Hour}, 1000, time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Delay(tt.attempt); got != tt.want {
				t.Errorf("Delay(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestPart4ChargeRequest(t *testing.T) {
	spec := validSpec(t)
	spec.ID = "INV-EUR"
	spec.Currency = "EUR"
	spec.Lines = []billing.LineItem{mustLine(t, "SKU-1", 600, 1)}
	spec.TaxBPS = 1_000 // tax 60 on 600, total 660
	inv := mustInvoice(t, spec)

	r := newRig(t, frozen, billing.DefaultRetryPolicy())
	txID, err := r.svc.Charge(context.Background(), inv, "tok_visa")
	checkErr(t, err, nil)
	if txID != "tx-1" {
		t.Errorf("transaction id = %q, want tx-1", txID)
	}
	want := billing.ChargeRequest{AmountCents: 660, Currency: "EUR", Token: "tok_visa", IdempotencyKey: "INV-EUR"}
	if len(r.gw.calls) != 1 || r.gw.calls[0] != want {
		t.Errorf("gateway calls = %+v, want exactly [%+v]", r.gw.calls, want)
	}
}

func TestPart4NonPositiveTotalNotCharged(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy())
	inv := invoiceWith(t, 0, mustLine(t, "FREE", 0, 1))
	_, err := r.svc.Charge(context.Background(), inv, "tok")
	checkErr(t, err, billing.ErrInvalidInvoice)
	if len(r.gw.calls) != 0 {
		t.Errorf("gateway called %d times for a zero-total invoice, want 0", len(r.gw.calls))
	}
}

func TestPart4RetriesTransientThenSucceeds(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy(),
		fmt.Errorf("503 from gateway: %w", billing.ErrTransient),
		billing.ErrTransient,
		nil,
	)
	txID, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, nil)
	if txID != "tx-3" {
		t.Errorf("transaction id = %q, want tx-3", txID)
	}
	if len(r.gw.calls) != 3 {
		t.Errorf("gateway calls = %d, want 3", len(r.gw.calls))
	}
	if want := []time.Duration{time.Second, 2 * time.Second}; !slices.Equal(r.sleeper.waits, want) {
		t.Errorf("waits = %v, want %v", r.sleeper.waits, want)
	}
}

func TestPart4ExhaustsAttempts(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy(),
		billing.ErrTransient, billing.ErrTransient, billing.ErrTransient, billing.ErrTransient, nil)
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, billing.ErrPaymentFailed)
	checkErr(t, err, billing.ErrTransient)
	if len(r.gw.calls) != 4 {
		t.Errorf("gateway calls = %d, want 4", len(r.gw.calls))
	}
	if want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}; !slices.Equal(r.sleeper.waits, want) {
		t.Errorf("waits = %v, want %v", r.sleeper.waits, want)
	}
}

func TestPart4MaxAttemptsOne(t *testing.T) {
	policy := billing.RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, MaxDelay: time.Second}
	r := newRig(t, frozen, policy, billing.ErrTransient, nil)
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, billing.ErrPaymentFailed)
	if len(r.gw.calls) != 1 || len(r.sleeper.waits) != 0 {
		t.Errorf("calls = %d, waits = %v; want 1 call and no waits", len(r.gw.calls), r.sleeper.waits)
	}
}

func TestPart4DeclineIsNotRetried(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy(),
		fmt.Errorf("issuer refused: %w", billing.ErrDeclined), nil)
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, billing.ErrDeclined)
	if errors.Is(err, billing.ErrPaymentFailed) {
		t.Errorf("decline reported as ErrPaymentFailed: %v", err)
	}
	if len(r.gw.calls) != 1 || len(r.sleeper.waits) != 0 {
		t.Errorf("calls = %d, waits = %v; want 1 call and no waits", len(r.gw.calls), r.sleeper.waits)
	}
}

func TestPart4UnknownErrorIsNotRetried(t *testing.T) {
	boom := errors.New("boom")
	r := newRig(t, frozen, billing.DefaultRetryPolicy(), boom, nil)
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, boom)
	if errors.Is(err, billing.ErrTransient) || errors.Is(err, billing.ErrPaymentFailed) {
		t.Errorf("unknown error classified as transient or exhausted: %v", err)
	}
	if len(r.gw.calls) != 1 {
		t.Errorf("gateway calls = %d, want 1", len(r.gw.calls))
	}
}

func TestPart4CancelDuringBackoffStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := newRig(t, frozen, billing.DefaultRetryPolicy(), billing.ErrTransient, nil)
	r.sleeper.cancel = cancel

	_, err := r.svc.Charge(ctx, invoiceWith(t, 0), "tok")
	checkErr(t, err, context.Canceled)
	if errors.Is(err, billing.ErrPaymentFailed) {
		t.Errorf("cancellation reported as ErrPaymentFailed: %v", err)
	}
	if len(r.gw.calls) != 1 {
		t.Errorf("gateway calls = %d after cancellation, want 1", len(r.gw.calls))
	}
}

func TestPart4RealSleeper(t *testing.T) {
	t.Run("zero duration returns immediately", func(t *testing.T) {
		checkErr(t, billing.RealSleeper{}.Sleep(context.Background(), 0), nil)
	})
	t.Run("short wait completes", func(t *testing.T) {
		checkErr(t, billing.RealSleeper{}.Sleep(context.Background(), 5*time.Millisecond), nil)
	})
	t.Run("cancelled context returns its error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		checkErr(t, billing.RealSleeper{}.Sleep(ctx, time.Hour), context.Canceled)
	})
}

// --- Part 5: receipts and logging ------------------------------------------

func TestPart5ReceiptContents(t *testing.T) {
	spec := validSpec(t)
	spec.ID = "INV-EUR"
	spec.Currency = "EUR"
	spec.TaxBPS = 1_000
	inv := mustInvoice(t, spec)

	r := newRig(t, frozen, billing.DefaultRetryPolicy())
	_, err := r.svc.Charge(context.Background(), inv, "tok")
	checkErr(t, err, nil)

	want := sentMail{
		to:      "ada@example.com",
		subject: "Receipt for invoice INV-EUR",
		body:    "Total: 6.60 EUR\nTransaction: tx-1\nCharged on: 2026-11-10\n",
	}
	if len(r.notifier.sent) != 1 || r.notifier.sent[0] != want {
		t.Errorf("sent = %+v, want exactly [%+v]", r.notifier.sent, want)
	}
}

func TestPart5ReceiptDateIsUTC(t *testing.T) {
	est := time.FixedZone("EST", -5*60*60)
	now := time.Date(2026, 11, 10, 23, 30, 0, 0, est) // 2026-11-11 in UTC
	r := newRig(t, now, billing.DefaultRetryPolicy())
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, nil)
	if len(r.notifier.sent) != 1 || !strings.Contains(r.notifier.sent[0].body, "Charged on: 2026-11-11\n") {
		t.Errorf("receipt = %+v, want Charged on: 2026-11-11", r.notifier.sent)
	}
}

func TestPart5OneReceiptAfterRetries(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy(), billing.ErrTransient, nil)
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, nil)
	if len(r.notifier.sent) != 1 {
		t.Errorf("receipts sent = %d, want 1", len(r.notifier.sent))
	}
}

func TestPart5ReceiptFailureDoesNotFailCharge(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy())
	r.notifier.err = errors.New("smtp down")
	txID, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, nil)
	if txID != "tx-1" {
		t.Errorf("transaction id = %q, want tx-1", txID)
	}
	out := r.logs.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "receipt not sent") {
		t.Errorf("log output lacks ERROR 'receipt not sent':\n%s", out)
	}
}

func TestPart5WarnsOnEachTransientFailure(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy(), billing.ErrTransient, billing.ErrTransient, nil)
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, nil)
	out := r.logs.String()
	if n := strings.Count(out, "level=WARN"); n != 2 {
		t.Errorf("WARN lines = %d, want 2:\n%s", n, out)
	}
	for _, attr := range []string{"attempt=1", "attempt=2", "max_attempts=4"} {
		if !strings.Contains(out, attr) {
			t.Errorf("log output lacks %q:\n%s", attr, out)
		}
	}
}

func TestPart5WarnsOnEveryExhaustedAttempt(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy(),
		billing.ErrTransient, billing.ErrTransient, billing.ErrTransient, billing.ErrTransient)
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, billing.ErrPaymentFailed)
	out := r.logs.String()
	if n := strings.Count(out, "level=WARN"); n != 4 {
		t.Errorf("WARN lines = %d, want 4:\n%s", n, out)
	}
	if strings.Contains(out, "level=ERROR") {
		t.Errorf("exhaustion logged at ERROR; per-attempt failures are WARN:\n%s", out)
	}
}

func TestPart5DeclineLoggedOnce(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy(), billing.ErrDeclined)
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, billing.ErrDeclined)
	out := r.logs.String()
	if n := strings.Count(out, "level=ERROR"); n != 1 || !strings.Contains(out, "payment declined") {
		t.Errorf("want one ERROR 'payment declined', got:\n%s", out)
	}
	if strings.Contains(out, "level=WARN") {
		t.Errorf("decline logged at WARN:\n%s", out)
	}
}

func TestPart5SuccessIsSilent(t *testing.T) {
	r := newRig(t, frozen, billing.DefaultRetryPolicy())
	_, err := r.svc.Charge(context.Background(), invoiceWith(t, 0), "tok")
	checkErr(t, err, nil)
	if r.logs.Len() != 0 {
		t.Errorf("success logged:\n%s", r.logs.String())
	}
}

// --- Part 6: formatting and CSV export -------------------------------------

func TestPart6FormatCents(t *testing.T) {
	tests := []struct {
		cents int64
		want  string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{100, "1.00"},
		{500, "5.00"},
		{123_456, "1234.56"},
	}
	for _, tt := range tests {
		if got := billing.FormatCents(tt.cents); got != tt.want {
			t.Errorf("FormatCents(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
}

func TestPart6ExportRows(t *testing.T) {
	inv1 := invoiceWith(t, 0, mustLine(t, "SKU-1", 600, 1))
	spec := validSpec(t)
	spec.ID = "INV-2"
	spec.Currency = "EUR"
	spec.TaxBPS = 1_000
	spec.Lines = []billing.LineItem{mustLine(t, "SKU-2", 10_000, 1)}
	inv2 := mustInvoice(t, spec)

	path := filepath.Join(t.TempDir(), "invoices.csv")
	rows, err := billing.ExportInvoicesCSV(path, invoices(inv1, inv2))
	checkErr(t, err, nil)
	if rows != 2 {
		t.Errorf("rows = %d, want 2", rows)
	}

	got := readCSV(t, path)
	want := [][]string{
		{"invoice_id", "customer_email", "currency", "issued_on", "subtotal", "discount", "tax", "total"},
		{"INV-1", "ada@example.com", "USD", "2026-11-10", "6.00", "0.00", "0.00", "6.00"},
		{"INV-2", "ada@example.com", "EUR", "2026-11-10", "100.00", "2.00", "9.80", "107.80"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("CSV rows =\n%v\nwant\n%v", got, want)
	}
	if !slices.Equal(billing.ExportFields, want[0]) {
		t.Errorf("ExportFields = %v, want %v", billing.ExportFields, want[0])
	}
}

func TestPart6ExportReplacesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invoices.csv")
	if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := billing.ExportInvoicesCSV(path, invoices(invoiceWith(t, 0)))
	checkErr(t, err, nil)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "stale") || !strings.Contains(string(data), "INV-1") {
		t.Errorf("file not replaced:\n%s", data)
	}
	if want := []string{"invoices.csv"}; !slices.Equal(dirNames(t, dir), want) {
		t.Errorf("directory = %v, want only %v (no temp files)", dirNames(t, dir), want)
	}
}

func TestPart6IteratorErrorKeepsOldFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invoices.csv")
	if err := os.WriteFile(path, []byte("previous\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	errSource := errors.New("database went away")
	seq := func(yield func(billing.Invoice, error) bool) {
		if !yield(invoiceWith(t, 0), nil) {
			return
		}
		yield(billing.Invoice{}, errSource)
	}

	rows, err := billing.ExportInvoicesCSV(path, seq)
	checkErr(t, err, errSource)
	if rows != 0 {
		t.Errorf("rows = %d on error, want 0", rows)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "previous\n" {
		t.Errorf("file changed after failed export: %q", data)
	}
	if want := []string{"invoices.csv"}; !slices.Equal(dirNames(t, dir), want) {
		t.Errorf("directory = %v, want only %v (temp file must be removed)", dirNames(t, dir), want)
	}
}

func TestPart6MissingDirectoryFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "invoices.csv")
	rows, err := billing.ExportInvoicesCSV(path, invoices(invoiceWith(t, 0)))
	if err == nil {
		t.Fatal("ExportInvoicesCSV into a missing directory succeeded")
	}
	if rows != 0 {
		t.Errorf("rows = %d on error, want 0", rows)
	}
}

func ExampleFormatCents() {
	fmt.Println(billing.FormatCents(123_456))
	// Output: 1234.56
}

func BenchmarkPart1InvoiceTotals(b *testing.B) {
	inv := invoiceWith(b, 1_000, mustLine(b, "SKU-1", 2_500, 7), mustLine(b, "SKU-2", 999, 3))
	for b.Loop() {
		_ = inv.Totals()
	}
}
