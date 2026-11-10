package funcs_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	funcs "goref/exercises/04_functions_and_closures"
)

func TestPart6Registry(t *testing.T) {
	ctx := context.Background()
	noop := func(context.Context, []string) error { return nil }

	t.Run("Run passes args through and returns the command's error unchanged", func(t *testing.T) {
		var r funcs.Registry
		var gotArgs []string
		errFail := errors.New("exit 2")
		if err := r.Register("deploy", func(_ context.Context, args []string) error {
			gotArgs = args
			return errFail
		}); err != nil {
			t.Fatalf("Register = %v, want nil", err)
		}
		err := r.Run(ctx, "deploy", []string{"prod", "--dry"})
		if err != errFail {
			t.Errorf("Run = %v, want the command's error unchanged", err)
		}
		if diff := cmp.Diff([]string{"prod", "--dry"}, gotArgs); diff != "" {
			t.Errorf("args mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Register rejects an empty name and a nil command", func(t *testing.T) {
		var r funcs.Registry
		if err := r.Register("", noop); !errors.Is(err, funcs.ErrInvalidCommand) {
			t.Errorf("empty name: err = %v, want ErrInvalidCommand", err)
		}
		if err := r.Register("x", nil); !errors.Is(err, funcs.ErrInvalidCommand) {
			t.Errorf("nil command: err = %v, want ErrInvalidCommand", err)
		}
	})

	t.Run("Register rejects duplicates", func(t *testing.T) {
		var r funcs.Registry
		if err := r.Register("x", noop); err != nil {
			t.Fatalf("first Register = %v, want nil", err)
		}
		if err := r.Register("x", noop); !errors.Is(err, funcs.ErrDuplicateCommand) {
			t.Errorf("duplicate: err = %v, want ErrDuplicateCommand", err)
		}
	})

	t.Run("Run of an unknown name returns ErrUnknownCommand", func(t *testing.T) {
		var r funcs.Registry
		if err := r.Run(ctx, "nope", nil); !errors.Is(err, funcs.ErrUnknownCommand) {
			t.Errorf("err = %v, want ErrUnknownCommand", err)
		}
	})

	t.Run("Names is sorted", func(t *testing.T) {
		var r funcs.Registry
		for _, name := range []string{"stop", "build", "start"} {
			if err := r.Register(name, noop); err != nil {
				t.Fatalf("Register(%q) = %v", name, err)
			}
		}
		if diff := cmp.Diff([]string{"build", "start", "stop"}, r.Names()); diff != "" {
			t.Errorf("Names mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("registries are independent", func(t *testing.T) {
		var a, b funcs.Registry
		if err := a.Register("x", noop); err != nil {
			t.Fatalf("Register = %v", err)
		}
		if err := b.Run(ctx, "x", nil); !errors.Is(err, funcs.ErrUnknownCommand) {
			t.Errorf("b.Run(x) = %v, want ErrUnknownCommand: registries must not share state", err)
		}
		if diff := cmp.Diff([]string{"x"}, a.Names()); diff != "" {
			t.Errorf("a.Names mismatch (-want +got):\n%s", diff)
		}
	})
}
