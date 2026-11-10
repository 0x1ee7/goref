// Package support holds the service-support helpers for exercise 09: errors
// and cleanup.
package support

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"
)

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

// Error renders the kind, the location, and the cause.
func (e *ConfigError) Error() string {
	var b strings.Builder
	if e.Kind != nil {
		b.WriteString(e.Kind.Error())
	} else {
		b.WriteString(ErrConfig.Error())
	}
	if e.Path != "" {
		fmt.Fprintf(&b, " (path %s)", e.Path)
	}
	if e.Key != "" {
		fmt.Fprintf(&b, " (key %s)", e.Key)
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Unwrap returns the underlying cause (Err), or nil.
func (e *ConfigError) Unwrap() error { return e.Err }

// Is reports whether target is the umbrella ErrConfig or this error's Kind.
func (e *ConfigError) Is(target error) bool {
	return target == ErrConfig || (e.Kind != nil && target == e.Kind)
}

// LoadConfig reads the file at path and returns its top-level JSON object.
//
// Failures are *ConfigError values. A missing file is ErrConfigNotFound; any
// other read failure is ErrConfigUnreadable. Invalid UTF-8, malformed JSON
// and a root that is not an object are ErrConfigInvalid.
func LoadConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		kind := ErrConfigUnreadable
		if errors.Is(err, fs.ErrNotExist) {
			kind = ErrConfigNotFound
		}
		return nil, &ConfigError{Kind: kind, Path: path, Err: err}
	}
	if off := invalidUTF8Offset(data); off >= 0 {
		return nil, &ConfigError{
			Kind: ErrConfigInvalid,
			Path: path,
			Err:  fmt.Errorf("invalid UTF-8 at byte %d", off),
		}
	}
	// Decode into any, not map: "null" would otherwise decode into a nil map
	// without error.
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, &ConfigError{Kind: ErrConfigInvalid, Path: path, Err: err}
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return nil, &ConfigError{Kind: ErrConfigInvalid, Path: path}
	}
	return obj, nil
}

// invalidUTF8Offset returns the byte offset of the first invalid UTF-8
// sequence in b, or -1.
func invalidUTF8Offset(b []byte) int {
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			return i
		}
		i += size
	}
	return -1
}

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
	var cur any = cfg
	for _, seg := range strings.Split(key, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return zero, &ConfigError{Kind: ErrSettingMissing, Key: key}
		}
		next, ok := obj[seg]
		if !ok {
			return zero, &ConfigError{Kind: ErrSettingMissing, Key: key}
		}
		cur = next
	}

	if convert == nil {
		v, ok := cur.(T)
		if !ok {
			return zero, &ConfigError{
				Kind: ErrConfigInvalid,
				Key:  key,
				Err:  fmt.Errorf("value %v is %T, want %T", cur, cur, zero),
			}
		}
		return v, nil
	}

	v, err := convert(cur)
	if err != nil {
		if errors.Is(err, ErrConfig) {
			return zero, err
		}
		return zero, &ConfigError{
			Kind: ErrConfigInvalid,
			Key:  key,
			Err:  fmt.Errorf("value %v: %w", cur, err),
		}
	}
	return v, nil
}

// Describe renders err and every error reachable from it through Unwrap,
// single or []error, depth-first in pre-order. Each entry has the form
// "%T: <Error()>". Each distinct error appears once, so cycles terminate.
// Describe(nil) returns no entries.
func Describe(err error) []string {
	var out []string
	seen := make(map[error]bool)
	var walk func(error)
	walk = func(e error) {
		if e == nil {
			return
		}
		// Only comparable dynamic types can be map keys.
		if reflect.TypeOf(e).Comparable() {
			if seen[e] {
				return
			}
			seen[e] = true
		}
		out = append(out, fmt.Sprintf("%T: %s", e, e.Error()))
		switch u := e.(type) {
		case interface{ Unwrap() error }:
			walk(u.Unwrap())
		case interface{ Unwrap() []error }:
			for _, c := range u.Unwrap() {
				walk(c)
			}
		}
	}
	walk(err)
	return out
}
