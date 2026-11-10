package funcs_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	funcs "goref/exercises/04_functions_and_closures"
)

func TestPart2Cleanup(t *testing.T) {
	t.Run("runs registered functions last-in first-out", func(t *testing.T) {
		var c funcs.Cleanup
		var order []string
		for _, name := range []string{"a", "b", "c"} {
			c.Defer(func() error { order = append(order, name); return nil })
		}
		if err := c.Run(); err != nil {
			t.Fatalf("Run() = %v, want nil", err)
		}
		if diff := cmp.Diff([]string{"c", "b", "a"}, order); diff != "" {
			t.Errorf("run order mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("runs everything and joins errors in run order", func(t *testing.T) {
		var c funcs.Cleanup
		errFirst := errors.New("first")
		errSecond := errors.New("second")
		var ran int
		c.Defer(func() error { ran++; return errFirst })
		c.Defer(func() error { ran++; return nil })
		c.Defer(func() error { ran++; return errSecond })

		err := c.Run()
		if ran != 3 {
			t.Errorf("ran %d functions, want 3: an error must not stop cleanup", ran)
		}
		if !errors.Is(err, errFirst) || !errors.Is(err, errSecond) {
			t.Fatalf("Run() = %v, want both errors joined", err)
		}
		if got, want := err.Error(), "second\nfirst"; got != want {
			t.Errorf("joined message = %q, want %q (LIFO order)", got, want)
		}
	})

	t.Run("second Run is a no-op", func(t *testing.T) {
		var c funcs.Cleanup
		var calls int
		c.Defer(func() error { calls++; return nil })
		if err := c.Run(); err != nil {
			t.Fatalf("first Run() = %v, want nil", err)
		}
		if err := c.Run(); err != nil {
			t.Errorf("second Run() = %v, want nil", err)
		}
		if calls != 1 {
			t.Errorf("function ran %d times, want exactly 1", calls)
		}
	})
}

func TestPart2Annotate(t *testing.T) {
	errBoom := errors.New("boom")

	t.Run("wraps a non-nil error and leaves nil alone", func(t *testing.T) {
		failing := func() (err error) {
			defer funcs.Annotate(&err, "open %s", "cfg")
			return errBoom
		}
		err := failing()
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want it to wrap errBoom", err)
		}
		if got, want := err.Error(), "open cfg: boom"; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}

		succeeding := func() (err error) {
			defer funcs.Annotate(&err, "open %s", "cfg")
			return nil
		}
		if err := succeeding(); err != nil {
			t.Errorf("nil error became %v, want nil", err)
		}
	})

	t.Run("arguments are evaluated when the defer runs", func(t *testing.T) {
		run := func() (err error) {
			n := 1
			defer funcs.Annotate(&err, "attempt %d", n)
			n = 2
			return errBoom
		}
		err := run()
		if err == nil {
			t.Fatal("run() = nil, want annotated error")
		}
		if got, want := err.Error(), "attempt 1: boom"; got != want {
			t.Errorf("message = %q, want %q", got, want)
		}
	})
}

func TestPart2WriteFileAtomic(t *testing.T) {
	t.Run("creates the file with the given content", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.txt")
		if err := funcs.WriteFileAtomic(path, []byte("hello")); err != nil {
			t.Fatalf("WriteFileAtomic: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading result: %v", err)
		}
		if string(got) != "hello" {
			t.Errorf("content = %q, want %q", got, "hello")
		}
	})

	t.Run("replaces an existing file and leaves no temporary files", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "out.txt")
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := funcs.WriteFileAtomic(path, []byte("new")); err != nil {
			t.Fatalf("WriteFileAtomic: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "new" {
			t.Errorf("content = %q, want %q", got, "new")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Errorf("directory holds %v, want only out.txt", names)
		}
	})

	t.Run("failure is prefixed with the path and keeps its cause", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing", "x.txt")
		err := funcs.WriteFileAtomic(path, []byte("data"))
		if err == nil {
			t.Fatal("WriteFileAtomic into a missing directory = nil, want an error")
		}
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("err = %v, want it to wrap fs.ErrNotExist", err)
		}
		if !strings.HasPrefix(err.Error(), "write "+path+": ") {
			t.Errorf("err = %q, want prefix %q", err, "write "+path+": ")
		}
	})
}

func TestPart2ForEachFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := write("a.txt", "alpha")
	b := write("b.txt", "beta")

	t.Run("visits each file in order and closes it before returning", func(t *testing.T) {
		var names, bodies []string
		var files []*os.File
		err := funcs.ForEachFile([]string{a, b}, func(path string, f *os.File) error {
			names = append(names, filepath.Base(path))
			files = append(files, f)
			buf, err := io.ReadAll(f)
			bodies = append(bodies, string(buf))
			return err
		})
		if err != nil {
			t.Fatalf("ForEachFile = %v, want nil", err)
		}
		if diff := cmp.Diff([]string{"a.txt", "b.txt"}, names); diff != "" {
			t.Errorf("visit order mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{"alpha", "beta"}, bodies); diff != "" {
			t.Errorf("contents mismatch (-want +got):\n%s", diff)
		}
		for _, f := range files {
			if err := f.Close(); !errors.Is(err, os.ErrClosed) {
				t.Errorf("close after ForEachFile = %v, want os.ErrClosed (file still open)", err)
			}
		}
	})

	t.Run("failures do not stop the loop and are joined", func(t *testing.T) {
		missing := filepath.Join(dir, "missing.txt")
		errBad := errors.New("bad content")
		var visited []string
		err := funcs.ForEachFile([]string{missing, a, b}, func(path string, f *os.File) error {
			visited = append(visited, filepath.Base(path))
			if filepath.Base(path) == "a.txt" {
				return errBad
			}
			return nil
		})
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("err = %v, want the open failure (fs.ErrNotExist) joined in", err)
		}
		if !errors.Is(err, errBad) {
			t.Errorf("err = %v, want the fn failure joined in", err)
		}
		if diff := cmp.Diff([]string{"a.txt", "b.txt"}, visited); diff != "" {
			t.Errorf("visited mismatch (-want +got):\n%s", diff)
		}
	})
}
