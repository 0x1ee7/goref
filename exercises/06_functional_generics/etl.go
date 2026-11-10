package functional

import (
	"errors"
	"iter"
	"time"
)

// Row is one raw record from the sales export. A missing column reads as "".
type Row map[string]string

// Sale is a validated sales record.
type Sale struct {
	Region         string
	Product        string
	Units          int
	UnitPriceCents int64
	SoldOn         time.Time
}

// ErrInvalidAmount reports malformed or out-of-range money.
var ErrInvalidAmount = errors.New("invalid amount")

// ParseCents parses a non-negative dollar amount with at most two decimals into
// integer cents, without floats. "12.5" is 1250, "12" is 1200 and "0.01" is 1. A
// leading sign, an exponent, ".5", "12." and more than two decimals are all
// rejected.
func ParseCents(raw string) (int64, error) {
	return 0, nil
}

// ParseRow converts one raw row into a Sale. It trims fields, lowercases the
// region and rejects negative units.
func ParseRow(row Row) (Sale, error) {
	return Sale{}, nil
}

// CleanSales drops rows with a blank region or zero units, then parses the rest
// in input order. It reads rows once, so a generator works, and never modifies
// the rows. It returns the first bad row as an error that names its 1-based
// position. The result is never nil.
func CleanSales(rows iter.Seq[Row]) ([]Sale, error) {
	return nil, nil
}

// RevenueCents is units times unit price.
func RevenueCents(s Sale) int64 {
	return 0
}

// TotalsBy sums revenue per key(sale).
func TotalsBy(sales []Sale, key func(Sale) string) map[string]int64 {
	return nil
}

// Total is one ranked line of a report.
type Total struct {
	Key          string
	RevenueCents int64
}

// Ranked orders totals by revenue descending, then key ascending. Map iteration
// order is random, so the second key is what makes the result deterministic.
func Ranked(totals map[string]int64) []Total {
	return nil
}

// SortSales returns a copy of sales ordered by date, then region, then product.
// The sort is stable, so sales that tie on all three keep their input order. The
// input is not modified.
func SortSales(sales []Sale) []Sale {
	return nil
}

// DistinctRegions returns the sorted, unique regions in upper case.
func DistinctRegions(sales []Sale) []string {
	return nil
}

// FormatCents renders cents as currency with thousands separators, for example
// 1234567 becomes "$12,345.67". Negative amounts are an error.
func FormatCents(cents int64, currency string) (string, error) {
	return "", nil
}

// BuildReport produces one "region: $amount" line per region, ranked by revenue.
// Each step is a function from the previous step's value to the next. Steps that
// can fail return an error, which ends the pipeline early.
func BuildReport(rows iter.Seq[Row], currency string) ([]string, error) {
	return nil, nil
}
