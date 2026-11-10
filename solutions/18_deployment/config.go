// Package deployment is a small, production-shaped HTTP service: 12-factor
// configuration, health endpoints, graceful shutdown, a testable command line
// and a container image. The logic lives in this package so tests can import
// it; cmd/goref-service is a thin wrapper around [Main].
package deployment

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Configuration defaults and limits.
const (
	defaultAppName        = "goref-service"
	defaultPort           = 8080
	defaultRequestTimeout = 30 * time.Second
	minAPITokenLen        = 16
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

func (e *FieldError) Error() string { return e.Var + ": " + e.Err.Error() }

// Unwrap exposes Err, so errors.Is and errors.As see through a joined error.
func (e *FieldError) Unwrap() error { return e.Err }

// Secret is a string whose value never reaches logs or fmt output. Read the
// value with string(s).
type Secret string

// String masks the value for %v and %s.
func (Secret) String() string { return "***" }

// GoString masks the value for %#v, which would otherwise print it raw.
func (Secret) GoString() string { return `deployment.Secret("***")` }

// LogValue masks the value for log/slog.
func (Secret) LogValue() slog.Value { return slog.StringValue("***") }

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
	cfg := Config{
		AppName:        defaultAppName,
		Port:           defaultPort,
		RequestTimeout: defaultRequestTimeout,
		LogLevel:       slog.LevelInfo,
	}
	var errs []error
	fail := func(name string, err error) {
		errs = append(errs, &FieldError{Var: name, Err: err})
	}
	env := func(name string) string { return strings.TrimSpace(getenv(name)) }

	if v := env("DATABASE_URL"); v == "" {
		fail("DATABASE_URL", ErrRequired)
	} else if err := checkDatabaseURL(v); err != nil {
		fail("DATABASE_URL", err)
	} else {
		cfg.DatabaseURL = v
	}

	if v := env("API_TOKEN"); v == "" {
		fail("API_TOKEN", ErrRequired)
	} else if len(v) < minAPITokenLen {
		fail("API_TOKEN", fmt.Errorf("%w: must be at least %d characters", ErrInvalid, minAPITokenLen))
	} else {
		cfg.APIToken = Secret(v)
	}

	if v := env("APP_NAME"); v != "" {
		cfg.AppName = v
	}

	if v := env("PORT"); v != "" {
		if p, err := parsePort(v); err != nil {
			fail("PORT", err)
		} else {
			cfg.Port = p
		}
	}

	if v := env("DEBUG"); v != "" {
		if b, err := parseBool(v); err != nil {
			fail("DEBUG", err)
		} else {
			cfg.Debug = b
		}
	}

	cfg.AllowedHosts = parseList(env("ALLOWED_HOSTS"))

	if v := env("REQUEST_TIMEOUT"); v != "" {
		if d, err := parseDuration(v); err != nil {
			fail("REQUEST_TIMEOUT", err)
		} else if d <= 0 {
			fail("REQUEST_TIMEOUT", fmt.Errorf("%w: must be greater than zero", ErrInvalid))
		} else {
			cfg.RequestTimeout = d
		}
	}

	if v := env("LOG_LEVEL"); v != "" {
		if lvl, err := parseLevel(v); err != nil {
			fail("LOG_LEVEL", err)
		} else {
			cfg.LogLevel = lvl
		}
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}

// RedactURL masks the password in a URL: "postgresql://app:hunter2@db:5432/x"
// becomes "postgresql://app:***@db:5432/x". URLs without a password are
// returned unchanged. An unparseable URL is returned unchanged too.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	if _, ok := u.User.Password(); !ok {
		return raw
	}
	u.User = url.UserPassword(u.User.Username(), "***")
	// url.String percent-encodes '*' in userinfo; put the mask back in its readable form.
	return strings.ReplaceAll(u.String(), "%2A%2A%2A", "***")
}

// String renders the configuration with secrets masked.
func (c Config) String() string {
	return fmt.Sprintf("{DatabaseURL:%s APIToken:%s AppName:%s Port:%d Debug:%t AllowedHosts:%q RequestTimeout:%s LogLevel:%s}",
		RedactURL(c.DatabaseURL), c.APIToken, c.AppName, c.Port, c.Debug, c.AllowedHosts, c.RequestTimeout, c.LogLevel)
}

// GoString keeps %#v as safe as %v: the database password must not leak.
func (c Config) GoString() string { return c.String() }

// LogValue lets slog.Any("cfg", cfg) log the configuration with secrets masked.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("database_url", RedactURL(c.DatabaseURL)),
		slog.Any("api_token", c.APIToken),
		slog.String("app_name", c.AppName),
		slog.Int("port", c.Port),
		slog.Bool("debug", c.Debug),
		slog.Any("allowed_hosts", c.AllowedHosts),
		slog.Duration("request_timeout", c.RequestTimeout),
		slog.String("log_level", c.LogLevel.String()),
	)
}

func checkDatabaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		// url.Parse errors quote the input, which may carry a password: do not wrap them.
		return fmt.Errorf("%w: not a valid URL", ErrInvalid)
	}
	switch u.Scheme {
	case "postgresql", "sqlite":
		return nil
	default:
		return fmt.Errorf("%w: scheme %q must be postgresql or sqlite", ErrInvalid, u.Scheme)
	}
}

// parsePort accepts plain decimal digits only: strconv.Atoi would also take "+80".
func parsePort(s string) (int, error) {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("%w: %q is not a port in 1..65535", ErrInvalid, s)
	}
	return int(n), nil
}

// parseBool accepts the forms documented on Config. strconv.ParseBool would
// reject "yes" and "on".
func parseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%w: %q is not a boolean", ErrInvalid, s)
	}
}

// parseDuration accepts Go duration syntax plus a bare number of seconds.
// time.ParseDuration alone rejects "2" (only "0" is unitless).
func parseDuration(s string) (time.Duration, error) {
	if isDigits(s) {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n > math.MaxInt64/int64(time.Second) {
			return 0, fmt.Errorf("%w: %q is out of range", ErrInvalid, s)
		}
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a duration", ErrInvalid, s)
	}
	return d, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseList splits on commas, trims items and drops empty ones. It returns nil
// when nothing is left.
func parseList(s string) []string {
	var out []string
	for item := range strings.SplitSeq(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "WARN":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: %q must be DEBUG, INFO, WARN or ERROR", ErrInvalid, s)
	}
}
