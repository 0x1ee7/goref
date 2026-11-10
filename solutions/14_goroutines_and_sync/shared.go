package goroutines

import (
	"errors"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
)

// Errors returned by the account and transfer functions.
var (
	ErrInvalidAmount      = errors.New("account: amount must be positive")
	ErrInsufficientFunds  = errors.New("account: insufficient funds")
	ErrSameAccount        = errors.New("account: transfer to the same account")
)

// Counter is a set of named int64 counters. The zero value is ready to use and
// it is safe for concurrent use. A Counter must not be copied after first use.
type Counter struct {
	mu sync.Mutex
	m  map[string]int64
}

// Add adds delta to the named counter (creating it at 0 if needed) and returns
// the new value. Negative deltas are allowed.
func (c *Counter) Add(name string, delta int64) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]int64)
	}
	c.m[name] += delta
	return c.m[name]
}

// Get returns the named counter's value, or 0 for an unknown name.
func (c *Counter) Get(name string) int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[name]
}

// Snapshot returns an independent copy of every counter.
func (c *Counter) Snapshot() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int64, len(c.m))
	maps.Copy(out, c.m)
	return out
}

// accountSeq gives every account a unique, totally ordered number used to
// order lock acquisition in Transfer.
var accountSeq atomic.Uint64

// Account is a balance guarded by its own lock. Create one with NewAccount;
// it must not be copied after first use.
type Account struct {
	id      string
	seq     uint64
	mu      sync.Mutex
	balance int64
}

// NewAccount returns an account with the given opening balance. It returns
// ErrInvalidAmount if balance is negative.
func NewAccount(id string, balance int64) (*Account, error) {
	if balance < 0 {
		return nil, ErrInvalidAmount
	}
	return &Account{id: id, seq: accountSeq.Add(1), balance: balance}, nil
}

// ID returns the account identifier.
func (a *Account) ID() string {
	return a.id
}

// Balance returns the current balance.
func (a *Account) Balance() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.balance
}

// Deposit adds amount to the balance. It returns ErrInvalidAmount if amount <= 0.
func (a *Account) Deposit(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.balance += amount
	return nil
}

// Withdraw subtracts amount from the balance. It returns ErrInvalidAmount if
// amount <= 0 and ErrInsufficientFunds if amount exceeds the balance; on error
// the balance is unchanged.
func (a *Account) Withdraw(amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if amount > a.balance {
		return ErrInsufficientFunds
	}
	a.balance -= amount
	return nil
}

// Transfer moves amount from src to dst as one atomic step: no goroutine can
// observe the money in flight. It returns ErrInvalidAmount for amount <= 0,
// ErrSameAccount when src == dst, and ErrInsufficientFunds when src lacks funds;
// on any error neither account changes. Locks are acquired in one global order,
// so concurrent transfers in opposite directions cannot deadlock.
func Transfer(src, dst *Account, amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if src == dst {
		return ErrSameAccount
	}
	first, second := src, dst
	if dst.seq < src.seq {
		first, second = dst, src
	}
	first.mu.Lock()
	defer first.mu.Unlock()
	second.mu.Lock()
	defer second.mu.Unlock()

	if src.balance < amount {
		return ErrInsufficientFunds
	}
	src.balance -= amount
	dst.balance += amount
	return nil
}

// Registry maps service names to addresses. It is read-mostly: many goroutines
// call Lookup while Register is rare. The zero value is ready to use and it is
// safe for concurrent use. A Registry must not be copied after first use.
type Registry struct {
	mu sync.RWMutex
	m  map[string]string
}

// Register sets the address for name, replacing any previous value.
func (r *Registry) Register(name, addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil {
		r.m = make(map[string]string)
	}
	r.m[name] = addr
}

// Lookup returns the address registered for name.
func (r *Registry) Lookup(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	addr, ok := r.m[name]
	return addr, ok
}

// Names returns every registered name in ascending order.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Sorted(maps.Keys(r.m))
}

// PermanentOnce returns a function that calls load at most once. Both its
// result and its error are cached forever, which is exactly sync.OnceValues.
// Concurrent callers block until the first call finishes and then share its
// outcome.
func PermanentOnce[T any](load func() (T, error)) func() (T, error) {
	return sync.OnceValues(load)
}

// Lazy caches a successful load. A failed load is not cached: the error is
// returned to that caller and the next Get calls load again. Concurrent Get
// calls are serialised, so load runs once per successful value.
type Lazy[T any] struct {
	mu     sync.Mutex
	load   func() (T, error)
	value  T
	loaded bool
}

// NewLazy returns a Lazy that loads its value with load.
func NewLazy[T any](load func() (T, error)) *Lazy[T] {
	return &Lazy[T]{load: load}
}

// Get returns the cached value, loading it first if necessary.
func (l *Lazy[T]) Get() (T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.loaded {
		v, err := l.load()
		if err != nil {
			var zero T
			return zero, err
		}
		l.value, l.loaded = v, true
	}
	return l.value, nil
}
