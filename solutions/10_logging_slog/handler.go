// Package shoplog is the structured logging toolkit of the shop HTTP service,
// built on log/slog: handler construction, redaction through slog.LogValuer,
// a logfmt handler written from scratch, context-scoped request ids, fan-out,
// per-component levels, sampling, HTTP access logs and default-logger
// management. The spec is README.md; this file implements Part 1.
package shoplog

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
)

// Format selects the encoding produced by NewHandler.
type Format string

const (
	// FormatJSON encodes each record as one JSON object per line.
	FormatJSON Format = "json"
	// FormatText encodes each record as one key=value line.
	FormatText Format = "text"
)

// ErrUnknownFormat is returned (wrapped) by NewHandler for an unsupported Format.
var ErrUnknownFormat = errors.New("shoplog: unknown log format")

// ErrUnknownLevel is returned (wrapped) by ParseLevel for an unrecognised name.
var ErrUnknownLevel = errors.New("shoplog: unknown log level")

// NewHandler returns a slog handler writing to w in the given format, with
// the top-level keys normalised by ReplaceAttr (see README, Part 1). A nil
// level means INFO, as in slog. Unknown formats return ErrUnknownFormat.
func NewHandler(w io.Writer, format Format, level slog.Leveler, addSource bool) (slog.Handler, error) {
	if level == nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{
		Level:       level,
		AddSource:   addSource,
		ReplaceAttr: replaceTopLevel,
	}
	switch format {
	case FormatJSON:
		return slog.NewJSONHandler(w, opts), nil
	case FormatText:
		return slog.NewTextHandler(w, opts), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownFormat, format)
	}
}

// replaceTopLevel normalises the built-in top-level attributes. Attributes
// inside groups are left alone: a user key named "msg" must stay as written.
func replaceTopLevel(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		if a.Value.Kind() == slog.KindTime {
			a.Value = slog.TimeValue(a.Value.Time().UTC())
		}
	case slog.LevelKey:
		if lv, ok := a.Value.Any().(slog.Level); ok {
			a.Value = slog.StringValue(strings.ToLower(lv.String()))
		}
	case slog.MessageKey:
		a.Key = "message"
	case slog.SourceKey:
		if src, ok := a.Value.Any().(*slog.Source); ok {
			a = slog.String("file", filepath.Base(src.File)+":"+strconv.Itoa(src.Line))
		}
	}
	return a
}

// ParseLevel parses a level name such as "info", "WARNING" or "error+2"
// (case-insensitive, surrounding space ignored). Invalid input, including the
// empty string, returns an error wrapping ErrUnknownLevel.
func ParseLevel(s string) (slog.Level, error) {
	name := strings.ToUpper(strings.TrimSpace(s))
	if name == "WARNING" {
		name = "WARN" // slog spells the level WARN; accept the pyref-style alias
	}
	var lv slog.Level
	if name == "" || lv.UnmarshalText([]byte(name)) != nil {
		return 0, fmt.Errorf("%w: %q", ErrUnknownLevel, s)
	}
	return lv, nil
}
