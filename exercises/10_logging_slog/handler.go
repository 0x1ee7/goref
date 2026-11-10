// Package shoplog is the structured logging toolkit of the shop HTTP service,
// built on log/slog: handler construction, redaction through slog.LogValuer,
// a logfmt handler written from scratch, context-scoped request ids, fan-out,
// per-component levels, sampling, HTTP access logs and default-logger
// management. The spec is README.md; this file implements Part 1.
package shoplog

import (
	"errors"
	"io"
	"log/slog"
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
	return nil, errors.New("not implemented")
}

// ParseLevel parses a level name such as "info", "WARNING" or "error+2"
// (case-insensitive, surrounding space ignored). Invalid input, including the
// empty string, returns an error wrapping ErrUnknownLevel.
func ParseLevel(s string) (slog.Level, error) {
	return 0, errors.New("not implemented")
}
