package shoplog

import (
	"log/slog"
	"net/http"
)

// RedactedValue is what secret-bearing values log instead of the secret.
const RedactedValue = "[REDACTED]"

// Secret is a string that must never reach a log line. It redacts itself
// through slog.LogValuer and through fmt (String), so %v and %s are safe too.
type Secret string

// LogValue always returns RedactedValue.
func (Secret) LogValue() slog.Value { return slog.StringValue(RedactedValue) }

// String always returns RedactedValue.
func (Secret) String() string { return RedactedValue }

// LazyString is a string computed only when a record is actually emitted.
// A nil LazyString logs as the empty string.
type LazyString func() string

// LogValue calls the function once per emitted record.
func (f LazyString) LogValue() slog.Value {
	if f == nil {
		return slog.StringValue("")
	}
	return slog.StringValue(f())
}

// Credentials logs as a group with the user name and a redacted password.
type Credentials struct {
	User     string
	Password string
}

// LogValue returns a group {user, password} where password is always
// RedactedValue, even when empty (the presence of a password is not secret).
func (c Credentials) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("user", c.User),
		slog.Any("password", Secret(c.Password)),
	)
}

// RequestAttr describes an inbound request as the group "http" with the
// attributes method, path and user_agent. The query string is deliberately
// omitted: it routinely carries tokens. A nil request returns the zero Attr,
// which handlers drop.
func RequestAttr(r *http.Request) slog.Attr {
	if r == nil {
		return slog.Attr{}
	}
	path := ""
	if r.URL != nil {
		path = r.URL.Path
	}
	return slog.Group("http",
		slog.String("method", r.Method),
		slog.String("path", path),
		slog.String("user_agent", r.UserAgent()),
	)
}
