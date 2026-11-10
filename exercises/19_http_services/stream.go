package httpsvc

import (
	"context"
	"iter"
	"net/http"
)

// Event is one NDJSON line on the event stream.
type Event struct {
	Seq     int64  `json:"seq"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Broker records events and fans them out to streaming subscribers.
type Broker struct {
	// engineer-owned fields
}

// NewBroker returns an empty, open Broker.
func NewBroker() *Broker {
	return &Broker{}
}

// Publish records an event and delivers it to subscribers without blocking.
// A subscriber that cannot keep up is dropped.
func (b *Broker) Publish(eventType, message string) {}

// Close ends every stream. Later Publish calls are no-ops.
func (b *Broker) Close() {}

// Subscribers reports how many live streams are attached.
func (b *Broker) Subscribers() int {
	return 0
}

// Handler serves the NDJSON stream. It accepts ?after=N to replay history
// after sequence N, then streams live events until disconnect or Close.
func (b *Broker) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
}

// StreamEvents connects to target (a Broker.Handler URL) with after=after and
// yields events as they arrive. Errors are yielded once and end the sequence.
func StreamEvents(ctx context.Context, client *http.Client, target string, after int64) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		yield(Event{}, errNotImplemented)
	}
}
