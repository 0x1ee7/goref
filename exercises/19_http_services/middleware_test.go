package httpsvc_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	httpsvc "goref/exercises/19_http_services"
)

func okHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	})
}

// serveNoPanic runs h through do and fails the test, rather than crashing the
// binary, if a panic escapes the middleware chain.
func serveNoPanic(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("panic escaped the middleware chain: %v", p)
		}
	}()
	return do(t, h, method, target, "", nil)
}

func TestPart2Chain(t *testing.T) {
	var order []string
	trace := func(name string) httpsvc.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+" in")
				next.ServeHTTP(w, r)
				order = append(order, name+" out")
			})
		}
	}
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		order = append(order, "handler")
	})
	h := httpsvc.Chain(inner, trace("a"), trace("b"))
	do(t, h, http.MethodGet, "/", "", nil)
	want := []string{"a in", "b in", "handler", "b out", "a out"}
	if diff := cmp.Diff(want, order); diff != "" {
		t.Errorf("middleware order (-want +got):\n%s", diff)
	}
}

func TestPart2RequestID(t *testing.T) {
	long := strings.Repeat("a", 128)
	tests := []struct {
		name     string
		incoming string
		keep     bool
	}{
		{"absent header generates an id", "", false},
		{"valid id is kept", "abc-123_XYZ", true},
		{"128 bytes is kept", long, true},
		{"129 bytes is replaced", long + "a", false},
		{"embedded space is replaced", "bad id", false},
		{"non-ASCII is replaced", "é", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var inCtx string
			h := httpsvc.Chain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				inCtx = httpsvc.RequestIDFrom(r.Context())
			}), httpsvc.RequestID)
			hdr := http.Header{}
			if tc.incoming != "" {
				hdr.Set("X-Request-ID", tc.incoming)
			}
			rec := do(t, h, http.MethodGet, "/", "", hdr)
			got := rec.Header().Get("X-Request-ID")
			if tc.keep && got != tc.incoming {
				t.Errorf("X-Request-ID = %q, want incoming %q kept", got, tc.incoming)
			}
			if !tc.keep {
				if len(got) != 32 {
					t.Fatalf("generated X-Request-ID = %q, want 32 hex characters", got)
				}
				if _, err := hex.DecodeString(got); err != nil {
					t.Errorf("generated X-Request-ID %q is not hex: %v", got, err)
				}
			}
			if inCtx != got {
				t.Errorf("RequestIDFrom = %q, want the response id %q", inCtx, got)
			}
		})
	}
	t.Run("RequestIDFrom is empty without middleware", func(t *testing.T) {
		var inside string
		h := httpsvc.Chain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			inside = httpsvc.RequestIDFrom(r.Context())
		}), httpsvc.RequestID)
		do(t, h, http.MethodGet, "/", "", nil)
		if len(inside) != 32 {
			t.Fatalf("generated id inside the middleware = %q, want 32 hex chars", inside)
		}
		if got := httpsvc.RequestIDFrom(t.Context()); got != "" {
			t.Errorf("RequestIDFrom = %q, want empty", got)
		}
	})
}

func TestPart2AccessLog(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantLevel string
	}{
		{"2xx logs INFO", http.StatusOK, "INFO"},
		{"4xx logs WARN", http.StatusNotFound, "WARN"},
		{"5xx logs ERROR", http.StatusInternalServerError, "ERROR"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))
			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("hello"))
			})
			h := httpsvc.Chain(inner, httpsvc.RequestID, httpsvc.AccessLog(logger))
			do(t, h, http.MethodGet, "/items/7", "", http.Header{"X-Request-Id": {"req-42"}})

			var line map[string]any
			if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
				t.Fatalf("log output %q is not one JSON record: %v", buf.String(), err)
			}
			want := map[string]any{
				"level":      tc.wantLevel,
				"msg":        "request",
				"method":     "GET",
				"path":       "/items/7",
				"status":     float64(tc.status),
				"bytes":      float64(5),
				"request_id": "req-42",
			}
			for k, v := range want {
				if line[k] != v {
					t.Errorf("log %s = %v, want %v", k, line[k], v)
				}
			}
			if line["duration"] == nil {
				t.Errorf("log record has no duration: %v", line)
			}
		})
	}
}

func TestPart2Recover(t *testing.T) {
	discard := slog.New(slog.DiscardHandler)
	t.Run("panic becomes a 500 problem and is logged", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		h := httpsvc.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}), httpsvc.Recover(logger))
		rec := serveNoPanic(t, h, http.MethodGet, "/")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
		if p := problemOf(t, rec); p.Status != http.StatusInternalServerError {
			t.Errorf("problem status = %d, want 500", p.Status)
		}
		if logged := buf.String(); !strings.Contains(logged, `"level":"ERROR"`) || !strings.Contains(logged, "boom") {
			t.Errorf("log %q should record an ERROR with the panic value", logged)
		}
	})
	t.Run("status already written is kept", func(t *testing.T) {
		h := httpsvc.Chain(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			panic("late")
		}), httpsvc.Recover(discard))
		rec := serveNoPanic(t, h, http.MethodGet, "/")
		if rec.Code != http.StatusAccepted {
			t.Errorf("status = %d, want 202 kept", rec.Code)
		}
	})
	t.Run("http.ErrAbortHandler is re-panicked", func(t *testing.T) {
		plain := httpsvc.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}), httpsvc.Recover(discard))
		if rec := serveNoPanic(t, plain, http.MethodGet, "/"); rec.Code != http.StatusInternalServerError {
			t.Fatalf("plain panic status = %d, want 500 before testing abort", rec.Code)
		}
		h := httpsvc.Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		}), httpsvc.Recover(discard))
		var got any
		func() {
			defer func() { got = recover() }()
			do(t, h, http.MethodGet, "/", "", nil)
		}()
		if got != http.ErrAbortHandler {
			t.Errorf("recovered %v, want http.ErrAbortHandler re-panicked", got)
		}
	})
}

func TestPart2CORS(t *testing.T) {
	const app = "https://app.example"
	h := httpsvc.Chain(okHandler(http.StatusOK), httpsvc.CORS(app))
	t.Run("allowed origin is echoed with exposed headers", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/items", "", http.Header{"Origin": {app}})
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != app {
			t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, app)
		}
		if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
			t.Errorf("Vary = %q, want Origin", rec.Header().Get("Vary"))
		}
		if exp := rec.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(exp, "ETag") {
			t.Errorf("Access-Control-Expose-Headers = %q, want ETag", exp)
		}
	})
	t.Run("other origin gets no allow header", func(t *testing.T) {
		if rec := do(t, h, http.MethodGet, "/items", "", http.Header{"Origin": {app}}); rec.Header().Get("Access-Control-Allow-Origin") != app {
			t.Fatalf("allowed origin must get Access-Control-Allow-Origin %q", app)
		}
		rec := do(t, h, http.MethodGet, "/items", "", http.Header{"Origin": {"https://evil.example"}})
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want the handler's 200", rec.Code)
		}
	})
	t.Run("no origin passes through untouched", func(t *testing.T) {
		if rec := do(t, h, http.MethodGet, "/items", "", http.Header{"Origin": {app}}); rec.Header().Get("Access-Control-Allow-Origin") != app {
			t.Fatalf("allowed origin must get Access-Control-Allow-Origin %q", app)
		}
		rec := do(t, h, http.MethodGet, "/items", "", nil)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin = %q, want none", got)
		}
	})
	t.Run("wildcard allows any origin", func(t *testing.T) {
		wild := httpsvc.Chain(okHandler(http.StatusOK), httpsvc.CORS("*"))
		rec := do(t, wild, http.MethodGet, "/items", "", http.Header{"Origin": {"https://anything.example"}})
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
		}
	})
	t.Run("preflight from allowed origin answers 204", func(t *testing.T) {
		hdr := http.Header{"Origin": {app}, "Access-Control-Request-Method": {"PUT"}}
		rec := do(t, h, http.MethodOptions, "/items/1", "", hdr)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "PUT") {
			t.Errorf("Access-Control-Allow-Methods = %q, want PUT", rec.Header().Get("Access-Control-Allow-Methods"))
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != app {
			t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, app)
		}
	})
	t.Run("preflight from other origin is 403", func(t *testing.T) {
		hdr := http.Header{"Origin": {"https://evil.example"}, "Access-Control-Request-Method": {"PUT"}}
		rec := do(t, h, http.MethodOptions, "/items/1", "", hdr)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}

func TestPart2BearerAuth(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		header     string
		wantStatus int
	}{
		{"valid token", "s3cret", "Bearer s3cret", http.StatusOK},
		{"scheme is case-insensitive", "s3cret", "bearer s3cret", http.StatusOK},
		{"wrong token", "s3cret", "Bearer nope", http.StatusUnauthorized},
		{"prefix of token rejected", "s3cret", "Bearer s3cre", http.StatusUnauthorized},
		{"missing header", "s3cret", "", http.StatusUnauthorized},
		{"other scheme rejected", "s3cret", "Basic s3cret", http.StatusUnauthorized},
		{"scheme without credentials rejected", "s3cret", "Bearer", http.StatusUnauthorized},
		{"empty configured token denies all", "", "Bearer ", http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := httpsvc.Chain(okHandler(http.StatusOK), httpsvc.BearerAuth(tc.token))
			if rec := do(t, h, http.MethodGet, "/items", "", http.Header{"Authorization": {"Bearer nope"}}); rec.Code != http.StatusUnauthorized {
				t.Fatalf("wrong token must be rejected with 401, got %d", rec.Code)
			}
			hdr := http.Header{}
			if tc.header != "" {
				hdr.Set("Authorization", tc.header)
			}
			rec := do(t, h, http.MethodGet, "/items", "", hdr)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusUnauthorized {
				if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
					t.Errorf("WWW-Authenticate = %q, want Bearer", got)
				}
			}
		})
	}
}
