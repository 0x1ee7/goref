package support_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	support "goref/exercises/09_errors_and_cleanup"
)

// setAbsentEnv makes name unset for the rest of the test. t.Setenv registers
// the restore step, so the original state comes back at cleanup.
func setAbsentEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}

func lookupEnv(name string) (string, bool) { return os.LookupEnv(name) }

func TestPart5WithEnv(t *testing.T) {
	t.Run("sets, unsets and restores", func(t *testing.T) {
		t.Setenv("GOREF_EX09_KEEP", "old")
		t.Setenv("GOREF_EX09_GONE", "x")
		setAbsentEnv(t, "GOREF_EX09_NEW")

		ran := false
		err := support.WithEnv(map[string]*string{
			"GOREF_EX09_KEEP": new("new"),
			"GOREF_EX09_GONE": nil,
			"GOREF_EX09_NEW":  new("fresh"),
		}, func() error {
			ran = true
			if v, _ := lookupEnv("GOREF_EX09_KEEP"); v != "new" {
				t.Errorf("inside: KEEP = %q, want %q", v, "new")
			}
			if _, ok := lookupEnv("GOREF_EX09_GONE"); ok {
				t.Errorf("inside: GONE is still set, want unset")
			}
			if v, _ := lookupEnv("GOREF_EX09_NEW"); v != "fresh" {
				t.Errorf("inside: NEW = %q, want %q", v, "fresh")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("WithEnv() error = %v", err)
		}
		if !ran {
			t.Fatal("WithEnv() did not call fn")
		}
		if v, _ := lookupEnv("GOREF_EX09_KEEP"); v != "old" {
			t.Errorf("after: KEEP = %q, want restored %q", v, "old")
		}
		if v, _ := lookupEnv("GOREF_EX09_GONE"); v != "x" {
			t.Errorf("after: GONE = %q, want restored %q", v, "x")
		}
		if _, ok := lookupEnv("GOREF_EX09_NEW"); ok {
			t.Errorf("after: NEW is still set, want removed")
		}
	})

	t.Run("fn error is returned unchanged and state is restored", func(t *testing.T) {
		boom := errors.New("boom")
		t.Setenv("GOREF_EX09_ERR", "orig")
		err := support.WithEnv(map[string]*string{"GOREF_EX09_ERR": new("tmp")}, func() error {
			if v, _ := lookupEnv("GOREF_EX09_ERR"); v != "tmp" {
				t.Errorf("inside: ERR = %q, want %q", v, "tmp")
			}
			return boom
		})
		if err != boom {
			t.Fatalf("WithEnv() error = %v, want fn error unchanged", err)
		}
		if v, _ := lookupEnv("GOREF_EX09_ERR"); v != "orig" {
			t.Errorf("after error: ERR = %q, want %q", v, "orig")
		}
	})

	t.Run("panic still restores and propagates", func(t *testing.T) {
		t.Setenv("GOREF_EX09_PANIC", "orig")
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = support.WithEnv(map[string]*string{"GOREF_EX09_PANIC": new("tmp")}, func() error {
				panic("bang")
			})
		}()
		if recovered != "bang" {
			t.Fatalf("recovered = %v, want the panic %q to propagate", recovered, "bang")
		}
		if v, _ := lookupEnv("GOREF_EX09_PANIC"); v != "orig" {
			t.Errorf("after panic: PANIC = %q, want %q", v, "orig")
		}
	})

	t.Run("apply failure rolls back earlier changes and skips fn", func(t *testing.T) {
		t.Setenv("GOREF_EX09_A", "orig")
		// The NUL byte makes this name invalid on every platform. Changes are
		// applied in sorted order, so GOREF_EX09_A is applied first.
		called := false
		err := support.WithEnv(map[string]*string{
			"GOREF_EX09_A":     new("1"),
			"GOREF_EX09_Z\x00": new("2"),
		}, func() error {
			called = true
			return nil
		})
		if err == nil {
			t.Fatal("WithEnv() error = nil, want the invalid name to fail")
		}
		if called {
			t.Error("fn was called after an apply failure")
		}
		if v, _ := lookupEnv("GOREF_EX09_A"); v != "orig" {
			t.Errorf("after apply failure: A = %q, want rolled back to %q", v, "orig")
		}
	})
}

// fakeRC is an io.ReadCloser that records Close calls in log.
type fakeRC struct {
	name     string
	closeErr error
	log      *[]string
}

func (f *fakeRC) Read([]byte) (int, error) { return 0, io.EOF }

func (f *fakeRC) Close() error {
	*f.log = append(*f.log, "close "+f.name)
	return f.closeErr
}

func TestPart5OpenAll(t *testing.T) {
	t.Run("opens every path in order", func(t *testing.T) {
		var log []string
		open := func(name string) (io.ReadCloser, error) {
			return &fakeRC{name: name, log: &log}, nil
		}
		rcs, err := support.OpenAll([]string{"a", "b", "c"}, open)
		if err != nil {
			t.Fatalf("OpenAll() error = %v", err)
		}
		var names []string
		for _, rc := range rcs {
			names = append(names, rc.(*fakeRC).name)
		}
		if diff := cmp.Diff([]string{"a", "b", "c"}, names); diff != "" {
			t.Errorf("opened names mismatch (-want +got):\n%s", diff)
		}
		if len(log) != 0 {
			t.Errorf("closed %v on success, want nothing closed", log)
		}
	})

	t.Run("failure closes opened readers in reverse order", func(t *testing.T) {
		openErr := errors.New("no such file")
		var log []string
		open := func(name string) (io.ReadCloser, error) {
			if name == "c" {
				return nil, openErr
			}
			return &fakeRC{name: name, log: &log}, nil
		}
		rcs, err := support.OpenAll([]string{"a", "b", "c"}, open)
		if !errors.Is(err, openErr) {
			t.Fatalf("OpenAll() error = %v, want it to wrap the open failure", err)
		}
		if rcs != nil {
			t.Errorf("OpenAll() returned %d readers on failure, want none", len(rcs))
		}
		if diff := cmp.Diff([]string{"close b", "close a"}, log); diff != "" {
			t.Errorf("close order mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("close failures are joined with the open error", func(t *testing.T) {
		openErr := errors.New("no such file")
		closeErr := errors.New("close failed")
		var log []string
		open := func(name string) (io.ReadCloser, error) {
			switch name {
			case "b":
				return &fakeRC{name: name, log: &log, closeErr: closeErr}, nil
			case "c":
				return nil, openErr
			}
			return &fakeRC{name: name, log: &log}, nil
		}
		_, err := support.OpenAll([]string{"a", "b", "c"}, open)
		if !errors.Is(err, openErr) {
			t.Errorf("error = %v, want it to match the open failure", err)
		}
		if !errors.Is(err, closeErr) {
			t.Errorf("error = %v, want it to match the close failure too", err)
		}
	})
}

func TestPart5RemoveFiles(t *testing.T) {
	t.Run("removes existing files and skips missing ones", func(t *testing.T) {
		dir := t.TempDir()
		f1 := filepath.Join(dir, "f1")
		f2 := filepath.Join(dir, "f2")
		missing := filepath.Join(dir, "missing")
		for _, p := range []string{f1, f2} {
			if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		n, err := support.RemoveFiles([]string{f1, missing, f2})
		if err != nil {
			t.Fatalf("RemoveFiles() error = %v, want nil for missing files", err)
		}
		if n != 2 {
			t.Errorf("RemoveFiles() count = %d, want 2", n)
		}
		for _, p := range []string{f1, f2} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("%s still exists after RemoveFiles", p)
			}
		}
	})

	t.Run("stops at the first real failure with the count so far", func(t *testing.T) {
		dir := t.TempDir()
		f1 := filepath.Join(dir, "f1")
		f2 := filepath.Join(dir, "f2")
		full := filepath.Join(dir, "full")
		if err := os.Mkdir(full, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(full, "child"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, p := range []string{f1, f2} {
			if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		n, err := support.RemoveFiles([]string{f1, full, f2})
		if err == nil {
			t.Fatal("RemoveFiles() error = nil, want failure on the non-empty directory")
		}
		if n != 1 {
			t.Errorf("RemoveFiles() count = %d, want 1", n)
		}
		if _, err := os.Stat(f2); err != nil {
			t.Errorf("f2 was removed after the failure: %v", err)
		}
	})
}

func TestPart5WriteFileAtomic(t *testing.T) {
	t.Run("creates the file", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "out.txt")
		if err := support.WriteFileAtomic(p, []byte("hello")); err != nil {
			t.Fatalf("WriteFileAtomic() error = %v", err)
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "hello" {
			t.Errorf("content = %q, want %q", got, "hello")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Errorf("dir has %d entries, want only the target (no temp file left)", len(entries))
		}
	})

	t.Run("replaces existing content", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "out.txt")
		if err := os.WriteFile(p, []byte("old"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := support.WriteFileAtomic(p, []byte("new")); err != nil {
			t.Fatalf("WriteFileAtomic() error = %v", err)
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new" {
			t.Errorf("content = %q, want %q", got, "new")
		}
	})

	t.Run("failure reports an error and leaves no temp file", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "d")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		// Renaming a file over a directory fails on every platform.
		err := support.WriteFileAtomic(target, []byte("data"))
		if err == nil {
			t.Fatal("WriteFileAtomic() error = nil, want rename failure over a directory")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 || entries[0].Name() != "d" {
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Errorf("dir entries = %q, want only %q (temp file must be removed)", names, "d")
		}
	})
}
