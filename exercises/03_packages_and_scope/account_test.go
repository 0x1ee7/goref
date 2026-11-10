package scope_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/03_packages_and_scope"
	"goref/exercises/03_packages_and_scope/internal/checksum"
)

// openAccount opens a checking-style account with sequence 1 or fails the test.
func openAccount(t *testing.T, k scope.Kind) *scope.Account {
	t.Helper()
	a, err := scope.NewAccount(k, 1)
	if err != nil {
		t.Fatalf("NewAccount(%v, 1): unexpected error: %v", k, err)
	}
	if a == nil {
		t.Fatal("NewAccount(...) returned nil account without error")
	}
	return a
}

func TestPart1Checksum(t *testing.T) {
	t.Run("Digit", func(t *testing.T) {
		got, err := checksum.Digit("7992739871")
		if err != nil || got != '3' {
			t.Fatalf("Digit(%q) = %q, %v; want '3', nil", "7992739871", got, err)
		}
		got, err = checksum.Digit("00000001")
		if err != nil || got != '8' {
			t.Errorf("Digit(%q) = %q, %v; want '8', nil", "00000001", got, err)
		}
	})

	t.Run("Append", func(t *testing.T) {
		got, err := checksum.Append("7992739871")
		if err != nil || got != "79927398713" {
			t.Fatalf("Append(%q) = %q, %v; want %q, nil", "7992739871", got, err, "79927398713")
		}
		got, err = checksum.Append("00000001")
		if err != nil || got != "000000018" {
			t.Errorf("Append(%q) = %q, %v; want %q, nil", "00000001", got, err, "000000018")
		}
	})

	t.Run("Valid", func(t *testing.T) {
		if !checksum.Valid("79927398713") {
			t.Fatal("Valid(79927398713) = false; want true")
		}
		cases := []struct {
			number string
			want   bool
		}{
			{"79927398713", true},
			{"79927398710", false},
			{"000000018", true},
			{"000000019", false},
		}
		for _, tc := range cases {
			if got := checksum.Valid(tc.number); got != tc.want {
				t.Errorf("Valid(%q) = %v; want %v", tc.number, got, tc.want)
			}
		}
	})

	t.Run("rejects bad input", func(t *testing.T) {
		if got, err := checksum.Append("00000001"); err != nil || got != "000000018" {
			t.Fatalf("Append(%q) = %q, %v; want %q, nil", "00000001", got, err, "000000018")
		}
		for _, in := range []string{"", "12a4", "-1", " 12"} {
			if _, err := checksum.Digit(in); err == nil {
				t.Errorf("Digit(%q): want error", in)
			}
			if _, err := checksum.Append(in); err == nil {
				t.Errorf("Append(%q): want error", in)
			}
		}
		for _, in := range []string{"", "5"} {
			if checksum.Valid(in) {
				t.Errorf("Valid(%q) = true; want false for fewer than two digits", in)
			}
		}
	})
}

func TestPart1Account(t *testing.T) {
	t.Run("number", func(t *testing.T) {
		a, err := scope.NewAccount(scope.KindChecking, 1)
		if err != nil || a == nil {
			t.Fatalf("NewAccount(KindChecking, 1) = %v, %v; want account, nil", a, err)
		}
		if got := a.Number(); got != "000000018" {
			t.Errorf("Number() = %q; want %q", got, "000000018")
		}
	})

	t.Run("largest sequence keeps nine digits with valid check", func(t *testing.T) {
		openAccount(t, scope.KindLoan)
		a, err := scope.NewAccount(scope.KindLoan, 99999999)
		if err != nil {
			t.Fatalf("NewAccount(KindLoan, 99999999): unexpected error: %v", err)
		}
		num := a.Number()
		if len(num) != 9 || num[:8] != "99999999" {
			t.Errorf("Number() = %q; want 8 digits 99999999 followed by a check digit", num)
		}
		if !checksum.Valid(num) {
			t.Errorf("Number() = %q does not pass checksum.Valid", num)
		}
	})

	t.Run("kind and opening balance", func(t *testing.T) {
		a := openAccount(t, scope.KindSavings)
		if got := a.Kind(); got != scope.KindSavings {
			t.Errorf("Kind() = %v; want %v", got, scope.KindSavings)
		}
		if got := a.Balance(); got != 0 {
			t.Errorf("Balance() = %d; want 0 for a new account", got)
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		cases := []struct {
			name string
			kind scope.Kind
			seq  int
		}{
			{"seq zero", scope.KindChecking, 0},
			{"seq negative", scope.KindSavings, -1},
			{"seq too large", scope.KindLoan, 100000000},
			{"zero kind", scope.Kind(0), 1},
			{"unknown kind", scope.Kind(99), 1},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := scope.NewAccount(tc.kind, tc.seq)
				if !errors.Is(err, scope.ErrInvalidArgument) {
					t.Errorf("NewAccount(%v, %d) error = %v; want ErrInvalidArgument", tc.kind, tc.seq, err)
				}
			})
		}
	})
}

func TestPart1AccountOperations(t *testing.T) {
	t.Run("deposit and withdraw", func(t *testing.T) {
		a := openAccount(t, scope.KindChecking)
		bal, err := a.Deposit(100)
		if err != nil || bal != 100 {
			t.Fatalf("Deposit(100) = %d, %v; want 100, nil", bal, err)
		}
		bal, err = a.Withdraw(30)
		if err != nil || bal != 70 {
			t.Errorf("Withdraw(30) = %d, %v; want 70, nil", bal, err)
		}
		if got := a.Balance(); got != 70 {
			t.Errorf("Balance() = %d; want 70", got)
		}
	})

	t.Run("overdraft rejected and exact withdrawal allowed", func(t *testing.T) {
		a := openAccount(t, scope.KindChecking)
		if _, err := a.Deposit(50); err != nil {
			t.Fatalf("Deposit(50): unexpected error: %v", err)
		}
		if _, err := a.Withdraw(51); !errors.Is(err, scope.ErrInsufficientFunds) {
			t.Errorf("Withdraw(51) error = %v; want ErrInsufficientFunds", err)
		}
		if got := a.Balance(); got != 50 {
			t.Errorf("Balance() after rejected overdraft = %d; want 50", got)
		}
		bal, err := a.Withdraw(50)
		if err != nil || bal != 0 {
			t.Errorf("Withdraw(50) = %d, %v; want 0, nil", bal, err)
		}
	})

	t.Run("non-positive amounts rejected", func(t *testing.T) {
		a := openAccount(t, scope.KindChecking)
		if _, err := a.Deposit(10); err != nil {
			t.Fatalf("Deposit(10): unexpected error: %v", err)
		}
		for _, amt := range []int64{0, -1} {
			if _, err := a.Deposit(amt); !errors.Is(err, scope.ErrInvalidArgument) {
				t.Errorf("Deposit(%d) error = %v; want ErrInvalidArgument", amt, err)
			}
		}
		for _, amt := range []int64{0, -5} {
			if _, err := a.Withdraw(amt); !errors.Is(err, scope.ErrInvalidArgument) {
				t.Errorf("Withdraw(%d) error = %v; want ErrInvalidArgument", amt, err)
			}
		}
		if got := a.Balance(); got != 10 {
			t.Errorf("Balance() = %d; want 10 after rejected calls", got)
		}
	})

	t.Run("history records movements and ignores failures", func(t *testing.T) {
		a := openAccount(t, scope.KindChecking)
		if _, err := a.Deposit(100); err != nil {
			t.Fatalf("Deposit(100): unexpected error: %v", err)
		}
		if _, err := a.Withdraw(30); err != nil {
			t.Fatalf("Withdraw(30): unexpected error: %v", err)
		}
		if _, err := a.Withdraw(500); err == nil {
			t.Fatal("Withdraw(500): want error")
		}
		want := []scope.Entry{{Delta: 100, Balance: 100}, {Delta: -30, Balance: 70}}
		if diff := cmp.Diff(want, a.History()); diff != "" {
			t.Errorf("History() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("history is a copy", func(t *testing.T) {
		a := openAccount(t, scope.KindChecking)
		if _, err := a.Deposit(10); err != nil {
			t.Fatalf("Deposit(10): unexpected error: %v", err)
		}
		h := a.History()
		if len(h) != 1 {
			t.Fatalf("History() len = %d; want 1", len(h))
		}
		h[0].Delta = 999
		if got := a.History(); len(got) != 1 || got[0].Delta != 10 {
			t.Errorf("History() after caller mutation = %+v; want the original entry {Delta:10}", got)
		}
	})
}

func TestPart1Kind(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		if got := scope.KindChecking.String(); got != "checking" {
			t.Fatalf("KindChecking.String() = %q; want %q", got, "checking")
		}
		cases := map[scope.Kind]string{
			scope.KindSavings: "savings",
			scope.KindLoan:    "loan",
			scope.Kind(99):    "Kind(99)",
		}
		for k, want := range cases {
			if got := k.String(); got != want {
				t.Errorf("Kind(%d).String() = %q; want %q", int(k), got, want)
			}
		}
	})

	t.Run("zero value is not a valid kind", func(t *testing.T) {
		if got := scope.KindLoan.String(); got != "loan" {
			t.Fatalf("KindLoan.String() = %q; want %q", got, "loan")
		}
		var k scope.Kind
		if got := k.String(); got != "Kind(0)" {
			t.Errorf("zero Kind String() = %q; want %q", got, "Kind(0)")
		}
	})

	t.Run("ParseKind round trip", func(t *testing.T) {
		if k, err := scope.ParseKind("savings"); err != nil || k != scope.KindSavings {
			t.Fatalf("ParseKind(%q) = %v, %v; want KindSavings, nil", "savings", k, err)
		}
		for _, k := range []scope.Kind{scope.KindChecking, scope.KindSavings, scope.KindLoan} {
			got, err := scope.ParseKind(k.String())
			if err != nil || got != k {
				t.Errorf("ParseKind(%q) = %v, %v; want %v, nil", k.String(), got, err, k)
			}
		}
	})

	t.Run("ParseKind rejects unknown names", func(t *testing.T) {
		for _, in := range []string{"", "Savings", "CHECKING", "mortgage", "Kind(1)"} {
			if _, err := scope.ParseKind(in); !errors.Is(err, scope.ErrInvalidArgument) {
				t.Errorf("ParseKind(%q) error = %v; want ErrInvalidArgument", in, err)
			}
		}
	})
}
