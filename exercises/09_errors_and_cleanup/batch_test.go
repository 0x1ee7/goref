package support_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	support "goref/exercises/09_errors_and_cleanup"
)

// timeoutError is transient because it reports Timeout() true.
type timeoutError struct{ msg string }

func (e timeoutError) Error() string { return e.msg }
func (e timeoutError) Timeout() bool { return true }

func failureNames(fs []*support.TaskError) []string {
	var names []string
	for _, f := range fs {
		names = append(names, f.Name)
	}
	return names
}

func TestPart6RunBatch(t *testing.T) {
	t.Run("all succeed", func(t *testing.T) {
		got, err := support.RunBatch([]support.Task[int]{
			{Name: "a", Run: func() (int, error) { return 1, nil }},
			{Name: "b", Run: func() (int, error) { return 2, nil }},
		})
		if err != nil {
			t.Fatalf("RunBatch() error = %v", err)
		}
		if diff := cmp.Diff(map[string]int{"a": 1, "b": 2}, got); diff != "" {
			t.Errorf("RunBatch() mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("empty batch gives an empty non-nil map", func(t *testing.T) {
		got, err := support.RunBatch[int](nil)
		if err != nil {
			t.Fatalf("RunBatch(nil) error = %v", err)
		}
		if got == nil {
			t.Fatal("RunBatch(nil) map = nil, want an empty non-nil map")
		}
		if len(got) != 0 {
			t.Errorf("RunBatch(nil) has %d entries, want 0", len(got))
		}
	})

	t.Run("runs every task and collects every failure", func(t *testing.T) {
		boomB := errors.New("boom b")
		boomD := errors.New("boom d")
		var ran []string
		task := func(name string, v int, err error) support.Task[int] {
			return support.Task[int]{Name: name, Run: func() (int, error) {
				ran = append(ran, name)
				return v, err
			}}
		}
		got, err := support.RunBatch([]support.Task[int]{
			task("a", 1, nil),
			task("b", 0, boomB),
			task("c", 3, nil),
			task("d", 0, boomD),
		})
		be, ok := errors.AsType[*support.BatchError](err)
		if !ok {
			t.Fatalf("error = %v (%T), want *BatchError", err, err)
		}
		if got != nil {
			t.Errorf("results = %v on failure, want nil map", got)
		}
		if be.Total != 4 {
			t.Errorf("Total = %d, want 4", be.Total)
		}
		if diff := cmp.Diff([]string{"b", "d"}, failureNames(be.Failures)); diff != "" {
			t.Errorf("failure names mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{"a", "b", "c", "d"}, ran); diff != "" {
			t.Errorf("tasks run mismatch (-want +got):\n%s", diff)
		}
		if !errors.Is(err, boomB) || !errors.Is(err, boomD) {
			t.Errorf("errors.Is must reach every task failure, got %v", err)
		}
	})

	t.Run("duplicate names are rejected before any task runs", func(t *testing.T) {
		ran := 0
		run := func() (int, error) { ran++; return 0, nil }
		_, err := support.RunBatch([]support.Task[int]{
			{Name: "a", Run: run},
			{Name: "a", Run: run},
		})
		if !errors.Is(err, support.ErrDuplicateTask) {
			t.Fatalf("error = %v, want ErrDuplicateTask", err)
		}
		if ran != 0 {
			t.Errorf("%d tasks ran, want none", ran)
		}
	})
}

func TestPart6Triage(t *testing.T) {
	t.Run("splits by transience and keeps Total", func(t *testing.T) {
		b := &support.BatchError{
			Total: 3,
			Failures: []*support.TaskError{
				{Name: "t", Err: timeoutError{"slow"}},
				{Name: "p", Err: errors.New("bad input")},
			},
		}
		transient, permanent := support.Triage(b)
		if transient == nil || permanent == nil {
			t.Fatalf("Triage() = (%v, %v), want both sides", transient, permanent)
		}
		if transient.Total != 3 || permanent.Total != 3 {
			t.Errorf("Totals = (%d, %d), want both 3", transient.Total, permanent.Total)
		}
		if diff := cmp.Diff([]string{"t"}, failureNames(transient.Failures)); diff != "" {
			t.Errorf("transient names mismatch (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff([]string{"p"}, failureNames(permanent.Failures)); diff != "" {
			t.Errorf("permanent names mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("an empty side is nil and nil input gives nil", func(t *testing.T) {
		b := &support.BatchError{
			Total:    1,
			Failures: []*support.TaskError{{Name: "t", Err: timeoutError{"slow"}}},
		}
		transient, permanent := support.Triage(b)
		if transient == nil || transient.Total != 1 {
			t.Fatalf("transient = %v, want a batch with Total 1", transient)
		}
		if permanent != nil {
			t.Errorf("permanent = %v, want nil when nothing is permanent", permanent)
		}
		if tr, pe := support.Triage(nil); tr != nil || pe != nil {
			t.Errorf("Triage(nil) = (%v, %v), want (nil, nil)", tr, pe)
		}
	})
}
