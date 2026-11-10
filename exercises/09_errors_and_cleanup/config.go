// Package support holds the service-support helpers for exercise 09: errors
// and cleanup. Every exported name here is part of the contract the tests
// check; the bodies are yours to write.
package support

import "errors"

// Sentinel error kinds for configuration loading and lookup. Match them with
// errors.Is. ErrConfig is the umbrella: every *ConfigError matches it.
var (
	ErrConfig           = errors.New("config error")
	ErrConfigNotFound   = errors.New("config not found")
	ErrConfigUnreadable = errors.New("config unreadable")
	ErrConfigInvalid    = errors.New("config invalid")
	ErrSettingMissing   = errors.New("setting not defined")
)

// ConfigError is the single error type for config loading and lookup.
//
// Kind is one of the sentinels above. Path and Key say where the failure
// happened; whichever applies is set. Err is the underlying cause and may be
// nil.
type ConfigError struct {
	Kind error
	Path string
	Key  string
	Err  error
}

// Error renders the error. See the README for the format.
func (e *ConfigError) Error() string { return "" }

// Unwrap returns the underlying cause (Err), or nil.
func (e *ConfigError) Unwrap() error { return nil }

// Is reports whether target is the umbrella ErrConfig or this error's Kind.
func (e *ConfigError) Is(target error) bool { return false }

// LoadConfig reads the file at path and returns its top-level JSON object.
//
// Failures are *ConfigError values. A missing file is ErrConfigNotFound; any
// other read failure is ErrConfigUnreadable. Invalid UTF-8, malformed JSON
// and a root that is not an object are ErrConfigInvalid.
func LoadConfig(path string) (map[string]any, error) { return nil, nil }

// Setting looks up a dotted key such as "db.pool.size" in cfg and returns its
// value as T.
//
// If convert is nil, the value must already be a T. Otherwise convert turns
// the raw value into a T. A missing key, or a non-object on the way to the
// key, is ErrSettingMissing. A value of the wrong type, or a convert error,
// is ErrConfigInvalid. An error from convert that already matches ErrConfig is
// returned unchanged.
func Setting[T any](cfg map[string]any, key string, convert func(any) (T, error)) (T, error) {
	var zero T
	return zero, nil
}

// Describe renders err and every error reachable from it through Unwrap,
// single or []error, depth-first in pre-order. Each entry has the form
// "%T: <Error()>". Each distinct error appears once, so cycles terminate.
// Describe(nil) returns no entries.
func Describe(err error) []string { return nil }
