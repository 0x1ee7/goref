package structs

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	// ErrInvalidOwner reports a blank account owner.
	ErrInvalidOwner = errors.New("invalid account owner")
	// ErrNonPositiveAmount reports a deposit or withdrawal of zero or less.
	ErrNonPositiveAmount = errors.New("amount must be positive")
	// ErrInsufficientFunds reports a withdrawal larger than the balance plus any overdraft.
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrWithdrawalLimit reports a savings withdrawal past the period limit.
	ErrWithdrawalLimit = errors.New("withdrawal limit reached for this period")
)

// Account is the behaviour shared by every account type. Both *CheckingAccount
// and *SavingsAccount implement it. Value types do not, because their
// methods take pointer receivers.
type Account interface {
	Owner() string
	Balance() Money
	Deposit(Money) error
	Withdraw(Money) error
}

// ledger holds the state shared by all accounts: owner, balance and a mutex.
// It is embedded by value in each account type, so the mutex is part of the
// account and is never copied. Accounts must be used through a pointer.
type ledger struct {
	mu      sync.Mutex
	owner   string
	balance Money
}

// init sets the owner and opening balance. It is called once, by a constructor,
// before the ledger is shared.
func (l *ledger) init(owner string, opening Money) error {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return ErrInvalidOwner
	}
	if !validCurrency(opening.currency) {
		return fmt.Errorf("%w: %q", ErrInvalidCurrency, opening.currency)
	}
	if opening.minor < 0 {
		return fmt.Errorf("%w: negative opening balance", ErrInvalidArgument)
	}
	l.owner = owner
	l.balance = opening
	return nil
}

// Owner returns the account owner. It is set at construction and never changes,
// so it needs no lock.
func (l *ledger) Owner() string { return l.owner }

// Balance returns the current balance.
func (l *ledger) Balance() Money {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.balance
}

// Deposit adds a positive amount in the account's currency.
func (l *ledger) Deposit(amount Money) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if amount.currency != l.balance.currency {
		return fmt.Errorf("%w: account is %q, deposit is %q", ErrCurrencyMismatch, l.balance.currency, amount.currency)
	}
	if amount.minor <= 0 {
		return ErrNonPositiveAmount
	}
	next, err := l.balance.Add(amount)
	if err != nil {
		return err
	}
	l.balance = next
	return nil
}

// withdrawLocked is the shared withdrawal rule. The caller must hold l.mu. The
// withdrawal succeeds if amount <= balance + overdraft. Overdraft is zero for
// accounts that do not allow overdrafts.
func (l *ledger) withdrawLocked(amount, overdraft Money) error {
	if amount.currency != l.balance.currency {
		return fmt.Errorf("%w: account is %q, withdrawal is %q", ErrCurrencyMismatch, l.balance.currency, amount.currency)
	}
	if amount.minor <= 0 {
		return ErrNonPositiveAmount
	}
	available, err := l.balance.Add(overdraft)
	if err != nil {
		return err
	}
	if available.minor < amount.minor {
		return ErrInsufficientFunds
	}
	next, err := l.balance.Sub(amount)
	if err != nil {
		return err
	}
	l.balance = next
	return nil
}

// CheckingAccount is a day-to-day account. It allows withdrawals to go below
// zero, down to -overdraft.
type CheckingAccount struct {
	ledger
	overdraft Money
}

// NewCheckingAccount opens a checking account. A zero-value overdraft means no
// overdraft is allowed. Otherwise overdraft must match the opening currency and
// must not be negative.
func NewCheckingAccount(owner string, opening, overdraft Money) (*CheckingAccount, error) {
	a := new(CheckingAccount)
	if err := a.init(owner, opening); err != nil {
		return nil, err
	}
	if overdraft.currency == "" && overdraft.minor == 0 {
		overdraft = Money{currency: opening.currency}
	}
	if overdraft.currency != opening.currency {
		return nil, fmt.Errorf("%w: overdraft is %q, account is %q", ErrCurrencyMismatch, overdraft.currency, opening.currency)
	}
	if overdraft.minor < 0 {
		return nil, fmt.Errorf("%w: negative overdraft", ErrInvalidArgument)
	}
	a.overdraft = overdraft
	return a, nil
}

// Overdraft returns the overdraft limit, in the account's currency.
func (a *CheckingAccount) Overdraft() Money { return a.overdraft }

// Withdraw takes amount out of the account, allowing the balance to go down to
// -overdraft. It overrides the shared rule, which does not allow overdrafts.
func (a *CheckingAccount) Withdraw(amount Money) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.withdrawLocked(amount, a.overdraft)
}

// SavingsAccount caps the number of withdrawals per period and never overdraws.
// Only successful withdrawals count toward the limit.
type SavingsAccount struct {
	ledger
	max   int
	count int
}

// NewSavingsAccount opens a savings account allowing maxWithdrawals withdrawals
// per period. maxWithdrawals must not be negative.
func NewSavingsAccount(owner string, opening Money, maxWithdrawals int) (*SavingsAccount, error) {
	if maxWithdrawals < 0 {
		return nil, fmt.Errorf("%w: negative withdrawal limit", ErrInvalidArgument)
	}
	s := new(SavingsAccount)
	if err := s.init(owner, opening); err != nil {
		return nil, err
	}
	s.max = maxWithdrawals
	return s, nil
}

// MaxWithdrawals returns the withdrawal limit per period.
func (s *SavingsAccount) MaxWithdrawals() int { return s.max }

// WithdrawalsThisPeriod returns the number of successful withdrawals since the
// last ResetPeriod.
func (s *SavingsAccount) WithdrawalsThisPeriod() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// ResetPeriod starts a new period, clearing the withdrawal count.
func (s *SavingsAccount) ResetPeriod() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count = 0
}

// Withdraw takes amount out of the account. It checks the period limit first,
// then applies the no-overdraft rule. A failed withdrawal is not counted.
func (s *SavingsAccount) Withdraw(amount Money) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count >= s.max {
		return ErrWithdrawalLimit
	}
	if err := s.withdrawLocked(amount, Money{currency: s.balance.currency}); err != nil {
		return err
	}
	s.count++
	return nil
}

var (
	_ Account = (*CheckingAccount)(nil)
	_ Account = (*SavingsAccount)(nil)
)
