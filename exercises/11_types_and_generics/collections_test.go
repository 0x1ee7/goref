package typed_test

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"goref/exercises/11_types_and_generics"
)

func TestPart4ZeroValueSet(t *testing.T) {
	var s typed.Set[string]
	s.Add("x", "y", "x")
	if s.Len() != 2 || !s.Has("x") {
		t.Fatalf("after Add(x,y,x): Len=%d Has(x)=%t, want 2 and true", s.Len(), s.Has("x"))
	}
	if !s.Remove("x") {
		t.Error("Remove(x) = false, want true")
	}
	if s.Remove("x") {
		t.Error("second Remove(x) = true, want false")
	}
	if s.Has("x") || s.Len() != 1 {
		t.Errorf("after removal Has(x)=%t Len=%d, want false and 1", s.Has("x"), s.Len())
	}
}

func TestPart4SetAllAndClone(t *testing.T) {
	s := typed.NewSet(1, 2, 3)
	if got := slices.Sorted(s.All()); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("All() = %v, want [1 2 3]", got)
	}
	c := s.Clone()
	c.Add(4)
	if c.Len() != 4 {
		t.Errorf("clone Len = %d, want 4", c.Len())
	}
	if s.Has(4) || s.Len() != 3 {
		t.Errorf("mutating clone changed original: Has(4)=%t Len=%d", s.Has(4), s.Len())
	}
}

func TestPart4SetAlgebra(t *testing.T) {
	a := typed.NewSet(1, 2, 3)
	b := typed.NewSet(2, 3, 4)
	tests := []struct {
		name string
		got  typed.Set[int]
		want []int
	}{
		{"union", a.Union(b), []int{1, 2, 3, 4}},
		{"intersect", a.Intersect(b), []int{2, 3}},
		{"a minus b", a.Difference(b), []int{1}},
		{"b minus a", b.Difference(a), []int{4}},
		{"union with empty", a.Union(typed.NewSet[int]()), []int{1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slices.Sorted(tt.got.All())
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
	if !a.Equal(typed.NewSet(3, 2, 1)) {
		t.Error("Equal of same elements = false, want true")
	}
	if a.Equal(b) {
		t.Error("Equal of different sets = true, want false")
	}
	if a.Len() != 3 || b.Len() != 3 {
		t.Errorf("operands mutated: Len a=%d b=%d, want 3 and 3", a.Len(), b.Len())
	}
}

func TestPart4SetAllEarlyBreak(t *testing.T) {
	s := typed.NewSet(1, 2, 3, 4)
	if !s.Has(1) {
		t.Fatal("setup: Has(1) = false")
	}
	yields := 0
	for range s.All() {
		yields++
		if yields == 2 {
			break
		}
	}
	if yields != 2 {
		t.Errorf("yields after break = %d, want 2", yields)
	}
}

func TestPart4OrderedMapInsertionOrder(t *testing.T) {
	var m typed.OrderedMap[string, int]
	m.Set("b", 2)
	m.Set("a", 1)
	m.Set("c", 3)
	m.Set("a", 10) // overwrite keeps the original position

	if got := slices.Collect(m.Keys()); !slices.Equal(got, []string{"b", "a", "c"}) {
		t.Fatalf("Keys() = %v, want [b a c]", got)
	}
	if v, ok := m.Get("a"); !ok || v != 10 {
		t.Errorf("Get(a) = (%d, %t), want (10, true)", v, ok)
	}
	if v, ok := m.Get("zz"); ok || v != 0 {
		t.Errorf("Get(zz) = (%d, %t), want (0, false)", v, ok)
	}
	if m.Len() != 3 {
		t.Errorf("Len() = %d, want 3", m.Len())
	}
}

func TestPart4OrderedMapDeleteAndReAdd(t *testing.T) {
	var m typed.OrderedMap[string, int]
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)
	if !m.Delete("a") {
		t.Fatal("Delete(a) = false, want true")
	}
	if m.Delete("a") {
		t.Error("second Delete(a) = true, want false")
	}
	m.Set("a", 9) // re-added keys go to the end
	if got := slices.Collect(m.Keys()); !slices.Equal(got, []string{"b", "c", "a"}) {
		t.Errorf("Keys() after delete and re-add = %v, want [b c a]", got)
	}
}

func TestPart4OrderedMapAllAndEarlyBreak(t *testing.T) {
	var m typed.OrderedMap[string, int]
	m.Set("x", 1)
	m.Set("y", 2)
	m.Set("z", 3)

	var pairs []string
	for k, v := range m.All() {
		pairs = append(pairs, fmt.Sprintf("%s=%d", k, v))
	}
	if diff := cmp.Diff([]string{"x=1", "y=2", "z=3"}, pairs); diff != "" {
		t.Errorf("All() mismatch (-want +got):\n%s", diff)
	}

	var first []string
	for k := range m.Keys() {
		first = append(first, k)
		break
	}
	if !slices.Equal(first, []string{"x"}) {
		t.Errorf("first key before break = %v, want [x]", first)
	}
}

func TestPart4IndexAlias(t *testing.T) {
	// A generic alias is the same type, not a new named type.
	if reflect.TypeFor[typed.Index[int]]() != reflect.TypeFor[typed.OrderedMap[string, int]]() {
		t.Fatal("Index[int] is not identical to OrderedMap[string, int]")
	}
	var idx typed.Index[int]
	idx.Set("x", 1)
	var om typed.OrderedMap[string, int] = idx
	if v, ok := om.Get("x"); !ok || v != 1 {
		t.Errorf("Get(x) through alias = (%d, %t), want (1, true)", v, ok)
	}
}
