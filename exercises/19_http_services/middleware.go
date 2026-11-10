package httpsvc

import (
	"context"
	"log/slog"
	"net/http"
)

// Middleware wraps a handler with cross-cutting behaviour.
type Middleware func(http.Handler) http.Handler

// Chain wraps h with mws. mws[0] is the outermost middleware, so a request
// passes through mws[0], then mws[1], and so on before reaching h.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	return h
}

// RequestID keeps a valid incoming X-Request-ID or generates a new one,
// echoes it on the response and stores it in the request context.
func RequestID(next http.Handler) http.Handler {
	return next
}

// RequestIDFrom returns the request ID stored by RequestID, or "".
func RequestIDFrom(ctx context.Context) string {
	return ""
}

// AccessLog logs one structured record per request at a level derived from
// the response status: ERROR for 5xx, WARN for 4xx, INFO otherwise.
func AccessLog(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return next
	}
}

// Recover turns a panic in the downstream handler into a 500 problem response
// (when no status was written yet), logging the panic and its stack.
// http.ErrAbortHandler is re-panicked untouched.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return next
	}
}

// CORS allows cross-origin requests from origins ("*" allows any origin) and
// answers preflight requests.
func CORS(origins ...string) Middleware {
	return func(next http.Handler) http.Handler {
		return next
	}
}

// BearerAuth requires "Authorization: Bearer <token>" with a constant-time
// comparison. An empty token denies every request.
func BearerAuth(token string) Middleware {
	return func(next http.Handler) http.Handler {
		return next
	}
}
