package interfaces_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/08_interfaces"
)

// mustRejectEmpty guards every "returns nil" case: a validator that accepts
// everything would otherwise pass vacuously.
func mustRejectEmpty(t *testing.T) {
	t.Helper()
	if err := interfaces.ValidateUser(interfaces.User{}); err == nil {
		t.Fatal("sanity: ValidateUser(User{}) must return an error")
	}
}

func TestPart6ValidateUserTrueNil(t *testing.T) {
	t.Run("valid user returns an untyped nil", func(t *testing.T) {
		mustRejectEmpty(t)
		u := interfaces.User{Name: "ann", Age: 30, Email: "ann@example.com"}
		if err := interfaces.ValidateUser(u); err != nil {
			t.Fatalf("ValidateUser(valid) = %#v, want untyped nil", err)
		}
	})

	t.Run("invalid user returns *ErrorList with every failure in field order", func(t *testing.T) {
		u := interfaces.User{Name: "", Age: 200, Email: "nope"}
		var list *interfaces.ErrorList
		if !errors.As(interfaces.ValidateUser(u), &list) {
			t.Fatalf("ValidateUser(%+v) is not a *ErrorList", u)
		}
		var got []string
		for _, fe := range list.Errors() {
			got = append(got, fe.Field+": "+fe.Reason)
		}
		want := []string{"Name: is required", "Age: must be between 0 and 150", "Email: must contain '@'"}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("findings mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("single failure is reported alone", func(t *testing.T) {
		mustRejectEmpty(t)
		u := interfaces.User{Name: "ann", Age: 3, Email: ""}
		var list *interfaces.ErrorList
		if !errors.As(interfaces.ValidateUser(u), &list) {
			t.Fatalf("ValidateUser(%+v) is not a *ErrorList", u)
		}
		if list.Len() != 1 || list.Errors()[0].Field != "Email" {
			t.Errorf("Len = %d, errors = %+v; want one Email finding", list.Len(), list.Errors())
		}
	})

	t.Run("error text names each failing field", func(t *testing.T) {
		err := interfaces.ValidateUser(interfaces.User{})
		if err == nil {
			t.Fatal("ValidateUser(User{}) = nil, want error")
		}
		for _, frag := range []string{"Name: is required", "Email: is required"} {
			if !strings.Contains(err.Error(), frag) {
				t.Errorf("error text %q lacks %q", err.Error(), frag)
			}
		}
	})
}

func TestPart6TypedNilGotcha(t *testing.T) {
	t.Run("typed nil pointer in an interface is detected by IsNilValue", func(t *testing.T) {
		var l *interfaces.ErrorList
		var err error = l // non-nil interface holding a nil pointer
		if !interfaces.IsNilValue(err) {
			t.Error("IsNilValue(typed nil error) = false, want true")
		}
		if interfaces.IsNilValue(&interfaces.ErrorList{}) {
			t.Error("IsNilValue(non-nil *ErrorList) = true, want false")
		}
	})

	t.Run("IsNilValue covers untyped nil and nilable kinds only", func(t *testing.T) {
		var l *interfaces.ErrorList
		var m map[string]int
		var s []int
		cases := []struct {
			name string
			v    any
			want bool
		}{
			{"untyped nil", nil, true},
			{"nil *ErrorList", l, true},
			{"nil map", m, true},
			{"nil slice", s, true},
			{"empty string", "", false},
			{"zero int", 0, false},
			{"non-nil *ErrorList", &interfaces.ErrorList{}, false},
		}
		for _, tc := range cases {
			if got := interfaces.IsNilValue(tc.v); got != tc.want {
				t.Errorf("IsNilValue(%s) = %v, want %v", tc.name, got, tc.want)
			}
		}
	})

	t.Run("methods on a nil *ErrorList are safe", func(t *testing.T) {
		var nilList *interfaces.ErrorList
		if n := nilList.Len(); n != 0 {
			t.Errorf("nil Len = %d, want 0", n)
		}
		if got := nilList.Errors(); len(got) != 0 {
			t.Errorf("nil Errors = %v, want empty", got)
		}
		_ = nilList.Error() // must not panic

		list := &interfaces.ErrorList{}
		list.Add(interfaces.SeverityError, "f", "r")
		if list.Len() != 1 {
			t.Errorf("Len after Add = %d, want 1", list.Len())
		}
	})

	t.Run("Errors returns a copy", func(t *testing.T) {
		list := &interfaces.ErrorList{}
		list.Add(interfaces.SeverityError, "f", "r")
		if got := list.Errors(); len(got) != 1 {
			t.Fatalf("Errors len = %d, want 1", len(got))
		}
		list.Errors()[0].Reason = "changed"
		if got := list.Errors()[0].Reason; got != "r" {
			t.Errorf("Reason after caller mutation = %q, want %q", got, "r")
		}
	})
}

func TestPart6SeverityStringer(t *testing.T) {
	cases := []struct {
		sev  interfaces.Severity
		want string
	}{
		{interfaces.SeverityInfo, "info"},
		{interfaces.SeverityWarning, "warning"},
		{interfaces.SeverityError, "error"},
		{interfaces.Severity(9), "Severity(9)"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := fmt.Sprint(tc.sev); got != tc.want {
				t.Errorf("fmt.Sprint(%d) = %q, want %q", int(tc.sev), got, tc.want)
			}
		})
	}

	t.Run("String is used inside slices and %s", func(t *testing.T) {
		sevs := []interfaces.Severity{interfaces.SeverityInfo, interfaces.SeverityError}
		if got := fmt.Sprint(sevs); got != "[info error]" {
			t.Errorf("Sprint(slice) = %q, want %q", got, "[info error]")
		}
		if got := fmt.Sprintf("%s", interfaces.SeverityWarning); got != "warning" {
			t.Errorf("Sprintf(%%s) = %q, want %q", got, "warning")
		}
	})
}
