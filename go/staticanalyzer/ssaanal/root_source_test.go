package ssaanal

import (
	"testing"

	"golang.org/x/tools/go/ssa"
)

// newTestRegistry creates a rootSourceRegistry with a test src value.
// srcIns is nil; do not call srcBlock() on the returned registry.
func newTestRegistry() *rootSourceRegistry {
	return newRootSourceRegistry(sourceID(newTestValue(0)), nil)
}

func TestNewRootSourceRegistry(t *testing.T) {
	r := newTestRegistry()
	if r == nil {
		t.Fatal("newRootSourceRegistry() returned nil")
	}
	if len(r.byValue) != 0 {
		t.Errorf("newRootSourceRegistry() len = %d, want 0", len(r.byValue))
	}
}

func TestRootSourceRegistry_Add(t *testing.T) {
	r := newTestRegistry()
	v1 := newTestValue(1)
	v2 := newTestValue(2)

	r.add(v1, rootSrc)
	if len(r.byValue) != 1 {
		t.Errorf("After adding v1, len = %d, want 1", len(r.byValue))
	}

	r.add(v2, rootOtherSrc)
	if len(r.byValue) != 2 {
		t.Errorf("After adding v2, len = %d, want 2", len(r.byValue))
	}
}

func TestRootSourceRegistry_Add_NoDuplicateOverwrite(t *testing.T) {
	r := newTestRegistry()
	v1 := newTestValue(1)

	r.add(v1, rootSrc)
	r.add(v1, rootSynthetic)

	kind, ok := r.lookup(v1)
	if !ok {
		t.Fatal("v1 should be in registry")
	}
	if kind != rootSrc {
		t.Errorf("kind = %d, want rootSrc (%d); second add should not overwrite", kind, rootSrc)
	}
	if len(r.byValue) != 1 {
		t.Errorf("len = %d, want 1; duplicate add should not increase size", len(r.byValue))
	}
}

func TestRootSourceRegistry_Lookup(t *testing.T) {
	r := newTestRegistry()
	v1 := newTestValue(1)
	v2 := newTestValue(2)
	v3 := newTestValue(3)

	r.add(v1, rootSrc)
	r.add(v2, rootOtherSrc)

	kind, ok := r.lookup(v1)
	if !ok {
		t.Fatal("v1 should be found")
	}
	if kind != rootSrc {
		t.Errorf("v1 kind = %d, want rootSrc (%d)", kind, rootSrc)
	}

	kind, ok = r.lookup(v2)
	if !ok {
		t.Fatal("v2 should be found")
	}
	if kind != rootOtherSrc {
		t.Errorf("v2 kind = %d, want rootOtherSrc (%d)", kind, rootOtherSrc)
	}

	_, ok = r.lookup(v3)
	if ok {
		t.Error("v3 should not be found")
	}
}

func TestRootSourceRegistry_IsRoot(t *testing.T) {
	r := newTestRegistry()
	v1 := newTestValue(1)
	v2 := newTestValue(2)
	v3 := newTestValue(3)

	r.add(v1, rootSrc)
	r.add(v2, rootSynthetic)

	if !r.isRoot(v1) {
		t.Error("v1 should be a root source")
	}
	if !r.isRoot(v2) {
		t.Error("v2 should be a root source")
	}
	if r.isRoot(v3) {
		t.Error("v3 should not be a root source")
	}
}

func TestRootSourceRegistry_AllKinds(t *testing.T) {
	r := newTestRegistry()
	v1 := newTestValue(1)
	v2 := newTestValue(2)
	v3 := newTestValue(3)

	r.add(v1, rootSrc)
	r.add(v2, rootOtherSrc)
	r.add(v3, rootSynthetic)

	tests := []struct {
		name     string
		v        ssa.Value
		wantKind rootSourceKind
	}{
		{"rootSrc", v1, rootSrc},
		{"rootOtherSrc", v2, rootOtherSrc},
		{"rootSynthetic", v3, rootSynthetic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, ok := r.lookup(tt.v)
			if !ok {
				t.Fatalf("value should be registered")
			}
			if kind != tt.wantKind {
				t.Errorf("kind = %d, want %d", kind, tt.wantKind)
			}
		})
	}
}

func TestRootSourceRegistry_SrcAccessors(t *testing.T) {
	srcVal := newTestValue(42)

	// Create a real SSA function with a block to get a valid ssa.Instruction.
	fn := &ssa.Function{}
	fn.Blocks = []*ssa.BasicBlock{{Index: 0}}
	block := fn.Blocks[0]
	jump := new(ssa.Jump)
	block.Instrs = []ssa.Instruction{jump}

	r := newRootSourceRegistry(sourceID(srcVal), jump)

	if r.srcID() != sourceID(srcVal) {
		t.Error("srcID() should return the source ID passed at construction")
	}
	if r.srcVal() != srcVal {
		t.Error("srcVal() should return the ssa.Value of the source")
	}
}
