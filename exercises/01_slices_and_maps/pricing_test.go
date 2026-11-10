package slicesmaps_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/01_slices_and_maps"
)

var standardTiers = []slicesmaps.Tier{
	{MinQty: 1, Unit: 100},
	{MinQty: 10, Unit: 90},
	{MinQty: 50, Unit: 80},
}

func mustPriceTiers(t *testing.T, tiers []slicesmaps.Tier) *slicesmaps.PriceTiers {
	t.Helper()
	p, err := slicesmaps.NewPriceTiers(tiers)
	if err != nil {
		t.Fatalf("NewPriceTiers(%v): %v", tiers, err)
	}
	return p
}

func TestPart2MoneyString(t *testing.T) {
	tests := []struct {
		m    slicesmaps.Money
		want string
	}{
		{1234, "12.34"},
		{5, "0.05"},
		{-5, "-0.05"},
		{0, "0.00"},
		{100, "1.00"},
		{-1234, "-12.34"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprint(int64(tc.m)), func(t *testing.T) {
			if got := tc.m.String(); got != tc.want {
				t.Fatalf("Money(%d).String() = %q, want %q", int64(tc.m), got, tc.want)
			}
		})
	}
}

func TestPart2UnitPrice(t *testing.T) {
	tests := []struct {
		qty  int
		want slicesmaps.Money
	}{
		{1, 100},
		{9, 100},
		{10, 90}, // boundary belongs to the new tier
		{49, 90},
		{50, 80},
		{500, 80},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("qty=%d", tc.qty), func(t *testing.T) {
			p := mustPriceTiers(t, standardTiers)
			got, err := p.UnitPrice(tc.qty)
			if err != nil {
				t.Fatalf("UnitPrice(%d): %v", tc.qty, err)
			}
			if got != tc.want {
				t.Fatalf("UnitPrice(%d) = %v, want %v", tc.qty, got, tc.want)
			}
		})
	}
}

func TestPart2UnitPriceSortsCopy(t *testing.T) {
	t.Run("caller slice is not reordered", func(t *testing.T) {
		unsorted := []slicesmaps.Tier{{MinQty: 50, Unit: 80}, {MinQty: 1, Unit: 100}, {MinQty: 10, Unit: 90}}
		before := slices.Clone(unsorted)
		p := mustPriceTiers(t, unsorted)
		if diff := cmp.Diff(before, unsorted); diff != "" {
			t.Fatalf("NewPriceTiers reordered the caller's slice (-before +after):\n%s", diff)
		}
		if got, err := p.UnitPrice(10); err != nil || got != 90 {
			t.Fatalf("UnitPrice(10) = %v, %v; want 90, nil", got, err)
		}
	})
}

func TestPart2NewPriceTiersRejects(t *testing.T) {
	tests := []struct {
		name  string
		tiers []slicesmaps.Tier
		want  error
	}{
		{"no tiers", nil, slicesmaps.ErrNoTiers},
		{"empty slice", []slicesmaps.Tier{}, slicesmaps.ErrNoTiers},
		{"lowest starts at zero", []slicesmaps.Tier{{MinQty: 0, Unit: 100}}, slicesmaps.ErrInvalidTier},
		{"lowest starts above one", []slicesmaps.Tier{{MinQty: 2, Unit: 100}}, slicesmaps.ErrInvalidTier},
		{"negative min qty", []slicesmaps.Tier{{MinQty: -1, Unit: 100}}, slicesmaps.ErrInvalidTier},
		{"duplicate min qty", []slicesmaps.Tier{{MinQty: 1, Unit: 100}, {MinQty: 1, Unit: 90}}, slicesmaps.ErrInvalidTier},
		{"zero price", []slicesmaps.Tier{{MinQty: 1, Unit: 0}}, slicesmaps.ErrInvalidTier},
		{"negative price", []slicesmaps.Tier{{MinQty: 1, Unit: 100}, {MinQty: 10, Unit: -5}}, slicesmaps.ErrInvalidTier},
		{"valid schedule accepted", standardTiers, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := slicesmaps.NewPriceTiers(tc.tiers)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("NewPriceTiers: unexpected error %v", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("NewPriceTiers error = %v, want errors.Is %v", err, tc.want)
			}
		})
	}
}

func TestPart2UnitPriceRejectsNonPositiveQty(t *testing.T) {
	for _, qty := range []int{0, -3} {
		t.Run(fmt.Sprintf("qty=%d", qty), func(t *testing.T) {
			p := mustPriceTiers(t, standardTiers)
			if _, err := p.UnitPrice(qty); !errors.Is(err, slicesmaps.ErrInvalidValue) {
				t.Fatalf("UnitPrice(%d) error = %v, want ErrInvalidValue", qty, err)
			}
			if _, err := p.LineTotal(qty); !errors.Is(err, slicesmaps.ErrInvalidValue) {
				t.Fatalf("LineTotal(%d) error = %v, want ErrInvalidValue", qty, err)
			}
		})
	}
}

func TestPart2LineTotal(t *testing.T) {
	tests := []struct {
		qty  int
		want slicesmaps.Money
	}{
		{1, 100},
		{3, 300},
		{10, 900},
		{60, 4800},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("qty=%d", tc.qty), func(t *testing.T) {
			p := mustPriceTiers(t, standardTiers)
			got, err := p.LineTotal(tc.qty)
			if err != nil {
				t.Fatalf("LineTotal(%d): %v", tc.qty, err)
			}
			if got != tc.want {
				t.Fatalf("LineTotal(%d) = %v, want %v", tc.qty, got, tc.want)
			}
		})
	}
}

func TestPart2SLATier(t *testing.T) {
	tests := []struct {
		minutes float64
		want    string
	}{
		{0, "critical"},
		{15, "critical"},
		{15.5, "high"},
		{60, "high"},
		{60.5, "normal"},
		{240, "normal"},
		{240.5, "low"},
		{math.Inf(1), "low"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("minutes=%v", tc.minutes), func(t *testing.T) {
			got, err := slicesmaps.SLATier(tc.minutes)
			if err != nil {
				t.Fatalf("SLATier(%v): %v", tc.minutes, err)
			}
			if got != tc.want {
				t.Fatalf("SLATier(%v) = %q, want %q", tc.minutes, got, tc.want)
			}
		})
	}
}

func TestPart2SLATierRejects(t *testing.T) {
	for _, m := range []float64{-1, math.NaN()} {
		t.Run(fmt.Sprintf("minutes=%v", m), func(t *testing.T) {
			if _, err := slicesmaps.SLATier(m); !errors.Is(err, slicesmaps.ErrInvalidValue) {
				t.Fatalf("SLATier(%v) error = %v, want ErrInvalidValue", m, err)
			}
		})
	}
}
