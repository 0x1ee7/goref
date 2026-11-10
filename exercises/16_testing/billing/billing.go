// Package billing is the production code for exercise 16.
//
// It is provided complete and must not be edited: your deliverable is the test
// suite in ../suite. Money is int64 cents and every rounding step is explicit.
package billing

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Sentinel errors. Callers match them with errors.Is; wrapped messages add context.
var (
	// ErrInvalidConfig reports an unusable PaymentService configuration.
	ErrInvalidConfig = errors.New("billing: invalid configuration")
	// ErrInvalidInvoice reports a line, invoice or charge that breaks a business rule.
	ErrInvalidInvoice = errors.New("billing: invalid invoice")
	// ErrInvalidTerms reports unusable payment terms.
	ErrInvalidTerms = errors.New("billing: invalid payment terms")
	// ErrTransient marks a gateway failure that is safe to retry (timeout, 503).
	ErrTransient = errors.New("billing: transient gateway failure")
	// ErrDeclined marks an issuer refusal. It is never retried.
	ErrDeclined = errors.New("billing: payment declined")
	// ErrPaymentFailed reports that every attempt failed with a transient error.
	ErrPaymentFailed = errors.New("billing: payment failed")
)

// DefaultNetDays is the payment term used when none is specified.
const DefaultNetDays = 30

// basisPoints is the denominator of every rate: 10000 bp = 100%.
const basisPoints = 10_000

// discountTiers are checked from the top down. Thresholds are inclusive and in cents.
var discountTiers = [...]struct {
	minSubtotal int64
	bps         int64
}{
	{100_000, 1000}, // 1000.00 and up: 10%
	{50_000, 500},   // 500.00 and up: 5%
	{10_000, 200},   // 100.00 and up: 2%
}

// roundDiv returns num/den rounded to the nearest integer, ties away from zero.
// den must be positive.
func roundDiv(num, den int64) int64 {
	if num < 0 {
		return -roundDiv(-num, den)
	}
	return (2*num + den) / (2 * den)
}

// DiscountRateBPS returns the volume discount, in basis points, for a subtotal in cents.
func DiscountRateBPS(subtotal int64) int64 {
	for _, tier := range discountTiers {
		if subtotal >= tier.minSubtotal {
			return tier.bps
		}
	}
	return 0
}

// FormatCents renders a non-negative amount in cents as a two-decimal string: 500 -> "5.00".
func FormatCents(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// utcDate returns the UTC calendar day of t, as midnight UTC.
func utcDate(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// LineItem is one priced line on an invoice. Build it with NewLineItem.
type LineItem struct {
	sku       string
	unitCents int64
	quantity  int
}

// NewLineItem validates a line. The SKU must be non-empty, the unit price
// non-negative and the quantity at least 1.
func NewLineItem(sku string, unitCents int64, quantity int) (LineItem, error) {
	switch {
	case sku == "":
		return LineItem{}, fmt.Errorf("%w: empty sku", ErrInvalidInvoice)
	case unitCents < 0:
		return LineItem{}, fmt.Errorf("%w: sku %q has negative unit price %d", ErrInvalidInvoice, sku, unitCents)
	case quantity < 1:
		return LineItem{}, fmt.Errorf("%w: sku %q has quantity %d", ErrInvalidInvoice, sku, quantity)
	}
	return LineItem{sku: sku, unitCents: unitCents, quantity: quantity}, nil
}

// SKU returns the product code.
func (l LineItem) SKU() string { return l.sku }

// UnitCents returns the price of one unit in cents.
func (l LineItem) UnitCents() int64 { return l.unitCents }

// Quantity returns the number of units.
func (l LineItem) Quantity() int { return l.quantity }

// TotalCents returns unit price times quantity. Prices are whole cents, so no rounding is needed.
func (l LineItem) TotalCents() int64 { return l.unitCents * int64(l.quantity) }

// InvoiceSpec holds the inputs to NewInvoice.
type InvoiceSpec struct {
	// ID identifies the invoice and doubles as the gateway idempotency key.
	ID string
	// CustomerEmail receives the receipt. It must contain an '@'.
	CustomerEmail string
	// Currency is an ISO 4217 code: three upper-case ASCII letters.
	Currency string
	// Lines must contain at least one valid line.
	Lines []LineItem
	// IssuedOn's UTC calendar day is the issue date; the time of day is ignored.
	IssuedOn time.Time
	// TaxBPS is the tax rate in basis points: 0 <= TaxBPS < 10000, i.e. [0%, 100%).
	TaxBPS int64
}

// Invoice is a validated invoice. Build it with NewInvoice.
type Invoice struct {
	id            string
	customerEmail string
	currency      string
	lines         []LineItem
	issuedOn      time.Time
	taxBPS        int64
}

// NewInvoice validates spec and returns an Invoice that owns a copy of its lines.
func NewInvoice(spec InvoiceSpec) (Invoice, error) {
	switch {
	case spec.ID == "":
		return Invoice{}, fmt.Errorf("%w: empty invoice id", ErrInvalidInvoice)
	case !strings.Contains(spec.CustomerEmail, "@"):
		return Invoice{}, fmt.Errorf("%w: invoice %s: customer email %q", ErrInvalidInvoice, spec.ID, spec.CustomerEmail)
	case !validCurrency(spec.Currency):
		return Invoice{}, fmt.Errorf("%w: invoice %s: currency %q is not an ISO 4217 code", ErrInvalidInvoice, spec.ID, spec.Currency)
	case len(spec.Lines) == 0:
		return Invoice{}, fmt.Errorf("%w: invoice %s has no lines", ErrInvalidInvoice, spec.ID)
	case spec.TaxBPS < 0 || spec.TaxBPS >= basisPoints:
		return Invoice{}, fmt.Errorf("%w: invoice %s: tax rate %d bp must be in [0, 10000)", ErrInvalidInvoice, spec.ID, spec.TaxBPS)
	}
	lines := slices.Clone(spec.Lines)
	for _, l := range lines {
		if _, err := NewLineItem(l.sku, l.unitCents, l.quantity); err != nil {
			return Invoice{}, fmt.Errorf("invoice %s: %w", spec.ID, err)
		}
	}
	return Invoice{
		id:            spec.ID,
		customerEmail: spec.CustomerEmail,
		currency:      spec.Currency,
		lines:         lines,
		issuedOn:      utcDate(spec.IssuedOn),
		taxBPS:        spec.TaxBPS,
	}, nil
}

func validCurrency(code string) bool {
	if len(code) != 3 {
		return false
	}
	for i := range len(code) {
		if code[i] < 'A' || code[i] > 'Z' {
			return false
		}
	}
	return true
}

// ID returns the invoice identifier.
func (inv Invoice) ID() string { return inv.id }

// CustomerEmail returns the address receipts are sent to.
func (inv Invoice) CustomerEmail() string { return inv.customerEmail }

// Currency returns the ISO 4217 code the invoice is priced in.
func (inv Invoice) Currency() string { return inv.currency }

// IssuedOn returns the issue date as midnight UTC.
func (inv Invoice) IssuedOn() time.Time { return inv.issuedOn }

// Lines returns a copy of the invoice's lines.
func (inv Invoice) Lines() []LineItem { return slices.Clone(inv.lines) }

// Totals is the priced breakdown of an invoice, in cents.
type Totals struct {
	Subtotal int64
	Discount int64
	Tax      int64
	Total    int64
}

// Totals prices the invoice. The volume discount is chosen from the subtotal and
// tax is charged on the discounted amount. Both round half away from zero.
func (inv Invoice) Totals() Totals {
	var subtotal int64
	for _, l := range inv.lines {
		subtotal += l.TotalCents()
	}
	discount := roundDiv(subtotal*DiscountRateBPS(subtotal), basisPoints)
	taxable := subtotal - discount
	tax := roundDiv(taxable*inv.taxBPS, basisPoints)
	return Totals{Subtotal: subtotal, Discount: discount, Tax: tax, Total: taxable + tax}
}

// Clock supplies the current time so tests can freeze it.
type Clock interface {
	Now() time.Time
}

// SystemClock is the production Clock.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// DueDate returns the due date for an invoice issued on the UTC calendar day of
// issuedOn, with netDays of payment terms. netDays must not be negative.
func DueDate(issuedOn time.Time, netDays int) (time.Time, error) {
	if netDays < 0 {
		return time.Time{}, fmt.Errorf("%w: net days %d", ErrInvalidTerms, netDays)
	}
	return utcDate(issuedOn).AddDate(0, 0, netDays), nil
}

// IssueDueDate is DueDate for an invoice issued today, where today is the UTC
// calendar day of clock.Now().
func IssueDueDate(clock Clock, netDays int) (time.Time, error) {
	return DueDate(clock.Now(), netDays)
}

// IsOverdue reports whether the UTC calendar day of clock.Now() is strictly after
// the UTC calendar day of due. An invoice is not overdue on its due day.
func IsOverdue(clock Clock, due time.Time) bool {
	return utcDate(clock.Now()).After(utcDate(due))
}

// ChargeRequest is what PaymentService asks a Gateway to do.
type ChargeRequest struct {
	AmountCents    int64
	Currency       string
	Token          string
	IdempotencyKey string
}

// Gateway charges a payment token and returns the transaction id. Implementations
// wrap ErrTransient for retryable failures and ErrDeclined for issuer refusals.
type Gateway interface {
	Charge(ctx context.Context, req ChargeRequest) (string, error)
}

// Notifier sends customer messages such as receipts.
type Notifier interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Sleeper pauses for d. It returns ctx.Err() if ctx is done before d elapses.
type Sleeper interface {
	Sleep(ctx context.Context, d time.Duration) error
}

// RealSleeper is the production Sleeper.
type RealSleeper struct{}

// Sleep blocks for d or until ctx is done.
func (RealSleeper) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// RetryPolicy controls how transient gateway failures are retried.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts, including the first. Must be at least 1.
	MaxAttempts int
	// BaseDelay is the wait after the first failed attempt. Must not be negative.
	BaseDelay time.Duration
	// MaxDelay caps every single wait. Must not be negative.
	MaxDelay time.Duration
}

// DefaultRetryPolicy is four attempts with waits of 1s, 2s and 4s, capped at 8s.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 4, BaseDelay: time.Second, MaxDelay: 8 * time.Second}
}

// Delay returns the wait after the attempt-th failed attempt (1-based):
// BaseDelay doubled once per earlier attempt, capped at MaxDelay.
func (p RetryPolicy) Delay(attempt int) time.Duration {
	d := p.BaseDelay
	for i := 1; i < attempt && d > 0 && d < p.MaxDelay; i++ {
		d *= 2
	}
	return min(d, p.MaxDelay)
}

func (p RetryPolicy) validate() error {
	switch {
	case p.MaxAttempts < 1:
		return fmt.Errorf("%w: max attempts %d must be at least 1", ErrInvalidConfig, p.MaxAttempts)
	case p.BaseDelay < 0 || p.MaxDelay < 0:
		return fmt.Errorf("%w: delays must not be negative", ErrInvalidConfig)
	}
	return nil
}

// Deps are the ports a PaymentService depends on. Gateway, Notifier, Clock and
// Sleeper are required. Logger defaults to slog.Default().
type Deps struct {
	Gateway  Gateway
	Notifier Notifier
	Clock    Clock
	Sleeper  Sleeper
	Logger   *slog.Logger
}

// PaymentService charges invoices through a Gateway, retries transient failures
// with exponential backoff and sends receipts through a Notifier.
type PaymentService struct {
	deps   Deps
	policy RetryPolicy
}

// NewPaymentService validates the dependencies and policy.
func NewPaymentService(deps Deps, policy RetryPolicy) (*PaymentService, error) {
	if deps.Gateway == nil || deps.Notifier == nil || deps.Clock == nil || deps.Sleeper == nil {
		return nil, fmt.Errorf("%w: gateway, notifier, clock and sleeper are required", ErrInvalidConfig)
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &PaymentService{deps: deps, policy: policy}, nil
}

// Charge charges the invoice total with token and returns the gateway transaction id.
//
// A non-positive total is rejected before the gateway is called. Transient
// failures are retried under the RetryPolicy, and each one is logged at WARN.
// When the attempts run out the error wraps both ErrPaymentFailed and the last
// transient error. A decline (ErrDeclined) and any other error are returned at
// once. Cancelling ctx during a backoff stops the retries. After a successful
// charge a receipt is sent. A receipt failure is logged at ERROR and never fails
// the charge.
func (s *PaymentService) Charge(ctx context.Context, inv Invoice, token string) (string, error) {
	totals := inv.Totals()
	if totals.Total <= 0 {
		return "", fmt.Errorf("%w: invoice %s total %s is not positive", ErrInvalidInvoice, inv.id, FormatCents(totals.Total))
	}
	req := ChargeRequest{AmountCents: totals.Total, Currency: inv.currency, Token: token, IdempotencyKey: inv.id}
	for attempt := 1; ; attempt++ {
		txID, err := s.deps.Gateway.Charge(ctx, req)
		if err == nil {
			body := fmt.Sprintf("Total: %s %s\nTransaction: %s\nCharged on: %s\n",
				FormatCents(totals.Total), inv.currency, txID, s.deps.Clock.Now().UTC().Format(time.DateOnly))
			if err := s.deps.Notifier.Send(ctx, inv.customerEmail, "Receipt for invoice "+inv.id, body); err != nil {
				s.deps.Logger.ErrorContext(ctx, "receipt not sent", "invoice_id", inv.id, "transaction_id", txID, "err", err)
			}
			return txID, nil
		}
		switch {
		case errors.Is(err, ErrDeclined):
			s.deps.Logger.ErrorContext(ctx, "payment declined", "invoice_id", inv.id, "err", err)
			return "", fmt.Errorf("charge invoice %s: %w", inv.id, err)
		case !errors.Is(err, ErrTransient):
			return "", fmt.Errorf("charge invoice %s: %w", inv.id, err)
		}
		s.deps.Logger.WarnContext(ctx, "transient gateway failure", "invoice_id", inv.id,
			"attempt", attempt, "max_attempts", s.policy.MaxAttempts, "err", err)
		if attempt >= s.policy.MaxAttempts {
			return "", fmt.Errorf("%w: invoice %s after %d attempts: %w", ErrPaymentFailed, inv.id, attempt, err)
		}
		if err := s.deps.Sleeper.Sleep(ctx, s.policy.Delay(attempt)); err != nil {
			return "", fmt.Errorf("charge invoice %s: backoff interrupted: %w", inv.id, err)
		}
	}
}

// ExportFields is the CSV header, in column order.
var ExportFields = []string{"invoice_id", "customer_email", "currency", "issued_on", "subtotal", "discount", "tax", "total"}

// ExportInvoicesCSV writes one CSV row per invoice to path and returns the row count.
//
// The file is written atomically. Rows go to a temporary file in the same
// directory, which is synced and renamed over path only after every invoice has
// been written. If invs yields an error, that error is returned, path keeps its
// previous contents, and no temporary file is left behind.
func ExportInvoicesCSV(path string, invs iter.Seq2[Invoice, error]) (int, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return 0, fmt.Errorf("export csv: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // a no-op once the rename has moved the file
	rows, err := writeCSV(tmp, invs)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return 0, fmt.Errorf("export csv: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return 0, fmt.Errorf("export csv: %w", err)
	}
	return rows, nil
}

func writeCSV(w io.Writer, invs iter.Seq2[Invoice, error]) (int, error) {
	cw := csv.NewWriter(w)
	if err := cw.Write(ExportFields); err != nil {
		return 0, err
	}
	rows := 0
	for inv, err := range invs {
		if err != nil {
			return 0, fmt.Errorf("next invoice: %w", err)
		}
		t := inv.Totals()
		record := []string{
			inv.id,
			inv.customerEmail,
			inv.currency,
			inv.issuedOn.Format(time.DateOnly),
			FormatCents(t.Subtotal),
			FormatCents(t.Discount),
			FormatCents(t.Tax),
			FormatCents(t.Total),
		}
		if err := cw.Write(record); err != nil {
			return 0, err
		}
		rows++
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return 0, err
	}
	return rows, nil
}
