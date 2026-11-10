package goroutines

import (
	"errors"
)

// Errors returned by the account and transfer functions.
var (
	ErrInvalidAmount     = errors.New("shared: amount must be positive")
	ErrInsufficientFunds = errors.New("shared: insufficient funds")
	ErrSameAccount       = errors.New("shared: source and destination are the same account")
)

// Counter is a set of named int64 counters. The zero value is ready to use and
// it is safe for concurrent use. A Counter must not be copied after first use.
type Counter struct {
	// engineer-owned fields
}

// Add adds delta to the named counter (creating it at 0 if needed) and returns
// the new value. Negative deltas are allowed.
func (c *Counter) Add(name string, delta int64) int64 {
	return 0
}

// Get returns the named counter's value, or 0 for an unknown name.
func (c *Counter) Get(name string) int64 {
	return 0
}

// Snapshot returns an independent copy of every counter.
func (c *Counter) Snapshot() map[string]int64 {
	return nil
}

// Account is a balance guarded by its own lock. Create one with NewAccount;
// it must not be copied after first use.
type Account struct {
	// engineer-owned fields
}

// NewAccount returns an account with the given opening balance. It returns
// ErrInvalidAmount if balance is negative.
func NewAccount(id string, balance int64) (*Account, error) {
	return nil, errors.New("not implemented")
}

// ID returns the account identifier.
func (a *Account) ID() string {
	return ""
}

// Balance returns the current balance.
func (a *Account) Balance() int64 {
	return 0
}

// Deposit adds amount to the balance. It returns ErrInvalidAmount if amount <= 0.
func (a *Account) Deposit(amount int64) error {
	return errors.New("not implemented")
}

// Withdraw subtracts amount from the balance. It returns ErrInvalidAmount if
// amount <= 0 and ErrInsufficientFunds if amount exceeds the balance; on error
// the balance is unchanged.
func (a *Account) Withdraw(amount int64) error {
	return errors.New("not implemented")
}

// Transfer moves amount from src to dst as one atomic step: no goroutine can
// observe the money in flight. It returns ErrInvalidAmount for amount <= 0,
// ErrSameAccount when src == dst, and ErrInsufficientFunds when src lacks funds;
// on any error neither account changes. Locks are acquired in one global order,
// so concurrent transfers in opposite directions cannot deadlock.
func Transfer(src, dst *Account, amount int64) error {
	return errors.New("not implemented")
}

// Registry maps service names to addresses. It is read-mostly: many goroutines
// call Lookup while Register is rare. The zero value is ready to use and it is
// safe for concurrent use. A Registry must not be copied after first use.
type Registry struct {
	// engineer-owned fields
}

// Register sets the address for name, replacing any previous value.
func (r *Registry) Register(name, addr string) {}

// Lookup returns the address registered for name.
func (r *Registry) Lookup(name string) (string, bool) {
	return "", false
}

// Names returns every registered name in ascending order.
func (r *Registry) Names() []string {
	return nil
}

// PermanentOnce returns a function that calls load at most once. Both its
// result and its error are cached forever, which is exactly sync.OnceValues.
// Concurrent callers block until the first call finishes and then share its
// outcome.
func PermanentOnce[T any](load func() (T, error)) func() (T, error) {
	return func() (T, error) {
		var zero T
		return zero, errors.New("not implemented")
	}
}

// Lazy caches a successful load. A failed load is not cached: the error is
// returned to that caller and the next Get calls load again. Concurrent Get
// calls are serialised, so load runs once per successful value.
type Lazy[T any] struct {
	// engineer-owned fields
}

// NewLazy returns a Lazy that loads its value with load.
func NewLazy[T any](load func() (T, error)) *Lazy[T] {
	return &Lazy[T]{}
}

// Get returns the cached value, loading it first if necessary.
func (l *Lazy[T]) Get() (T, error) {
	var zero T
	return zero, errors.New("not implemented")
}
