package structs

import "errors"

var (
	// ErrInvalidOwner reports an empty or blank account owner.
	ErrInvalidOwner = errors.New("invalid owner")
	// ErrNonPositiveAmount reports a deposit or withdrawal of zero or less.
	ErrNonPositiveAmount = errors.New("amount must be positive")
	// ErrInsufficientFunds reports a withdrawal larger than the available funds.
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrWithdrawalLimit reports a savings withdrawal after the period limit.
	ErrWithdrawalLimit = errors.New("withdrawal limit reached for this period")
)

// Account is the behaviour shared by every account kind. It is satisfied by the
// pointers *CheckingAccount and *SavingsAccount, which hold mutable state.
type Account interface {
	Owner() string
	Balance() Money
	Deposit(amount Money) error
	Withdraw(amount Money) error
}

// ledger is the state and behaviour common to all accounts. It is embedded in
// the concrete account types; that is composition, not inheritance: ledger
// knows nothing about the types that embed it.
//
// ledger holds a sync.Mutex, so it must be initialised in place and never copied
// after first use.
type ledger struct{}

// Owner returns the account owner.
func (l *ledger) Owner() string { return "" }

// Balance returns the current balance.
func (l *ledger) Balance() Money { return Money{} }

// Deposit adds a strictly positive amount in the account currency.
func (l *ledger) Deposit(amount Money) error { return errNotImplemented }

// Withdraw is the strict base rule: the balance may not go below zero.
func (l *ledger) Withdraw(amount Money) error { return errNotImplemented }

// CheckingAccount may overdraw down to -overdraft.
type CheckingAccount struct {
	ledger
}

var _ Account = (*CheckingAccount)(nil)

// NewCheckingAccount opens a checking account. A zero-value overdraft means no
// overdraft. A non-zero overdraft must be >= 0 and in the opening currency.
func NewCheckingAccount(owner string, opening, overdraft Money) (*CheckingAccount, error) {
	return nil, errNotImplemented
}

// Overdraft returns the overdraft limit in the account currency.
func (a *CheckingAccount) Overdraft() Money { return Money{} }

// Withdraw takes amount if balance + overdraft covers it, otherwise ErrInsufficientFunds.
func (a *CheckingAccount) Withdraw(amount Money) error { return errNotImplemented }

// SavingsAccount allows a fixed number of withdrawals per period and never overdraws.
type SavingsAccount struct {
	ledger
}

var _ Account = (*SavingsAccount)(nil)

// NewSavingsAccount opens a savings account with a withdrawal limit per period.
// maxWithdrawals must be >= 0 (ErrInvalidArgument).
func NewSavingsAccount(owner string, opening Money, maxWithdrawals int) (*SavingsAccount, error) {
	return nil, errNotImplemented
}

// MaxWithdrawals returns the withdrawal limit per period.
func (s *SavingsAccount) MaxWithdrawals() int { return 0 }

// WithdrawalsThisPeriod returns the successful withdrawals since the last ResetPeriod.
func (s *SavingsAccount) WithdrawalsThisPeriod() int { return 0 }

// ResetPeriod starts a new period and zeroes the withdrawal count.
func (s *SavingsAccount) ResetPeriod() {}

// Withdraw checks the period limit first, then the amount. Only successful withdrawals count.
func (s *SavingsAccount) Withdraw(amount Money) error { return errNotImplemented }
