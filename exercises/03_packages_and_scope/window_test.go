package scope_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"goref/exercises/03_packages_and_scope"
)

func TestPart3ParseWindow(t *testing.T) {
	t.Run("valid windows", func(t *testing.T) {
		cases := []struct {
			in   string
			want scope.Window
		}{
			{"09:00-17:30", scope.Window{Start: 9 * time.Hour, End: 17*time.Hour + 30*time.Minute}},
			{"00:00-23:59", scope.Window{Start: 0, End: 23*time.Hour + 59*time.Minute}},
			{"12:05-12:06", scope.Window{Start: 12*time.Hour + 5*time.Minute, End: 12*time.Hour + 6*time.Minute}},
		}
		for _, tc := range cases {
			t.Run(tc.in, func(t *testing.T) {
				got, err := scope.ParseWindow(tc.in)
				if err != nil {
					t.Fatalf("ParseWindow(%q): unexpected error: %v", tc.in, err)
				}
				if got != tc.want {
					t.Errorf("ParseWindow(%q) = %+v; want %+v", tc.in, got, tc.want)
				}
			})
		}
	})

	t.Run("String round trip", func(t *testing.T) {
		w, err := scope.ParseWindow("09:00-17:30")
		if err != nil {
			t.Fatalf("ParseWindow(09:00-17:30): unexpected error: %v", err)
		}
		if got := w.String(); got != "09:00-17:30" {
			t.Errorf("String() = %q; want %q", got, "09:00-17:30")
		}
		if got := (scope.Window{}).String(); got != "00:00-00:00" {
			t.Errorf("zero Window String() = %q; want %q", got, "00:00-00:00")
		}
	})

	t.Run("error from the second clock is not swallowed", func(t *testing.T) {
		_, err := scope.ParseWindow("09:00-25:00")
		if err == nil {
			t.Fatal(`ParseWindow("09:00-25:00"): want error, got nil`)
		}
		if !errors.Is(err, scope.ErrInvalidWindow) {
			t.Errorf("error = %v; want it to wrap ErrInvalidWindow", err)
		}
	})
}

func TestPart3RejectsBadWindows(t *testing.T) {
	bad := []string{
		"",
		"0900-1700",
		"09:00",
		"09:00-17:00-18:00",
		"-09:00-17:00",
		"9:00-17:00",
		"09:00-17:0",
		"24:00-24:30",
		"09:60-10:00",
		"17:00-09:00",
		"10:00-10:00",
		"09:00-25:00",
		"ab:cd-ef:gh",
		" 09:00-17:00",
		"09:00-17:00 ",
		"09:00 - 17:00",
	}
	for _, in := range bad {
		t.Run(fmt.Sprintf("%q", in), func(t *testing.T) {
			if _, err := scope.ParseWindow("08:00-09:00"); err != nil {
				t.Fatalf("ParseWindow(08:00-09:00): unexpected error: %v", err)
			}
			_, err := scope.ParseWindow(in)
			if err == nil {
				t.Fatalf("ParseWindow(%q): want error, got nil", in)
			}
			if !errors.Is(err, scope.ErrInvalidWindow) {
				t.Errorf("ParseWindow(%q) error = %v; want it to wrap ErrInvalidWindow", in, err)
			}
			if !strings.Contains(err.Error(), in) {
				t.Errorf("ParseWindow(%q) error = %q; want the input in the message", in, err)
			}
		})
	}
}
