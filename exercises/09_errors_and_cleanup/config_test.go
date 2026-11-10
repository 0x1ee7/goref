package support_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	support "goref/exercises/09_errors_and_cleanup"
)

func writeConfig(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// wantConfigError asserts err is a *ConfigError of the given kind, reachable
// both through the kind sentinel and through the umbrella ErrConfig.
func wantConfigError(t *testing.T, err error, kind error) *support.ConfigError {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want kind %v", kind)
	}
	if !errors.Is(err, kind) {
		t.Fatalf("errors.Is(%v, %v) = false", err, kind)
	}
	if !errors.Is(err, support.ErrConfig) {
		t.Fatalf("errors.Is(%v, ErrConfig) = false, want umbrella match", err)
	}
	ce, ok := errors.AsType[*support.ConfigError](err)
	if !ok {
		t.Fatalf("error %T (%v) does not unwrap to *ConfigError", err, err)
	}
	return ce
}

func TestPart1LoadConfig(t *testing.T) {
	t.Run("valid object", func(t *testing.T) {
		p := writeConfig(t, []byte(`{"name":"svc","db":{"pool":{"size":4}}}`))
		cfg, err := support.LoadConfig(p)
		if err != nil {
			t.Fatalf("LoadConfig(valid) error = %v", err)
		}
		want := map[string]any{
			"name": "svc",
			"db":   map[string]any{"pool": map[string]any{"size": float64(4)}},
		}
		if diff := cmp.Diff(want, cfg); diff != "" {
			t.Errorf("LoadConfig() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("missing file is not found", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "absent.json")
		_, err := support.LoadConfig(p)
		ce := wantConfigError(t, err, support.ErrConfigNotFound)
		if ce.Path != p {
			t.Errorf("Path = %q, want %q", ce.Path, p)
		}
	})

	t.Run("directory is unreadable", func(t *testing.T) {
		dir := t.TempDir()
		_, err := support.LoadConfig(dir)
		ce := wantConfigError(t, err, support.ErrConfigUnreadable)
		if ce.Path != dir {
			t.Errorf("Path = %q, want %q", ce.Path, dir)
		}
		if errors.Is(err, support.ErrConfigNotFound) {
			t.Errorf("unreadable directory must not match ErrConfigNotFound")
		}
	})

	t.Run("invalid UTF-8 is invalid", func(t *testing.T) {
		p := writeConfig(t, []byte("{\"a\":\"\xff\"}"))
		_, err := support.LoadConfig(p)
		ce := wantConfigError(t, err, support.ErrConfigInvalid)
		if ce.Path != p {
			t.Errorf("Path = %q, want %q", ce.Path, p)
		}
	})

	t.Run("malformed JSON is invalid", func(t *testing.T) {
		p := writeConfig(t, []byte("{\"a\":"))
		_, err := support.LoadConfig(p)
		ce := wantConfigError(t, err, support.ErrConfigInvalid)
		if ce.Path != p {
			t.Errorf("Path = %q, want %q", ce.Path, p)
		}
		if !strings.Contains(err.Error(), p) {
			t.Errorf("Error() = %q, want it to contain the path %q", err.Error(), p)
		}
	})

	t.Run("non-object root is invalid without cause", func(t *testing.T) {
		p := writeConfig(t, []byte(`[1,2,3]`))
		_, err := support.LoadConfig(p)
		ce := wantConfigError(t, err, support.ErrConfigInvalid)
		if ce.Err != nil {
			t.Errorf("Err = %v, want nil for a non-object root", ce.Err)
		}
	})
}

func TestPart2ConfigErrorIs(t *testing.T) {
	t.Run("kind and umbrella match, other kinds do not", func(t *testing.T) {
		ce := &support.ConfigError{Kind: support.ErrConfigInvalid, Key: "db.pool"}
		if !errors.Is(ce, support.ErrConfigInvalid) {
			t.Errorf("errors.Is(kind) = false, want true")
		}
		if !errors.Is(ce, support.ErrConfig) {
			t.Errorf("errors.Is(ErrConfig) = false, want true")
		}
		if errors.Is(ce, support.ErrConfigNotFound) {
			t.Errorf("errors.Is(other kind) = true, want false")
		}
	})

	t.Run("cause is reachable through Unwrap", func(t *testing.T) {
		boom := errors.New("boom")
		ce := &support.ConfigError{Kind: support.ErrConfigUnreadable, Path: "p", Err: boom}
		if !errors.Is(ce, boom) {
			t.Errorf("errors.Is(cause) = false, want true")
		}
		if got := ce.Unwrap(); got != boom {
			t.Errorf("Unwrap() = %v, want %v", got, boom)
		}
	})
}

func TestPart2Setting(t *testing.T) {
	boom := errors.New("boom")
	cfg := map[string]any{
		"name":  "svc",
		"debug": true,
		"db": map[string]any{
			"pool": map[string]any{"size": float64(4)},
		},
	}
	asInt := func(v any) (int, error) {
		f, ok := v.(float64)
		if !ok {
			return 0, fmt.Errorf("want number, got %T", v)
		}
		return int(f), nil
	}

	t.Run("nested key with converter", func(t *testing.T) {
		got, err := support.Setting(cfg, "db.pool.size", asInt)
		if err != nil {
			t.Fatalf("Setting(db.pool.size) error = %v", err)
		}
		if got != 4 {
			t.Errorf("Setting(db.pool.size) = %d, want 4", got)
		}
	})

	t.Run("nil converter accepts matching type", func(t *testing.T) {
		got, err := support.Setting[string](cfg, "name", nil)
		if err != nil {
			t.Fatalf("Setting(name) error = %v", err)
		}
		if got != "svc" {
			t.Errorf("Setting(name) = %q, want %q", got, "svc")
		}
	})

	t.Run("missing key is setting missing", func(t *testing.T) {
		_, err := support.Setting(cfg, "db.pool.timeout", asInt)
		ce := wantConfigError(t, err, support.ErrSettingMissing)
		if ce.Key != "db.pool.timeout" {
			t.Errorf("Key = %q, want %q", ce.Key, "db.pool.timeout")
		}
	})

	t.Run("non-object segment is setting missing", func(t *testing.T) {
		_, err := support.Setting[string](cfg, "name.first", nil)
		wantConfigError(t, err, support.ErrSettingMissing)
	})

	t.Run("nil converter with wrong type is invalid", func(t *testing.T) {
		_, err := support.Setting[string](cfg, "debug", nil)
		ce := wantConfigError(t, err, support.ErrConfigInvalid)
		if ce.Key != "debug" {
			t.Errorf("Key = %q, want %q", ce.Key, "debug")
		}
	})

	t.Run("converter error is invalid and keeps its cause", func(t *testing.T) {
		_, err := support.Setting(cfg, "name", func(any) (int, error) { return 0, boom })
		wantConfigError(t, err, support.ErrConfigInvalid)
		if !errors.Is(err, boom) {
			t.Errorf("errors.Is(err, boom) = false, want true")
		}
	})

	t.Run("converter ConfigError passes through unchanged", func(t *testing.T) {
		want := &support.ConfigError{Kind: support.ErrSettingMissing, Key: "custom"}
		_, err := support.Setting(cfg, "name", func(any) (int, error) { return 0, want })
		if err != want {
			t.Errorf("Setting() error = %v (%p), want the converter's own error %p", err, err, want)
		}
	})

	t.Run("failure returns zero value", func(t *testing.T) {
		got, err := support.Setting(cfg, "db.pool.size", func(any) (int, error) { return 9, boom })
		if err == nil {
			t.Fatalf("error = nil, want converter failure")
		}
		if got != 0 {
			t.Errorf("value = %d on failure, want zero", got)
		}
	})
}

func TestPart2Describe(t *testing.T) {
	t.Run("wrapped chain in pre-order", func(t *testing.T) {
		inner := errors.New("disk full")
		mid := fmt.Errorf("write: %w", inner)
		outer := &support.ConfigError{Kind: support.ErrConfigUnreadable, Path: "p", Err: mid}
		got := support.Describe(outer)
		want := []string{
			"*support.ConfigError: ",
			"*fmt.wrapError: write: disk full",
			"*errors.errorString: disk full",
		}
		if len(got) != len(want) {
			t.Fatalf("Describe() = %q, want %d entries", got, len(want))
		}
		if !strings.HasPrefix(got[0], want[0]) {
			t.Errorf("entry 0 = %q, want prefix %q", got[0], want[0])
		}
		for _, part := range []string{support.ErrConfigUnreadable.Error(), "p", "disk full"} {
			if !strings.Contains(got[0], part) {
				t.Errorf("entry 0 = %q, want it to mention %q", got[0], part)
			}
		}
		if diff := cmp.Diff(want[1:], got[1:]); diff != "" {
			t.Errorf("Describe() tail mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("join is expanded and shared errors appear once", func(t *testing.T) {
		shared := errors.New("shared")
		got := support.Describe(errors.Join(shared, shared))
		want := []string{
			"*errors.joinError: shared\nshared",
			"*errors.errorString: shared",
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("Describe(join) mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("nil is empty and a plain error is one entry", func(t *testing.T) {
		got := support.Describe(errors.New("solo"))
		if diff := cmp.Diff([]string{"*errors.errorString: solo"}, got); diff != "" {
			t.Errorf("Describe(plain) mismatch (-want +got):\n%s", diff)
		}
		if n := len(support.Describe(nil)); n != 0 {
			t.Errorf("Describe(nil) has %d entries, want 0", n)
		}
	})

	t.Run("cycle terminates", func(t *testing.T) {
		l := &loopErr{n: 1}
		got := support.Describe(l)
		if diff := cmp.Diff([]string{"*support_test.loopErr: loop"}, got); diff != "" {
			t.Errorf("Describe(cycle) mismatch (-want +got):\n%s", diff)
		}
	})
}

// loopErr unwraps to itself, so Describe must stop on the second visit.
type loopErr struct{ n int }

func (e *loopErr) Error() string { return "loop" }
func (e *loopErr) Unwrap() error { return e }
