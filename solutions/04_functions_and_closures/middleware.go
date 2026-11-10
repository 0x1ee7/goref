package funcs

import (
	"context"
	"net/http"
	"runtime/debug"
	"time"
)

// Middleware wraps an http.Handler with additional behaviour.
type Middleware func(http.Handler) http.Handler

// Sample is one observation produced by Instrument.
type Sample struct {
	Method   string
	Path     string
	Status   int
	Duration time.Duration
}

// Chain composes middlewares so the first argument is the outermost wrapper.
// With no arguments it returns the identity middleware.
func Chain(mws ...Middleware) Middleware {
	return func(h http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}

// statusWriter records the first status code written so that middlewares
// further out can observe it. It does not buffer.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

type requestIDKey struct{}

const requestIDHeader = "X-Request-Id"

// RequestID honours an incoming X-Request-Id header. Otherwise it calls gen to
// create one. The id is echoed in the response header and stored in the request
// context for RequestIDFrom.
func RequestID(gen func() string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(requestIDHeader)
			if id == "" {
				id = gen()
			}
			w.Header().Set(requestIDHeader, id)
			ctx := context.WithValue(r.Context(), requestIDKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFrom returns the request id stored by RequestID, or "" if none.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// Recover turns a handler panic into a logged 500 response. The log line
// carries the method, path, panic value, and stack. If the handler has already
// written headers, no second status is sent. http.ErrAbortHandler is re-panicked
// unlogged, because net/http uses it to abort a response on purpose.
func Recover(logf func(format string, args ...any)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w}
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler {
					panic(v)
				}
				logf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, v, debug.Stack())
				if sw.status == 0 {
					http.Error(sw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(sw, r)
		})
	}
}

// Instrument measures each request with now and reports it to record once the
// handler returns normally. Status defaults to 200 when the handler writes no
// header. A panic propagates without a sample.
func Instrument(now func() time.Time, record func(Sample)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := now()
			sw := &statusWriter{ResponseWriter: w}
			next.ServeHTTP(sw, r)
			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			record(Sample{
				Method:   r.Method,
				Path:     r.URL.Path,
				Status:   status,
				Duration: now().Sub(start),
			})
		})
	}
}
