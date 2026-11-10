package funcs_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	funcs "goref/exercises/04_functions_and_closures"
)

// tracer records "<name> in" before and "<name> out" after the wrapped handler.
func tracer(name string, trace *[]string) funcs.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*trace = append(*trace, name+" in")
			next.ServeHTTP(w, r)
			*trace = append(*trace, name+" out")
		})
	}
}

func discardLog(string, ...any) {}

func TestPart4Chain(t *testing.T) {
	t.Run("the first middleware is the outermost", func(t *testing.T) {
		var trace []string
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			trace = append(trace, "handler")
		})
		chained := funcs.Chain(tracer("a", &trace), tracer("b", &trace))(h)
		serve(t, chained, httptest.NewRequest(http.MethodGet, "/", nil))

		want := []string{"a in", "b in", "handler", "b out", "a out"}
		if diff := cmp.Diff(want, trace); diff != "" {
			t.Errorf("call order mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("headers set by a middleware reach the response", func(t *testing.T) {
		mw := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Mw", "yes")
				next.ServeHTTP(w, r)
			})
		}
		h := funcs.Chain(mw)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		rec := serve(t, h, httptest.NewRequest(http.MethodPost, "/", nil))
		if got := rec.Header().Get("X-Mw"); got != "yes" {
			t.Errorf("X-Mw = %q, want %q", got, "yes")
		}
		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201", rec.Code)
		}
	})
}

func TestPart4RequestID(t *testing.T) {
	t.Run("generates an id when the request has none", func(t *testing.T) {
		var calls int
		gen := func() string { calls++; return fmt.Sprintf("gen-%d", calls) }
		var inCtx string
		h := funcs.RequestID(gen)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inCtx = funcs.RequestIDFrom(r.Context())
		}))
		rec := serve(t, h, httptest.NewRequest(http.MethodGet, "/x", nil))

		if got := rec.Header().Get("X-Request-Id"); got != "gen-1" {
			t.Errorf("response X-Request-Id = %q, want %q", got, "gen-1")
		}
		if inCtx != "gen-1" {
			t.Errorf("RequestIDFrom(ctx) = %q, want %q", inCtx, "gen-1")
		}
		if calls != 1 {
			t.Errorf("gen called %d times, want 1", calls)
		}
	})

	t.Run("an incoming X-Request-Id is honoured and gen is not called", func(t *testing.T) {
		var calls int
		gen := func() string { calls++; return "generated" }
		var inCtx string
		h := funcs.RequestID(gen)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inCtx = funcs.RequestIDFrom(r.Context())
		}))
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Request-Id", "abc-123")
		rec := serve(t, h, req)

		if got := rec.Header().Get("X-Request-Id"); got != "abc-123" {
			t.Errorf("response X-Request-Id = %q, want %q", got, "abc-123")
		}
		if inCtx != "abc-123" {
			t.Errorf("RequestIDFrom(ctx) = %q, want %q", inCtx, "abc-123")
		}
		if calls != 0 {
			t.Errorf("gen called %d times, want 0", calls)
		}
	})
}

func TestPart4Recover(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	t.Run("a panic becomes a logged 500 with the stack", func(t *testing.T) {
		var logs []string
		logf := func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		}
		h := funcs.Recover(logf)(panicking)
		rec := serve(t, h, httptest.NewRequest(http.MethodGet, "/orders", nil))

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Internal Server Error") {
			t.Errorf("body = %q, want the status text", rec.Body.String())
		}
		if len(logs) != 1 {
			t.Fatalf("logged %d times, want 1", len(logs))
		}
		for _, want := range []string{"GET", "/orders", "boom", "goroutine"} {
			if !strings.Contains(logs[0], want) {
				t.Errorf("log line missing %q:\n%s", want, logs[0])
			}
		}
	})

	t.Run("headers already written are not overwritten", func(t *testing.T) {
		var logs []string
		logf := func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		}
		partial := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			fmt.Fprint(w, "partial")
			panic("late")
		})
		rec := serve(t, funcs.Recover(logf)(partial), httptest.NewRequest(http.MethodGet, "/", nil))

		if rec.Code != http.StatusAccepted {
			t.Errorf("status = %d, want the already-sent 202", rec.Code)
		}
		if len(logs) != 1 {
			t.Errorf("logged %d times, want 1", len(logs))
		}
	})

	t.Run("ErrAbortHandler is re-panicked without logging", func(t *testing.T) {
		// Sanity check on the same chain: an ordinary panic still becomes a 500.
		var logs []string
		logf := func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		}
		if rec := serve(t, funcs.Recover(logf)(panicking), httptest.NewRequest(http.MethodGet, "/", nil)); rec.Code != http.StatusInternalServerError {
			t.Fatalf("ordinary panic: status = %d, want 500", rec.Code)
		}
		logs = nil

		abort := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(http.ErrAbortHandler)
		})
		h := funcs.Recover(logf)(abort)
		v := panicOf(func() { h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)) })
		if v != http.ErrAbortHandler {
			t.Errorf("re-panic value = %v, want http.ErrAbortHandler", v)
		}
		if len(logs) != 0 {
			t.Errorf("ErrAbortHandler was logged: %q", logs)
		}
	})
}

func TestPart4Instrument(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)

	t.Run("records method, path, status and duration", func(t *testing.T) {
		clk := &fakeClock{now: t0, step: 250 * time.Millisecond}
		var got []funcs.Sample
		h := funcs.Instrument(clk.Now, func(s funcs.Sample) { got = append(got, s) })(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			}))
		serve(t, h, httptest.NewRequest(http.MethodPost, "/items/7", nil))

		want := []funcs.Sample{{Method: "POST", Path: "/items/7", Status: 201, Duration: 250 * time.Millisecond}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("samples mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("status defaults to 200 for body-only responses", func(t *testing.T) {
		clk := &fakeClock{now: t0, step: time.Millisecond}
		var got []funcs.Sample
		h := funcs.Instrument(clk.Now, func(s funcs.Sample) { got = append(got, s) })(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "hi")
			}))
		serve(t, h, httptest.NewRequest(http.MethodGet, "/", nil))

		if len(got) != 1 || got[0].Status != 200 {
			t.Errorf("samples = %+v, want one sample with Status 200", got)
		}
	})

	t.Run("placed outside Recover it records the 500", func(t *testing.T) {
		clk := &fakeClock{now: t0, step: 250 * time.Millisecond}
		var got []funcs.Sample
		chain := funcs.Chain(
			funcs.Instrument(clk.Now, func(s funcs.Sample) { got = append(got, s) }),
			funcs.Recover(discardLog),
		)
		h := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		}))
		serve(t, h, httptest.NewRequest(http.MethodGet, "/x", nil))

		if len(got) != 1 || got[0].Status != http.StatusInternalServerError {
			t.Errorf("samples = %+v, want one sample with status 500", got)
		}
	})
}
