// Package deployment is a small, production-shaped HTTP service: 12-factor
// configuration, health endpoints, graceful shutdown, a testable command line
// and a container image. The logic lives in this package so tests can import
// it; cmd/goref-service is a thin wrapper around [Main].
package deployment

import (
	"errors"
	"log/slog"
	"time"
)

var (
	// ErrRequired reports a variable that must be set but is empty or blank.
	ErrRequired = errors.New("required but not set")

	// ErrInvalid reports a set variable whose value cannot be used. Errors
	// wrapping it describe the problem without echoing secret values.
	ErrInvalid = errors.New("invalid value")
)

// FieldError describes one bad environment variable.
type FieldError struct {
	Var string // environment variable name, e.g. "PORT"
	Err error  // ErrRequired, or an error wrapping ErrInvalid
}

// Error returns "VAR: problem".
func (e *FieldError) Error() string { return "" }

// Unwrap exposes Err, so errors.Is and errors.As see through a joined error.
func (e *FieldError) Unwrap() error { return nil }

// Secret is a string whose value never reaches logs or fmt output. Read the
// value with string(s).
type Secret string

// String masks the value for %v and %s.
func (Secret) String() string { return "" }

// GoString masks the value for %#v, which would otherwise print it raw.
func (Secret) GoString() string { return "" }

// LogValue masks the value for log/slog.
func (Secret) LogValue() slog.Value { return slog.Value{} }

// Config is the validated service configuration. Build it with [LoadConfig].
type Config struct {
	DatabaseURL    string        // DATABASE_URL (required): scheme postgresql or sqlite
	APIToken       Secret        // API_TOKEN (required): at least 16 characters
	AppName        string        // APP_NAME: default "goref-service"
	Port           int           // PORT: 1..65535, default 8080
	Debug          bool          // DEBUG: default false
	AllowedHosts   []string      // ALLOWED_HOSTS: comma list; nil when empty
	RequestTimeout time.Duration // REQUEST_TIMEOUT: > 0, default 30s
	LogLevel       slog.Level    // LOG_LEVEL: DEBUG, INFO, WARN or ERROR; default INFO
}

// LoadConfig builds a Config from getenv (pass os.Getenv in main). A variable
// that is empty or blank counts as unset. Every problem is collected: the error
// is an errors.Join of *FieldError values, or nil when the configuration is valid.
func LoadConfig(getenv func(string) string) (Config, error) {
	return Config{}, errors.New("not implemented")
}

// RedactURL masks the password in a URL: "postgresql://app:hunter2@db:5432/x"
// becomes "postgresql://app:***@db:5432/x". URLs without a password are
// returned unchanged. An unparseable URL is returned unchanged too.
func RedactURL(raw string) string { return "" }

// String renders the configuration with secrets masked.
func (c Config) String() string { return "" }

// GoString keeps %#v as safe as %v: the database password must not leak.
func (c Config) GoString() string { return "" }

// LogValue lets slog.Any("cfg", cfg) log the configuration with secrets masked.
func (c Config) LogValue() slog.Value { return slog.Value{} }
