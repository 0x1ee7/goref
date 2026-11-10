package httpsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"sync"
)

// Event is one NDJSON line on the event stream.
type Event struct {
	Seq     int64  `json:"seq"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// subBuffer is how many undelivered events a subscriber may hold before it is
// dropped.
const subBuffer = 16

// Broker records events and fans them out to streaming subscribers.
type Broker struct {
	mu      sync.Mutex
	seq     int64
	history []Event
	subs    map[*subscriber]struct{}
	closed  bool
}

type subscriber struct {
	ch chan Event
}

// NewBroker returns an empty, open Broker.
func NewBroker() *Broker {
	return &Broker{subs: map[*subscriber]struct{}{}}
}

// Publish records an event and delivers it to subscribers without blocking.
// A subscriber whose buffer is full is dropped: its channel is closed.
func (b *Broker) Publish(eventType, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.seq++
	ev := Event{Seq: b.seq, Type: eventType, Message: message}
	b.history = append(b.history, ev)
	for s := range b.subs {
		select {
		case s.ch <- ev:
		default:
			close(s.ch)
			delete(b.subs, s)
		}
	}
}

// Close ends every stream. Later Publish calls are no-ops.
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for s := range b.subs {
		close(s.ch)
	}
	clear(b.subs)
}

// Subscribers reports how many live streams are attached.
func (b *Broker) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

// subscribe snapshots the history after seq and attaches a live channel, under
// one lock so no event is lost or duplicated between replay and live.
func (b *Broker) subscribe(after int64) ([]Event, *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := &subscriber{ch: make(chan Event, subBuffer)}
	if b.closed {
		close(s.ch)
	} else {
		b.subs[s] = struct{}{}
	}
	var backlog []Event
	for _, ev := range b.history {
		if ev.Seq > after {
			backlog = append(backlog, ev)
		}
	}
	return backlog, s
}

func (b *Broker) unsubscribe(s *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.subs, s)
}

// Handler serves the NDJSON stream. It accepts ?after=N to replay history
// after sequence N, then streams live events until disconnect or Close.
func (b *Broker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var after int64
		if s := r.URL.Query().Get("after"); s != "" {
			n, err := strconv.ParseInt(s, 10, 64)
			if err != nil || n < 0 {
				writeProblem(w, http.StatusBadRequest, "after must be a non-negative integer")
				return
			}
			after = n
		}

		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		rc := http.NewResponseController(w)
		_ = rc.Flush()

		backlog, sub := b.subscribe(after)
		defer b.unsubscribe(sub)

		enc := json.NewEncoder(w)
		send := func(ev Event) bool {
			if err := enc.Encode(ev); err != nil {
				return false
			}
			return rc.Flush() == nil
		}
		for _, ev := range backlog {
			if !send(ev) {
				return
			}
		}
		for {
			select {
			case <-r.Context().Done():
				return
			case ev, ok := <-sub.ch:
				if !ok || !send(ev) {
					return
				}
			}
		}
	})
}

// StreamEvents connects to target (a Broker.Handler URL) with after=after and
// yields events as they arrive. Errors are yielded once and end the sequence.
func StreamEvents(ctx context.Context, client *http.Client, target string, after int64) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		if client == nil {
			client = http.DefaultClient
		}
		u, err := url.Parse(target)
		if err != nil {
			yield(Event{}, fmt.Errorf("parse stream url: %w", err))
			return
		}
		q := u.Query()
		q.Set("after", strconv.FormatInt(after, 10))
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			yield(Event{}, fmt.Errorf("build stream request: %w", err))
			return
		}
		req.Header.Set("Accept", "application/x-ndjson")
		resp, err := client.Do(req)
		if err != nil {
			yield(Event{}, fmt.Errorf("stream events: %w", err))
			return
		}
		// Close without draining: the stream is unbounded, so a drain would block.
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			yield(Event{}, readAPIError(resp))
			return
		}

		dec := json.NewDecoder(resp.Body)
		for {
			var ev Event
			err := dec.Decode(&ev)
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				if cerr := ctx.Err(); cerr != nil {
					err = cerr
				}
				yield(Event{}, fmt.Errorf("decode event: %w", err))
				return
			}
			if !yield(ev, nil) {
				return
			}
		}
	}
}
