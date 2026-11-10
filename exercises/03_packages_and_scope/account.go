// Package scope is the Go packages-and-scope exercise: encapsulation through
// exported and unexported identifiers, an internal/ helper package, closures,
// shadowing, instance registries versus package state, a scoped test hook and
// a whitelist-only formula evaluator. See README.md for the per-part spec.
package scope

import "errors"

var (
	// ErrInvalidArgument reports a caller-supplied value outside the documented domain.
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrInsufficientFunds reports a withdrawal larger than the current balance.
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// Kind is the product type of an account. The zero value is deliberately not
// a valid kind so that an uninitialised Kind can be detected.
type Kind int

const (
	KindChecking Kind = iota + 1
	KindSavings
	KindLoan
)

// String returns "checking", "savings" or "loan"; any other value renders as "Kind(n)".
func (k Kind) String() string {
	return ""
}

// ParseKind is the inverse of String. Any other input returns ErrInvalidArgument.
func ParseKind(s string) (Kind, error) {
	return 0, errors.New("not implemented")
}

// Entry is one posted movement on an account.
type Entry struct {
	Delta   int64 // positive for deposits, negative for withdrawals
	Balance int64 // balance after the movement
}

// Account is a ledger account. Its state is unexported so that every change
// goes through a method that keeps the balance and history consistent.
type Account struct {
	// The unexported fields are yours to declare.
}

// NewAccount opens an account of the given kind. seq must be in 1..99999999.
// The account number is seq zero-padded to 8 digits followed by a Luhn check
// digit from internal/checksum, 9 digits in total. An invalid kind or seq
// returns ErrInvalidArgument.
func NewAccount(kind Kind, seq int) (*Account, error) {
	return nil, errors.New("not implemented")
}

// Number returns the 9-digit account number.
func (a *Account) Number() string { return "" }

// Kind returns the account kind.
func (a *Account) Kind() Kind { return 0 }

// Balance returns the current balance in minor units.
func (a *Account) Balance() int64 { return 0 }

// Deposit adds amount, which must be positive, and returns the new balance.
// A non-positive amount returns ErrInvalidArgument.
func (a *Account) Deposit(amount int64) (int64, error) {
	return 0, errors.New("not implemented")
}

// Withdraw removes amount and returns the new balance. A non-positive amount
// returns ErrInvalidArgument; an amount above the balance returns
// ErrInsufficientFunds. On any error the balance and history are unchanged.
func (a *Account) Withdraw(amount int64) (int64, error) {
	return 0, errors.New("not implemented")
}

// History returns every movement in order. The result is a copy: mutating it
// must not affect the account.
func (a *Account) History() []Entry { return nil }
