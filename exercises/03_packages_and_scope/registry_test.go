package scope_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/03_packages_and_scope"
)

func upperPlugin(in string) (string, error) { return strings.ToUpper(in), nil }

func suffixPlugin(suffix string) scope.Plugin {
	return func(in string) (string, error) { return in + suffix, nil }
}

// uniqueName avoids collisions with the process-wide default registry when tests run with -count>1.
func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestPart4Registry(t *testing.T) {
	t.Run("register and lookup", func(t *testing.T) {
		r := scope.NewRegistry()
		if r == nil {
			t.Fatal("NewRegistry() returned nil")
		}
		if err := r.Register("upper", upperPlugin); err != nil {
			t.Fatalf("Register(upper): unexpected error: %v", err)
		}
		p, err := r.Lookup("upper")
		if err != nil || p == nil {
			t.Fatalf("Lookup(upper) = %v, %v; want plugin, nil", p != nil, err)
		}
		if out, err := p("ab"); err != nil || out != "AB" {
			t.Errorf("plugin(ab) = %q, %v; want AB, nil", out, err)
		}
	})

	t.Run("lookup unknown name", func(t *testing.T) {
		r := scope.NewRegistry()
		if err := r.Register("a", upperPlugin); err != nil {
			t.Fatalf("Register(a): unexpected error: %v", err)
		}
		if _, err := r.Lookup("missing"); !errors.Is(err, scope.ErrNotFound) {
			t.Errorf("Lookup(missing) error = %v; want ErrNotFound", err)
		}
	})

	t.Run("names are sorted", func(t *testing.T) {
		r := scope.NewRegistry()
		for _, n := range []string{"gamma", "alpha", "beta"} {
			if err := r.Register(n, upperPlugin); err != nil {
				t.Fatalf("Register(%s): unexpected error: %v", n, err)
			}
		}
		if diff := cmp.Diff([]string{"alpha", "beta", "gamma"}, r.Names()); diff != "" {
			t.Errorf("Names() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("names returns a copy", func(t *testing.T) {
		r := scope.NewRegistry()
		if err := r.Register("a", upperPlugin); err != nil {
			t.Fatalf("Register(a): unexpected error: %v", err)
		}
		names := r.Names()
		if len(names) != 1 || names[0] != "a" {
			t.Fatalf("Names() = %v; want [a]", names)
		}
		names[0] = "mutated"
		if got := r.Names(); got[0] != "a" {
			t.Errorf("Names() after caller mutation = %v; want [a]", got)
		}
	})

	t.Run("duplicate rejected and original kept", func(t *testing.T) {
		r := scope.NewRegistry()
		if err := r.Register("a", suffixPlugin("!")); err != nil {
			t.Fatalf("Register(a): unexpected error: %v", err)
		}
		if err := r.Register("a", upperPlugin); !errors.Is(err, scope.ErrDuplicate) {
			t.Errorf("second Register(a) error = %v; want ErrDuplicate", err)
		}
		p, err := r.Lookup("a")
		if err != nil || p == nil {
			t.Fatalf("Lookup(a) = %v, %v; want plugin, nil", p != nil, err)
		}
		if out, _ := p("x"); out != "x!" {
			t.Errorf("plugin(x) = %q; want the original plugin result x!", out)
		}
	})

	t.Run("invalid arguments leave registry unchanged", func(t *testing.T) {
		r := scope.NewRegistry()
		if err := r.Register("ok", upperPlugin); err != nil {
			t.Fatalf("Register(ok): unexpected error: %v", err)
		}
		if err := r.Register("", upperPlugin); !errors.Is(err, scope.ErrInvalidArgument) {
			t.Errorf("Register(empty name) error = %v; want ErrInvalidArgument", err)
		}
		if err := r.Register("nilplugin", nil); !errors.Is(err, scope.ErrInvalidArgument) {
			t.Errorf("Register(nil plugin) error = %v; want ErrInvalidArgument", err)
		}
		if diff := cmp.Diff([]string{"ok"}, r.Names()); diff != "" {
			t.Errorf("Names() after rejected registrations mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("zero value is usable", func(t *testing.T) {
		var r scope.Registry
		if err := r.Register("z", upperPlugin); err != nil {
			t.Fatalf("zero Registry Register(z): unexpected error: %v", err)
		}
		p, err := r.Lookup("z")
		if err != nil || p == nil {
			t.Fatalf("Lookup(z) = %v, %v; want plugin, nil", p != nil, err)
		}
		if diff := cmp.Diff([]string{"z"}, r.Names()); diff != "" {
			t.Errorf("Names() mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestPart4Isolation(t *testing.T) {
	t.Run("instances do not share state", func(t *testing.T) {
		r1 := scope.NewRegistry()
		r2 := scope.NewRegistry()
		if err := r1.Register("only-r1", upperPlugin); err != nil {
			t.Fatalf("r1.Register: unexpected error: %v", err)
		}
		if _, err := r1.Lookup("only-r1"); err != nil {
			t.Fatalf("r1.Lookup(only-r1): unexpected error: %v", err)
		}
		if _, err := r2.Lookup("only-r1"); !errors.Is(err, scope.ErrNotFound) {
			t.Errorf("r2.Lookup(only-r1) error = %v; want ErrNotFound", err)
		}
		if got := r2.Names(); len(got) != 0 {
			t.Errorf("r2.Names() = %v; want empty", got)
		}
	})

	t.Run("default registry is separate from instances", func(t *testing.T) {
		name := uniqueName("iso")
		if err := scope.Register(name, upperPlugin); err != nil {
			t.Fatalf("Register(%s): unexpected error: %v", name, err)
		}
		if _, err := scope.Lookup(name); err != nil {
			t.Fatalf("Lookup(%s): unexpected error: %v", name, err)
		}
		if _, err := scope.NewRegistry().Lookup(name); !errors.Is(err, scope.ErrNotFound) {
			t.Errorf("NewRegistry().Lookup(%s) error = %v; want ErrNotFound", name, err)
		}
	})

	t.Run("Default is stable", func(t *testing.T) {
		d := scope.Default()
		if d == nil {
			t.Fatal("Default() returned nil")
		}
		if scope.Default() != d {
			t.Error("Default() returned different registries on two calls")
		}
	})
}

func TestPart4Default(t *testing.T) {
	name := uniqueName("pkg")
	if err := scope.Register(name, upperPlugin); err != nil {
		t.Fatalf("Register(%s): unexpected error: %v", name, err)
	}
	p, err := scope.Lookup(name)
	if err != nil || p == nil {
		t.Fatalf("Lookup(%s) = %v, %v; want plugin, nil", name, p != nil, err)
	}
	if out, err := p("ab"); err != nil || out != "AB" {
		t.Errorf("plugin(ab) = %q, %v; want AB, nil", out, err)
	}
	if err := scope.Register(name, upperPlugin); !errors.Is(err, scope.ErrDuplicate) {
		t.Errorf("second Register(%s) error = %v; want ErrDuplicate", name, err)
	}
	if !slices.Contains(scope.Names(), name) {
		t.Errorf("Names() does not contain %s", name)
	}
	if !slices.IsSorted(scope.Names()) {
		t.Error("Names() is not sorted")
	}
	if _, err := scope.Default().Lookup(name); err != nil {
		t.Errorf("Default().Lookup(%s): unexpected error: %v", name, err)
	}
}

func TestPart4ConcurrentRegister(t *testing.T) {
	r := scope.NewRegistry()
	if err := r.Register("warm", upperPlugin); err != nil {
		t.Fatalf("Register(warm): unexpected error: %v", err)
	}

	const n = 20
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			name := fmt.Sprintf("p-%02d", i)
			if err := r.Register(name, suffixPlugin("!")); err != nil {
				errs <- fmt.Errorf("Register(%s): %w", name, err)
				return
			}
			if _, err := r.Lookup(name); err != nil {
				errs <- fmt.Errorf("Lookup(%s): %w", name, err)
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	names := r.Names()
	if len(names) != n+1 {
		t.Fatalf("Names() has %d entries; want %d", len(names), n+1)
	}
	if !slices.IsSorted(names) {
		t.Errorf("Names() = %v; want sorted", names)
	}
}
