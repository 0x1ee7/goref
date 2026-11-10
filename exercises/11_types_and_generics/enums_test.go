package typed_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/11_types_and_generics"
)

func TestPart2ParseOrderStatus(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    typed.OrderStatus
		wantErr bool
	}{
		{"lowercase", "pending", typed.OrderStatusPending, false},
		{"uppercase", "PAID", typed.OrderStatusPaid, false},
		{"padded and mixed case", "  shipped ", typed.OrderStatusShipped, false},
		{"cancelled", "Cancelled", typed.OrderStatusCancelled, false},
		{"empty", "", 0, true},
		{"unspecified is not a status", "unspecified", 0, true},
		{"unknown", "refunded", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typed.ParseOrderStatus(tt.in)
			if tt.wantErr {
				if !errors.Is(err, typed.ErrInvalidOrderStatus) {
					t.Fatalf("ParseOrderStatus(%q) error = %v, want ErrInvalidOrderStatus", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOrderStatus(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseOrderStatus(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestPart2IotaValuesAndString(t *testing.T) {
	// Unspecified is the zero value, so the first real state is 1.
	if typed.OrderStatusUnspecified != 0 || typed.OrderStatusPending != 1 || typed.OrderStatusCancelled != 4 {
		t.Errorf("iota values = (%d,%d,%d), want (0,1,4)",
			typed.OrderStatusUnspecified, typed.OrderStatusPending, typed.OrderStatusCancelled)
	}
	if got := fmt.Sprintf("%v %d", typed.OrderStatusPending, typed.OrderStatusPending); got != "pending 1" {
		t.Errorf("Sprintf(%%v %%d) = %q, want %q", got, "pending 1")
	}
	if got := typed.OrderStatusUnspecified.String(); got != "unspecified" {
		t.Errorf("Unspecified.String() = %q, want %q", got, "unspecified")
	}
	if got := typed.OrderStatus(99).String(); got != "OrderStatus(99)" {
		t.Errorf("OrderStatus(99).String() = %q, want %q", got, "OrderStatus(99)")
	}
}

func TestPart2Valid(t *testing.T) {
	tests := []struct {
		s    typed.OrderStatus
		want bool
	}{
		{typed.OrderStatusUnspecified, false},
		{typed.OrderStatusPending, true},
		{typed.OrderStatusPaid, true},
		{typed.OrderStatusShipped, true},
		{typed.OrderStatusCancelled, true},
		{typed.OrderStatus(5), false},
		{typed.OrderStatus(-1), false},
	}
	for _, tt := range tests {
		if got := tt.s.Valid(); got != tt.want {
			t.Errorf("%d.Valid() = %t, want %t", tt.s, got, tt.want)
		}
	}
}

func TestPart2CanTransition(t *testing.T) {
	allowed := map[typed.OrderStatus][]typed.OrderStatus{
		typed.OrderStatusPending: {typed.OrderStatusPaid, typed.OrderStatusCancelled},
		typed.OrderStatusPaid:    {typed.OrderStatusShipped, typed.OrderStatusCancelled},
	}
	// Every pair of states, including invalid values, is checked against the
	// explicit allow-list: anything not listed must be false.
	states := []typed.OrderStatus{-1, 0, 1, 2, 3, 4, 5, 99}
	for _, from := range states {
		for _, to := range states {
			want := slices.Contains(allowed[from], to)
			if got := from.CanTransition(to); got != want {
				t.Errorf("%v.CanTransition(%v) = %t, want %t", from, to, got, want)
			}
		}
	}
}

func TestPart2AllStatuses(t *testing.T) {
	got := slices.Collect(typed.AllOrderStatuses())
	want := []typed.OrderStatus{
		typed.OrderStatusPending, typed.OrderStatusPaid,
		typed.OrderStatusShipped, typed.OrderStatusCancelled,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("AllOrderStatuses mismatch (-want +got):\n%s", diff)
	}
}

type orderView struct {
	Status typed.OrderStatus `json:"status"`
}

func TestPart2JSON(t *testing.T) {
	b, err := json.Marshal(orderView{Status: typed.OrderStatusPaid})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"status":"paid"}`; string(b) != want {
		t.Errorf("Marshal = %s, want %s", b, want)
	}

	var v orderView
	if err := json.Unmarshal([]byte(`{"status":"SHIPPED"}`), &v); err != nil {
		t.Fatalf("Unmarshal(SHIPPED): %v", err)
	}
	if v.Status != typed.OrderStatusShipped {
		t.Errorf("decoded status = %v, want shipped", v.Status)
	}

	if err := json.Unmarshal([]byte(`{"status":"refunded"}`), &v); !errors.Is(err, typed.ErrInvalidOrderStatus) {
		t.Errorf("Unmarshal(refunded) error = %v, want ErrInvalidOrderStatus", err)
	}
	if _, err := json.Marshal(orderView{}); !errors.Is(err, typed.ErrInvalidOrderStatus) {
		t.Errorf("Marshal of unset status error = %v, want ErrInvalidOrderStatus", err)
	}
	if _, err := json.Marshal(orderView{Status: 42}); !errors.Is(err, typed.ErrInvalidOrderStatus) {
		t.Errorf("Marshal of OrderStatus(42) error = %v, want ErrInvalidOrderStatus", err)
	}
}
