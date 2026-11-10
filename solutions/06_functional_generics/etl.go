package functional

import (
	"cmp"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
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
	bad := fmt.Errorf("%w: %q", ErrInvalidAmount, raw)
	whole, frac, hasFrac := strings.Cut(raw, ".")
	if !isDigits(whole) {
		return 0, bad
	}
	var cents int64
	if hasFrac {
		if len(frac) < 1 || len(frac) > 2 || !isDigits(frac) {
			return 0, bad
		}
		if len(frac) == 1 {
			frac += "0"
		}
		cents, _ = strconv.ParseInt(frac, 10, 64) // two digits, cannot fail
	}
	dollars, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || dollars > (math.MaxInt64-cents)/100 {
		return 0, bad
	}
	return dollars*100 + cents, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ParseRow converts one raw row into a Sale. It trims fields, lowercases the
// region and rejects negative units.
func ParseRow(row Row) (Sale, error) {
	units, err := strconv.Atoi(strings.TrimSpace(row["units"]))
	if err != nil {
		return Sale{}, fmt.Errorf("units: %w", err)
	}
	if units < 0 {
		return Sale{}, fmt.Errorf("units: must not be negative, got %d", units)
	}
	price, err := ParseCents(strings.TrimSpace(row["unit_price"]))
	if err != nil {
		return Sale{}, fmt.Errorf("unit_price: %w", err)
	}
	soldOn, err := time.Parse(time.DateOnly, strings.TrimSpace(row["sold_on"]))
	if err != nil {
		return Sale{}, fmt.Errorf("sold_on: %w", err)
	}
	return Sale{
		Region:         strings.ToLower(strings.TrimSpace(row["region"])),
		Product:        strings.TrimSpace(row["product"]),
		Units:          units,
		UnitPriceCents: price,
		SoldOn:         soldOn,
	}, nil
}

// CleanSales drops rows with a blank region or zero units, then parses the rest
// in input order. It reads rows once, so a generator works, and never modifies
// the rows. It returns the first bad row as an error that names its 1-based
// position. The result is never nil.
//
// This is a plain loop rather than a Filter/Map chain on purpose. The row number
// must come from the original position, and the early return on error is clearer
// than threading an error through a pipeline.
func CleanSales(rows iter.Seq[Row]) ([]Sale, error) {
	sales := []Sale{}
	for i, row := range Enumerate(rows) {
		if strings.TrimSpace(row["region"]) == "" || isZeroUnits(row["units"]) {
			continue
		}
		sale, err := ParseRow(row)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+1, err)
		}
		sales = append(sales, sale)
	}
	return sales, nil
}

func isZeroUnits(raw string) bool {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	return err == nil && n == 0
}

// RevenueCents is units times unit price.
func RevenueCents(s Sale) int64 {
	return int64(s.Units) * s.UnitPriceCents
}

// TotalsBy sums revenue per key(sale).
func TotalsBy(sales []Sale, key func(Sale) string) map[string]int64 {
	totals := make(map[string]int64)
	for _, s := range sales {
		totals[key(s)] += RevenueCents(s)
	}
	return totals
}

// Total is one ranked line of a report.
type Total struct {
	Key          string
	RevenueCents int64
}

// Ranked orders totals by revenue descending, then key ascending. Map iteration
// order is random, so the second key is what makes the result deterministic.
func Ranked(totals map[string]int64) []Total {
	out := make([]Total, 0, len(totals))
	for k, v := range totals {
		out = append(out, Total{Key: k, RevenueCents: v})
	}
	slices.SortStableFunc(out, func(a, b Total) int {
		return cmp.Or(
			cmp.Compare(b.RevenueCents, a.RevenueCents),
			cmp.Compare(a.Key, b.Key),
		)
	})
	return out
}

// SortSales returns a copy of sales ordered by date, then region, then product.
// The sort is stable, so sales that tie on all three keep their input order. The
// input is not modified.
func SortSales(sales []Sale) []Sale {
	out := slices.Clone(sales)
	slices.SortStableFunc(out, func(a, b Sale) int {
		return cmp.Or(
			a.SoldOn.Compare(b.SoldOn),
			cmp.Compare(a.Region, b.Region),
			cmp.Compare(a.Product, b.Product),
		)
	})
	return out
}

// DistinctRegions returns the sorted, unique regions in upper case.
func DistinctRegions(sales []Sale) []string {
	seen := make(map[string]struct{}, len(sales))
	for _, s := range sales {
		seen[s.Region] = struct{}{}
	}
	return Map(slices.Sorted(maps.Keys(seen)), strings.ToUpper)
}

// FormatCents renders cents as currency with thousands separators, for example
// 1234567 becomes "$12,345.67". Negative amounts are an error.
func FormatCents(cents int64, currency string) (string, error) {
	if cents < 0 {
		return "", fmt.Errorf("%w: negative amount %d", ErrInvalidAmount, cents)
	}
	digits := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	b.WriteString(currency)
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	fmt.Fprintf(&b, ".%02d", cents%100)
	return b.String(), nil
}

// BuildReport produces one "region: $amount" line per region, ranked by revenue.
// Each step is a function from the previous step's value to the next. Steps that
// can fail return an error, which ends the pipeline early.
func BuildReport(rows iter.Seq[Row], currency string) ([]string, error) {
	sales, err := CleanSales(rows)
	if err != nil {
		return nil, err
	}
	ranked := Ranked(TotalsBy(sales, func(s Sale) string { return s.Region }))
	lines := make([]string, 0, len(ranked))
	for _, t := range ranked {
		amount, err := FormatCents(t.RevenueCents, currency)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fmt.Sprintf("%s: %s", t.Key, amount))
	}
	return lines, nil
}
