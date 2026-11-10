package httpsvc_test

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	httpsvc "goref/exercises/19_http_services"
)

// newStreamServer serves a broker at /events. Cleanup closes the broker before
// the server so that open streams end and Close does not block.
func newStreamServer(t *testing.T) (*httpsvc.Broker, string) {
	t.Helper()
	b := httpsvc.NewBroker()
	mux := http.NewServeMux()
	mux.Handle("/events", b.Handler())
	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		b.Close()
		srv.Close()
	})
	return b, srv.URL
}

// waitForSubscribers polls until the broker has n subscribers. It is safe to
// call from goroutines because it never calls testing methods.
func waitForSubscribers(b *httpsvc.Broker, n int) bool {
	deadline := time.Now().Add(2 * time.Second)
	for b.Subscribers() != n {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}

func TestPart5HandlerValidatesAfter(t *testing.T) {
	b := httpsvc.NewBroker()
	t.Cleanup(b.Close)
	for _, v := range []string{"abc", "-1", "1.5"} {
		t.Run("after="+v, func(t *testing.T) {
			rec := do(t, b.Handler(), http.MethodGet, "/events?after="+v, "", nil)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", rec.Code)
			}
		})
	}
}

func TestPart5HandlerReplaysAfterClose(t *testing.T) {
	b := httpsvc.NewBroker()
	b.Publish("created", "item 1")
	b.Close()
	rec := do(t, b.Handler(), http.MethodGet, "/events?after=0", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"seq":1`) {
		t.Errorf("body = %q, want the replayed event with seq 1", rec.Body)
	}
}

func TestPart5StreamContentType(t *testing.T) {
	b, base := newStreamServer(t)
	b.Publish("created", "item 1")
	resp, err := http.Get(base + "/events?after=0")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want application/x-ndjson", ct)
	}
	sc := bufio.NewScanner(resp.Body)
	if !sc.Scan() {
		t.Fatalf("no NDJSON line received: %v", sc.Err())
	}
	if !strings.Contains(sc.Text(), `"seq":1`) {
		t.Errorf("first line = %q, want an event with seq 1", sc.Text())
	}
}

func TestPart5StreamEvents(t *testing.T) {
	tests := []struct {
		name     string
		initial  int   // events published before the client connects
		after    int64 // resume cursor
		extra    int   // events published after the client subscribes
		wantSeqs []int64
	}{
		{"full replay", 3, 0, 0, []int64{1, 2, 3}},
		{"replay after cursor", 3, 1, 0, []int64{2, 3}},
		{"caught up then live", 3, 3, 2, []int64{4, 5}},
		{"live only", 0, 0, 2, []int64{1, 2}},
		{"replay then live", 1, 0, 2, []int64{1, 2, 3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, base := newStreamServer(t)
			for range tc.initial {
				b.Publish("created", "item")
			}
			go func() {
				// Always Close, even if the subscriber never attaches, so the
				// stream cannot hang the test.
				defer b.Close()
				if waitForSubscribers(b, 1) {
					for range tc.extra {
						b.Publish("created", "live")
					}
				}
			}()

			var got []int64
			for ev, err := range httpsvc.StreamEvents(t.Context(), nil, base+"/events", tc.after) {
				if err != nil {
					t.Fatalf("stream error: %v", err)
				}
				got = append(got, ev.Seq)
			}
			if diff := cmp.Diff(tc.wantSeqs, got); diff != "" {
				t.Errorf("seqs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPart5StreamErrors(t *testing.T) {
	t.Run("non-200 yields an APIError", func(t *testing.T) {
		_, base := newStreamServer(t)
		for _, err := range httpsvc.StreamEvents(t.Context(), nil, base+"/missing", 0) {
			wantAPIError(t, err, http.StatusNotFound)
			return
		}
		t.Fatal("iterator yielded nothing, want an error")
	})
	t.Run("cancelled context ends the stream with its error", func(t *testing.T) {
		_, base := newStreamServer(t)
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		var gotErr error
		for _, err := range httpsvc.StreamEvents(ctx, nil, base+"/events", 0) {
			if err != nil {
				gotErr = err
			}
		}
		if !errors.Is(gotErr, context.DeadlineExceeded) {
			t.Errorf("err = %v, want context.DeadlineExceeded", gotErr)
		}
	})
	t.Run("breaking out closes the connection", func(t *testing.T) {
		b, base := newStreamServer(t)
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		for range 3 {
			b.Publish("created", "item")
		}
		count := 0
		for _, err := range httpsvc.StreamEvents(ctx, nil, base+"/events", 0) {
			if err != nil {
				t.Fatalf("stream error: %v", err)
			}
			count++
			if count == 2 {
				break
			}
		}
		if !waitForSubscribers(b, 0) {
			t.Errorf("subscribers = %d after the client left, want 0", b.Subscribers())
		}
	})
}

func TestPart5SlowSubscriberDropped(t *testing.T) {
	b := httpsvc.NewBroker()
	w := &blockingWriter{release: make(chan struct{})}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/events", nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		b.Handler().ServeHTTP(w, req)
	}()
	t.Cleanup(func() {
		b.Close()
		close(w.release)
		<-done
	})
	if !waitForSubscribers(b, 1) {
		t.Fatal("subscriber never attached")
	}
	// The handler blocks in Write after one event, so its 16-slot buffer fills
	// and a later Publish must drop the subscriber rather than block.
	for range 40 {
		b.Publish("tick", "x")
	}
	if n := b.Subscribers(); n != 0 {
		t.Errorf("subscribers = %d after overflow, want 0", n)
	}
}

func TestPart5BrokerClose(t *testing.T) {
	t.Run("Close is idempotent and Publish after Close is a no-op", func(t *testing.T) {
		b := httpsvc.NewBroker()
		b.Publish("created", "item 1")
		b.Close()
		b.Close()
		b.Publish("late", "ignored")
		rec := do(t, b.Handler(), http.MethodGet, "/events?after=0", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("handler after Close: status = %d, want 200 replay", rec.Code)
		}
		if body := rec.Body.String(); !strings.Contains(body, `"seq":1`) || strings.Contains(body, "ignored") {
			t.Errorf("body = %q, want history replayed and the post-Close publish dropped", body)
		}
	})
}
