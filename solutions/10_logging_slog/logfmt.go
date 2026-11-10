package shoplog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// logfmtHandler renders records as logfmt lines. Derived handlers share the
// writer and its mutex, so a single Write per record keeps lines whole.
type logfmtHandler struct {
	mu    *sync.Mutex
	w     io.Writer
	level slog.Leveler
	pre   []byte // pre-rendered WithAttrs output, each pair led by a space
	group string // dotted prefix of the open groups, e.g. "http."
}

// NewLogfmtHandler returns a handler written from scratch against the
// slog.Handler contract. Each record is one line:
//
//	level=INFO msg="hello world" svc=api http.method=GET
//
// No timestamp is written. Groups flatten to dotted keys, values are
// resolved (LogValuer included) and quoted when they contain space, '=', '"',
// control characters or are empty. A nil level means INFO. See README, Part 3.
func NewLogfmtHandler(w io.Writer, level slog.Leveler) slog.Handler {
	if level == nil {
		level = slog.LevelInfo
	}
	return &logfmtHandler{mu: new(sync.Mutex), w: w, level: level}
}

// Enabled reports whether lv meets the handler's threshold.
func (h *logfmtHandler) Enabled(_ context.Context, lv slog.Level) bool {
	return lv >= h.level.Level()
}

// Handle renders r as one logfmt line and writes it in a single call.
func (h *logfmtHandler) Handle(_ context.Context, r slog.Record) error {
	line := appendKV(nil, "level", r.Level.String())
	line = appendKV(line, "msg", r.Message)
	line = append(line, h.pre...)
	r.Attrs(func(a slog.Attr) bool {
		line = appendAttr(line, h.group, a)
		return true
	})
	line = append(line, '\n')

	h.mu.Lock()
	_, err := h.w.Write(line[1:]) // drop the leading space of the first pair
	h.mu.Unlock()
	if err != nil {
		return fmt.Errorf("logfmt: write: %w", err)
	}
	return nil
}

// WithAttrs returns a handler whose lines carry attrs after the record's own
// pre-rendered prefix, keyed under the current group.
func (h *logfmtHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	pre := append([]byte(nil), h.pre...)
	for _, a := range attrs {
		pre = appendAttr(pre, h.group, a)
	}
	nh := *h
	nh.pre = pre
	return &nh
}

// WithGroup returns a handler whose later attributes are nested under name.
// An empty name is a no-op, as slog requires.
func (h *logfmtHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := *h
	nh.group = h.group + name + "."
	return &nh
}

// appendAttr renders one attribute, recursing into groups. Empty groups
// render nothing; an empty group name inlines its attributes.
func appendAttr(buf []byte, prefix string, a slog.Attr) []byte {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return buf
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, ga := range a.Value.Group() {
			buf = appendAttr(buf, prefix, ga)
		}
		return buf
	}
	return appendKV(buf, prefix+a.Key, valueString(a.Value))
}

// appendKV appends " key=value", quoting value when logfmt requires it.
// The leading space is always written; callers strip the first one.
func appendKV(buf []byte, key, value string) []byte {
	buf = append(buf, ' ')
	buf = append(buf, key...)
	buf = append(buf, '=')
	if needsQuote(value) {
		return strconv.AppendQuote(buf, value)
	}
	return append(buf, value...)
}

// needsQuote reports whether value cannot appear bare in a logfmt line.
func needsQuote(value string) bool {
	if value == "" {
		return true
	}
	for _, r := range value {
		if r <= ' ' || r == '=' || r == '"' || r == 0x7f {
			return true
		}
	}
	return false
}

// valueString renders a resolved, non-group value as text.
func valueString(v slog.Value) string {
	if v.Kind() == slog.KindTime {
		return v.Time().UTC().Format(time.RFC3339Nano)
	}
	return v.String()
}
