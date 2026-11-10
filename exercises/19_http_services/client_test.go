package httpsvc_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	httpsvc "goref/exercises/19_http_services"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func okResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}}
}

func newTestServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func newClient(t *testing.T, base string, opts ...httpsvc.Option) *httpsvc.Client {
	t.Helper()
	c, err := httpsvc.NewClient(base, append([]httpsvc.Option{httpsvc.WithBackoff(time.Millisecond)}, opts...)...)
	if err != nil {
		t.Fatalf("NewClient(%q): %v", base, err)
	}
	return c
}

func wantAPIError(t *testing.T, err error, status int) {
	t.Helper()
	var apiErr *httpsvc.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.StatusCode != status {
		t.Fatalf("APIError.StatusCode = %d, want %d", apiErr.StatusCode, status)
	}
}

func TestPart4NewClient(t *testing.T) {
	tests := []struct {
		name    string
		base    string
		wantErr bool
	}{
		{"empty base rejected", "", true},
		{"relative path rejected", "/api", true},
		{"unsupported scheme rejected", "ftp://example.com", true},
		{"missing host rejected", "http://", true},
		{"http accepted", "http://127.0.0.1:8080", false},
		{"https with base path accepted", "https://example.com/api/", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := httpsvc.NewClient("http://127.0.0.1:8080"); err != nil {
				t.Fatalf("a valid base must be accepted: %v", err)
			}
			c, err := httpsvc.NewClient(tc.base)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewClient(%q) err = %v, wantErr %v", tc.base, err, tc.wantErr)
			}
			if !tc.wantErr && c == nil {
				t.Fatal("NewClient returned a nil client")
			}
		})
	}

	t.Run("base path prefix is kept", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.Handle("/api/", http.StripPrefix("/api", httpsvc.NewInventoryHandler()))
		srv := newTestServer(t, mux)
		c := newClient(t, srv.URL+"/api")
		item, err := c.CreateItem(t.Context(), "bolt", 1)
		if err != nil {
			t.Fatalf("CreateItem: %v", err)
		}
		if item.ID != "1" {
			t.Errorf("item.ID = %q, want \"1\"", item.ID)
		}
	})
}

func TestPart4Operations(t *testing.T) {
	srv := newTestServer(t, httpsvc.NewInventoryHandler())
	c := newClient(t, srv.URL)
	ctx := t.Context()

	item, err := c.CreateItem(ctx, "bolt", 5)
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}
	if diff := cmp.Diff(httpsvc.Item{ID: "1", Name: "bolt", Quantity: 5}, item); diff != "" {
		t.Errorf("CreateItem (-want +got):\n%s", diff)
	}

	got, etag, err := c.GetItem(ctx, "1")
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if etag != `"1"` || got.Name != "bolt" {
		t.Errorf("GetItem = %+v, etag %q; want bolt with etag \"1\"", got, etag)
	}

	updated, newTag, err := c.UpdateItem(ctx, "1", etag, "bolt", 9)
	if err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if newTag != `"2"` || updated.Quantity != 9 {
		t.Errorf("UpdateItem = %+v, etag %q; want quantity 9 and etag \"2\"", updated, newTag)
	}

	_, _, err = c.UpdateItem(ctx, "1", `"1"`, "bolt", 3)
	wantAPIError(t, err, http.StatusPreconditionFailed)

	_, _, err = c.GetItem(ctx, "99")
	wantAPIError(t, err, http.StatusNotFound)

	if err := c.DeleteItem(ctx, "1"); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	_, _, err = c.GetItem(ctx, "1")
	wantAPIError(t, err, http.StatusNotFound)

	err = c.DeleteItem(ctx, "1")
	wantAPIError(t, err, http.StatusNotFound)
}

func TestPart4UpdateNeedsETag(t *testing.T) {
	var requests atomic.Int32
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("ETag", `"1"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","name":"bolt","quantity":1}`))
	}))
	c := newClient(t, srv.URL)
	if _, _, err := c.UpdateItem(t.Context(), "1", "", "bolt", 1); err == nil {
		t.Fatal("UpdateItem with empty etag: want error")
	}
	if _, _, err := c.UpdateItem(t.Context(), "1", `"1"`, "bolt", 1); err != nil {
		t.Fatalf("UpdateItem with a valid etag: %v", err)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("requests sent = %d, want 1 (only the valid update reaches the server)", n)
	}
}

func TestPart4Retry(t *testing.T) {
	tests := []struct {
		name         string
		method       string // "get", "create", "update"
		status       int
		failures     int32
		retryAfter   string
		retries      int
		wantErr      bool
		wantAttempts int32
	}{
		{"GET retries 503 then succeeds", "get", http.StatusServiceUnavailable, 2, "", 3, false, 3},
		{"GET gives up after retries", "get", http.StatusInternalServerError, 100, "", 2, true, 3},
		{"POST is never retried on 503", "create", http.StatusServiceUnavailable, 1, "", 3, true, 1},
		{"POST retries 429 honouring Retry-After", "create", http.StatusTooManyRequests, 1, "0", 3, false, 2},
		{"PUT is retried on 503", "update", http.StatusServiceUnavailable, 1, "", 3, false, 2},
		{"client errors are not retried", "get", http.StatusBadRequest, 100, "", 3, true, 1},
		{"retries disabled", "get", http.StatusServiceUnavailable, 1, "", -1, true, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inner := httpsvc.NewInventoryHandler()
			createItem(t, inner, "seed", 1)

			var attempts atomic.Int32
			srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attempts.Add(1) <= tc.failures {
					if tc.retryAfter != "" {
						w.Header().Set("Retry-After", tc.retryAfter)
					}
					w.WriteHeader(tc.status)
					return
				}
				inner.ServeHTTP(w, r)
			}))
			c := newClient(t, srv.URL, httpsvc.WithRetries(tc.retries))

			ctx := t.Context()
			var err error
			switch tc.method {
			case "get":
				_, _, err = c.GetItem(ctx, "1")
			case "create":
				_, err = c.CreateItem(ctx, "bolt", 1)
			case "update":
				_, _, err = c.UpdateItem(ctx, "1", `"1"`, "bolt", 2)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got := attempts.Load(); got != tc.wantAttempts {
				t.Errorf("attempts = %d, want %d", got, tc.wantAttempts)
			}
		})
	}
}

func TestPart4ContextCancelsBackoff(t *testing.T) {
	srv := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	c := newClient(t, srv.URL, httpsvc.WithBackoff(time.Hour))
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, _, err := c.GetItem(ctx, "1")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded from the backoff sleep", err)
	}
}

func TestPart4Auth(t *testing.T) {
	srv := newTestServer(t, httpsvc.BearerAuth("s3cret")(httpsvc.NewInventoryHandler()))

	t.Run("WithToken sends the bearer header", func(t *testing.T) {
		c := newClient(t, srv.URL, httpsvc.WithToken("s3cret"))
		if _, err := c.CreateItem(t.Context(), "bolt", 1); err != nil {
			t.Errorf("CreateItem: %v", err)
		}
	})
	t.Run("missing token is a 401 APIError", func(t *testing.T) {
		c := newClient(t, srv.URL)
		_, err := c.CreateItem(t.Context(), "nut", 1)
		wantAPIError(t, err, http.StatusUnauthorized)
	})
	t.Run("AuthTransport adds the token", func(t *testing.T) {
		hc := &http.Client{Transport: &httpsvc.AuthTransport{Token: "s3cret"}}
		c := newClient(t, srv.URL, httpsvc.WithHTTPClient(hc))
		if _, err := c.CreateItem(t.Context(), "washer", 1); err != nil {
			t.Errorf("CreateItem: %v", err)
		}
	})
	t.Run("AuthTransport does not mutate the request", func(t *testing.T) {
		var seen *http.Request
		base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			seen = r
			return okResponse(), nil
		})
		tr := &httpsvc.AuthTransport{Base: base, Token: "x"}
		req := httptest.NewRequest(http.MethodGet, "/items", nil)
		resp, err := tr.RoundTrip(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			t.Fatalf("RoundTrip: %v", err)
		}
		if seen == nil {
			t.Fatal("base transport was not called")
		}
		if got := seen.Header.Get("Authorization"); got != "Bearer x" {
			t.Errorf("base saw Authorization %q, want \"Bearer x\"", got)
		}
		if got := req.Header.Get("Authorization"); got != "" {
			t.Errorf("original request mutated: Authorization = %q", got)
		}
	})
}

func TestPart4MetricsTransport(t *testing.T) {
	t.Run("reports method and status", func(t *testing.T) {
		var got []string
		base := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusCreated, Body: http.NoBody, Header: http.Header{}}, nil
		})
		mt := &httpsvc.MetricsTransport{Base: base, Observe: func(method string, status int, _ time.Duration, err error) {
			got = append(got, fmt.Sprintf("%s %d %v", method, status, err))
		}}
		resp, err := mt.RoundTrip(httptest.NewRequest(http.MethodPost, "/items", nil))
		if resp != nil {
			_ = resp.Body.Close()
		}
		if err != nil {
			t.Fatalf("RoundTrip: %v", err)
		}
		if diff := cmp.Diff([]string{"POST 201 <nil>"}, got); diff != "" {
			t.Errorf("observations (-want +got):\n%s", diff)
		}
	})
	t.Run("transport failure reports status 0 and the error", func(t *testing.T) {
		var got []string
		base := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("boom")
		})
		mt := &httpsvc.MetricsTransport{Base: base, Observe: func(method string, status int, _ time.Duration, err error) {
			got = append(got, fmt.Sprintf("%s %d %v", method, status, err))
		}}
		_, err := mt.RoundTrip(httptest.NewRequest(http.MethodGet, "/items", nil))
		if err == nil {
			t.Fatal("RoundTrip: want error")
		}
		if diff := cmp.Diff([]string{"GET 0 boom"}, got); diff != "" {
			t.Errorf("observations (-want +got):\n%s", diff)
		}
	})
}

func TestPart4ParseRetryAfter(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		value  string
		want   time.Duration
		wantOK bool
	}{
		{"seconds", "5", 5 * time.Second, true},
		{"zero seconds", "0", 0, true},
		{"padded seconds", " 7 ", 7 * time.Second, true},
		{"negative rejected", "-1", 0, false},
		{"empty rejected", "", 0, false},
		{"garbage rejected", "soon", 0, false},
		{"HTTP date in the future", now.Add(30 * time.Second).Format(http.TimeFormat), 30 * time.Second, true},
		{"HTTP date in the past clamps to zero", now.Add(-time.Minute).Format(http.TimeFormat), 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if d, ok := httpsvc.ParseRetryAfter("7", now); !ok || d != 7*time.Second {
				t.Fatalf("ParseRetryAfter(\"7\") = %v, %v; valid input must parse", d, ok)
			}
			got, ok := httpsvc.ParseRetryAfter(tc.value, now)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("duration = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPart4AllItems(t *testing.T) {
	srv := newTestServer(t, httpsvc.NewInventoryHandler())
	c := newClient(t, srv.URL)
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		if _, err := c.CreateItem(t.Context(), n, 1); err != nil {
			t.Fatalf("CreateItem %q: %v", n, err)
		}
	}

	t.Run("follows cursors across pages", func(t *testing.T) {
		var names []string
		for item, err := range c.AllItems(t.Context(), httpsvc.ListQuery{Limit: 2}) {
			if err != nil {
				t.Fatalf("AllItems: %v", err)
			}
			names = append(names, item.Name)
		}
		if diff := cmp.Diff([]string{"a", "b", "c", "d", "e"}, names); diff != "" {
			t.Errorf("names (-want +got):\n%s", diff)
		}
	})
	t.Run("stops when the loop breaks", func(t *testing.T) {
		var names []string
		for item, err := range c.AllItems(t.Context(), httpsvc.ListQuery{Limit: 2}) {
			if err != nil {
				t.Fatalf("AllItems: %v", err)
			}
			names = append(names, item.Name)
			if len(names) == 2 {
				break
			}
		}
		if diff := cmp.Diff([]string{"a", "b"}, names); diff != "" {
			t.Errorf("names (-want +got):\n%s", diff)
		}
	})
	t.Run("yields a single error on failure", func(t *testing.T) {
		failing := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		fc := newClient(t, failing.URL, httpsvc.WithRetries(0))
		var items, errs int
		var gotErr error
		for _, err := range fc.AllItems(t.Context(), httpsvc.ListQuery{}) {
			if err != nil {
				errs++
				gotErr = err
				continue
			}
			items++
		}
		if items != 0 || errs != 1 {
			t.Fatalf("items = %d, errors = %d; want 0 items and exactly 1 error", items, errs)
		}
		wantAPIError(t, gotErr, http.StatusInternalServerError)
	})
}
