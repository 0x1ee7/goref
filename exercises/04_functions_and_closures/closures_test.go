package funcs_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	funcs "goref/exercises/04_functions_and_closures"
)

func TestPart1IDGenerator(t *testing.T) {
	t.Run("sequential ids carry the prefix", func(t *testing.T) {
		next := funcs.NewIDGenerator("req")
		var got []string
		for range 3 {
			got = append(got, next())
		}
		if diff := cmp.Diff([]string{"req-1", "req-2", "req-3"}, got); diff != "" {
			t.Errorf("ids mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("generators keep separate counters", func(t *testing.T) {
		a := funcs.NewIDGenerator("a")
		b := funcs.NewIDGenerator("b")
		a()
		a()
		if got := b(); got != "b-1" {
			t.Errorf("b() = %q, want %q: generators must not share state", got, "b-1")
		}
	})

	t.Run("concurrent callers never see a duplicate", func(t *testing.T) {
		next := funcs.NewIDGenerator("job")
		const n = 200
		ids := make(chan string, n)
		var wg sync.WaitGroup
		for range n {
			wg.Go(func() { ids <- next() })
		}
		wg.Wait()
		close(ids)

		seen := make(map[string]bool, n)
		for id := range ids {
			if seen[id] {
				t.Errorf("duplicate id %q", id)
			}
			seen[id] = true
		}
		if len(seen) != n {
			t.Errorf("got %d distinct ids, want %d", len(seen), n)
		}
	})
}

func TestPart1Compose(t *testing.T) {
	addOne := func(v int) (int, error) { return v + 1, nil }
	timesTen := func(v int) (int, error) { return v * 10, nil }

	t.Run("applies steps left to right", func(t *testing.T) {
		got, err := funcs.Compose(addOne, timesTen)(2)
		if err != nil || got != 30 {
			t.Errorf("Compose(addOne, timesTen)(2) = %d, %v; want 30, <nil>", got, err)
		}
	})

	t.Run("no steps is the identity", func(t *testing.T) {
		got, err := funcs.Compose[int]()(7)
		if err != nil || got != 7 {
			t.Errorf("empty Compose(7) = %d, %v; want 7, <nil>", got, err)
		}
	})

	t.Run("first error stops the chain and is wrapped with its step", func(t *testing.T) {
		errBoom := errors.New("boom")
		var laterCalls int
		steps := []func(int) (int, error){
			addOne,
			func(v int) (int, error) { return v, errBoom },
			func(v int) (int, error) { laterCalls++; return v, nil },
		}
		_, err := funcs.Compose(steps...)(1)
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want it to wrap errBoom", err)
		}
		if !strings.Contains(err.Error(), "step 2") {
			t.Errorf("err = %q, want it to name step 2", err)
		}
		if laterCalls != 0 {
			t.Errorf("step after the failure ran %d times, want 0", laterCalls)
		}
	})

	t.Run("generic over strings", func(t *testing.T) {
		trim := func(s string) (string, error) { return strings.TrimSpace(s), nil }
		upper := func(s string) (string, error) { return strings.ToUpper(s), nil }
		got, err := funcs.Compose(trim, upper)("  go  ")
		if err != nil || got != "GO" {
			t.Errorf("Compose(trim, upper)(%q) = %q, %v; want %q, <nil>", "  go  ", got, err, "GO")
		}
	})

	t.Run("steps built in a loop each keep their own index", func(t *testing.T) {
		var steps []func(int) (int, error)
		for i := 1; i <= 3; i++ {
			steps = append(steps, func(v int) (int, error) { return v + i, nil })
		}
		got, err := funcs.Compose(steps...)(0)
		if err != nil || got != 6 {
			t.Errorf("got %d, %v; want 6 (1+2+3), <nil>", got, err)
		}
	})
}
