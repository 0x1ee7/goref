package shoplog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	shoplog "goref/exercises/10_logging_slog"
)

// errBoom is returned by failWriter.
var errBoom = errors.New("boom")

// syncBuffer is a bytes.Buffer safe for concurrent writers and readers.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// failWriter fails every write with errBoom.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errBoom }

// nonNil fails the test if a constructor returned the zero value.
func nonNil[T comparable](t *testing.T, what string, v T) T {
	t.Helper()
	var zero T
	if v == zero {
		t.Fatalf("%s returned the zero value (not implemented?)", what)
	}
	return v
}

// newHandler calls NewHandler and fails the test on error or a nil handler.
func newHandler(t *testing.T, w io.Writer, f shoplog.Format, lv slog.Leveler, addSource bool) slog.Handler {
	t.Helper()
	h, err := shoplog.NewHandler(w, f, lv, addSource)
	if err != nil {
		t.Fatalf("NewHandler(%q): %v", f, err)
	}
	return nonNil(t, "NewHandler", h)
}

// newLogfmt returns a logfmt handler writing to w.
func newLogfmt(t *testing.T, w io.Writer, lv slog.Leveler) slog.Handler {
	t.Helper()
	return nonNil(t, "NewLogfmtHandler", shoplog.NewLogfmtHandler(w, lv))
}

// jsonStd returns a stdlib JSON handler at Debug level, for tests that must
// not depend on this package's handlers.
func jsonStd(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
}

// lines returns the non-empty lines of s.
func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// decodeJSON parses every line of s as a JSON object.
func decodeJSON(t *testing.T, s string) []map[string]any {
	t.Helper()
	var recs []map[string]any
	for _, l := range lines(s) {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("line is not a JSON object: %v\n%s", err, l)
		}
		recs = append(recs, m)
	}
	return recs
}

// msgs returns the stdlib "msg" value of each record.
func msgs(recs []map[string]any) []string {
	var out []string
	for _, r := range recs {
		s, _ := r["msg"].(string)
		out = append(out, s)
	}
	return out
}

// normalisedMsgs returns the "message" value of each record, as written by
// NewHandler's ReplaceAttr (top-level msg is renamed to message).
func normalisedMsgs(recs []map[string]any) []string {
	var out []string
	for _, r := range recs {
		s, _ := r["message"].(string)
		out = append(out, s)
	}
	return out
}

// isolateDefault restores slog's default logger when the test ends.
func isolateDefault(t *testing.T) {
	t.Helper()
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
}

// attrAdder adds an attribute to the record it receives.
type attrAdder struct{}

func (attrAdder) Enabled(context.Context, slog.Level) bool { return true }
func (attrAdder) Handle(_ context.Context, r slog.Record) error {
	r.AddAttrs(slog.Int("added", 1))
	return nil
}
func (a attrAdder) WithAttrs([]slog.Attr) slog.Handler { return a }
func (a attrAdder) WithGroup(string) slog.Handler      { return a }

// attrCounter records how many attributes the record it receives carries.
type attrCounter struct{ got *int }

func (c attrCounter) Enabled(context.Context, slog.Level) bool { return true }
func (c attrCounter) Handle(_ context.Context, r slog.Record) error {
	*c.got = r.NumAttrs()
	return nil
}
func (c attrCounter) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c attrCounter) WithGroup(string) slog.Handler      { return c }

// ---------------------------------------------------------------- Part 1

func TestPart1Handlers(t *testing.T) {
	t.Run("json record has normalised top-level keys", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(newHandler(t, &buf, shoplog.FormatJSON, nil, false))
		logger.Info("hello", "k", 1)
		recs := decodeJSON(t, buf.String())
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1: %q", len(recs), buf.String())
		}
		r := recs[0]
		want := map[string]any{"level": "info", "message": "hello", "k": 1.0}
		for k, v := range want {
			if r[k] != v {
				t.Errorf("%s = %v, want %v", k, r[k], v)
			}
		}
		if _, ok := r["msg"]; ok {
			t.Error(`top-level "msg" should be renamed to "message"`)
		}
		if ts, _ := r["time"].(string); ts == "" {
			t.Error(`top-level "time" missing`)
		}
	})

	t.Run("time is rendered in UTC", func(t *testing.T) {
		var buf syncBuffer
		h := newHandler(t, &buf, shoplog.FormatJSON, nil, false)
		at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("CET", 3600))
		if err := h.Handle(context.Background(), slog.NewRecord(at, slog.LevelInfo, "x", 0)); err != nil {
			t.Fatal(err)
		}
		recs := decodeJSON(t, buf.String())
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1", len(recs))
		}
		if got := recs[0]["time"]; got != "2026-01-02T02:04:05Z" {
			t.Errorf("time = %v, want 2026-01-02T02:04:05Z", got)
		}
	})

	t.Run("text output uses normalised keys", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newHandler(t, &buf, shoplog.FormatText, nil, false)).Info("hello", "k", 1)
		out := buf.String()
		if !strings.Contains(out, "level=info message=hello k=1") {
			t.Errorf("text line = %q, want it to contain %q", out, "level=info message=hello k=1")
		}
	})

	t.Run("level threshold filters records", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(newHandler(t, &buf, shoplog.FormatJSON, slog.LevelWarn, false))
		logger.Info("dropped")
		logger.Warn("kept")
		if got := normalisedMsgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"kept"}) {
			t.Errorf("messages = %q, want [kept]", got)
		}
	})

	t.Run("nil level means INFO", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(newHandler(t, &buf, shoplog.FormatJSON, nil, false))
		logger.Debug("dropped")
		logger.Info("kept")
		if got := normalisedMsgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"kept"}) {
			t.Errorf("messages = %q, want [kept]", got)
		}
	})

	t.Run("source is shortened to file:line", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newHandler(t, &buf, shoplog.FormatJSON, nil, true)).Info("x")
		recs := decodeJSON(t, buf.String())
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1", len(recs))
		}
		file, _ := recs[0]["file"].(string)
		if !regexp.MustCompile(`^shoplog_test\.go:\d+$`).MatchString(file) {
			t.Errorf("file = %v, want shoplog_test.go:<line>", recs[0]["file"])
		}
	})

	t.Run("keys inside groups are not normalised", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(newHandler(t, &buf, shoplog.FormatJSON, nil, false)).WithGroup("g")
		logger.Info("x", "msg", "inner", "level", "keep")
		recs := decodeJSON(t, buf.String())
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1", len(recs))
		}
		g, _ := recs[0]["g"].(map[string]any)
		if g["msg"] != "inner" || g["level"] != "keep" {
			t.Errorf("group g = %v, want msg=inner level=keep", g)
		}
	})

	t.Run("unknown format is rejected", func(t *testing.T) {
		_, err := shoplog.NewHandler(io.Discard, "xml", nil, false)
		if !errors.Is(err, shoplog.ErrUnknownFormat) {
			t.Errorf("err = %v, want ErrUnknownFormat", err)
		}
	})

	parseCases := []struct {
		in   string
		want slog.Level
	}{
		{"info", slog.LevelInfo},
		{"  WARN ", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"Error+2", slog.LevelError + 2},
		{"debug", slog.LevelDebug},
	}
	for _, tc := range parseCases {
		t.Run(fmt.Sprintf("ParseLevel(%q)", tc.in), func(t *testing.T) {
			got, err := shoplog.ParseLevel(tc.in)
			if err != nil {
				t.Fatalf("ParseLevel(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}

	for _, in := range []string{"", "loud"} {
		t.Run(fmt.Sprintf("ParseLevel(%q) is rejected", in), func(t *testing.T) {
			_, err := shoplog.ParseLevel(in)
			if !errors.Is(err, shoplog.ErrUnknownLevel) {
				t.Errorf("err = %v, want ErrUnknownLevel", err)
			}
		})
	}
}

// ---------------------------------------------------------------- Part 2

func TestPart2Values(t *testing.T) {
	t.Run("Secret is redacted in records and fmt", func(t *testing.T) {
		var buf syncBuffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", "token", shoplog.Secret("abc"))
		out := buf.String()
		if !strings.Contains(out, `"token":"[REDACTED]"`) {
			t.Errorf("record = %s, want token redacted", out)
		}
		if strings.Contains(out, "abc") {
			t.Errorf("record = %s leaks the secret", out)
		}
		if got := fmt.Sprintf("%v|%s", shoplog.Secret("abc"), shoplog.Secret("abc")); got != "[REDACTED]|[REDACTED]" {
			t.Errorf("fmt = %q, want [REDACTED]|[REDACTED]", got)
		}
	})

	t.Run("LazyString is computed once when emitted", func(t *testing.T) {
		var buf syncBuffer
		calls := 0
		f := shoplog.LazyString(func() string { calls++; return "computed" })
		slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", "v", f)
		if calls != 1 {
			t.Errorf("function called %d times, want 1", calls)
		}
		if recs := decodeJSON(t, buf.String()); len(recs) == 1 && recs[0]["v"] != "computed" {
			t.Errorf("v = %v, want computed", recs[0]["v"])
		}
	})

	t.Run("LazyString is not computed for disabled records", func(t *testing.T) {
		var buf syncBuffer
		calls := 0
		f := shoplog.LazyString(func() string { calls++; return "x" })
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
		logger.Warn("sanity", "v", f) // emitted, so the function runs once
		logger.Info("disabled", "v", f)
		if calls != 1 {
			t.Errorf("function called %d times, want exactly 1 (only for the emitted WARN)", calls)
		}
	})

	t.Run("nil LazyString logs empty string", func(t *testing.T) {
		var buf syncBuffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", "v", shoplog.LazyString(nil))
		if recs := decodeJSON(t, buf.String()); len(recs) == 1 && recs[0]["v"] != "" {
			t.Errorf("v = %v, want empty string", recs[0]["v"])
		}
	})

	t.Run("Credentials logs a group with redacted password", func(t *testing.T) {
		var buf syncBuffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", "cred", shoplog.Credentials{User: "ann", Password: "pw"})
		recs := decodeJSON(t, buf.String())
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1", len(recs))
		}
		want := map[string]any{"user": "ann", "password": "[REDACTED]"}
		if diff := cmp.Diff(want, recs[0]["cred"]); diff != "" {
			t.Errorf("cred (-want +got):\n%s", diff)
		}
	})

	t.Run("Credentials redacts an empty password too", func(t *testing.T) {
		var buf syncBuffer
		slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", "cred", shoplog.Credentials{User: "bob"})
		recs := decodeJSON(t, buf.String())
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1", len(recs))
		}
		cred, _ := recs[0]["cred"].(map[string]any)
		if cred["password"] != "[REDACTED]" {
			t.Errorf("password = %v, want [REDACTED]", cred["password"])
		}
	})

	t.Run("RequestAttr carries method, path and user agent without query", func(t *testing.T) {
		var buf syncBuffer
		req := httptest.NewRequest("POST", "/pay?token=s3cr3t&card=4111", nil)
		req.Header.Set("User-Agent", "curl/8")
		slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", shoplog.RequestAttr(req))
		out := buf.String()
		if strings.Contains(out, "s3cr3t") || strings.Contains(out, "card") {
			t.Errorf("record = %s leaks the query string", out)
		}
		recs := decodeJSON(t, out)
		if len(recs) != 1 {
			t.Fatalf("got %d records, want 1", len(recs))
		}
		want := map[string]any{"method": "POST", "path": "/pay", "user_agent": "curl/8"}
		if diff := cmp.Diff(want, recs[0]["http"]); diff != "" {
			t.Errorf("http group (-want +got):\n%s", diff)
		}
	})

	t.Run("RequestAttr of nil request is the zero Attr", func(t *testing.T) {
		if a := shoplog.RequestAttr(httptest.NewRequest("GET", "/", nil)); a.Key != "http" {
			t.Fatalf("sanity: RequestAttr(non-nil).Key = %q, want http", a.Key)
		}
		if got := shoplog.RequestAttr(nil); !got.Equal(slog.Attr{}) {
			t.Errorf("RequestAttr(nil) = %v, want zero Attr", got)
		}
	})
}

// ---------------------------------------------------------------- Part 3

func TestPart3Logfmt(t *testing.T) {
	t.Run("basic line has level, msg and sorted attrs in call order", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newLogfmt(t, &buf, nil)).Info("hello world", "svc", "api", "n", 3)
		want := "level=INFO msg=\"hello world\" svc=api n=3\n"
		if got := buf.String(); got != want {
			t.Errorf("line = %q, want %q", got, want)
		}
	})

	t.Run("nil level hides DEBUG", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newLogfmt(t, &buf, nil)).Debug("hidden")
		if got := buf.String(); got != "" {
			t.Errorf("output = %q, want none", got)
		}
	})

	t.Run("custom level passes DEBUG", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newLogfmt(t, &buf, slog.LevelDebug)).Debug("shown")
		if got, want := buf.String(), "level=DEBUG msg=shown\n"; got != want {
			t.Errorf("line = %q, want %q", got, want)
		}
	})

	quoting := []struct{ name, in, want string }{
		{"bare", "abc_1.2", "abc_1.2"},
		{"empty", "", `""`},
		{"equals", "a=b", `"a=b"`},
		{"space", "a b", `"a b"`},
		{"quote", `say "hi"`, `"say \"hi\""`},
		{"newline", "a\nb", `"a\nb"`},
	}
	for _, tc := range quoting {
		t.Run("quoting "+tc.name, func(t *testing.T) {
			var buf syncBuffer
			slog.New(newLogfmt(t, &buf, nil)).Info("m", "k", tc.in)
			if want := "level=INFO msg=m k=" + tc.want + "\n"; buf.String() != want {
				t.Errorf("line = %q, want %q", buf.String(), want)
			}
		})
	}

	t.Run("WithGroup prefixes keys with dots", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newLogfmt(t, &buf, nil)).WithGroup("http").Info("x", "method", "GET", "status", 200)
		if want := "level=INFO msg=x http.method=GET http.status=200\n"; buf.String() != want {
			t.Errorf("line = %q, want %q", buf.String(), want)
		}
	})

	t.Run("nested and empty group names", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newLogfmt(t, &buf, nil)).WithGroup("a").WithGroup("").WithGroup("b").Info("x", "k", 1)
		if want := "level=INFO msg=x a.b.k=1\n"; buf.String() != want {
			t.Errorf("line = %q, want %q", buf.String(), want)
		}
	})

	t.Run("With attributes keep their group prefix", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newLogfmt(t, &buf, nil)).With("svc", "api").WithGroup("g").With("x", 1).Info("m", "y", 2)
		if want := "level=INFO msg=m svc=api g.x=1 g.y=2\n"; buf.String() != want {
			t.Errorf("line = %q, want %q", buf.String(), want)
		}
	})

	t.Run("times are RFC3339Nano in UTC", func(t *testing.T) {
		var buf syncBuffer
		h := newLogfmt(t, &buf, nil)
		at := time.Date(2026, 1, 2, 3, 4, 5, 6, time.FixedZone("X", 3600))
		r := slog.NewRecord(time.Time{}, slog.LevelInfo, "m", 0)
		r.AddAttrs(slog.Time("at", at))
		if err := h.Handle(context.Background(), r); err != nil {
			t.Fatal(err)
		}
		if want := "level=INFO msg=m at=2026-01-02T02:04:05.000000006Z\n"; buf.String() != want {
			t.Errorf("line = %q, want %q", buf.String(), want)
		}
	})

	t.Run("durations, errors and LogValuers", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newLogfmt(t, &buf, nil)).Info("m",
			"d", 1500*time.Millisecond,
			"err", errBoom,
			"tok", shoplog.Secret("abc"),
			"cred", shoplog.Credentials{User: "ann", Password: "pw"},
		)
		want := "level=INFO msg=m d=1.5s err=boom tok=[REDACTED] cred.user=ann cred.password=[REDACTED]\n"
		if buf.String() != want {
			t.Errorf("line = %q, want %q", buf.String(), want)
		}
	})

	t.Run("empty group renders nothing", func(t *testing.T) {
		var buf syncBuffer
		slog.New(newLogfmt(t, &buf, nil)).Info("x", slog.Group("g"))
		if want := "level=INFO msg=x\n"; buf.String() != want {
			t.Errorf("line = %q, want %q", buf.String(), want)
		}
	})

	t.Run("write errors are wrapped and returned", func(t *testing.T) {
		h := newLogfmt(t, failWriter{}, nil)
		r := slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0)
		if err := h.Handle(context.Background(), r); !errors.Is(err, errBoom) {
			t.Errorf("Handle error = %v, want one wrapping errBoom", err)
		}
	})

	t.Run("concurrent writers never interleave lines", func(t *testing.T) {
		var buf syncBuffer
		base := slog.New(newLogfmt(t, &buf, nil))
		var wg sync.WaitGroup
		for w := range 8 {
			child := base.With("w", w)
			wg.Go(func() {
				for i := range 50 {
					child.Info("m", "i", i)
				}
			})
		}
		wg.Wait()
		got := lines(buf.String())
		if len(got) != 400 {
			t.Fatalf("got %d lines, want 400", len(got))
		}
		re := regexp.MustCompile(`^level=INFO msg=m w=\d+ i=\d+$`)
		for _, l := range got {
			if !re.MatchString(l) {
				t.Fatalf("interleaved or malformed line: %q", l)
			}
		}
	})
}

// ---------------------------------------------------------------- Part 4

func TestPart4RequestIDsAndFanout(t *testing.T) {
	t.Run("request id round trips through context", func(t *testing.T) {
		ctx := shoplog.WithRequestID(context.Background(), "r-1")
		if id, ok := shoplog.RequestIDFromContext(ctx); id != "r-1" || !ok {
			t.Errorf("RequestIDFromContext = (%q, %v), want (r-1, true)", id, ok)
		}
		if id, ok := shoplog.RequestIDFromContext(context.Background()); id != "" || ok {
			t.Errorf("empty context = (%q, %v), want (\"\", false)", id, ok)
		}
	})

	t.Run("handler adds request_id from the context", func(t *testing.T) {
		var buf syncBuffer
		h := nonNil(t, "NewRequestIDHandler", shoplog.NewRequestIDHandler(jsonStd(&buf)))
		slog.New(h).InfoContext(shoplog.WithRequestID(context.Background(), "abc"), "m")
		recs := decodeJSON(t, buf.String())
		if len(recs) != 1 || recs[0]["request_id"] != "abc" {
			t.Errorf("records = %v, want one with request_id=abc", recs)
		}
	})

	t.Run("no id in context means no attribute", func(t *testing.T) {
		var buf syncBuffer
		h := nonNil(t, "NewRequestIDHandler", shoplog.NewRequestIDHandler(jsonStd(&buf)))
		slog.New(h).InfoContext(context.Background(), "m")
		recs := decodeJSON(t, buf.String())
		if _, ok := recs[0]["request_id"]; len(recs) != 1 || ok {
			t.Errorf("records = %v, want one without request_id", recs)
		}
	})

	t.Run("empty id in context is ignored", func(t *testing.T) {
		var buf syncBuffer
		h := nonNil(t, "NewRequestIDHandler", shoplog.NewRequestIDHandler(jsonStd(&buf)))
		slog.New(h).InfoContext(shoplog.WithRequestID(context.Background(), ""), "m")
		if _, ok := decodeJSON(t, buf.String())[0]["request_id"]; ok {
			t.Errorf("record = %s, want no request_id", buf.String())
		}
	})

	t.Run("explicit record attribute wins", func(t *testing.T) {
		var buf syncBuffer
		h := nonNil(t, "NewRequestIDHandler", shoplog.NewRequestIDHandler(jsonStd(&buf)))
		ctx := shoplog.WithRequestID(context.Background(), "abc")
		slog.New(h).InfoContext(ctx, "m", "request_id", "explicit")
		if got := decodeJSON(t, buf.String())[0]["request_id"]; got != "explicit" {
			t.Errorf("request_id = %v, want explicit", got)
		}
	})

	t.Run("explicit With attribute wins and is not duplicated", func(t *testing.T) {
		var buf syncBuffer
		h := nonNil(t, "NewRequestIDHandler", shoplog.NewRequestIDHandler(jsonStd(&buf)))
		logger := slog.New(h).With("request_id", "pre")
		logger.InfoContext(shoplog.WithRequestID(context.Background(), "abc"), "m")
		out := buf.String()
		if got := decodeJSON(t, out)[0]["request_id"]; got != "pre" {
			t.Errorf("request_id = %v, want pre", got)
		}
		if n := strings.Count(out, `"request_id"`); n != 1 {
			t.Errorf("request_id appears %d times, want 1: %s", n, out)
		}
	})

	t.Run("attribute lands inside an open group", func(t *testing.T) {
		var buf syncBuffer
		h := nonNil(t, "NewRequestIDHandler", shoplog.NewRequestIDHandler(jsonStd(&buf)))
		logger := slog.New(h).WithGroup("job")
		logger.InfoContext(shoplog.WithRequestID(context.Background(), "abc"), "m", "n", 1)
		job, _ := decodeJSON(t, buf.String())[0]["job"].(map[string]any)
		if job["request_id"] != "abc" {
			t.Errorf("job group = %v, want request_id=abc inside it", job)
		}
	})

	t.Run("Enabled delegates to next", func(t *testing.T) {
		h := nonNil(t, "NewRequestIDHandler", shoplog.NewRequestIDHandler(slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn})))
		ctx := context.Background()
		if h.Enabled(ctx, slog.LevelInfo) {
			t.Error("Enabled(INFO) = true, want false")
		}
		if !h.Enabled(ctx, slog.LevelWarn) {
			t.Error("Enabled(WARN) = false, want true")
		}
	})

	t.Run("fan-out delivers only to enabled children", func(t *testing.T) {
		var a, b syncBuffer
		ha := jsonStd(&a)
		hb := slog.NewJSONHandler(&b, &slog.HandlerOptions{Level: slog.LevelError})
		logger := slog.New(nonNil(t, "NewFanout", shoplog.NewFanout(ha, hb)))
		logger.Info("i")
		logger.Error("e")
		if got := msgs(decodeJSON(t, a.String())); !cmp.Equal(got, []string{"i", "e"}) {
			t.Errorf("first child messages = %q, want [i e]", got)
		}
		if got := msgs(decodeJSON(t, b.String())); !cmp.Equal(got, []string{"e"}) {
			t.Errorf("second child messages = %q, want [e]", got)
		}
	})

	t.Run("fan-out children get their own record copy", func(t *testing.T) {
		got := -1
		h := nonNil(t, "NewFanout", shoplog.NewFanout(attrAdder{}, attrCounter{&got}))
		if err := h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0)); err != nil {
			t.Fatal(err)
		}
		if got != 0 {
			t.Errorf("second child saw %d attrs, want 0 (records must be cloned)", got)
		}
	})

	t.Run("fan-out joins child errors and keeps going", func(t *testing.T) {
		var good syncBuffer
		h := nonNil(t, "NewFanout", shoplog.NewFanout(jsonStd(failWriter{}), jsonStd(&good)))
		err := h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "m", 0))
		if !errors.Is(err, errBoom) {
			t.Errorf("Handle error = %v, want one wrapping errBoom", err)
		}
		if got := msgs(decodeJSON(t, good.String())); !cmp.Equal(got, []string{"m"}) {
			t.Errorf("healthy child messages = %q, want [m]", got)
		}
	})

	t.Run("fan-out forwards With and WithGroup", func(t *testing.T) {
		var a, b syncBuffer
		logger := slog.New(nonNil(t, "NewFanout", shoplog.NewFanout(jsonStd(&a), jsonStd(&b)))).With("svc", "api").WithGroup("g")
		logger.Info("m", "k", 1)
		for name, buf := range map[string]*syncBuffer{"a": &a, "b": &b} {
			recs := decodeJSON(t, buf.String())
			if len(recs) != 1 || recs[0]["svc"] != "api" {
				t.Errorf("child %s records = %v, want svc=api", name, recs)
				continue
			}
			if g, _ := recs[0]["g"].(map[string]any); g["k"] != 1.0 {
				t.Errorf("child %s group g = %v, want k=1", name, g)
			}
		}
	})

	t.Run("zero children are never enabled", func(t *testing.T) {
		h := nonNil(t, "NewFanout", shoplog.NewFanout())
		if h.Enabled(context.Background(), slog.LevelError) {
			t.Error("Enabled = true with no children, want false")
		}
		if err := h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelError, "m", 0)); err != nil {
			t.Errorf("Handle = %v, want nil", err)
		}
	})
}

// ---------------------------------------------------------------- Part 5

func TestPart5ComponentsAndSampling(t *testing.T) {
	components := func(t *testing.T, w io.Writer, def slog.Level, levels map[string]slog.Level) slog.Handler {
		t.Helper()
		return nonNil(t, "NewComponentLevels", shoplog.NewComponentLevels(jsonStd(w), def, levels))
	}
	base := map[string]slog.Level{"payments": slog.LevelDebug, "db": slog.LevelError}

	t.Run("component level overrides the default", func(t *testing.T) {
		var buf syncBuffer
		slog.New(components(t, &buf, slog.LevelInfo, base)).With("component", "payments").Debug("p")
		if got := msgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"p"}) {
			t.Errorf("messages = %q, want [p]", got)
		}
	})

	t.Run("absent or unknown component uses the default", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(components(t, &buf, slog.LevelInfo, base))
		logger.Info("i1")
		logger.Debug("d1")
		logger.With("component", "cache").Info("i2")
		logger.With("component", "cache").Debug("d2")
		if got := msgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"i1", "i2"}) {
			t.Errorf("messages = %q, want [i1 i2]", got)
		}
	})

	t.Run("db component threshold is ERROR", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(components(t, &buf, slog.LevelInfo, base)).With("component", "db")
		logger.Warn("w")
		logger.Error("e")
		if got := msgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"e"}) {
			t.Errorf("messages = %q, want [e]", got)
		}
	})

	t.Run("record attribute sets the component and overrides With", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(components(t, &buf, slog.LevelInfo, base))
		logger.Debug("r1", "component", "payments")
		logger.With("component", "db").Debug("r2", "component", "payments")
		if got := msgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"r1", "r2"}) {
			t.Errorf("messages = %q, want [r1 r2]", got)
		}
	})

	t.Run("component inside a group is ignored", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(components(t, &buf, slog.LevelInfo, base)).WithGroup("g")
		logger.Debug("dropped", "component", "payments")
		logger.Info("y", "component", "db")
		if got := msgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"y"}) {
			t.Errorf("messages = %q, want [y]", got)
		}
	})

	t.Run("Enabled uses the lowest configured threshold", func(t *testing.T) {
		h := components(t, io.Discard, slog.LevelInfo, base)
		if !h.Enabled(context.Background(), slog.LevelDebug) {
			t.Error("Enabled(DEBUG) = false, want true (payments allows DEBUG)")
		}
		if h.Enabled(context.Background(), slog.LevelDebug-4) {
			t.Error("Enabled(below DEBUG) = true, want false")
		}
	})

	t.Run("levels map is copied", func(t *testing.T) {
		var buf syncBuffer
		m := map[string]slog.Level{"db": slog.LevelError}
		logger := slog.New(components(t, &buf, slog.LevelInfo, m))
		m["db"] = slog.LevelDebug
		logger.With("component", "db").Info("i")
		logger.With("component", "db").Error("e")
		if got := msgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"e"}) {
			t.Errorf("messages = %q, want [e]", got)
		}
	})

	t.Run("next's own level still applies", func(t *testing.T) {
		var buf syncBuffer
		next := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
		logger := slog.New(nonNil(t, "NewComponentLevels", shoplog.NewComponentLevels(next, slog.LevelDebug, base)))
		logger.With("component", "payments").Info("dropped by next")
		logger.With("component", "payments").Warn("w")
		if got := msgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"w"}) {
			t.Errorf("messages = %q, want [w]", got)
		}
	})

	sampled := func(t *testing.T, w io.Writer, first, thereafter int) slog.Handler {
		t.Helper()
		h, err := shoplog.NewSampler(jsonStd(w), first, thereafter)
		if err != nil {
			t.Fatalf("NewSampler(%d, %d): %v", first, thereafter, err)
		}
		return nonNil(t, "NewSampler", h)
	}

	t.Run("first N then every Mth occurrence", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(sampled(t, &buf, 2, 3))
		for i := 1; i <= 10; i++ {
			logger.Info("m", "n", i)
		}
		var got []float64
		for _, r := range decodeJSON(t, buf.String()) {
			got = append(got, r["n"].(float64))
		}
		if want := []float64{1, 2, 5, 8}; !cmp.Equal(got, want) {
			t.Errorf("kept occurrences = %v, want %v", got, want)
		}
	})

	t.Run("messages are sampled independently", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(sampled(t, &buf, 1, 1000))
		for range 3 {
			logger.Info("a")
			logger.Info("b")
		}
		if got := msgs(decodeJSON(t, buf.String())); !cmp.Equal(got, []string{"a", "b"}) {
			t.Errorf("messages = %q, want [a b]", got)
		}
	})

	t.Run("WARN and above are never sampled", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(sampled(t, &buf, 0, 1000))
		for range 5 {
			logger.Warn("w")
		}
		if n := len(decodeJSON(t, buf.String())); n != 5 {
			t.Errorf("got %d WARN records, want 5", n)
		}
	})

	t.Run("counters are shared with derived handlers", func(t *testing.T) {
		var buf syncBuffer
		h := sampled(t, &buf, 1, 1000)
		slog.New(h).Info("m")
		slog.New(h.WithAttrs([]slog.Attr{slog.String("svc", "x")})).Info("m")
		if n := len(decodeJSON(t, buf.String())); n != 1 {
			t.Errorf("got %d records, want 1 (second occurrence is shared and dropped)", n)
		}
	})

	t.Run("thereafter of 1 keeps everything after first", func(t *testing.T) {
		var buf syncBuffer
		logger := slog.New(sampled(t, &buf, 0, 1))
		for range 5 {
			logger.Info("m")
		}
		if n := len(decodeJSON(t, buf.String())); n != 5 {
			t.Errorf("got %d records, want 5", n)
		}
	})

	t.Run("invalid arguments wrap ErrInvalidSampling", func(t *testing.T) {
		for _, tc := range []struct{ first, thereafter int }{{-1, 1}, {0, 0}} {
			_, err := shoplog.NewSampler(jsonStd(io.Discard), tc.first, tc.thereafter)
			if !errors.Is(err, shoplog.ErrInvalidSampling) {
				t.Errorf("NewSampler(%d, %d) error = %v, want ErrInvalidSampling", tc.first, tc.thereafter, err)
			}
		}
	})
}

// ---------------------------------------------------------------- Part 6

// accessRun is the outcome of one request through AccessLog.
type accessRun struct {
	rec  *httptest.ResponseRecorder
	logs []map[string]any
}

// runAccessLog serves req through AccessLog(next) and decodes the logs.
func runAccessLog(t *testing.T, next http.Handler, req *http.Request) accessRun {
	t.Helper()
	var buf syncBuffer
	h := nonNil(t, "AccessLog", shoplog.AccessLog(slog.New(slog.NewJSONHandler(&buf, nil)), next))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return accessRun{rec: rec, logs: decodeJSON(t, buf.String())}
}

var hexID = regexp.MustCompile(`^[0-9a-f]{16}$`)

func TestPart6AccessLog(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })

	t.Run("generates a 16 hex id when none is sent", func(t *testing.T) {
		run := runAccessLog(t, ok, httptest.NewRequest("GET", "/items", nil))
		id := run.rec.Header().Get(shoplog.RequestIDHeader)
		if !hexID.MatchString(id) {
			t.Errorf("response %s = %q, want 16 lowercase hex chars", shoplog.RequestIDHeader, id)
		}
		if len(run.logs) != 1 || run.logs[0]["request_id"] != id {
			t.Errorf("logged request_id = %v, want %q", run.logs, id)
		}
	})

	t.Run("honours a valid inbound id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/items", nil)
		req.Header.Set(shoplog.RequestIDHeader, "abc-123")
		run := runAccessLog(t, ok, req)
		if got := run.rec.Header().Get(shoplog.RequestIDHeader); got != "abc-123" {
			t.Errorf("response id = %q, want abc-123", got)
		}
		if run.logs[0]["request_id"] != "abc-123" {
			t.Errorf("logged request_id = %v, want abc-123", run.logs[0]["request_id"])
		}
	})

	invalid := []struct {
		name, in string
		kept     bool
	}{
		{"128 bytes is kept", strings.Repeat("y", 128), true},
		{"129 bytes is replaced", strings.Repeat("x", 129), false},
		{"control character is replaced", "a\tb", false},
	}
	for _, tc := range invalid {
		t.Run("inbound id: "+tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(shoplog.RequestIDHeader, tc.in)
			got := runAccessLog(t, ok, req).rec.Header().Get(shoplog.RequestIDHeader)
			if tc.kept {
				if got != tc.in {
					t.Errorf("response id = %q, want the inbound id kept", got)
				}
				return
			}
			if got == tc.in || !hexID.MatchString(got) {
				t.Errorf("response id = %q, want a fresh 16 hex id", got)
			}
		})
	}

	t.Run("handler sees the id in its context", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, _ := shoplog.RequestIDFromContext(r.Context())
			fmt.Fprint(w, id)
		})
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set(shoplog.RequestIDHeader, "ctx-7")
		run := runAccessLog(t, next, req)
		if body := run.rec.Body.String(); body != "ctx-7" {
			t.Errorf("handler saw id %q, want ctx-7", body)
		}
	})

	t.Run("logs one record with request fields", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, "hello")
		})
		req := httptest.NewRequest("GET", "/items?x=1", nil)
		req.Header.Set("User-Agent", "test-agent")
		run := runAccessLog(t, next, req)
		if len(run.logs) != 1 {
			t.Fatalf("got %d log records, want 1", len(run.logs))
		}
		r := run.logs[0]
		if r["msg"] != "request" || r["level"] != "INFO" {
			t.Errorf("msg=%v level=%v, want request INFO", r["msg"], r["level"])
		}
		if r["status"] != 201.0 || r["bytes"] != 5.0 {
			t.Errorf("status=%v bytes=%v, want 201 and 5", r["status"], r["bytes"])
		}
		if d, _ := r["duration_ms"].(float64); d < 0 {
			t.Errorf("duration_ms = %v, want a non-negative number", r["duration_ms"])
		}
		want := map[string]any{"method": "GET", "path": "/items", "user_agent": "test-agent"}
		if diff := cmp.Diff(want, r["http"]); diff != "" {
			t.Errorf("http group (-want +got):\n%s", diff)
		}
	})

	t.Run("level follows the status class", func(t *testing.T) {
		cases := []struct {
			status int
			level  string
		}{
			{200, "INFO"}, {302, "INFO"}, {404, "WARN"}, {503, "ERROR"},
		}
		for _, tc := range cases {
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) })
			run := runAccessLog(t, next, httptest.NewRequest("GET", "/", nil))
			if run.logs[0]["level"] != tc.level {
				t.Errorf("status %d logged at %v, want %s", tc.status, run.logs[0]["level"], tc.level)
			}
		}
	})

	t.Run("body without WriteHeader logs status 200", func(t *testing.T) {
		run := runAccessLog(t, ok, httptest.NewRequest("GET", "/", nil))
		if run.logs[0]["status"] != 200.0 {
			t.Errorf("status = %v, want 200", run.logs[0]["status"])
		}
	})

	t.Run("first WriteHeader wins", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.WriteHeader(http.StatusInternalServerError)
		})
		run := runAccessLog(t, next, httptest.NewRequest("GET", "/", nil))
		if run.logs[0]["status"] != 201.0 || run.rec.Code != 201 {
			t.Errorf("logged status=%v, response code=%d, want both 201", run.logs[0]["status"], run.rec.Code)
		}
	})

	t.Run("informational responses are not the final status", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusContinue)
			w.WriteHeader(http.StatusAccepted)
		})
		run := runAccessLog(t, next, httptest.NewRequest("GET", "/", nil))
		if run.logs[0]["status"] != 202.0 {
			t.Errorf("logged status = %v, want 202", run.logs[0]["status"])
		}
	})

	t.Run("ResponseController reaches the underlying writer", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if err := http.NewResponseController(w).Flush(); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		})
		run := runAccessLog(t, next, httptest.NewRequest("GET", "/", nil))
		if !run.rec.Flushed {
			t.Error("underlying writer was not flushed; statusWriter must implement Unwrap")
		}
	})

	t.Run("nil logger uses slog.Default at request time", func(t *testing.T) {
		isolateDefault(t)
		h := nonNil(t, "AccessLog", shoplog.AccessLog(nil, ok))
		var buf syncBuffer
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if recs := decodeJSON(t, buf.String()); len(recs) != 1 || recs[0]["msg"] != "request" {
			t.Errorf("default logger records = %v, want one request record", recs)
		}
	})
}

// ---------------------------------------------------------------- Part 7

func TestPart7DefaultLogger(t *testing.T) {
	t.Run("installs the logger and restores the previous one", func(t *testing.T) {
		isolateDefault(t)
		prev := slog.Default()
		l := slog.New(slog.NewJSONHandler(io.Discard, nil))
		restore := shoplog.UseDefault(l)
		if slog.Default() != l {
			t.Fatal("slog.Default() is not the installed logger")
		}
		restore()
		if slog.Default() != prev {
			t.Error("restore did not reinstate the previous default logger")
		}
	})

	t.Run("slog and log output reach the installed handler", func(t *testing.T) {
		isolateDefault(t)
		var buf syncBuffer
		restore := shoplog.UseDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		defer restore()
		slog.Info("hi", "k", 1)
		log.Print("legacy")
		got := msgs(decodeJSON(t, buf.String()))
		if len(got) != 2 || got[0] != "hi" || strings.TrimSpace(got[1]) != "legacy" {
			t.Errorf("messages = %q, want [hi legacy]", got)
		}
	})

	t.Run("restore is idempotent and does not undo a newer install", func(t *testing.T) {
		isolateDefault(t)
		prev := slog.Default()
		a := slog.New(slog.NewJSONHandler(io.Discard, nil))
		r1 := shoplog.UseDefault(a)
		if slog.Default() != a {
			t.Fatal("slog.Default() is not the installed logger")
		}
		r1()
		c := slog.New(slog.NewJSONHandler(io.Discard, nil))
		slog.SetDefault(c)
		r1()
		if slog.Default() != c {
			t.Error("second restore undid a newer installation")
		}
		slog.SetDefault(prev)
	})

	t.Run("nested installs unwind in order", func(t *testing.T) {
		isolateDefault(t)
		prev := slog.Default()
		a := slog.New(slog.NewJSONHandler(io.Discard, nil))
		b := slog.New(slog.NewJSONHandler(io.Discard, nil))
		r1 := shoplog.UseDefault(a)
		r2 := shoplog.UseDefault(b)
		if slog.Default() != b {
			t.Fatal("inner install not active")
		}
		r2()
		if slog.Default() != a {
			t.Error("after inner restore, default is not the outer logger")
		}
		r1()
		if slog.Default() != prev {
			t.Error("after outer restore, default is not the original logger")
		}
	})
}
