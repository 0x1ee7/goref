package shoplog

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

// RequestIDHeader is the header that carries the request id in and out.
const RequestIDHeader = "X-Request-Id"

// maxInboundIDLen bounds inbound ids so a client cannot bloat logs.
const maxInboundIDLen = 128

// AccessLog wraps next with one structured record per request, carrying the
// request id, RequestAttr, status, bytes and duration_ms. The id is taken from
// an inbound X-Request-Id if it is printable ASCII of at most 128 bytes, and
// is generated otherwise. It is echoed in the response and stored in the
// request context. A nil logger means slog.Default(), read per request.
// See README, Part 6.
func AccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := logger
		if l == nil {
			l = slog.Default()
		}
		id := validInboundID(r.Header.Get(RequestIDHeader))
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := WithRequestID(r.Context(), id)

		sw := &statusWriter{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(sw, r.WithContext(ctx))
		if sw.status == 0 {
			sw.status = http.StatusOK
		}

		lv := slog.LevelInfo
		switch {
		case sw.status >= 500:
			lv = slog.LevelError
		case sw.status >= 400:
			lv = slog.LevelWarn
		}
		l.LogAttrs(ctx, lv, "request",
			slog.String("request_id", id),
			RequestAttr(r),
			slog.Int("status", sw.status),
			slog.Int("bytes", sw.bytes),
			slog.Float64("duration_ms", float64(time.Since(start))/float64(time.Millisecond)),
		)
	})
}

// validInboundID returns s if it is a usable inbound request id, else "".
func validInboundID(s string) string {
	if s == "" || len(s) > maxInboundIDLen {
		return ""
	}
	for i := range len(s) {
		if s[i] < 0x20 || s[i] > 0x7e {
			return ""
		}
	}
	return s
}

// newRequestID returns 16 lowercase hex characters. crypto/rand.Read does not
// fail on supported platforms since Go 1.24, so its error is ignored.
func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// statusWriter records the first final status code and the bytes written.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

// WriteHeader records the first final (non-1xx) status and forwards every
// informational response without recording it.
func (w *statusWriter) WriteHeader(code int) {
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write counts bytes; a body written before any header implies 200.
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
