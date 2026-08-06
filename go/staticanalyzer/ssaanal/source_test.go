package ssaanal

import (
	"go/constant"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// Helper to create unique test values - use ssa.Const which implements ssa.Value
func newTestValue(i int64) ssa.Value {
	return &ssa.Const{
		Value: constant.MakeInt64(i),
	}
}

func TestNewSourceSet(t *testing.T) {
	s := newSourceSet()
	if s == nil {
		t.Fatal("newSourceSet() returned nil")
	}
	if len(s) != 0 {
		t.Errorf("newSourceSet() len = %d, want 0", len(s))
	}
}

func TestSourceSet_Add(t *testing.T) {
	s := newSourceSet()
	v1 := newTestValue(1)
	v2 := newTestValue(2)

	s.add(v1)
	if len(s) != 1 {
		t.Errorf("After adding v1, len = %d, want 1", len(s))
	}

	s.add(v2)
	if len(s) != 2 {
		t.Errorf("After adding v2, len = %d, want 2", len(s))
	}

	// Adding same value again should not increase size
	s.add(v1)
	if len(s) != 2 {
		t.Errorf("After adding v1 again, len = %d, want 2", len(s))
	}
}

func TestSourceSet_Contains(t *testing.T) {
	s := newSourceSet()
	v1 := newTestValue(1)
	v2 := newTestValue(2)

	// Empty set
	if s.contains(v1) {
		t.Error("Empty set should not contain v1")
	}

	// After adding v1
	s.add(v1)
	if !s.contains(v1) {
		t.Error("Set should contain v1 after adding")
	}
	if s.contains(v2) {
		t.Error("Set should not contain v2")
	}

	// After adding v2
	s.add(v2)
	if !s.contains(v1) {
		t.Error("Set should still contain v1")
	}
	if !s.contains(v2) {
		t.Error("Set should contain v2 after adding")
	}
}

func TestSourceSet_Union(t *testing.T) {
	v1 := newTestValue(1)
	v2 := newTestValue(2)
	v3 := newTestValue(3)

	s1 := newSourceSet()
	s1.add(v1)
	s1.add(v2)

	s2 := newSourceSet()
	s2.add(v2)
	s2.add(v3)

	// Union s2 into s1
	s1.union(s2)

	if len(s1) != 3 {
		t.Errorf("After union, s1 len = %d, want 3", len(s1))
	}
	if !s1.contains(v1) || !s1.contains(v2) || !s1.contains(v3) {
		t.Error("s1 should contain v1, v2, and v3 after union")
	}

	// s2 should be unchanged
	if len(s2) != 2 {
		t.Errorf("s2 len = %d, want 2 (should be unchanged)", len(s2))
	}
}

func TestSourceSet_Union_Empty(t *testing.T) {
	v1 := newTestValue(1)

	s1 := newSourceSet()
	s1.add(v1)

	s2 := newSourceSet()

	// Union empty set into s1
	s1.union(s2)
	if len(s1) != 1 {
		t.Errorf("After union with empty, s1 len = %d, want 1", len(s1))
	}

	// Union s1 into empty set
	s3 := newSourceSet()
	s3.union(s1)
	if len(s3) != 1 {
		t.Errorf("After union into empty, s3 len = %d, want 1", len(s3))
	}
}

func TestSourceSet_IsEmpty(t *testing.T) {
	s := newSourceSet()

	// Empty set
	if !s.isEmpty() {
		t.Error("New set should be empty")
	}

	// After adding
	v1 := newTestValue(1)
	s.add(v1)
	if s.isEmpty() {
		t.Error("Set should not be empty after adding")
	}
}

func TestSourceSet_Len(t *testing.T) {
	s := newSourceSet()

	if s.len() != 0 {
		t.Errorf("Empty set len = %d, want 0", s.len())
	}

	v1 := newTestValue(1)
	s.add(v1)
	if s.len() != 1 {
		t.Errorf("After adding 1 element, len = %d, want 1", s.len())
	}

	v2 := newTestValue(2)
	s.add(v2)
	if s.len() != 2 {
		t.Errorf("After adding 2 elements, len = %d, want 2", s.len())
	}

	v3 := newTestValue(3)
	s.add(v3)
	if s.len() != 3 {
		t.Errorf("After adding 3 elements, len = %d, want 3", s.len())
	}
}
