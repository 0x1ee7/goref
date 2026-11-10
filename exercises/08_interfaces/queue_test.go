package interfaces_test

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"goref/exercises/08_interfaces"
)

func TestPart6ByPrioritySortInterface(t *testing.T) {
	t.Run("sort.Sort orders by priority desc then name asc", func(t *testing.T) {
		jobs := []interfaces.Job{{Name: "b", Priority: 1}, {Name: "c", Priority: 5}, {Name: "a", Priority: 5}, {Name: "d", Priority: 3}}
		sort.Sort(interfaces.ByPriority(jobs))
		want := []interfaces.Job{{Name: "a", Priority: 5}, {Name: "c", Priority: 5}, {Name: "d", Priority: 3}, {Name: "b", Priority: 1}}
		if diff := cmp.Diff(want, jobs); diff != "" {
			t.Errorf("sorted jobs (-want +got):\n%s", diff)
		}
	})

	t.Run("IsSorted tells sorted from unsorted", func(t *testing.T) {
		sorted := interfaces.ByPriority{{Name: "a", Priority: 2}, {Name: "b", Priority: 1}}
		unsorted := interfaces.ByPriority{{Name: "a", Priority: 1}, {Name: "b", Priority: 2}}
		if !sort.IsSorted(sorted) {
			t.Error("IsSorted(sorted) = false, want true")
		}
		if sort.IsSorted(unsorted) {
			t.Error("IsSorted(unsorted) = true, want false")
		}
	})

	t.Run("Less is a strict order on priority and name", func(t *testing.T) {
		p := interfaces.ByPriority{{Name: "a", Priority: 2}, {Name: "b", Priority: 1}, {Name: "b", Priority: 1}}
		if !p.Less(0, 1) {
			t.Error("Less(higher priority, lower) = false, want true")
		}
		if p.Less(1, 0) {
			t.Error("Less(lower priority, higher) = true, want false")
		}
		if p.Less(1, 2) {
			t.Error("Less(equal, equal) = true, want false")
		}
	})
}

func TestPart6QueueHeap(t *testing.T) {
	t.Run("pops highest priority first", func(t *testing.T) {
		var q interfaces.Queue
		q.Push(interfaces.Job{Name: "low", Priority: 1})
		q.Push(interfaces.Job{Name: "high", Priority: 9})
		q.Push(interfaces.Job{Name: "mid", Priority: 5})
		var got []string
		for range 3 {
			j, ok := q.Pop()
			if !ok {
				t.Fatal("Pop returned ok=false with jobs queued")
			}
			got = append(got, j.Name)
		}
		if diff := cmp.Diff([]string{"high", "mid", "low"}, got); diff != "" {
			t.Errorf("pop order (-want +got):\n%s", diff)
		}
	})

	t.Run("equal priorities pop in FIFO order", func(t *testing.T) {
		var q interfaces.Queue
		for _, name := range []string{"x", "y", "z"} {
			q.Push(interfaces.Job{Name: name, Priority: 1})
		}
		var got []string
		for q.Len() > 0 {
			j, _ := q.Pop()
			got = append(got, j.Name)
		}
		if diff := cmp.Diff([]string{"x", "y", "z"}, got); diff != "" {
			t.Errorf("pop order (-want +got):\n%s", diff)
		}
	})

	t.Run("Len tracks pushes and pops", func(t *testing.T) {
		var q interfaces.Queue
		if q.Len() != 0 {
			t.Fatalf("zero Queue Len = %d, want 0", q.Len())
		}
		q.Push(interfaces.Job{Name: "a", Priority: 1})
		q.Push(interfaces.Job{Name: "b", Priority: 2})
		if q.Len() != 2 {
			t.Errorf("Len after 2 pushes = %d, want 2", q.Len())
		}
		q.Pop()
		if q.Len() != 1 {
			t.Errorf("Len after pop = %d, want 1", q.Len())
		}
	})

	t.Run("Pop on empty returns false; queue still works afterwards", func(t *testing.T) {
		var q interfaces.Queue
		if _, ok := q.Pop(); ok {
			t.Fatal("Pop on empty queue: ok = true, want false")
		}
		q.Push(interfaces.Job{Name: "a", Priority: 1})
		if j, ok := q.Pop(); !ok || j.Name != "a" {
			t.Errorf("Pop after Push = %+v, %v; want a, true", j, ok)
		}
	})

	t.Run("interleaved pushes and pops keep the invariant", func(t *testing.T) {
		var q interfaces.Queue
		q.Push(interfaces.Job{Name: "p5", Priority: 5})
		q.Push(interfaces.Job{Name: "p1", Priority: 1})
		if j, _ := q.Pop(); j.Name != "p5" {
			t.Errorf("first pop = %q, want p5", j.Name)
		}
		q.Push(interfaces.Job{Name: "p9", Priority: 9})
		if j, _ := q.Pop(); j.Name != "p9" {
			t.Errorf("second pop = %q, want p9", j.Name)
		}
		if j, _ := q.Pop(); j.Name != "p1" {
			t.Errorf("third pop = %q, want p1", j.Name)
		}
	})
}
