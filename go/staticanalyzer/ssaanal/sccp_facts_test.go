package ssaanal

import (
	"go/constant"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// Test helper to create a minimal analyzer
func newTestAnalyzer() *analyzer {
	return &analyzer{
		blockState: make(map[*ssa.BasicBlock]*blkState),
	}
}

// Test helper to create mock SSA blocks
func newMockBlock(index int) *ssa.BasicBlock {
	return &ssa.BasicBlock{
		Index: index,
	}
}

// Test helper to create mock SSA values
func newMockValue(id int) ssa.Value {
	return &ssa.Const{
		Value: constant.MakeInt64(int64(id)),
	}
}

func TestGetBlockFacts(t *testing.T) {
	a := newTestAnalyzer()
	b := newMockBlock(0)

	// Block with no facts
	facts := a.getBlockFacts(b)
	if facts != nil {
		t.Errorf("getBlockFacts() for new block = %v, want nil", facts)
	}

	// Block with facts
	bs := a.getBlockState(b)
	expectedFacts := map[ssa.Value]nilness{
		newMockValue(1): nlNil,
		newMockValue(2): nlNonNil,
	}
	bs.nilFacts = expectedFacts

	facts = a.getBlockFacts(b)
	if len(facts) != len(expectedFacts) {
		t.Errorf("getBlockFacts() len = %d, want %d", len(facts), len(expectedFacts))
	}
}

func TestCloneFacts_Nil(t *testing.T) {
	a := newTestAnalyzer()

	cloned := a.cloneFacts(nil)
	if cloned == nil {
		t.Fatal("cloneFacts(nil) should return empty map, not nil")
	}
	if len(cloned) != 0 {
		t.Errorf("cloneFacts(nil) len = %d, want 0", len(cloned))
	}
}

func TestCloneFacts_NonNil(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)
	v2 := newMockValue(2)

	original := map[ssa.Value]nilness{
		v1: nlNil,
		v2: nlNonNil,
	}

	cloned := a.cloneFacts(original)

	// Check same contents
	if len(cloned) != len(original) {
		t.Errorf("cloneFacts() len = %d, want %d", len(cloned), len(original))
	}
	if cloned[v1] != nlNil || cloned[v2] != nlNonNil {
		t.Error("cloneFacts() did not preserve values")
	}

	// Check independence - modify original
	v3 := newMockValue(3)
	original[v3] = nlTop

	if len(cloned) != 2 {
		t.Errorf("After modifying original, clone len = %d, want 2", len(cloned))
	}
	if _, ok := cloned[v3]; ok {
		t.Error("Clone should not reflect changes to original")
	}
}

func TestJoinFactsInto_BothNil(t *testing.T) {
	a := newTestAnalyzer()

	existing := make(map[ssa.Value]nilness)
	changed := a.joinFactsInto(existing, nil)
	if changed {
		t.Error("joinFactsInto(empty, nil) should report no change")
	}
	if len(existing) != 0 {
		t.Errorf("joinFactsInto(empty, nil) len = %d, want 0", len(existing))
	}
}

func TestJoinFactsInto_IncomingNil(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)

	existing := map[ssa.Value]nilness{
		v1: nlNil,
	}

	changed := a.joinFactsInto(existing, nil)
	if changed {
		t.Error("joinFactsInto(existing, nil) should report no change")
	}
	if len(existing) != 1 || existing[v1] != nlNil {
		t.Error("joinFactsInto(existing, nil) should leave existing unchanged")
	}
}

func TestJoinFactsInto_IncomingIntoEmpty(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)

	existing := make(map[ssa.Value]nilness)
	incoming := map[ssa.Value]nilness{
		v1: nlNil,
	}

	changed := a.joinFactsInto(existing, incoming)
	if !changed {
		t.Error("joinFactsInto(empty, incoming) should report change")
	}
	if len(existing) != 1 {
		t.Errorf("joinFactsInto(empty, incoming) len = %d, want 1", len(existing))
	}
	if existing[v1] != nlNil {
		t.Errorf("joinFactsInto(empty, incoming)[v1] = %v, want nlNil", existing[v1])
	}
}

func TestJoinFactsInto_BothHaveSameValue(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)

	existing := map[ssa.Value]nilness{
		v1: nlNil,
	}
	incoming := map[ssa.Value]nilness{
		v1: nlNonNil,
	}

	changed := a.joinFactsInto(existing, incoming)
	if !changed {
		t.Error("joinFactsInto() with conflicting values should report change")
	}
	if len(existing) != 1 {
		t.Errorf("joinFactsInto() len = %d, want 1", len(existing))
	}
	if existing[v1] != nlTop {
		t.Errorf("joinFactsInto()[v1] = %v, want nlTop (joined)", existing[v1])
	}
}

func TestJoinFactsInto_DisjointSets(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)
	v2 := newMockValue(2)

	existing := map[ssa.Value]nilness{
		v1: nlNil,
	}
	incoming := map[ssa.Value]nilness{
		v2: nlNonNil,
	}

	changed := a.joinFactsInto(existing, incoming)
	if !changed {
		t.Error("joinFactsInto() with new key should report change")
	}
	if len(existing) != 2 {
		t.Errorf("joinFactsInto() len = %d, want 2", len(existing))
	}
	if existing[v1] != nlNil {
		t.Errorf("joinFactsInto()[v1] = %v, want nlNil", existing[v1])
	}
	if existing[v2] != nlNonNil {
		t.Errorf("joinFactsInto()[v2] = %v, want nlNonNil", existing[v2])
	}
}

func TestJoinFactsInto_OverlappingAndDisjoint(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)
	v2 := newMockValue(2)
	v3 := newMockValue(3)

	existing := map[ssa.Value]nilness{
		v1: nlNil,
		v2: nlNonNil,
	}
	incoming := map[ssa.Value]nilness{
		v2: nlNonNil, // Same as existing
		v3: nlTop,
	}

	changed := a.joinFactsInto(existing, incoming)
	if !changed {
		t.Error("joinFactsInto() with new key v3 should report change")
	}
	if len(existing) != 3 {
		t.Errorf("joinFactsInto() len = %d, want 3", len(existing))
	}
	if existing[v1] != nlNil {
		t.Errorf("joinFactsInto()[v1] = %v, want nlNil", existing[v1])
	}
	if existing[v2] != nlNonNil {
		t.Errorf("joinFactsInto()[v2] = %v, want nlNonNil (same)", existing[v2])
	}
	if existing[v3] != nlTop {
		t.Errorf("joinFactsInto()[v3] = %v, want nlTop", existing[v3])
	}
}

func TestJoinFactsInto_NoChange(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)

	existing := map[ssa.Value]nilness{
		v1: nlNil,
	}
	incoming := map[ssa.Value]nilness{
		v1: nlNil, // Same value — no change
	}

	changed := a.joinFactsInto(existing, incoming)
	if changed {
		t.Error("joinFactsInto() with identical values should report no change")
	}
	if existing[v1] != nlNil {
		t.Errorf("joinFactsInto()[v1] = %v, want nlNil", existing[v1])
	}
}

func TestFactsEqual_BothEmpty(t *testing.T) {
	a := newTestAnalyzer()

	facts1 := make(map[ssa.Value]nilness)
	facts2 := make(map[ssa.Value]nilness)

	if !a.factsEqual(facts1, facts2) {
		t.Error("factsEqual() for empty maps should be true")
	}
}

func TestFactsEqual_DifferentLengths(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)

	facts1 := map[ssa.Value]nilness{
		v1: nlNil,
	}
	facts2 := make(map[ssa.Value]nilness)

	if a.factsEqual(facts1, facts2) {
		t.Error("factsEqual() for different lengths should be false")
	}
}

func TestFactsEqual_DifferentValues(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)

	facts1 := map[ssa.Value]nilness{
		v1: nlNil,
	}
	facts2 := map[ssa.Value]nilness{
		v1: nlNonNil,
	}

	if a.factsEqual(facts1, facts2) {
		t.Error("factsEqual() for different values should be false")
	}
}

func TestFactsEqual_DifferentKeys(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)
	v2 := newMockValue(2)

	facts1 := map[ssa.Value]nilness{
		v1: nlNil,
	}
	facts2 := map[ssa.Value]nilness{
		v2: nlNil,
	}

	if a.factsEqual(facts1, facts2) {
		t.Error("factsEqual() for different keys should be false")
	}
}

func TestFactsEqual_SameFacts(t *testing.T) {
	a := newTestAnalyzer()
	v1 := newMockValue(1)
	v2 := newMockValue(2)

	facts1 := map[ssa.Value]nilness{
		v1: nlNil,
		v2: nlNonNil,
	}
	facts2 := map[ssa.Value]nilness{
		v1: nlNil,
		v2: nlNonNil,
	}

	if !a.factsEqual(facts1, facts2) {
		t.Error("factsEqual() for same facts should be true")
	}
}
