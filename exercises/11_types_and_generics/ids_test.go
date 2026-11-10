package typed_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/11_types_and_generics"
)

func TestPart1ParseUserID(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    typed.UserID
		wantErr bool
	}{
		{"canonical", "usr_42", 42, false},
		{"one", "usr_1", 1, false},
		{"empty", "", 0, true},
		{"bare number", "42", 0, true},
		{"wrong prefix", "ord_42", 0, true},
		{"no digits", "usr_", 0, true},
		{"zero is unset", "usr_0", 0, true},
		{"negative", "usr_-1", 0, true},
		{"not a number", "usr_abc", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typed.ParseUserID(tt.in)
			if tt.wantErr {
				if !errors.Is(err, typed.ErrInvalidID) {
					t.Fatalf("ParseUserID(%q) error = %v, want ErrInvalidID", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseUserID(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseUserID(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestPart1ParseOrderID(t *testing.T) {
	// A UserID string must not parse as an OrderID: the prefix is the guard
	// against passing one kind of ID where the other belongs.
	tests := []struct {
		name    string
		in      string
		want    typed.OrderID
		wantErr bool
	}{
		{"canonical", "ord_7", 7, false},
		{"user prefix rejected", "usr_7", 0, true},
		{"bare number", "7", 0, true},
		{"zero is unset", "ord_0", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := typed.ParseOrderID(tt.in)
			if tt.wantErr {
				if !errors.Is(err, typed.ErrInvalidID) {
					t.Fatalf("ParseOrderID(%q) error = %v, want ErrInvalidID", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseOrderID(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ParseOrderID(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestPart1StringForms(t *testing.T) {
	if got := fmt.Sprint(typed.UserID(42)); got != "usr_42" {
		t.Errorf("fmt.Sprint(UserID(42)) = %q, want %q", got, "usr_42")
	}
	if got := fmt.Sprint(typed.OrderID(7)); got != "ord_7" {
		t.Errorf("fmt.Sprint(OrderID(7)) = %q, want %q", got, "ord_7")
	}
	// %d bypasses String: the verb decides, not the Stringer.
	if got := fmt.Sprintf("%d", typed.UserID(42)); got != "42" {
		t.Errorf("Sprintf(%%d, UserID(42)) = %q, want %q", got, "42")
	}
	// String never fails, even for values that cannot be marshalled.
	if got := typed.UserID(-3).String(); got != "usr_-3" {
		t.Errorf("UserID(-3).String() = %q, want %q", got, "usr_-3")
	}
	id, err := typed.ParseUserID(typed.UserID(9).String())
	if err != nil || id != 9 {
		t.Errorf("round trip of UserID(9) = (%d, %v), want (9, nil)", id, err)
	}
}

func TestPart1MarshalTextRejectsNonPositive(t *testing.T) {
	b, err := typed.UserID(9).MarshalText()
	if err != nil || string(b) != "usr_9" {
		t.Fatalf("UserID(9).MarshalText() = (%q, %v), want (usr_9, nil)", b, err)
	}
	for _, id := range []typed.UserID{0, -1} {
		if _, err := id.MarshalText(); !errors.Is(err, typed.ErrInvalidID) {
			t.Errorf("UserID(%d).MarshalText() error = %v, want ErrInvalidID", id, err)
		}
	}
	ob, err := typed.OrderID(3).MarshalText()
	if err != nil || string(ob) != "ord_3" {
		t.Errorf("OrderID(3).MarshalText() = (%q, %v), want (ord_3, nil)", ob, err)
	}
	if _, err := typed.OrderID(0).MarshalText(); !errors.Is(err, typed.ErrInvalidID) {
		t.Errorf("OrderID(0).MarshalText() error = %v, want ErrInvalidID", err)
	}
}

func TestPart1UnmarshalTextLeavesReceiverOnError(t *testing.T) {
	id := typed.UserID(5)
	err := id.UnmarshalText([]byte("nope"))
	if !errors.Is(err, typed.ErrInvalidID) {
		t.Fatalf("UnmarshalText(nope) error = %v, want ErrInvalidID", err)
	}
	if id != 5 {
		t.Errorf("after failed UnmarshalText id = %d, want 5 (unchanged)", id)
	}
	if err := id.UnmarshalText([]byte("usr_8")); err != nil || id != 8 {
		t.Errorf("UnmarshalText(usr_8) -> id=%d, err=%v; want 8, nil", id, err)
	}
}

type envelope struct {
	Owner typed.UserID  `json:"owner"`
	Order typed.OrderID `json:"order"`
}

func TestPart1JSONRoundTrip(t *testing.T) {
	in := envelope{Owner: 42, Order: 7}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal(%+v): %v", in, err)
	}
	if want := `{"owner":"usr_42","order":"ord_7"}`; string(b) != want {
		t.Errorf("Marshal = %s, want %s", b, want)
	}
	var out envelope
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal(%s): %v", b, err)
	}
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("round trip mismatch (-want +got):\n%s", diff)
	}
}

func TestPart1JSONMapKeys(t *testing.T) {
	// encoding/json uses MarshalText for non-string map keys too.
	in := map[typed.UserID]string{1: "a", 2: "b"}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"usr_1":"a","usr_2":"b"}`; string(b) != want {
		t.Errorf("Marshal = %s, want %s", b, want)
	}
	var out map[typed.UserID]string
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal(%s): %v", b, err)
	}
	if diff := cmp.Diff(in, out); diff != "" {
		t.Errorf("map round trip mismatch (-want +got):\n%s", diff)
	}
}

func TestPart1JSONRejectsZeroAndNumericIDs(t *testing.T) {
	// Valid IDs marshal, so the zero-value rejection below is meaningful.
	if _, err := json.Marshal(envelope{Owner: 1, Order: 1}); err != nil {
		t.Fatalf("Marshal of valid envelope: %v", err)
	}
	if _, err := json.Marshal(envelope{Owner: 0, Order: 7}); !errors.Is(err, typed.ErrInvalidID) {
		t.Errorf("Marshal of unset owner error = %v, want ErrInvalidID", err)
	}

	var e envelope
	if err := json.Unmarshal([]byte(`{"owner":"usr_3","order":"ord_4"}`), &e); err != nil {
		t.Fatalf("Unmarshal of valid text IDs: %v", err)
	}
	if e.Owner != 3 || e.Order != 4 {
		t.Errorf("decoded %+v, want owner=3 order=4", e)
	}
	if err := json.Unmarshal([]byte(`{"owner":42,"order":"ord_4"}`), &e); err == nil {
		t.Error("Unmarshal of numeric owner: want error, got nil")
	}
}
