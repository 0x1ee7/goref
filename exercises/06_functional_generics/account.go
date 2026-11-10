package functional

import "errors"

// Status is the lifecycle state of an account.
type Status string

const (
	StatusOpen   Status = "open"
	StatusFrozen Status = "frozen"
)

// Account is the folded state of an event log. Accounts are treated as values:
// Apply never mutates the account it is given.
type Account struct {
	AccountID    string
	Owner        string
	BalanceCents int64
	Status       Status
}

// Events are plain data. Apply switches on their concrete type.

// Opened creates an account.
type Opened struct {
	AccountID string
	Owner     string
}

// Deposited adds money to an open account.
type Deposited struct{ AmountCents int64 }

// Withdrawn removes money from an open account.
type Withdrawn struct{ AmountCents int64 }

// Frozen blocks deposits and withdrawals. Freezing twice is harmless.
type Frozen struct{}

// Unfrozen restores an account to StatusOpen.
type Unfrozen struct{}

// Sentinel errors returned by Apply and the functions built on it.
var (
	ErrAlreadyOpened     = errors.New("account already opened")
	ErrNotOpened         = errors.New("account not opened")
	ErrNonPositiveAmount = errors.New("amount must be positive")
	ErrAccountFrozen     = errors.New("account is frozen")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrEmptyLog          = errors.New("event log is empty")
	ErrUnknownEvent      = errors.New("unknown event")
)

// Apply is the fold step. acc is nil before the account exists. Apply returns a
// fresh *Account and leaves acc untouched, so a snapshot taken earlier stays
// valid.
func Apply(acc *Account, event any) (*Account, error) {
	return nil, nil
}

// Replay folds a complete log from nothing. An empty log is ErrEmptyLog, because
// there is no account to return.
func Replay(events []any) (*Account, error) {
	return nil, nil
}

// ReplayFrom resumes from a snapshot and applies events on top of it. This is how
// a snapshot avoids replaying the whole log. The returned account is always a
// copy, even when events is empty. Error indexes count from events[0].
func ReplayFrom(snapshot *Account, events []any) (*Account, error) {
	return nil, nil
}

// Snapshots returns the account state after each event, so the result has
// len(events) entries. The last entry equals Replay(events). Every entry is a
// separate value; later events do not change earlier snapshots.
func Snapshots(events []any) ([]*Account, error) {
	return nil, nil
}

// DecideWithdraw is the command half of "decide then evolve". It validates a
// withdrawal against the current state and returns the event to record. It does
// not change acc. The errors are the ones Apply would return.
func DecideWithdraw(acc *Account, amountCents int64) (Withdrawn, error) {
	return Withdrawn{}, nil
}
