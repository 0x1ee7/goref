package slicesmaps

import "errors"

// Money is an amount in integer minor units (cents). Floats never hold money.
type Money int64

// String formats m with two decimals: 1234 → "12.34", -5 → "-0.05".
func (m Money) String() string {
	return ""
}

// Tier is a volume price band: from MinQty units upward each unit costs Unit.
type Tier struct {
	MinQty int
	Unit   Money
}

// PriceTiers is a validated, sorted schedule of volume tiers.
type PriceTiers struct{}

// NewPriceTiers validates tiers and returns a schedule. The input may be in any
// order and must not be reordered. The schedule must have a tier starting at
// quantity 1; see ErrNoTiers and ErrInvalidTier.
func NewPriceTiers(tiers []Tier) (*PriceTiers, error) {
	return nil, errors.New("not implemented")
}

// UnitPrice returns the unit price for qty; a quantity on a tier boundary
// belongs to the new tier. qty < 1 yields ErrInvalidValue.
func (p *PriceTiers) UnitPrice(qty int) (Money, error) {
	return 0, errors.New("not implemented")
}

// LineTotal returns UnitPrice(qty) multiplied by qty.
func (p *PriceTiers) LineTotal(qty int) (Money, error) {
	return 0, errors.New("not implemented")
}

// SLATier maps a response time in minutes to "critical", "high", "normal" or
// "low". Upper bounds (15, 60, 240) are inclusive. Negative or NaN minutes
// yield ErrInvalidValue.
func SLATier(responseMinutes float64) (string, error) {
	return "", errors.New("not implemented")
}
