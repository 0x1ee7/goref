package functional_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	fp "goref/exercises/06_functional_generics"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func goodRow() fp.Row {
	return fp.Row{
		"region": "west", "product": "Widget", "units": "3",
		"unit_price": "12.50", "sold_on": "2024-03-01",
	}
}

func TestPart7ParseCents(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
	}{
		{"whole dollars", "12", 1200},
		{"one decimal", "12.5", 1250},
		{"two decimals", "1234.56", 123456},
		{"one cent", "0.01", 1},
		{"leading zero", "0.5", 50},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fp.ParseCents(tc.raw)
			if err != nil {
				t.Fatalf("ParseCents(%q) error: %v", tc.raw, err)
			}
			if got != tc.want {
				t.Errorf("ParseCents(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
	t.Run("rejects malformed and overflowing amounts", func(t *testing.T) {
		for _, raw := range []string{"", ".5", "12.", "-1", "+1", "1e3", "1.234", "abc", "92233720368547759.00"} {
			if _, err := fp.ParseCents(raw); !errors.Is(err, fp.ErrInvalidAmount) {
				t.Errorf("ParseCents(%q) err = %v, want ErrInvalidAmount", raw, err)
			}
		}
	})
}

func TestPart7ParseRow(t *testing.T) {
	t.Run("normalises a valid row", func(t *testing.T) {
		row := goodRow()
		row["region"] = " West "
		row["product"] = " Widget "
		got, err := fp.ParseRow(row)
		if err != nil {
			t.Fatalf("ParseRow error: %v", err)
		}
		want := fp.Sale{Region: "west", Product: "Widget", Units: 3, UnitPriceCents: 1250, SoldOn: day(2024, time.March, 1)}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ParseRow mismatch (-want +got):\n%s", diff)
		}
	})
	tests := []struct {
		name   string
		mutate func(fp.Row)
		prefix string
		is     error
	}{
		{"negative units", func(r fp.Row) { r["units"] = "-1" }, "units: ", nil},
		{"non-numeric units", func(r fp.Row) { r["units"] = "x" }, "units: ", strconv.ErrSyntax},
		{"bad price", func(r fp.Row) { r["unit_price"] = "abc" }, "unit_price: ", fp.ErrInvalidAmount},
		{"bad date", func(r fp.Row) { r["sold_on"] = "03/01/2024" }, "sold_on: ", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := goodRow()
			tc.mutate(row)
			_, err := fp.ParseRow(row)
			if err == nil || !strings.HasPrefix(err.Error(), tc.prefix) {
				t.Fatalf("ParseRow err = %v, want prefix %q", err, tc.prefix)
			}
			if tc.is != nil && !errors.Is(err, tc.is) {
				t.Errorf("ParseRow err = %v, want it to wrap %v", err, tc.is)
			}
		})
	}
}

func TestPart7CleanSales(t *testing.T) {
	east := fp.Row{"region": "east", "product": "Gadget", "units": "2", "unit_price": "5.00", "sold_on": "2024-03-02"}
	blank := goodRow()
	blank["region"] = "  "
	zero := goodRow()
	zero["units"] = "0"

	t.Run("drops blank-region and zero-unit rows", func(t *testing.T) {
		rows := []fp.Row{goodRow(), blank, zero, east}
		got, err := fp.CleanSales(slices.Values(rows))
		if err != nil {
			t.Fatalf("CleanSales error: %v", err)
		}
		want := []fp.Sale{
			{Region: "west", Product: "Widget", Units: 3, UnitPriceCents: 1250, SoldOn: day(2024, time.March, 1)},
			{Region: "east", Product: "Gadget", Units: 2, UnitPriceCents: 500, SoldOn: day(2024, time.March, 2)},
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("CleanSales mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("error names the 1-based row", func(t *testing.T) {
		bad := goodRow()
		bad["units"] = "x"
		_, err := fp.CleanSales(slices.Values([]fp.Row{goodRow(), bad}))
		if err == nil || !strings.HasPrefix(err.Error(), "row 2: units: ") {
			t.Errorf("CleanSales err = %v, want prefix \"row 2: units: \"", err)
		}
	})
	t.Run("empty input yields non-nil empty slice", func(t *testing.T) {
		got, err := fp.CleanSales(slices.Values([]fp.Row{}))
		if err != nil {
			t.Fatalf("CleanSales error: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("CleanSales(empty) = %v, want non-nil empty slice", got)
		}
	})
}

func TestPart7Aggregates(t *testing.T) {
	sales := []fp.Sale{
		{Region: "west", Product: "Widget", Units: 3, UnitPriceCents: 1250, SoldOn: day(2024, 3, 1)},
		{Region: "east", Product: "Gadget", Units: 2, UnitPriceCents: 500, SoldOn: day(2024, 3, 2)},
		{Region: "west", Product: "Gizmo", Units: 1, UnitPriceCents: 4000, SoldOn: day(2024, 3, 1)},
	}
	t.Run("RevenueCents is units times unit price", func(t *testing.T) {
		if got := fp.RevenueCents(sales[0]); got != 3750 {
			t.Errorf("RevenueCents = %d, want 3750", got)
		}
	})
	t.Run("TotalsBy sums revenue per key", func(t *testing.T) {
		got := fp.TotalsBy(sales, func(s fp.Sale) string { return s.Region })
		want := map[string]int64{"west": 7750, "east": 1000}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("TotalsBy mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("Ranked orders by revenue then key", func(t *testing.T) {
		got := fp.Ranked(map[string]int64{"b": 100, "a": 100, "c": 300})
		want := []fp.Total{{Key: "c", RevenueCents: 300}, {Key: "a", RevenueCents: 100}, {Key: "b", RevenueCents: 100}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Ranked mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("SortSales orders by date, region, product without mutating input", func(t *testing.T) {
		got := fp.SortSales(sales)
		want := []fp.Sale{sales[2], sales[0], sales[1]}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("SortSales mismatch (-want +got):\n%s", diff)
		}
		if sales[0].Product != "Widget" || sales[1].Product != "Gadget" {
			t.Error("SortSales modified its input")
		}
	})
	t.Run("SortSales is stable on full ties", func(t *testing.T) {
		a := fp.Sale{Region: "x", Product: "p", Units: 1, SoldOn: day(2024, 1, 1)}
		b := a
		b.Units = 2
		got := fp.SortSales([]fp.Sale{a, b})
		if len(got) != 2 || got[0].Units != 1 || got[1].Units != 2 {
			t.Errorf("SortSales ties = %+v, want input order preserved", got)
		}
	})
	t.Run("DistinctRegions returns sorted upper-case regions", func(t *testing.T) {
		got := fp.DistinctRegions(sales)
		if diff := cmp.Diff([]string{"EAST", "WEST"}, got); diff != "" {
			t.Errorf("DistinctRegions mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPart7FormatCents(t *testing.T) {
	tests := []struct {
		name     string
		cents    int64
		currency string
		want     string
	}{
		{"zero", 0, "$", "$0.00"},
		{"cents only", 5, "$", "$0.05"},
		{"under a thousand", 999, "$", "$9.99"},
		{"thousands separator", 1234567, "$", "$12,345.67"},
		{"millions", 100000000, "$", "$1,000,000.00"},
		{"other currency symbol", 123400, "€", "€1,234.00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fp.FormatCents(tc.cents, tc.currency)
			if err != nil {
				t.Fatalf("FormatCents(%d) error: %v", tc.cents, err)
			}
			if got != tc.want {
				t.Errorf("FormatCents(%d, %q) = %q, want %q", tc.cents, tc.currency, got, tc.want)
			}
		})
	}
	t.Run("rejects negative amounts", func(t *testing.T) {
		if _, err := fp.FormatCents(-1, "$"); !errors.Is(err, fp.ErrInvalidAmount) {
			t.Errorf("FormatCents(-1) err = %v, want ErrInvalidAmount", err)
		}
	})
}

func TestPart7BuildReport(t *testing.T) {
	t.Run("builds ranked report lines", func(t *testing.T) {
		rows := []fp.Row{
			{"region": "west", "product": "Widget", "units": "2", "unit_price": "10.00", "sold_on": "2024-01-01"},
			{"region": "east", "product": "Gadget", "units": "1", "unit_price": "50.00", "sold_on": "2024-01-01"},
			{"region": "west", "product": "Gizmo", "units": "3", "unit_price": "5.50", "sold_on": "2024-01-02"},
		}
		got, err := fp.BuildReport(slices.Values(rows), "$")
		if err != nil {
			t.Fatalf("BuildReport error: %v", err)
		}
		want := []string{"east: $50.00", "west: $36.50"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("BuildReport mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("a bad row aborts the report", func(t *testing.T) {
		bad := goodRow()
		bad["units"] = "x"
		_, err := fp.BuildReport(slices.Values([]fp.Row{goodRow(), bad}), "$")
		if err == nil || !strings.HasPrefix(err.Error(), "row 2: ") {
			t.Errorf("BuildReport err = %v, want prefix \"row 2: \"", err)
		}
	})
	t.Run("empty input yields non-nil empty report", func(t *testing.T) {
		got, err := fp.BuildReport(slices.Values([]fp.Row{}), "$")
		if err != nil {
			t.Fatalf("BuildReport error: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Errorf("BuildReport(empty) = %v, want non-nil empty slice", got)
		}
	})
}
