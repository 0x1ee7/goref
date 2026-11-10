package structs_test

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	structs "goref/exercises/07_structs_and_methods"
)

func usd(t *testing.T, minor int64) structs.Money {
	t.Helper()
	return mustMoney(t, minor, "USD")
}

func mustChecking(t *testing.T, owner string, opening, overdraft structs.Money) *structs.CheckingAccount {
	t.Helper()
	a, err := structs.NewCheckingAccount(owner, opening, overdraft)
	if err != nil {
		t.Fatalf("NewCheckingAccount(%q): unexpected error: %v", owner, err)
	}
	return a
}

func mustSavings(t *testing.T, owner string, opening structs.Money, limit int) *structs.SavingsAccount {
	t.Helper()
	a, err := structs.NewSavingsAccount(owner, opening, limit)
	if err != nil {
		t.Fatalf("NewSavingsAccount(%q): unexpected error: %v", owner, err)
	}
	return a
}

func TestPart3Constructors(t *testing.T) {
	t.Run("checking valid owner is trimmed", func(t *testing.T) {
		a := mustChecking(t, "  Ada  ", usd(t, 100), usd(t, 0))
		if a.Owner() != "Ada" {
			t.Errorf("Owner() = %q, want %q", a.Owner(), "Ada")
		}
		if a.Balance() != usd(t, 100) {
			t.Errorf("Balance() = %v, want 1.00 USD", a.Balance())
		}
	})

	t.Run("invalid owner", func(t *testing.T) {
		for _, owner := range []string{"", "   "} {
			t.Run(fmt.Sprintf("%q", owner), func(t *testing.T) {
				_ = usd(t, 1)
				if _, err := structs.NewCheckingAccount(owner, usd(t, 0), usd(t, 0)); !errors.Is(err, structs.ErrInvalidOwner) {
					t.Errorf("checking error = %v, want ErrInvalidOwner", err)
				}
				if _, err := structs.NewSavingsAccount(owner, usd(t, 0), 1); !errors.Is(err, structs.ErrInvalidOwner) {
					t.Errorf("savings error = %v, want ErrInvalidOwner", err)
				}
			})
		}
	})

	t.Run("overdraft in another currency", func(t *testing.T) {
		_ = usd(t, 1)
		eur := mustMoney(t, 100, "EUR")
		if _, err := structs.NewCheckingAccount("Ada", usd(t, 0), eur); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("error = %v, want ErrCurrencyMismatch", err)
		}
	})

	t.Run("negative overdraft", func(t *testing.T) {
		_ = usd(t, 1)
		if _, err := structs.NewCheckingAccount("Ada", usd(t, 0), usd(t, -1)); !errors.Is(err, structs.ErrInvalidArgument) {
			t.Errorf("error = %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("negative savings limit", func(t *testing.T) {
		_ = usd(t, 1)
		if _, err := structs.NewSavingsAccount("Bo", usd(t, 0), -1); !errors.Is(err, structs.ErrInvalidArgument) {
			t.Errorf("error = %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("savings accessors", func(t *testing.T) {
		s := mustSavings(t, "Bo", usd(t, 0), 3)
		if s.MaxWithdrawals() != 3 || s.WithdrawalsThisPeriod() != 0 {
			t.Errorf("MaxWithdrawals=%d WithdrawalsThisPeriod=%d, want 3 and 0", s.MaxWithdrawals(), s.WithdrawalsThisPeriod())
		}
	})
}

func TestPart3Checking(t *testing.T) {
	t.Run("zero overdraft means none", func(t *testing.T) {
		a := mustChecking(t, "Ada", usd(t, 100), structs.Money{})
		if err := a.Withdraw(usd(t, 101)); !errors.Is(err, structs.ErrInsufficientFunds) {
			t.Errorf("Withdraw error = %v, want ErrInsufficientFunds", err)
		}
		if a.Balance() != usd(t, 100) {
			t.Errorf("balance changed to %v after failed withdrawal", a.Balance())
		}
	})

	t.Run("overdraft allows balance below zero", func(t *testing.T) {
		a := mustChecking(t, "Ada", usd(t, 1000), usd(t, 2000))
		if err := a.Withdraw(usd(t, 2500)); err != nil {
			t.Fatalf("Withdraw: %v", err)
		}
		if a.Balance() != usd(t, -1500) {
			t.Errorf("Balance() = %v, want -15.00 USD", a.Balance())
		}
		if a.Overdraft() != usd(t, 2000) {
			t.Errorf("Overdraft() = %v, want 20.00 USD", a.Overdraft())
		}
	})

	t.Run("withdraw beyond overdraft", func(t *testing.T) {
		a := mustChecking(t, "Ada", usd(t, 1000), usd(t, 2000))
		if err := a.Withdraw(usd(t, 3001)); !errors.Is(err, structs.ErrInsufficientFunds) {
			t.Errorf("error = %v, want ErrInsufficientFunds", err)
		}
		if a.Balance() != usd(t, 1000) {
			t.Errorf("balance = %v after failed withdrawal, want 10.00 USD", a.Balance())
		}
	})

	t.Run("withdraw non-positive", func(t *testing.T) {
		a := mustChecking(t, "Ada", usd(t, 1000), usd(t, 0))
		for _, amt := range []structs.Money{usd(t, 0), usd(t, -5)} {
			if err := a.Withdraw(amt); !errors.Is(err, structs.ErrNonPositiveAmount) {
				t.Errorf("Withdraw(%v) error = %v, want ErrNonPositiveAmount", amt, err)
			}
		}
	})

	t.Run("deposit", func(t *testing.T) {
		a := mustChecking(t, "Ada", usd(t, 0), usd(t, 0))
		if err := a.Deposit(usd(t, 250)); err != nil {
			t.Fatalf("Deposit: %v", err)
		}
		if a.Balance() != usd(t, 250) {
			t.Errorf("Balance() = %v, want 2.50 USD", a.Balance())
		}
	})

	t.Run("deposit invalid", func(t *testing.T) {
		a := mustChecking(t, "Ada", usd(t, 0), usd(t, 0))
		if err := a.Deposit(usd(t, 0)); !errors.Is(err, structs.ErrNonPositiveAmount) {
			t.Errorf("zero deposit error = %v, want ErrNonPositiveAmount", err)
		}
		if err := a.Deposit(mustMoney(t, 5, "EUR")); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("EUR deposit error = %v, want ErrCurrencyMismatch", err)
		}
		if err := a.Deposit(structs.Money{}); !errors.Is(err, structs.ErrCurrencyMismatch) {
			t.Errorf("zero Money deposit error = %v, want ErrCurrencyMismatch", err)
		}
	})
}

func TestPart3Savings(t *testing.T) {
	t.Run("never overdraws", func(t *testing.T) {
		s := mustSavings(t, "Bo", usd(t, 500), 5)
		if err := s.Withdraw(usd(t, 501)); !errors.Is(err, structs.ErrInsufficientFunds) {
			t.Errorf("error = %v, want ErrInsufficientFunds", err)
		}
		if s.Balance() != usd(t, 500) {
			t.Errorf("balance changed to %v", s.Balance())
		}
	})

	t.Run("limit reached", func(t *testing.T) {
		s := mustSavings(t, "Bo", usd(t, 1000), 2)
		for i := range 2 {
			if err := s.Withdraw(usd(t, 100)); err != nil {
				t.Fatalf("withdrawal %d: %v", i+1, err)
			}
		}
		if err := s.Withdraw(usd(t, 100)); !errors.Is(err, structs.ErrWithdrawalLimit) {
			t.Errorf("third withdrawal error = %v, want ErrWithdrawalLimit", err)
		}
		if s.WithdrawalsThisPeriod() != 2 || s.Balance() != usd(t, 800) {
			t.Errorf("count=%d balance=%v, want 2 and 8.00 USD", s.WithdrawalsThisPeriod(), s.Balance())
		}
	})

	t.Run("limit checked before amount", func(t *testing.T) {
		s := mustSavings(t, "Bo", usd(t, 1000), 0)
		if err := s.Withdraw(usd(t, 0)); !errors.Is(err, structs.ErrWithdrawalLimit) {
			t.Errorf("zero-limit withdrawal of 0 error = %v, want ErrWithdrawalLimit", err)
		}
	})

	t.Run("failed withdrawals are not counted", func(t *testing.T) {
		s := mustSavings(t, "Bo", usd(t, 100), 1)
		if err := s.Withdraw(usd(t, 500)); !errors.Is(err, structs.ErrInsufficientFunds) {
			t.Fatalf("error = %v, want ErrInsufficientFunds", err)
		}
		if err := s.Withdraw(usd(t, -1)); !errors.Is(err, structs.ErrNonPositiveAmount) {
			t.Fatalf("error = %v, want ErrNonPositiveAmount", err)
		}
		if s.WithdrawalsThisPeriod() != 0 {
			t.Errorf("WithdrawalsThisPeriod = %d, want 0", s.WithdrawalsThisPeriod())
		}
		if err := s.Withdraw(usd(t, 100)); err != nil {
			t.Errorf("valid withdrawal after failures: %v", err)
		}
	})

	t.Run("reset period", func(t *testing.T) {
		s := mustSavings(t, "Bo", usd(t, 1000), 1)
		if err := s.Withdraw(usd(t, 1)); err != nil {
			t.Fatalf("Withdraw: %v", err)
		}
		s.ResetPeriod()
		if s.WithdrawalsThisPeriod() != 0 {
			t.Errorf("WithdrawalsThisPeriod = %d after reset, want 0", s.WithdrawalsThisPeriod())
		}
		if err := s.Withdraw(usd(t, 1)); err != nil {
			t.Errorf("withdrawal after reset: %v", err)
		}
	})
}

func TestPart3Embedding(t *testing.T) {
	t.Run("promoted methods work through the interface", func(t *testing.T) {
		c := mustChecking(t, "Ada", usd(t, 100), usd(t, 0))
		var acct structs.Account = c
		if err := acct.Deposit(usd(t, 50)); err != nil {
			t.Fatalf("Deposit via interface: %v", err)
		}
		if acct.Owner() != "Ada" || acct.Balance() != usd(t, 150) {
			t.Errorf("owner=%q balance=%v, want Ada and 1.50 USD", acct.Owner(), acct.Balance())
		}
	})

	t.Run("overriding Withdraw changes behaviour per type", func(t *testing.T) {
		c := mustChecking(t, "Ada", usd(t, 100), usd(t, 1000))
		s := mustSavings(t, "Bo", usd(t, 100), 5)
		var checking structs.Account = c
		if err := checking.Withdraw(usd(t, 200)); err != nil {
			t.Fatalf("checking Withdraw(200) = %v, want overdraft allowed", err)
		}
		if got := c.Balance(); got != usd(t, -100) {
			t.Errorf("checking balance = %v, want -1.00 USD", got)
		}
		var savings structs.Account = s
		if err := savings.Withdraw(usd(t, 200)); !errors.Is(err, structs.ErrInsufficientFunds) {
			t.Errorf("savings Withdraw(200) error = %v, want ErrInsufficientFunds", err)
		}
		if got := s.Balance(); got != usd(t, 100) {
			t.Errorf("savings balance = %v, want 1.00 USD", got)
		}
	})

	t.Run("pointer types satisfy Account, value types do not", func(t *testing.T) {
		_ = usd(t, 1)
		accountType := reflect.TypeFor[structs.Account]()
		for _, typ := range []reflect.Type{reflect.TypeFor[*structs.CheckingAccount](), reflect.TypeFor[*structs.SavingsAccount]()} {
			if !typ.Implements(accountType) {
				t.Errorf("%v does not implement Account", typ)
			}
		}
		for _, typ := range []reflect.Type{reflect.TypeFor[structs.CheckingAccount](), reflect.TypeFor[structs.SavingsAccount]()} {
			if typ.Implements(accountType) {
				t.Errorf("value type %v implements Account; pointer receivers should prevent it", typ)
			}
		}
	})
}

func TestPart3Concurrency(t *testing.T) {
	t.Run("concurrent deposits all land", func(t *testing.T) {
		c := mustChecking(t, "Ada", usd(t, 0), usd(t, 0))
		var wg sync.WaitGroup
		for range 50 {
			wg.Go(func() {
				if err := c.Deposit(usd(t, 1)); err != nil {
					t.Errorf("Deposit: %v", err)
				}
			})
		}
		wg.Wait()
		if c.Balance() != usd(t, 50) {
			t.Errorf("Balance() = %v, want 0.50 USD", c.Balance())
		}
	})

	t.Run("concurrent withdrawals respect the period limit", func(t *testing.T) {
		s := mustSavings(t, "Bo", usd(t, 1000), 10)
		var ok, limited atomic.Int64
		var wg sync.WaitGroup
		for range 50 {
			wg.Go(func() {
				switch err := s.Withdraw(usd(t, 1)); {
				case err == nil:
					ok.Add(1)
				case errors.Is(err, structs.ErrWithdrawalLimit):
					limited.Add(1)
				default:
					t.Errorf("Withdraw: unexpected error %v", err)
				}
			})
		}
		wg.Wait()
		if ok.Load() != 10 || limited.Load() != 40 {
			t.Errorf("ok=%d limited=%d, want 10 and 40", ok.Load(), limited.Load())
		}
		if s.WithdrawalsThisPeriod() != 10 || s.Balance() != usd(t, 990) {
			t.Errorf("count=%d balance=%v, want 10 and 9.90 USD", s.WithdrawalsThisPeriod(), s.Balance())
		}
	})
}
