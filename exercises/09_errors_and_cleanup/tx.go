package support

import "errors"

// Errors reported by the key-value store and its transactions.
var (
	ErrKeyNotFound       = errors.New("key not found")
	ErrTxActive          = errors.New("transaction already active")
	ErrTxDone            = errors.New("transaction finished")
	ErrRollbackSavepoint = errors.New("savepoint rollback requested")
)

// Store is an in-memory string key-value store with at most one open
// transaction at a time.
type Store struct {
	// Engineer: state goes here.
}

// NewStore returns a store seeded with a copy of initial. A nil initial map
// is allowed.
func NewStore(initial map[string]string) *Store { return nil }

// Get returns the committed value for key.
func (s *Store) Get(key string) (string, bool) { return "", false }

// Snapshot returns a copy of the committed contents.
func (s *Store) Snapshot() map[string]string { return nil }

// Begin opens a transaction. It returns ErrTxActive if another transaction on
// s is still open.
func (s *Store) Begin() (*Tx, error) { return nil, nil }

// Tx is a transaction on a Store. A Tx is not safe for concurrent use.
type Tx struct {
	// Engineer: state goes here.
}

// Get returns the value for key as seen by this transaction, including its
// own uncommitted writes. A missing or deleted key returns an error matching
// ErrKeyNotFound.
func (tx *Tx) Get(key string) (string, error) { return "", nil }

// Set stores value under key in the transaction.
func (tx *Tx) Set(key, value string) error { return nil }

// Delete removes key in the transaction. A key that is not visible returns an
// error matching ErrKeyNotFound.
func (tx *Tx) Delete(key string) error { return nil }

// Savepoint runs fn and undoes fn's writes if it fails.
//
// If fn returns an error matching ErrRollbackSavepoint, the writes are undone
// and Savepoint returns nil. Any other error is undone and returned. The
// transaction stays open. A panic from fn also undoes the writes and then
// propagates.
func (tx *Tx) Savepoint(fn func() error) error { return nil }

// Commit applies the transaction's writes to the store and closes it.
// Commit, Rollback and every other operation fail with ErrTxDone after the
// transaction is finished.
func (tx *Tx) Commit() error { return nil }

// Rollback discards the transaction's writes and closes it.
func (tx *Tx) Rollback() error { return nil }

// Update runs fn in a new transaction on s. It commits if fn returns nil and
// rolls back if fn returns an error or panics. A panic propagates after the
// rollback. The error from fn is returned unchanged.
func Update(s *Store, fn func(tx *Tx) error) error { return nil }
