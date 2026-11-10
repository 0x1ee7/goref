package support

import (
	"errors"
	"fmt"
	"maps"
	"sync"
)

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
	mu     sync.Mutex
	data   map[string]string
	active bool
}

// NewStore returns a store seeded with a copy of initial. A nil initial map
// is allowed.
func NewStore(initial map[string]string) *Store {
	data := make(map[string]string, len(initial))
	maps.Copy(data, initial)
	return &Store{data: data}
}

// Get returns the committed value for key.
func (s *Store) Get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	return v, ok
}

// Snapshot returns a copy of the committed contents.
func (s *Store) Snapshot() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.data)
}

// Begin opens a transaction. It returns ErrTxActive if another transaction on
// s is still open.
func (s *Store) Begin() (*Tx, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return nil, ErrTxActive
	}
	s.active = true
	return &Tx{store: s, pending: make(map[string]change)}, nil
}

// change is one uncommitted write: a new value, or a deletion.
type change struct {
	value   string
	deleted bool
}

// Tx is a transaction on a Store. A Tx is not safe for concurrent use.
type Tx struct {
	store   *Store
	pending map[string]change
	done    bool
}

func (tx *Tx) check() error {
	if tx.done {
		return ErrTxDone
	}
	return nil
}

func notFound(key string) error {
	return fmt.Errorf("get %q: %w", key, ErrKeyNotFound)
}

// Get returns the value for key as seen by this transaction, including its
// own uncommitted writes. A missing or deleted key returns an error matching
// ErrKeyNotFound.
func (tx *Tx) Get(key string) (string, error) {
	if err := tx.check(); err != nil {
		return "", err
	}
	if c, ok := tx.pending[key]; ok {
		if c.deleted {
			return "", notFound(key)
		}
		return c.value, nil
	}
	v, ok := tx.store.Get(key)
	if !ok {
		return "", notFound(key)
	}
	return v, nil
}

// Set stores value under key in the transaction.
func (tx *Tx) Set(key, value string) error {
	if err := tx.check(); err != nil {
		return err
	}
	tx.pending[key] = change{value: value}
	return nil
}

// Delete removes key in the transaction. A key that is not visible returns an
// error matching ErrKeyNotFound.
func (tx *Tx) Delete(key string) error {
	if _, err := tx.Get(key); err != nil {
		return err
	}
	tx.pending[key] = change{deleted: true}
	return nil
}

// Savepoint runs fn and undoes fn's writes if it fails.
//
// If fn returns an error matching ErrRollbackSavepoint, the writes are undone
// and Savepoint returns nil. Any other error is undone and returned. The
// transaction stays open. A panic from fn also undoes the writes and then
// propagates.
func (tx *Tx) Savepoint(fn func() error) error {
	if err := tx.check(); err != nil {
		return err
	}
	saved := maps.Clone(tx.pending)
	completed := false
	// Undo unless fn completed. This also runs when fn panics.
	defer func() {
		if !completed {
			tx.pending = saved
		}
	}()

	if err := fn(); err != nil {
		if errors.Is(err, ErrRollbackSavepoint) {
			return nil
		}
		return err
	}
	completed = true
	return nil
}

// Commit applies the transaction's writes to the store and closes it.
// Commit, Rollback and every other operation fail with ErrTxDone after the
// transaction is finished.
func (tx *Tx) Commit() error {
	if err := tx.check(); err != nil {
		return err
	}
	s := tx.store
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, c := range tx.pending {
		if c.deleted {
			delete(s.data, k)
		} else {
			s.data[k] = c.value
		}
	}
	tx.close()
	return nil
}

// Rollback discards the transaction's writes and closes it.
func (tx *Tx) Rollback() error {
	if err := tx.check(); err != nil {
		return err
	}
	tx.store.mu.Lock()
	defer tx.store.mu.Unlock()
	tx.close()
	return nil
}

// close marks the transaction finished and frees the store. The caller holds
// the store lock.
func (tx *Tx) close() {
	tx.done = true
	tx.pending = nil
	tx.store.active = false
}

// Update runs fn in a new transaction on s. It commits if fn returns nil and
// rolls back if fn returns an error or panics. A panic propagates after the
// rollback. The error from fn is returned unchanged.
func Update(s *Store, fn func(tx *Tx) error) error {
	tx, err := s.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}
	committed = true
	return tx.Commit()
}
