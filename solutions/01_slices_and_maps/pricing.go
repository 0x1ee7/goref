package slicesmaps

import (
	"cmp"
	"fmt"
	"math"
	"slices"
)

// Money is an amount in integer minor units (cents). Floats never hold money.
type Money int64

// String formats m with two decimals: 1234 → "12.34", -5 → "-0.05".
func (m Money) String() string {
	sign := ""
	u := uint64(m)
	if m < 0 {
		sign = "-"
		u = -u // two's-complement negation yields the magnitude, even for MinInt64
	}
	return fmt.Sprintf("%s%d.%02d", sign, u/100, u%100)
}

// Tier is a volume price band: from MinQty units upward each unit costs Unit.
type Tier struct {
	MinQty int
	Unit   Money
}

// PriceTiers is a validated, sorted schedule of volume tiers.
type PriceTiers struct {
	tiers []Tier // ascending by MinQty; tiers[0].MinQty == 1
}

// NewPriceTiers validates tiers and returns a schedule. The input may be in any
// order and is not modified; the schedule keeps a sorted copy.
func NewPriceTiers(tiers []Tier) (*PriceTiers, error) {
	if len(tiers) == 0 {
		return nil, ErrNoTiers
	}
	sorted := slices.Clone(tiers)
	slices.SortFunc(sorted, func(a, b Tier) int { return cmp.Compare(a.MinQty, b.MinQty) })

	if sorted[0].MinQty != 1 {
		return nil, fmt.Errorf("%w: lowest tier starts at quantity %d, want 1", ErrInvalidTier, sorted[0].MinQty)
	}
	for i, t := range sorted {
		if t.Unit <= 0 {
			return nil, fmt.Errorf("%w: tier at quantity %d has price %v", ErrInvalidTier, t.MinQty, t.Unit)
		}
		if i > 0 && sorted[i-1].MinQty == t.MinQty {
			return nil, fmt.Errorf("%w: duplicate tier at quantity %d", ErrInvalidTier, t.MinQty)
		}
	}
	return &PriceTiers{tiers: sorted}, nil
}

// UnitPrice returns the unit price for qty. A quantity on a tier boundary
// belongs to the new tier. qty < 1 yields ErrInvalidValue.
func (p *PriceTiers) UnitPrice(qty int) (Money, error) {
	if qty < 1 {
		return 0, fmt.Errorf("%w: quantity %d", ErrInvalidValue, qty)
	}
	i, found := slices.BinarySearchFunc(p.tiers, qty, func(t Tier, q int) int {
		return cmp.Compare(t.MinQty, q)
	})
	if !found {
		i-- // insertion point is one past the governing tier; i >= 1 since tiers[0].MinQty == 1
	}
	return p.tiers[i].Unit, nil
}

// LineTotal returns UnitPrice(qty) multiplied by qty.
func (p *PriceTiers) LineTotal(qty int) (Money, error) {
	unit, err := p.UnitPrice(qty)
	if err != nil {
		return 0, err
	}
	return unit * Money(qty), nil
}

// slaBand is an inclusive upper bound and the label it maps to.
type slaBand struct {
	upper float64
	name  string
}

// slaBands is sorted ascending by upper bound.
var slaBands = []slaBand{
	{upper: 15, name: "critical"},
	{upper: 60, name: "high"},
	{upper: 240, name: "normal"},
}

// SLATier maps a response time in minutes to "critical", "high", "normal" or
// "low". Upper bounds are inclusive. Negative or NaN minutes yield ErrInvalidValue.
func SLATier(responseMinutes float64) (string, error) {
	if math.IsNaN(responseMinutes) || responseMinutes < 0 {
		return "", fmt.Errorf("%w: response minutes %v", ErrInvalidValue, responseMinutes)
	}
	// First band whose upper bound is >= the value; none means "low".
	i, _ := slices.BinarySearchFunc(slaBands, responseMinutes, func(b slaBand, v float64) int {
		return cmp.Compare(b.upper, v)
	})
	if i == len(slaBands) {
		return "low", nil
	}
	return slaBands[i].name, nil
}
