package goroutines

import (
	"errors"
)

// ErrQueueClosed is returned by BoundedQueue.Put after Close.
var ErrQueueClosed = errors.New("queue: closed")

// BoundedQueue is a FIFO with a fixed capacity, built on sync.Cond. Put blocks
// while the queue is full and Get blocks while it is empty. It is safe for
// concurrent use by any number of producers and consumers.
type BoundedQueue[T any] struct {
	// engineer-owned fields
}

// NewBoundedQueue returns an empty queue that holds at most capacity items. It
// returns ErrInvalidCapacity if capacity < 1.
func NewBoundedQueue[T any](capacity int) (*BoundedQueue[T], error) {
	return nil, errors.New("not implemented")
}

// Put appends item, blocking while the queue is full. It returns ErrQueueClosed
// if the queue is closed before or while Put waits.
func (q *BoundedQueue[T]) Put(item T) error {
	return errors.New("not implemented")
}

// Get removes and returns the oldest item, blocking while the queue is empty
// and open. After Close, Get keeps returning the remaining items and then
// returns (zero, false).
func (q *BoundedQueue[T]) Get() (T, bool) {
	var zero T
	return zero, false
}

// Close marks the queue closed and wakes every blocked Put and Get. It is
// idempotent. Items already queued stay available to Get.
func (q *BoundedQueue[T]) Close() {}

// Len returns the number of queued items.
func (q *BoundedQueue[T]) Len() int {
	return 0
}
