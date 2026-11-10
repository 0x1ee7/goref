package functional_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	fp "goref/exercises/06_functional_generics"
)

type unknownAction struct{}

func mug() fp.LineItem {
	return fp.LineItem{SKU: "A1", Name: "Mug", UnitPriceCents: 500, Quantity: 1}
}

func pen() fp.LineItem {
	return fp.LineItem{SKU: "B2", Name: "Pen", UnitPriceCents: 250, Quantity: 1}
}

func TestPart3AddItem(t *testing.T) {
	t.Run("adds a new line to an empty cart", func(t *testing.T) {
		got, err := fp.CartReducer(fp.Cart{}, fp.NewAddItem("A1", "Mug", 500))
		if err != nil {
			t.Fatalf("CartReducer error: %v", err)
		}
		want := fp.Cart{Items: []fp.LineItem{mug()}}
		if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("CartReducer mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("existing sku increments in place and keeps original name and price", func(t *testing.T) {
		state := fp.Cart{Items: []fp.LineItem{mug(), pen()}}
		got, err := fp.CartReducer(state, fp.AddItem{SKU: "A1", Name: "Other", UnitPriceCents: 999, Quantity: 2})
		if err != nil {
			t.Fatalf("CartReducer error: %v", err)
		}
		want := fp.Cart{Items: []fp.LineItem{
			{SKU: "A1", Name: "Mug", UnitPriceCents: 500, Quantity: 3},
			pen(),
		}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("CartReducer mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPart3Lines(t *testing.T) {
	stateA := fp.Cart{Items: []fp.LineItem{mug()}}
	stateAB := fp.Cart{Items: []fp.LineItem{mug(), pen()}}
	tests := []struct {
		name   string
		state  fp.Cart
		action any
		want   fp.Cart
	}{
		{"remove drops the sku", stateAB, fp.RemoveItem{SKU: "A1"}, fp.Cart{Items: []fp.LineItem{pen()}}},
		{"remove unknown sku is a no-op", stateA, fp.RemoveItem{SKU: "ZZ"}, stateA},
		{"set quantity", stateA, fp.SetQuantity{SKU: "A1", Quantity: 5}, fp.Cart{Items: []fp.LineItem{
			{SKU: "A1", Name: "Mug", UnitPriceCents: 500, Quantity: 5},
		}}},
		{"zero quantity removes the line", stateAB, fp.SetQuantity{SKU: "A1", Quantity: 0}, fp.Cart{Items: []fp.LineItem{pen()}}},
		{"removing the last line keeps the coupon",
			fp.Cart{Items: []fp.LineItem{mug()}, Coupon: "SAVE10"},
			fp.RemoveItem{SKU: "A1"},
			fp.Cart{Coupon: "SAVE10"}},
		{"apply coupon", stateA, fp.ApplyCoupon{Code: "SAVE10"}, fp.Cart{Items: []fp.LineItem{mug()}, Coupon: "SAVE10"}},
		{"empty coupon clears", fp.Cart{Items: []fp.LineItem{mug()}, Coupon: "HALF"}, fp.ApplyCoupon{Code: ""}, stateA},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fp.CartReducer(tc.state, tc.action)
			if err != nil {
				t.Fatalf("CartReducer(%T) error: %v", tc.action, err)
			}
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("CartReducer mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart3Errors(t *testing.T) {
	stateA := fp.Cart{Items: []fp.LineItem{mug()}}
	tests := []struct {
		name   string
		state  fp.Cart
		action any
		want   error
	}{
		{"zero quantity add", stateA, fp.AddItem{SKU: "A1", Quantity: 0}, fp.ErrInvalidQuantity},
		{"negative price", fp.Cart{}, fp.AddItem{SKU: "X", Quantity: 1, UnitPriceCents: -1}, fp.ErrInvalidPrice},
		{"negative set quantity", stateA, fp.SetQuantity{SKU: "A1", Quantity: -1}, fp.ErrInvalidQuantity},
		{"set quantity on unknown sku", stateA, fp.SetQuantity{SKU: "ZZ", Quantity: 1}, fp.ErrUnknownSKU},
		{"unknown coupon", stateA, fp.ApplyCoupon{Code: "NOPE"}, fp.ErrUnknownCoupon},
		{"unknown action type", stateA, unknownAction{}, fp.ErrUnknownAction},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fp.CartReducer(tc.state, tc.action)
			if !errors.Is(err, tc.want) {
				t.Fatalf("CartReducer err = %v, want %v", err, tc.want)
			}
			if got.Items != nil || got.Coupon != "" {
				t.Errorf("CartReducer on error returned %+v, want the zero Cart", got)
			}
		})
	}
}

func makeCart() fp.Cart {
	return fp.Cart{Items: []fp.LineItem{{
		SKU: "A1", Name: "Mug", UnitPriceCents: 500, Quantity: 1,
		Labels: map[string]string{"color": "red"},
	}}}
}

func TestPart3Immutability(t *testing.T) {
	t.Run("reducer never mutates its input state", func(t *testing.T) {
		state := makeCart()
		got, err := fp.CartReducer(state, fp.AddItem{SKU: "A1", Quantity: 2})
		if err != nil {
			t.Fatalf("CartReducer error: %v", err)
		}
		want := makeCart()
		want.Items[0].Quantity = 3
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("CartReducer mismatch (-want +got):\n%s", diff)
		}
		got.Items[0].Labels["color"] = "blue"
		if diff := cmp.Diff(makeCart(), state); diff != "" {
			t.Errorf("input state changed through the result (-want +got):\n%s", diff)
		}
	})
	t.Run("result does not alias the action's labels", func(t *testing.T) {
		labels := map[string]string{"k": "v"}
		got, err := fp.CartReducer(makeCart(), fp.AddItem{SKU: "B2", Name: "Pen", UnitPriceCents: 250, Quantity: 1, Labels: labels})
		if err != nil {
			t.Fatalf("CartReducer error: %v", err)
		}
		want := fp.Cart{Items: []fp.LineItem{
			{SKU: "A1", Name: "Mug", UnitPriceCents: 500, Quantity: 1, Labels: map[string]string{"color": "red"}},
			{SKU: "B2", Name: "Pen", UnitPriceCents: 250, Quantity: 1, Labels: map[string]string{"k": "v"}},
		}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("CartReducer mismatch (-want +got):\n%s", diff)
		}
		labels["k"] = "changed"
		if v := got.Items[1].Labels["k"]; v != "v" {
			t.Errorf("result labels changed with the action's map: got %q, want \"v\"", v)
		}
	})
}

func TestPart3Totals(t *testing.T) {
	base := []fp.LineItem{
		{SKU: "A1", UnitPriceCents: 500, Quantity: 2},
		{SKU: "B2", UnitPriceCents: 250, Quantity: 1},
	}
	t.Run("subtotal sums price times quantity", func(t *testing.T) {
		if got := fp.CartSubtotal(fp.Cart{Items: base}); got != 1250 {
			t.Errorf("CartSubtotal = %d, want 1250", got)
		}
	})
	tests := []struct {
		name string
		cart fp.Cart
		want int64
	}{
		{"no coupon", fp.Cart{Items: base}, 1250},
		{"percent coupon", fp.Cart{Items: base, Coupon: "SAVE10"}, 1125},
		{"discount floors to whole cents", fp.Cart{Items: []fp.LineItem{{UnitPriceCents: 1251, Quantity: 1}}, Coupon: "HALF"}, 626},
		{"unknown coupon grants nothing", fp.Cart{Items: base, Coupon: "BOGUS"}, 1250},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fp.CartTotal(tc.cart); got != tc.want {
				t.Errorf("CartTotal = %d, want %d", got, tc.want)
			}
		})
	}
	t.Run("coupon table", func(t *testing.T) {
		if pct, ok := fp.CouponPercent("SAVE10"); !ok || pct != 10 {
			t.Errorf("CouponPercent(SAVE10) = (%d, %v), want (10, true)", pct, ok)
		}
		if pct, ok := fp.CouponPercent("NOPE"); ok || pct != 0 {
			t.Errorf("CouponPercent(NOPE) = (%d, %v), want (0, false)", pct, ok)
		}
	})
}

func TestPart3RunActions(t *testing.T) {
	t.Run("folds actions in order", func(t *testing.T) {
		actions := []any{
			fp.NewAddItem("A1", "Mug", 500),
			fp.NewAddItem("A1", "Mug", 500),
			fp.ApplyCoupon{Code: "SAVE10"},
		}
		got, err := fp.RunActions(fp.Cart{}, actions)
		if err != nil {
			t.Fatalf("RunActions error: %v", err)
		}
		want := fp.Cart{Items: []fp.LineItem{{SKU: "A1", Name: "Mug", UnitPriceCents: 500, Quantity: 2}}, Coupon: "SAVE10"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("RunActions mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("stops at the first error", func(t *testing.T) {
		actions := []any{
			fp.NewAddItem("A1", "Mug", 500),
			fp.SetQuantity{SKU: "NOPE", Quantity: 1},
			fp.NewAddItem("B2", "Pen", 250),
		}
		if _, err := fp.RunActions(fp.Cart{}, actions); !errors.Is(err, fp.ErrUnknownSKU) {
			t.Errorf("RunActions err = %v, want ErrUnknownSKU", err)
		}
	})
	t.Run("ClearCart resets mid-stream", func(t *testing.T) {
		actions := []any{fp.NewAddItem("A1", "Mug", 500), fp.ClearCart{}, fp.NewAddItem("B2", "Pen", 250)}
		got, err := fp.RunActions(fp.Cart{}, actions)
		if err != nil {
			t.Fatalf("RunActions error: %v", err)
		}
		want := fp.Cart{Items: []fp.LineItem{pen()}}
		if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("RunActions mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("no actions returns the state unchanged", func(t *testing.T) {
		state := fp.Cart{Items: []fp.LineItem{mug()}, Coupon: "SAVE10"}
		got, err := fp.RunActions(state, nil)
		if err != nil {
			t.Fatalf("RunActions error: %v", err)
		}
		if diff := cmp.Diff(state, got); diff != "" {
			t.Errorf("RunActions mismatch (-want +got):\n%s", diff)
		}
	})
}
