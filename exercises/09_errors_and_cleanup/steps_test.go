package support_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	support "goref/exercises/09_errors_and_cleanup"
)

func TestPart3RunStep(t *testing.T) {
	t.Run("success traces start ok end", func(t *testing.T) {
		var events []string
		got, err := support.RunStep("load", func(e string) { events = append(events, e) },
			func() (string, error) { return "v", nil })
		if err != nil {
			t.Fatalf("RunStep() error = %v", err)
		}
		if got != "v" {
			t.Errorf("RunStep() = %q, want %q", got, "v")
		}
		want := []string{"start load", "ok load", "end load"}
		if diff := cmp.Diff(want, events); diff != "" {
			t.Errorf("trace mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("failure is a StepError with zero value", func(t *testing.T) {
		boom := errors.New("boom")
		var events []string
		got, err := support.RunStep("sync", func(e string) { events = append(events, e) },
			func() (int, error) { return 7, boom })
		se, ok := errors.AsType[*support.StepError](err)
		if !ok {
			t.Fatalf("error = %v (%T), want *StepError", err, err)
		}
		if se.Step != "sync" {
			t.Errorf("Step = %q, want %q", se.Step, "sync")
		}
		if !errors.Is(err, boom) {
			t.Errorf("errors.Is(err, boom) = false, want true")
		}
		if got != 0 {
			t.Errorf("value = %d on failure, want zero", got)
		}
		want := []string{"start sync", "failed sync", "end sync"}
		if diff := cmp.Diff(want, events); diff != "" {
			t.Errorf("trace mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("panic propagates and end is still traced", func(t *testing.T) {
		var events []string
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_, _ = support.RunStep("crash", func(e string) { events = append(events, e) },
				func() (int, error) { panic("kaboom") })
		}()
		if recovered != "kaboom" {
			t.Fatalf("recovered = %v, want the step's panic %q to propagate", recovered, "kaboom")
		}
		want := []string{"start crash", "end crash"}
		if diff := cmp.Diff(want, events); diff != "" {
			t.Errorf("trace mismatch (-want +got):\n%s", diff)
		}
	})
}
