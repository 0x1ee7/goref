package slicesmaps

import (
	"fmt"
	"time"
)

// clientWindow is a fixed-size ring of admitted request timestamps for one client.
// Events occupy buf[head], buf[head+1], … (mod len(buf)), oldest first.
type clientWindow struct {
	buf   []time.Time
	head  int
	count int
}

// evict drops events that have left the window: an event at t is live while
// now - t < window.
func (c *clientWindow) evict(now time.Time, window time.Duration) {
	for c.count > 0 && now.Sub(c.buf[c.head]) >= window {
		c.head = (c.head + 1) % len(c.buf)
		c.count--
	}
}

// push appends an event. The caller must ensure count < len(buf).
func (c *clientWindow) push(t time.Time) {
	c.buf[(c.head+c.count)%len(c.buf)] = t
	c.count++
}

// SlidingWindowLimiter allows at most limit requests per client in any trailing
// window. Each client's state is a ring buffer of limit timestamps, so memory is
// bounded per client. It is not safe for concurrent use.
type SlidingWindowLimiter struct {
	limit   int
	window  time.Duration
	now     func() time.Time
	clients map[string]*clientWindow
}

// NewSlidingWindowLimiter returns a limiter that allows limit requests per
// client in any window. A nil now uses time.Now. limit < 1 or window <= 0
// yields ErrInvalidConfig.
func NewSlidingWindowLimiter(limit int, window time.Duration, now func() time.Time) (*SlidingWindowLimiter, error) {
	if limit < 1 {
		return nil, fmt.Errorf("%w: limit %d", ErrInvalidConfig, limit)
	}
	if window <= 0 {
		return nil, fmt.Errorf("%w: window %v", ErrInvalidConfig, window)
	}
	if now == nil {
		now = time.Now
	}
	return &SlidingWindowLimiter{
		limit:   limit,
		window:  window,
		now:     now,
		clients: make(map[string]*clientWindow),
	}, nil
}

// Allow admits and records a request from client if fewer than limit requests
// are live in the window. A denied request is not recorded.
func (l *SlidingWindowLimiter) Allow(client string) bool {
	now := l.now()
	c, ok := l.clients[client]
	if !ok {
		c = &clientWindow{buf: make([]time.Time, l.limit)}
		l.clients[client] = c
	}
	c.evict(now, l.window)
	if c.count >= l.limit {
		return false
	}
	c.push(now)
	return true
}

// Remaining returns how many more requests client may make right now. Unknown
// clients have the full limit; the call does not create state.
func (l *SlidingWindowLimiter) Remaining(client string) int {
	c, ok := l.clients[client]
	if !ok {
		return l.limit
	}
	c.evict(l.now(), l.window)
	return l.limit - c.count
}

// RetryAfter returns how long client must wait before a request is admitted:
// zero while Remaining is positive, otherwise the time until the oldest live
// event leaves the window.
func (l *SlidingWindowLimiter) RetryAfter(client string) time.Duration {
	c, ok := l.clients[client]
	if !ok {
		return 0
	}
	now := l.now()
	c.evict(now, l.window)
	if c.count < l.limit {
		return 0
	}
	return l.window - now.Sub(c.buf[c.head])
}

// Prune drops state for clients with no live events and returns how many
// clients were dropped.
func (l *SlidingWindowLimiter) Prune() int {
	now := l.now()
	dropped := 0
	for id, c := range l.clients {
		c.evict(now, l.window)
		if c.count == 0 {
			delete(l.clients, id)
			dropped++
		}
	}
	return dropped
}
