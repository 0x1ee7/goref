// Package scope is the Go packages-and-scope exercise: encapsulation through
// exported and unexported identifiers, an internal/ helper package, closures,
// shadowing, instance registries versus package state, a scoped test hook and
// a whitelist-only formula evaluator. See README.md for the per-part spec.
package scope

import (
	"errors"
	"fmt"
	"slices"

	"goref/exercises/03_packages_and_scope/internal/checksum"
)

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
	switch k {
	case KindChecking:
		return "checking"
	case KindSavings:
		return "savings"
	case KindLoan:
		return "loan"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// ParseKind is the inverse of String. Any other input returns ErrInvalidArgument.
func ParseKind(s string) (Kind, error) {
	switch s {
	case "checking":
		return KindChecking, nil
	case "savings":
		return KindSavings, nil
	case "loan":
		return KindLoan, nil
	}
	return 0, fmt.Errorf("%w: unknown kind %q", ErrInvalidArgument, s)
}

// Entry is one posted movement on an account.
type Entry struct {
	Delta   int64 // positive for deposits, negative for withdrawals
	Balance int64 // balance after the movement
}

// Account is a ledger account. Its state is unexported so that every change
// goes through a method that keeps the balance and history consistent.
type Account struct {
	kind    Kind
	number  string
	balance int64
	history []Entry
}

// NewAccount opens an account of the given kind. seq must be in 1..99999999.
// The account number is seq zero-padded to 8 digits followed by a Luhn check
// digit from internal/checksum, 9 digits in total. An invalid kind or seq
// returns ErrInvalidArgument.
func NewAccount(kind Kind, seq int) (*Account, error) {
	if kind < KindChecking || kind > KindLoan {
		return nil, fmt.Errorf("%w: kind %d", ErrInvalidArgument, int(kind))
	}
	if seq < 1 || seq > 99999999 {
		return nil, fmt.Errorf("%w: seq %d outside 1..99999999", ErrInvalidArgument, seq)
	}
	number, err := checksum.Append(fmt.Sprintf("%08d", seq))
	if err != nil {
		return nil, err
	}
	return &Account{kind: kind, number: number}, nil
}

// Number returns the 9-digit account number.
func (a *Account) Number() string { return a.number }

// Kind returns the account kind.
func (a *Account) Kind() Kind { return a.kind }

// Balance returns the current balance in minor units.
func (a *Account) Balance() int64 { return a.balance }

// Deposit adds amount, which must be positive, and returns the new balance.
// A non-positive amount returns ErrInvalidArgument.
func (a *Account) Deposit(amount int64) (int64, error) {
	if amount <= 0 {
		return a.balance, fmt.Errorf("%w: deposit %d", ErrInvalidArgument, amount)
	}
	a.balance += amount
	a.history = append(a.history, Entry{Delta: amount, Balance: a.balance})
	return a.balance, nil
}

// Withdraw removes amount and returns the new balance. A non-positive amount
// returns ErrInvalidArgument; an amount above the balance returns
// ErrInsufficientFunds. On any error the balance and history are unchanged.
func (a *Account) Withdraw(amount int64) (int64, error) {
	if amount <= 0 {
		return a.balance, fmt.Errorf("%w: withdrawal %d", ErrInvalidArgument, amount)
	}
	if amount > a.balance {
		return a.balance, fmt.Errorf("%w: balance %d, withdrawal %d", ErrInsufficientFunds, a.balance, amount)
	}
	a.balance -= amount
	a.history = append(a.history, Entry{Delta: -amount, Balance: a.balance})
	return a.balance, nil
}

// History returns every movement in order. The result is a copy: mutating it
// must not affect the account.
func (a *Account) History() []Entry { return slices.Clone(a.history) }
