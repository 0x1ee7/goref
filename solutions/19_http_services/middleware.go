package httpsvc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"
)

// Middleware wraps a handler with cross-cutting behaviour.
type Middleware func(http.Handler) http.Handler

// Chain wraps h with mws. mws[0] is the outermost middleware, so a request
// passes through mws[0], then mws[1], and so on before reaching h.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type requestIDKey struct{}

// RequestID keeps a valid incoming X-Request-ID or generates a new one,
// echoes it on the response and stores it in the request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// validRequestID accepts 1-128 bytes of printable ASCII without spaces.
func validRequestID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for i := range len(id) {
		if id[i] < 0x21 || id[i] > 0x7e {
			return false
		}
	}
	return true
}

// newRequestID returns 16 random bytes as 32 hex characters. crypto/rand.Read
// never fails since Go 1.24, so its error is ignored.
func newRequestID() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// RequestIDFrom returns the request ID stored by RequestID, or "".
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// statusRecorder remembers the status code and body size of a response.
type statusRecorder struct {
	http.ResponseWriter
	code  int
	bytes int64
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.code == 0 {
		s.code = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(p []byte) (int, error) {
	if s.code == 0 {
		s.code = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(p)
	s.bytes += int64(n)
	return n, err
}

// FlushError flushes the underlying writer, reporting whether it supports
// flushing. It is what http.ResponseController consults first.
func (s *statusRecorder) FlushError() error {
	if s.code == 0 {
		s.code = http.StatusOK
	}
	return http.NewResponseController(s.ResponseWriter).Flush()
}

// Flush implements http.Flusher, ignoring unsupported writers.
func (s *statusRecorder) Flush() {
	_ = s.FlushError()
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter {
	return s.ResponseWriter
}

// AccessLog logs one structured record per request at a level derived from
// the response status: ERROR for 5xx, WARN for 4xx, INFO otherwise.
func AccessLog(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			status := rec.code
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}
			logger.LogAttrs(r.Context(), level, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Int64("bytes", rec.bytes),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", RequestIDFrom(r.Context())),
			)
		})
	}
}

// Recover turns a panic in the downstream handler into a 500 problem response
// (when no status was written yet), logging the panic and its stack.
// http.ErrAbortHandler is re-panicked untouched.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w}
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler {
					panic(v)
				}
				logger.LogAttrs(r.Context(), slog.LevelError, "panic",
					slog.Any("panic", v),
					slog.String("stack", string(debug.Stack())),
					slog.String("request_id", RequestIDFrom(r.Context())),
				)
				if rec.code == 0 {
					writeProblem(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(rec, r)
		})
	}
}

// CORS allows cross-origin requests from origins ("*" allows any origin) and
// answers preflight requests.
func CORS(origins ...string) Middleware {
	allowAll := slices.Contains(origins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			allowed := allowAll || slices.Contains(origins, origin)
			w.Header().Add("Vary", "Origin")

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if !allowed {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				setAllowOrigin(w, origin, allowAll)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match, X-Request-ID")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if allowed {
				setAllowOrigin(w, origin, allowAll)
				w.Header().Set("Access-Control-Expose-Headers", "ETag, Location, X-Request-ID")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func setAllowOrigin(w http.ResponseWriter, origin string, allowAll bool) {
	if allowAll {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
}

// BearerAuth requires "Authorization: Bearer <token>" with a constant-time
// comparison. An empty token denies every request.
func BearerAuth(token string) Middleware {
	want := sha256.Sum256([]byte(token))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme, cred, found := strings.Cut(r.Header.Get("Authorization"), " ")
			got := sha256.Sum256([]byte(strings.TrimSpace(cred)))
			ok := token != "" && found &&
				strings.EqualFold(scheme, "Bearer") &&
				subtle.ConstantTimeCompare(got[:], want[:]) == 1
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				writeProblem(w, http.StatusUnauthorized, "missing or invalid bearer token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
