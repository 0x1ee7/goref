package goroutines

import (
	"errors"
	"sync"
)

// ErrQueueClosed is returned by BoundedQueue.Put after Close.
var ErrQueueClosed = errors.New("queue: closed")

// BoundedQueue is a FIFO with a fixed capacity, built on sync.Cond. Put blocks
// while the queue is full and Get blocks while it is empty. It is safe for
// concurrent use by any number of producers and consumers.
type BoundedQueue[T any] struct {
	mu       sync.Mutex
	notFull  *sync.Cond
	notEmpty *sync.Cond
	items    []T
	capacity int
	closed   bool
}

// NewBoundedQueue returns an empty queue that holds at most capacity items. It
// returns ErrInvalidCapacity if capacity < 1.
func NewBoundedQueue[T any](capacity int) (*BoundedQueue[T], error) {
	if capacity < 1 {
		return nil, ErrInvalidCapacity
	}
	q := &BoundedQueue[T]{capacity: capacity}
	q.notFull = sync.NewCond(&q.mu)
	q.notEmpty = sync.NewCond(&q.mu)
	return q, nil
}

// Put appends item, blocking while the queue is full. It returns ErrQueueClosed
// if the queue is closed before or while Put waits.
func (q *BoundedQueue[T]) Put(item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == q.capacity && !q.closed {
		q.notFull.Wait()
	}
	if q.closed {
		return ErrQueueClosed
	}
	q.items = append(q.items, item)
	q.notEmpty.Signal()
	return nil
}

// Get removes and returns the oldest item, blocking while the queue is empty
// and open. After Close, Get keeps returning the remaining items and then
// returns (zero, false).
func (q *BoundedQueue[T]) Get() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.notEmpty.Wait()
	}
	var zero T
	if len(q.items) == 0 {
		return zero, false
	}
	item := q.items[0]
	q.items[0] = zero // release the reference for the GC
	q.items = q.items[1:]
	q.notFull.Signal()
	return item, true
}

// Close marks the queue closed and wakes every blocked Put and Get. It is
// idempotent. Items already queued stay available to Get.
func (q *BoundedQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.notFull.Broadcast()
	q.notEmpty.Broadcast()
}

// Len returns the number of queued items.
func (q *BoundedQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
