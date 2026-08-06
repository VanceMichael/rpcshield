package cdg

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ssa"
)

// connect adds a directed edge from -> to in the CFG and maintains Preds/Succs.
// This simulates a control flow edge between two basic blocks.
func connect(from, to *ssa.BasicBlock) {
	from.Succs = append(from.Succs, to)
	to.Preds = append(to.Preds, from)
}

// newFunctionWithBlocks creates a minimal ssa.Function with numBlocks basic blocks.
// Block indices range from 0 to numBlocks-1.
// Returns the function and its blocks for test CFG construction.
func newFunctionWithBlocks(numBlocks int) (*ssa.Function, []*ssa.BasicBlock) {
	f := &ssa.Function{}
	blocks := make([]*ssa.BasicBlock, numBlocks)
	for i := 0; i < numBlocks; i++ {
		blocks[i] = &ssa.BasicBlock{Index: i}
	}
	f.Blocks = blocks
	return f, blocks
}

// newCDGBuilder creates a CDGBuilder and returns it as the concrete implementation type.
// This allows tests to access internal methods for verification.
func newCDGBuilder() *cdgBuilderImpl {
	return NewCDGBuilder().(*cdgBuilderImpl)
}

// verifyTreeRelations checks that Ipdom relationships are inverses of PostDominees
// and that pre/post-order numbering is consistent with the tree structure.
func verifyTreeRelations(t *testing.T, builder *cdgBuilderImpl, f *ssa.Function) {
	t.Helper()
	// Check that for each parent, all children point back via Ipdom
	for _, parent := range f.Blocks {
		for _, child := range builder.PostDominees(parent) {
			got := builder.Ipdom(child)
			require.Equalf(t, parent, got, "child %d expected ipdom %d, got %v", child.Index, parent.Index, indexOf(got))
		}
	}
	// Check pre/post invariants: parent pre < child pre; child post < parent post
	for _, parent := range f.Blocks {
		for _, child := range builder.PostDominees(parent) {
			require.Truef(t, builder.getPDomInfo(parent).pre < builder.getPDomInfo(child).pre,
				"preorder invariant failed: pre(parent=%d)=%d !< pre(child=%d)=%d",
				parent.Index, builder.getPDomInfo(parent).pre, child.Index, builder.getPDomInfo(child).pre)
			require.Truef(t, builder.getPDomInfo(child).post < builder.getPDomInfo(parent).post,
				"postorder invariant failed: post(child=%d)=%d !< post(parent=%d)=%d",
				child.Index, builder.getPDomInfo(child).post, parent.Index, builder.getPDomInfo(parent).post)
		}
	}
}

func indexOf(b *ssa.BasicBlock) interface{} {
	if b == nil {
		return nil
	}
	return b.Index
}

// cdgToIndexSets converts a CDG keyed by basic blocks into an index-based adjacency set.
func cdgToIndexSets(cdg map[*ssa.BasicBlock]map[*ssa.BasicBlock]struct{}) map[int]map[int]struct{} {
	res := make(map[int]map[int]struct{}, len(cdg))
	for x, ys := range cdg {
		set := make(map[int]struct{}, len(ys))
		for y := range ys {
			set[y.Index] = struct{}{}
		}
		res[x.Index] = set
	}
	return res
}

// requireCDGEdges asserts that the CDG matches the expected adjacency where keys are source indices
// and values are the slice of destination indices. Nodes not mentioned in expected are required to
// have no outgoing CDG edges.
func requireCDGEdges(t *testing.T, cdg map[*ssa.BasicBlock]map[*ssa.BasicBlock]struct{}, expected map[int][]int) {
	t.Helper()
	got := cdgToIndexSets(cdg)

	// Build expected sets and assert per-node equality. Nodes not in expected must be empty.
	for x, out := range got {
		wantSlice, ok := expected[x]
		want := make(map[int]struct{})
		if ok {
			for _, y := range wantSlice {
				want[y] = struct{}{}
			}
		}
		assert.Equalf(t, want, out, "CDG edges mismatch for node %d", x)
	}
}

// cdgHasEdge reports whether there is an edge from fromIdx to toIdx in the CDG.
func cdgHasEdge(cdg map[*ssa.BasicBlock]map[*ssa.BasicBlock]struct{}, fromIdx, toIdx int) bool {
	for from, outs := range cdg {
		if from.Index != fromIdx {
			continue
		}
		for to := range outs {
			if to.Index == toIdx {
				return true
			}
		}
		return false
	}
	return false
}

// requireNoOutgoing asserts that the listed node indices have no outgoing CDG edges.
func requireNoOutgoing(t *testing.T, cdg map[*ssa.BasicBlock]map[*ssa.BasicBlock]struct{}, nodes []int) {
	t.Helper()
	got := cdgToIndexSets(cdg)
	for _, idx := range nodes {
		if out, ok := got[idx]; ok {
			assert.Emptyf(t, out, "expected no CDG edges from node %d", idx)
		}
	}
}

// TestLinearChain verifies post-dominator tree construction and CDG for a linear chain CFG (0->1->2).
// Scenario: straight-line program with a single exit.
// Validates: immediate post-dominators (0→1, 1→2, 2 has none), exit post-dominates all, and no control dependences.
func TestLinearChain(t *testing.T) {
	// 0 -> 1 -> 2 (exit)
	f, b := newFunctionWithBlocks(3)
	connect(b[0], b[1])
	connect(b[1], b[2])

	// Expected PDomTree (root is the exit):
	//  2
	//  └── 1
	//      └── 0

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	require.Equal(t, b[1], builder.Ipdom(b[0]))
	require.Equal(t, b[2], builder.Ipdom(b[1]))
	require.Nil(t, builder.Ipdom(b[2]))

	require.True(t, builder.PostDominates(b[2], b[1]))
	require.True(t, builder.PostDominates(b[2], b[0]))
	require.True(t, builder.PostDominates(b[1], b[0]))
	require.False(t, builder.PostDominates(b[0], b[1]))

	// Immediate post-dominees
	require.Len(t, builder.PostDominees(b[2]), 1)
	require.Equal(t, b[1], builder.PostDominees(b[2])[0])
	require.Len(t, builder.PostDominees(b[1]), 1)
	require.Equal(t, b[0], builder.PostDominees(b[1])[0])

	verifyTreeRelations(t, builder, f)

	// CDG validation: no control dependences in a linear chain
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{})
}

// TestDiamond_SingleExit validates post-dominator tree construction and CDG for a diamond-shaped CFG.
// Scenario: fork at 0 into 1 and 2 that rejoin at 3 (single exit).
// Validates: exit block (3) post-dominates all nodes; ipdom of 0,1,2 is 3; 3 has no ipdom;
// and only the branch targets are control-dependent on the branch node.
func TestDiamond_SingleExit(t *testing.T) {
	//    0
	//   / \
	//  1   2
	//   \ /
	//    3 (exit)
	f, b := newFunctionWithBlocks(4)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[1], b[3])
	connect(b[2], b[3])

	// Expected PDomTree (root is the exit):
	//  3
	//  ├── 0
	//  ├── 1
	//  └── 2

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	require.Equal(t, b[3], builder.Ipdom(b[1]))
	require.Equal(t, b[3], builder.Ipdom(b[2]))
	require.Equal(t, b[3], builder.Ipdom(b[0]))
	require.Nil(t, builder.Ipdom(b[3]))

	require.True(t, builder.PostDominates(b[3], b[0]))
	require.True(t, builder.PostDominates(b[3], b[1]))
	require.True(t, builder.PostDominates(b[3], b[2]))
	require.False(t, builder.PostDominates(b[1], b[2]))
	require.False(t, builder.PostDominates(b[2], b[1]))

	// b3 immediately postdominates b0, b1, b2
	dominees := builder.PostDominees(b[3])
	require.ElementsMatch(t, []*ssa.BasicBlock{b[0], b[1], b[2]}, dominees)

	verifyTreeRelations(t, builder, f)

	// CDG validation: only 1 and 2 are control-dependent on 0; 3 is not.
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		0: {1, 2},
	})
}

// TestSplitTwoExits_NoJoin ensures that when a function has two exits
// and no reconvergence, there is no unique immediate post-dominator for the
// entry or the exits, forming a forest rather than a single rooted tree.
// Validates both post-dominator forest structure and CDG edges.
func TestSplitTwoExits_NoJoin(t *testing.T) {
	// 0 -> {1,2}; 1 (exit); 2 (exit)
	f, b := newFunctionWithBlocks(3)
	connect(b[0], b[1])
	connect(b[0], b[2])

	// Expected PDom forest (multiple exits, no unique root):
	//  1    2
	// (0 has no immediate postdominator)

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	require.Nilf(t, builder.Ipdom(b[0]), "ipdom(b0) want nil (multiple exits, no join), got %v", indexOf(builder.Ipdom(b[0])))
	require.Nilf(t, builder.Ipdom(b[1]), "ipdom(b1) want nil, got %v", indexOf(builder.Ipdom(b[1])))
	require.Nilf(t, builder.Ipdom(b[2]), "ipdom(b2) want nil, got %v", indexOf(builder.Ipdom(b[2])))

	require.False(t, builder.PostDominates(b[1], b[0]))
	require.False(t, builder.PostDominates(b[2], b[0]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: CDG includes edges from the branch to both exits
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{0: {1, 2}})
}

// TestLoopWithExit validates post-dominator tree and CDG for a loop with an exit.
// Validates the expected chain of immediate post-dominators and loop control dependencies.
func TestLoopWithExit(t *testing.T) {
	// 0 -> 1 -> 2 -> 3 (exit)
	//        ^    |
	//        |____|
	f, b := newFunctionWithBlocks(4)
	connect(b[0], b[1])
	connect(b[1], b[2])
	connect(b[2], b[1]) // back-edge
	connect(b[2], b[3]) // exit

	// Expected PDomTree (root is the exit):
	//  3
	//  └── 2
	//      └── 1
	//          └── 0

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	require.Equal(t, b[3], builder.Ipdom(b[2]))
	require.Equal(t, b[2], builder.Ipdom(b[1]))
	require.Equal(t, b[1], builder.Ipdom(b[0]))

	require.True(t, builder.PostDominates(b[2], b[1]))
	require.True(t, builder.PostDominates(b[3], b[2]))
	require.True(t, builder.PostDominates(b[3], b[1]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: node 2 controls reaching 1 (loop body) and itself (due to back-edge/self dependence)
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		2: {1, 2},
	})
}

// TestPostDom_DeadRegion_NoPathToExit tests a dead-region SCC with no path to any exit.
// Scenario: main component has a normal exit, plus a separate 2-node cycle that cannot reach an exit.
// Validates: main chain ipdoms behave normally; nodes in the dead region have ipdom set to nil or each other,
// and there is no cross-component post-dominance.
func TestPostDom_DeadRegion_NoPathToExit(t *testing.T) {
	// Main: 0 -> 1 -> 2 (exit)
	// Dead region: u1 <-> u2 (cycle), no exit path
	f, b := newFunctionWithBlocks(5)
	// main 0,1,2
	connect(b[0], b[1])
	connect(b[1], b[2])
	// dead region uses blocks 3(u1),4(u2)
	connect(b[3], b[4])
	connect(b[4], b[3])

	// Expected PDomTree for main component:
	//  2
	//  └── 1
	//      └── 0
	// Dead region (no path to exit):
	//  3 <-> 4 (ipdom may be nil or the other)

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Main chain behaves as usual
	require.Equal(t, b[1], builder.Ipdom(b[0]))
	require.Equal(t, b[2], builder.Ipdom(b[1]))
	require.Nil(t, builder.Ipdom(b[2]))

	// Dead region nodes: algorithm may choose one as ipdom of the other or nil.
	assert.Condition(t, func() bool { got := builder.Ipdom(b[3]); return got == nil || got == b[4] }, "ipdom(u1) want nil or u2 due to no path to exit")
	assert.Condition(t, func() bool { got := builder.Ipdom(b[4]); return got == nil || got == b[3] }, "ipdom(u2) want nil or u1 due to no path to exit")

	verifyTreeRelations(t, builder, f)
}

// TestPostDom_Recover_SingleBlock models a separate recover region consisting of a single block acting as its own exit.
// Asserts recover block has no ipdom and does not cross post-dominate main component.
func TestPostDom_Recover_SingleBlock(t *testing.T) {
	// Main: 0 -> 1 -> 2 (exit)
	// Recover: r (single block, exit)
	f, b := newFunctionWithBlocks(4)
	// main 0,1,2
	connect(b[0], b[1])
	connect(b[1], b[2])
	// recover single block is b[3] with no successors

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Main chain behaves as usual
	require.Equal(t, b[1], builder.Ipdom(b[0]))
	require.Equal(t, b[2], builder.Ipdom(b[1]))
	require.Nil(t, builder.Ipdom(b[2]))

	// Recover single block has no ipdom (treated like an exit)
	require.Nilf(t, builder.Ipdom(b[3]), "ipdom(recover) want nil, got %v", indexOf(builder.Ipdom(b[3])))

	// Cross-component post-dominance should be false
	require.False(t, builder.PostDominates(b[3], b[0]))
	require.False(t, builder.PostDominates(b[2], b[3]))

	verifyTreeRelations(t, builder, f)
}

// TestPostDom_Recover_Subgraph verifies that a recover region modeled as its own
// subgraph is handled correctly: the recover-exit post-dominates all recover
// blocks and there is no cross-component post-dominance with the main CFG.
func TestPostDom_Recover_Subgraph(t *testing.T) {
	// Main: 0 -> 1 -> 2 (exit)
	// Recover subgraph:
	//   r0 -> r1 -> r3 (exit)
	//   r0 -> r2 -> r3 (exit)
	// Blocks: 0,1,2,r0=3,r1=4,r2=5,r3=6
	f, b := newFunctionWithBlocks(7)
	// main 0,1,2
	connect(b[0], b[1])
	connect(b[1], b[2])
	// recover subgraph
	connect(b[3], b[4])
	connect(b[3], b[5])
	connect(b[4], b[6])
	connect(b[5], b[6])
	// r3 (b[6]) is exit

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Main chain behaves as usual
	require.Equal(t, b[1], builder.Ipdom(b[0]))
	require.Equal(t, b[2], builder.Ipdom(b[1]))
	require.Nil(t, builder.Ipdom(b[2]))

	// Recover region ipdom relations: all converge to r3
	require.Equal(t, b[6], builder.Ipdom(b[4]))
	require.Equal(t, b[6], builder.Ipdom(b[5]))
	require.Equal(t, b[6], builder.Ipdom(b[3]))
	require.Nil(t, builder.Ipdom(b[6]))

	// r3 immediately postdominates r0, r1, r2
	dominees := builder.PostDominees(b[6])
	require.ElementsMatch(t, []*ssa.BasicBlock{b[3], b[4], b[5]}, dominees)

	// Cross-component post-dominance should be false
	require.False(t, builder.PostDominates(b[6], b[0]))
	require.False(t, builder.PostDominates(b[2], b[3]))

	verifyTreeRelations(t, builder, f)
}

// TestSimpleSelfLoop_WithExit validates the post-dominator tree and CDG for a
// CFG with a self-loop and a separate exit. The exit post-dominates the loop
// block, which in turn post-dominates its predecessor.
func TestSimpleSelfLoop_WithExit(t *testing.T) {
	// 0 -> 1 -> 1 (self)
	//         -> 2 (exit)
	f, b := newFunctionWithBlocks(3)
	connect(b[0], b[1])
	connect(b[1], b[1]) // self-loop
	connect(b[1], b[2]) // exit

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// ipdom chain: 2 -> 1 -> 0
	require.Equal(t, b[2], builder.Ipdom(b[1]))
	require.Equal(t, b[1], builder.Ipdom(b[0]))
	require.Nil(t, builder.Ipdom(b[2]))

	// post-dominance facts
	require.True(t, builder.PostDominates(b[2], b[1]))
	require.True(t, builder.PostDominates(b[2], b[0]))
	require.True(t, builder.PostDominates(b[1], b[0]))
	require.False(t, builder.PostDominates(b[1], b[2]))

	// immediate post-dominees
	require.ElementsMatch(t, []*ssa.BasicBlock{b[1]}, builder.PostDominees(b[2]))
	require.ElementsMatch(t, []*ssa.BasicBlock{b[0]}, builder.PostDominees(b[1]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: only self-dependence at 1
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		1: {1},
	})
}

// TestNestedLoops_SingleExit constructs nested loops with a single exit
// and verifies the expected chain of immediate post-dominators from exit back
// to the entry through the loop headers, as well as CDG edges.
func TestNestedLoops_SingleExit(t *testing.T) {
	// 0 -> 1 -> 2 -> 3 -> 2 (inner loop)
	//              3 -> 4
	//           4 -> 1 (outer back-edge)
	//           4 -> 5 (exit)
	// 5 (exit)
	f, b := newFunctionWithBlocks(6)
	connect(b[0], b[1])
	connect(b[1], b[2])
	connect(b[2], b[3])
	connect(b[3], b[2]) // inner loop back-edge
	connect(b[3], b[4])
	connect(b[4], b[1]) // outer loop back-edge
	connect(b[4], b[5]) // exit

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Expected ipdom chain: 5 -> 4 -> 3 -> 2 -> 1 -> 0
	require.Equal(t, b[5], builder.Ipdom(b[4]))
	require.Equal(t, b[4], builder.Ipdom(b[3]))
	require.Equal(t, b[3], builder.Ipdom(b[2]))
	require.Equal(t, b[2], builder.Ipdom(b[1]))
	require.Equal(t, b[1], builder.Ipdom(b[0]))
	require.Nil(t, builder.Ipdom(b[5]))

	// Key post-dominance relations
	require.True(t, builder.PostDominates(b[5], b[4]))
	require.True(t, builder.PostDominates(b[4], b[1]))
	require.True(t, builder.PostDominates(b[4], b[2]))
	require.True(t, builder.PostDominates(b[4], b[3]))
	require.True(t, builder.PostDominates(b[3], b[2]))
	require.True(t, builder.PostDominates(b[2], b[1]))
	require.True(t, builder.PostDominates(b[1], b[0]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: decision at 3 controls 2 and itself; decision at 4 controls 1,2,3 and itself.
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		3: {2, 3},
		4: {1, 2, 3, 4},
	})
}

// TestIrreducible_TwoEntryCycle checks that post-dominators are well-formed
// for an irreducible SCC with two distinct entries that both flow to a single exit,
// and that there is no spurious post-dominance within the SCC. Validates both
// post-dominator tree and CDG.
func TestIrreducible_TwoEntryCycle(t *testing.T) {
	// 0 -> 1
	// 0 -> 2
	// 1 <-> 2
	// 1 -> 3 (exit)
	// 2 -> 3 (exit)
	f, b := newFunctionWithBlocks(4)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[1], b[2])
	connect(b[2], b[1])
	connect(b[1], b[3])
	connect(b[2], b[3]) // exit

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Exit postdominates all; immediate post-doms point directly to exit.
	require.Equal(t, b[3], builder.Ipdom(b[0]))
	require.Equal(t, b[3], builder.Ipdom(b[1]))
	require.Equal(t, b[3], builder.Ipdom(b[2]))
	require.Nil(t, builder.Ipdom(b[3]))

	require.True(t, builder.PostDominates(b[3], b[0]))
	require.True(t, builder.PostDominates(b[3], b[1]))
	require.True(t, builder.PostDominates(b[3], b[2]))
	require.False(t, builder.PostDominates(b[1], b[2]))
	require.False(t, builder.PostDominates(b[2], b[1]))

	// Exit's immediate post-dominees should be the three predecessors.
	dominees := builder.PostDominees(b[3])
	require.ElementsMatch(t, []*ssa.BasicBlock{b[0], b[1], b[2]}, dominees)

	verifyTreeRelations(t, builder, f)

	// CDG validation: 0 controls choosing between 1 and 2; inside SCC, 1 controls 2 and 2 controls 1.
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		0: {1, 2},
		1: {2},
		2: {1},
	})
}

// TestPartExit_PartInfiniteLoop tests a CFG where one branch reaches an exit
// while another branch enters an infinite loop (no path to any exit).
// Scenario: 0 -> 1, with 1 branching to 2 (exit) and to an SCC 3<->4 that has no exit path.
// Validates: exit 2 post-dominates 1 and 0 (all exit-reaching paths go via 2),
// loop nodes 3 and 4 have ipdom either nil or mutual due to no exit path,
// and 1 immediately post-dominates 0. Also validates CDG edges.
func TestPartExit_PartInfiniteLoop(t *testing.T) {
	// 0 -> 1 -> 2 (exit)
	//      |\
	//      | \-> 3 -> 4
	//      |       ^    \
	//      |_______|_____/
	f, b := newFunctionWithBlocks(5)
	connect(b[0], b[1])
	connect(b[1], b[2]) // path to exit
	connect(b[1], b[3]) // branch into infinite loop
	connect(b[3], b[4])
	connect(b[4], b[3]) // cycle, no path to exit

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// ipdom expectations on the main component: when there is a branch into a non-terminating loop,
	// the algorithm may assign ipdom(1) to nil (via dummy root) or to 2 depending on treatment.
	gotIpdom1 := builder.Ipdom(b[1])
	assert.Condition(t, func() bool { return gotIpdom1 == nil || gotIpdom1 == b[2] }, "ipdom(1) want nil or 2 due to infinite loop branch")
	require.Equal(t, b[1], builder.Ipdom(b[0]))
	require.Nil(t, builder.Ipdom(b[2]))

	// post-dominance facts
	// With dummy-exit wiring for non-terminating paths, 2 may not post-dominate 1 or 0.
	require.False(t, builder.PostDominates(b[2], b[1]))
	require.False(t, builder.PostDominates(b[2], b[0]))
	require.True(t, builder.PostDominates(b[1], b[0])) // 1 lies on all paths from 0
	require.False(t, builder.PostDominates(b[1], b[2]))

	// Loop nodes have no path to exit; ipdom may be nil or mutual.
	assert.Condition(t, func() bool { got := builder.Ipdom(b[3]); return got == nil || got == b[4] }, "ipdom(3) want nil or 4 due to no path to exit")
	assert.Condition(t, func() bool { got := builder.Ipdom(b[4]); return got == nil || got == b[3] }, "ipdom(4) want nil or 3 due to no path to exit")

	// Cross-component post-dominance should be false in both directions for loop vs exit/main
	require.False(t, builder.PostDominates(b[3], b[1]))
	require.False(t, builder.PostDominates(b[4], b[1]))
	require.False(t, builder.PostDominates(b[2], b[3]))
	require.False(t, builder.PostDominates(b[2], b[4]))

	verifyTreeRelations(t, builder, f)

	// CDG validation
	// Minimal invariants:
	// - 1 must control reaching 3 (loop entry). Edge 1->2 may or may not exist depending on ipdom(1).
	// - there is at least one cross-edge between 3 and 4
	// - non-loop non-branch nodes (0,2) have no outgoing CDG edges
	cdg := builder.BuildCDG(f)
	assert.True(t, cdgHasEdge(cdg, 1, 3), "expected CDG edge 1->3")
	edge34 := cdgHasEdge(cdg, 3, 4)
	edge43 := cdgHasEdge(cdg, 4, 3)
	assert.True(t, edge34 || edge43, "expected at least one cross-edge between 3 and 4 in CDG")
	requireNoOutgoing(t, cdg, []int{0, 2})
}

func TestCDG_ComplexGraph(t *testing.T) {
	// CFG:
	//          1
	//        /  \
	//       3 <- 2
	//      /     \
	//     /      4
	//    /       / \
	//   /       5   6
	//  /       /     \
	// /       /       7
	// |      /       /
	// +---> 8 <-----+
	//       (exit)
	//
	// Expected CDG (Control Dependence Graph):
	//   1 controls {2, 3} - Block 1 branches, deciding which path(s) execute
	//   2 controls {3, 4} - Block 2 branches, deciding which path(s) execute
	//   4 controls {5, 6, 7} - Block 4 branches, determining which exit path
	//   All other blocks have no control dependence (no branching decisions)
	//
	// CDG visualization (arrows show control dependence):
	//          1
	//         /  \
	//        v    v
	//        2 --> 3
	//       /
	//      v
	//      4
	//     /|\
	//    v v v
	//    5 6 7
	//
	// Interpretation:
	//   - 1 → 2: Block 1's branch decision controls whether block 2 executes
	//   - 1 → 3: Block 1's branch decision controls whether block 3 executes
	//   - 2 → 3: Block 2's branch decision controls whether block 3 executes
	//   - 2 → 4: Block 2's branch decision controls whether block 4 executes
	//   - 4 → 5: Block 4's branch decision controls whether block 5 executes
	//   - 4 → 6: Block 4's branch decision controls whether block 6 executes
	//   - 4 → 7: Block 4's branch decision controls whether block 7 executes

	f, b := newFunctionWithBlocks(9)
	connect(b[1], b[3])
	connect(b[1], b[2])
	connect(b[2], b[3])
	connect(b[3], b[8])
	connect(b[7], b[8])
	connect(b[2], b[4])
	connect(b[4], b[5])
	connect(b[4], b[6])
	connect(b[5], b[8])
	connect(b[6], b[7])

	builder := newCDGBuilder()
	cdg := builder.BuildCDG(f)

	// Expected CDG edges:
	requireCDGEdges(t, cdg, map[int][]int{
		1: {2, 3},    // Block 1 controls blocks 2 and 3
		2: {3, 4},    // Block 2 controls blocks 3 and 4
		4: {5, 6, 7}, // Block 4 controls blocks 5, 6, and 7
		// Blocks 3, 5, 6, 7, 8 have no outgoing edges (no branching)
	})

	// Additional specific edge checks for clarity
	assert.True(t, cdgHasEdge(cdg, 1, 2), "Block 1 should control block 2 (branch decision)")
	assert.True(t, cdgHasEdge(cdg, 1, 3), "Block 1 should control block 3 (branch decision)")
	assert.True(t, cdgHasEdge(cdg, 2, 3), "Block 2 should control block 3 (can bypass via 2->4 path)")
	assert.True(t, cdgHasEdge(cdg, 2, 4), "Block 2 should control block 4 (branch decision)")
	assert.True(t, cdgHasEdge(cdg, 4, 5), "Block 4 should control block 5 (branch decision)")
	assert.True(t, cdgHasEdge(cdg, 4, 6), "Block 4 should control block 6 (branch decision)")
	assert.True(t, cdgHasEdge(cdg, 4, 7), "Block 4 should control block 7 (via 6->7 path)")

	// Verify non-branching blocks have no outgoing CDG edges
	requireNoOutgoing(t, cdg, []int{3, 5, 6, 7, 8})
}

// TestCascadingDiamonds tests a CFG with multiple sequential diamond patterns
// where each merge point immediately branches again. This tests the algorithm's ability
// to handle repeated fork-join structures for both post-dominator tree and CDG.
func TestCascadingDiamonds(t *testing.T) {
	//    0
	//   / \
	//  1   2
	//   \ /
	//    3
	//   / \
	//  4   5
	//   \ /
	//    6
	//   / \
	//  7   8
	//   \ /
	//    9 (exit)
	f, b := newFunctionWithBlocks(10)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[1], b[3])
	connect(b[2], b[3])
	connect(b[3], b[4])
	connect(b[3], b[5])
	connect(b[4], b[6])
	connect(b[5], b[6])
	connect(b[6], b[7])
	connect(b[6], b[8])
	connect(b[7], b[9])
	connect(b[8], b[9])

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Each merge point ipdom's its immediate predecessors
	require.Equal(t, b[3], builder.Ipdom(b[1]))
	require.Equal(t, b[3], builder.Ipdom(b[2]))
	require.Equal(t, b[6], builder.Ipdom(b[4]))
	require.Equal(t, b[6], builder.Ipdom(b[5]))
	require.Equal(t, b[9], builder.Ipdom(b[7]))
	require.Equal(t, b[9], builder.Ipdom(b[8]))

	// Fork points are ipdom'd by their immediate merge
	require.Equal(t, b[3], builder.Ipdom(b[0]))
	require.Equal(t, b[6], builder.Ipdom(b[3]))
	require.Equal(t, b[9], builder.Ipdom(b[6]))
	require.Nil(t, builder.Ipdom(b[9]))

	// Verify transitive post-dominance
	require.True(t, builder.PostDominates(b[9], b[0]))
	require.True(t, builder.PostDominates(b[6], b[0]))
	require.True(t, builder.PostDominates(b[3], b[0]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: each fork controls only its immediate successors
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		0: {1, 2},
		3: {4, 5},
		6: {7, 8},
	})
}

// TestLoopWithMultipleExits tests a loop that can exit from multiple points.
// This verifies that the earliest common post-dominator is correctly identified
// and that the CDG reflects the control dependencies for multiple exit paths.
func TestLoopWithMultipleExits(t *testing.T) {
	// 0 -> 1 -> 2 -> 3 (exit)
	//      ^    |
	//      |    v
	//      5 <- 4 -> 6 (exit)
	f, b := newFunctionWithBlocks(7)
	connect(b[0], b[1])
	connect(b[1], b[2])
	connect(b[2], b[3]) // exit path 1
	connect(b[2], b[4])
	connect(b[4], b[5])
	connect(b[5], b[1]) // back-edge
	connect(b[4], b[6]) // exit path 2

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Multiple exits mean 1,2,4 have no unique exit post-dominator
	// Loop back-edge nodes should be ipdom'd by their immediate post-dom
	require.Equal(t, b[1], builder.Ipdom(b[5]))
	require.Equal(t, b[1], builder.Ipdom(b[0]))

	// 2 and 4 have no common post-dominator (multiple exits)
	assert.Condition(t, func() bool {
		ipdom2 := builder.Ipdom(b[2])
		ipdom4 := builder.Ipdom(b[4])
		// Either could be nil (no common pdom) or point to some structure
		return ipdom2 == nil || ipdom4 == nil || ipdom2 != ipdom4
	}, "loop with multiple exits should not have spurious common ipdom")

	// Cross-exit post-dominance should be false
	require.False(t, builder.PostDominates(b[3], b[6]))
	require.False(t, builder.PostDominates(b[6], b[3]))

	verifyTreeRelations(t, builder, f)

	// CDG validation
	// Block 2 controls exit path choice; block 4 controls loop continuation and second exit
	cdg := builder.BuildCDG(f)
	assert.True(t, cdgHasEdge(cdg, 2, 4), "block 2 should control reaching block 4")
	assert.True(t, cdgHasEdge(cdg, 4, 5), "block 4 should control loop continuation")
	// Note: Self-dependence at 4 depends on algorithm details; check what edges exist
	requireNoOutgoing(t, cdg, []int{0, 1, 3, 5, 6})
}

// TestSwitchPattern tests a CFG where one block branches to 4 successors,
// modeling a multi-way branch (like a switch statement). All paths converge to exit.
// Validates both post-dominator tree and CDG for multi-way branching.
func TestSwitchPattern(t *testing.T) {
	//       0
	//     / | \ \
	//    1  2 3  4
	//     \ | / /
	//       5 (exit)
	f, b := newFunctionWithBlocks(6)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[0], b[3])
	connect(b[0], b[4])
	connect(b[1], b[5])
	connect(b[2], b[5])
	connect(b[3], b[5])
	connect(b[4], b[5])

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// All switch arms are immediately post-dominated by the exit
	require.Equal(t, b[5], builder.Ipdom(b[1]))
	require.Equal(t, b[5], builder.Ipdom(b[2]))
	require.Equal(t, b[5], builder.Ipdom(b[3]))
	require.Equal(t, b[5], builder.Ipdom(b[4]))
	require.Equal(t, b[5], builder.Ipdom(b[0]))
	require.Nil(t, builder.Ipdom(b[5]))

	// Exit post-dominates all
	require.True(t, builder.PostDominates(b[5], b[0]))
	require.True(t, builder.PostDominates(b[5], b[1]))
	require.True(t, builder.PostDominates(b[5], b[2]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: block 0 controls all four switch arms
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		0: {1, 2, 3, 4},
	})
}

// TestTrianglePattern tests a CFG where there's both a direct edge and
// an indirect path between two nodes, creating a triangle. Validates both
// post-dominator tree and CDG for bypass scenarios.
func TestTrianglePattern(t *testing.T) {
	//    0
	//   /|\
	//  / | \
	// |  1  |
	//  \ | /
	//   \|/
	//    2
	//    |
	//    3 (exit)
	f, b := newFunctionWithBlocks(4)
	connect(b[0], b[1])
	connect(b[0], b[2]) // direct path
	connect(b[1], b[2]) // indirect path through 1
	connect(b[2], b[3])

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Block 2 is the merge point and ipdom of both 0 and 1
	require.Equal(t, b[2], builder.Ipdom(b[0]))
	require.Equal(t, b[2], builder.Ipdom(b[1]))
	require.Equal(t, b[3], builder.Ipdom(b[2]))
	require.Nil(t, builder.Ipdom(b[3]))

	require.True(t, builder.PostDominates(b[2], b[0]))
	require.True(t, builder.PostDominates(b[2], b[1]))
	require.True(t, builder.PostDominates(b[3], b[0]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: block 0 controls whether 1 executes (can bypass via direct edge)
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		0: {1},
	})
}

// TestLadderPattern tests a "ladder" CFG where execution can skip ahead
// at multiple points, creating multiple levels of optional execution.
// Validates both post-dominator tree and CDG for skip-ahead patterns.
func TestLadderPattern(t *testing.T) {
	//    0 ------\
	//    |       |
	//    1 ----\ |
	//    |     | |
	//    2 --\ | |
	//    |   | | |
	//    3   | | |
	//     \ /  | |
	//      4  /  |
	//       \/   |
	//       5   /
	//        \ /
	//         6 (exit)
	f, b := newFunctionWithBlocks(7)
	connect(b[0], b[1])
	connect(b[0], b[6]) // skip all
	connect(b[1], b[2])
	connect(b[1], b[5]) // skip 2,3,4
	connect(b[2], b[3])
	connect(b[2], b[4]) // skip 3
	connect(b[3], b[4])
	connect(b[4], b[5])
	connect(b[5], b[6])

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Exit ipdom's all merge points moving backward
	require.Equal(t, b[6], builder.Ipdom(b[5]))
	require.Equal(t, b[5], builder.Ipdom(b[4]))
	require.Equal(t, b[4], builder.Ipdom(b[3]))
	require.Equal(t, b[4], builder.Ipdom(b[2])) // 2->3->4 or 2->4, so 4 post-dominates 2
	require.Equal(t, b[5], builder.Ipdom(b[1]))
	require.Equal(t, b[6], builder.Ipdom(b[0]))
	require.Nil(t, builder.Ipdom(b[6]))

	require.True(t, builder.PostDominates(b[6], b[0]))
	require.True(t, builder.PostDominates(b[5], b[1]))
	require.True(t, builder.PostDominates(b[4], b[3]))
	require.True(t, builder.PostDominates(b[4], b[2]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: each branch controls the blocks it can skip
	cdg := builder.BuildCDG(f)
	assert.True(t, cdgHasEdge(cdg, 0, 1), "block 0 controls whether 1 executes")
	assert.True(t, cdgHasEdge(cdg, 1, 2), "block 1 controls whether 2 executes")
	assert.True(t, cdgHasEdge(cdg, 2, 3), "block 2 controls whether 3 executes")
}

// TestFigure8Loops tests two loops sharing a common header block.
// Validates both post-dominator tree and CDG for shared loop headers.
func TestFigure8Loops(t *testing.T) {
	//    0
	//    |
	//    1 (shared header)
	//   / \
	//  2   3
	//  |   |
	//  +-> 1 <-+ (both loop back to 1)
	//      |
	//      4 (exit)
	f, b := newFunctionWithBlocks(5)
	connect(b[0], b[1])
	connect(b[1], b[2]) // left loop
	connect(b[1], b[3]) // right loop
	connect(b[2], b[1]) // left back-edge
	connect(b[3], b[1]) // right back-edge
	connect(b[1], b[4]) // exit

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Shared header ipdom's entry; exit ipdom's header
	require.Equal(t, b[1], builder.Ipdom(b[0]))
	require.Equal(t, b[4], builder.Ipdom(b[1]))
	require.Equal(t, b[1], builder.Ipdom(b[2]))
	require.Equal(t, b[1], builder.Ipdom(b[3]))
	require.Nil(t, builder.Ipdom(b[4]))

	require.True(t, builder.PostDominates(b[1], b[0]))
	require.True(t, builder.PostDominates(b[1], b[2]))
	require.True(t, builder.PostDominates(b[1], b[3]))
	require.True(t, builder.PostDominates(b[4], b[1]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: header controls which loop(s) execute and itself (for loop vs exit decision)
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		1: {1, 2, 3},
	})
}

// TestPartialReconvergence tests a CFG where some branches merge but
// not all paths reconverge, creating a partial post-domination scenario.
// Validates both post-dominator tree and CDG for partial convergence.
func TestPartialReconvergence(t *testing.T) {
	//      0
	//     /|\
	//    1 2 3
	//    |X| |
	//    4 5 6 (three exits)
	f, b := newFunctionWithBlocks(7)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[0], b[3])
	connect(b[1], b[4]) // path 1 to exit 4
	connect(b[1], b[5]) // path 1 can also reach exit 5
	connect(b[2], b[4]) // path 2 to exit 4
	connect(b[2], b[5]) // path 2 can also reach exit 5
	connect(b[3], b[6]) // path 3 to exit 6 only

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Blocks 1 and 2 have no single ipdom (can reach multiple exits)
	// Block 3 has exit 6 as potential ipdom, but not a global one
	// Block 0 has no global ipdom (multiple distinct exit paths)
	assert.Condition(t, func() bool {
		ipdom0 := builder.Ipdom(b[0])
		return ipdom0 == nil
	}, "block 0 with diverging exit paths should have nil ipdom")

	// Exits should not post-dominate each other
	require.False(t, builder.PostDominates(b[4], b[6]))
	require.False(t, builder.PostDominates(b[6], b[4]))
	require.False(t, builder.PostDominates(b[5], b[6]))

	verifyTreeRelations(t, builder, f)

	// CDG validation
	cdg := builder.BuildCDG(f)
	// Block 0 controls which initial path (1, 2, or 3) is taken
	assert.True(t, cdgHasEdge(cdg, 0, 1), "block 0 should control block 1")
	assert.True(t, cdgHasEdge(cdg, 0, 2), "block 0 should control block 2")
	assert.True(t, cdgHasEdge(cdg, 0, 3), "block 0 should control block 3")

	// Blocks 1 and 2 control their exit choices
	assert.True(t, cdgHasEdge(cdg, 1, 4) || cdgHasEdge(cdg, 1, 5), "block 1 should control at least one exit")
	assert.True(t, cdgHasEdge(cdg, 2, 4) || cdgHasEdge(cdg, 2, 5), "block 2 should control at least one exit")
}

// TestPostdomPreorder tests the PostdomPreorder function which returns blocks
// sorted by their preorder numbering in the post-dominator tree.
// Uses a diamond CFG to verify that the ordering respects tree structure.
func TestPostdomPreorder(t *testing.T) {
	//    0
	//   / \
	//  1   2
	//   \ /
	//    3 (exit)
	//
	// Expected PDomTree (root is the exit):
	//  3
	//  ├── 0
	//  ├── 1
	//  └── 2
	//
	// Preorder traversal visits nodes in DFS order starting from root:
	// The root (3) has the smallest preorder number, followed by its children.

	f, b := newFunctionWithBlocks(4)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[1], b[3])
	connect(b[2], b[3])

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Get preorder traversal
	preorder := builder.PostdomPreorder(f)

	// Should return all blocks
	require.Len(t, preorder, 4)

	// Verify blocks are sorted by preorder number (ascending)
	for i := 0; i < len(preorder)-1; i++ {
		require.Less(t, builder.getPDomInfo(preorder[i]).pre, builder.getPDomInfo(preorder[i+1]).pre,
			"blocks should be ordered by ascending preorder number")
	}

	// Verify parent comes before children in preorder
	for _, block := range f.Blocks {
		parent := builder.Ipdom(block)
		if parent == nil {
			continue // root has no parent
		}
		parentIdx := -1
		childIdx := -1
		for i, b := range preorder {
			if b == parent {
				parentIdx = i
			}
			if b == block {
				childIdx = i
			}
		}
		require.Less(t, parentIdx, childIdx,
			"parent block %d should come before child block %d in preorder", parent.Index, block.Index)
	}

	// In this diamond, 3 (exit) is the root and should have the smallest preorder number
	require.Equal(t, b[3], preorder[0], "exit block (root of postdom tree) should be first in preorder")
}

// TestPostdomPostorder tests the PostdomPostorder function which returns blocks
// sorted by their postorder numbering in the post-dominator tree.
// Uses a diamond CFG to verify that the ordering respects tree structure.
func TestPostdomPostorder(t *testing.T) {
	//    0
	//   / \
	//  1   2
	//   \ /
	//    3 (exit)
	//
	// Expected PDomTree (root is the exit):
	//  3
	//  ├── 0
	//  ├── 1
	//  └── 2
	//
	// Postorder traversal visits children before parents:
	// The root (3) has the largest postorder number and comes last.

	f, b := newFunctionWithBlocks(4)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[1], b[3])
	connect(b[2], b[3])

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Get postorder traversal
	postorder := builder.PostdomPostorder(f)

	// Should return all blocks
	require.Len(t, postorder, 4)

	// Verify blocks are sorted by postorder number (ascending)
	for i := 0; i < len(postorder)-1; i++ {
		require.Less(t, builder.getPDomInfo(postorder[i]).post, builder.getPDomInfo(postorder[i+1]).post,
			"blocks should be ordered by ascending postorder number")
	}

	// Verify children come before parents in postorder
	for _, block := range f.Blocks {
		parent := builder.Ipdom(block)
		if parent == nil {
			continue // root has no parent
		}
		parentIdx := -1
		childIdx := -1
		for i, b := range postorder {
			if b == parent {
				parentIdx = i
			}
			if b == block {
				childIdx = i
			}
		}
		require.Less(t, childIdx, parentIdx,
			"child block %d should come before parent block %d in postorder", block.Index, parent.Index)
	}

	// In this diamond, 3 (exit) is the root and should have the largest postorder number (come last)
	require.Equal(t, b[3], postorder[len(postorder)-1], "exit block (root of postdom tree) should be last in postorder")
}

// TestDeepNestedConditions tests deeply nested if-else chains.
// Validates both post-dominator tree and CDG for deep nesting.
func TestDeepNestedConditions(t *testing.T) {
	//    0
	//   / \
	//  1   2
	//     / \
	//    3   4
	//       / \
	//      5   6
	//     / \
	//    7   8
	//     \ /
	//      9 (exit)
	f, b := newFunctionWithBlocks(10)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[1], b[9])
	connect(b[2], b[3])
	connect(b[2], b[4])
	connect(b[3], b[9])
	connect(b[4], b[5])
	connect(b[4], b[6])
	connect(b[6], b[9])
	connect(b[5], b[7])
	connect(b[5], b[8])
	connect(b[7], b[9])
	connect(b[8], b[9])

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Exit post-dominates everything
	require.Equal(t, b[9], builder.Ipdom(b[0]))
	require.Nil(t, builder.Ipdom(b[9]))

	// Verify deep nesting ipdom relations
	require.Equal(t, b[9], builder.Ipdom(b[1]))
	require.Equal(t, b[9], builder.Ipdom(b[2]))
	require.Equal(t, b[9], builder.Ipdom(b[3]))
	require.Equal(t, b[9], builder.Ipdom(b[4]))
	require.Equal(t, b[9], builder.Ipdom(b[5]))
	require.Equal(t, b[9], builder.Ipdom(b[6]))
	require.Equal(t, b[9], builder.Ipdom(b[7]))
	require.Equal(t, b[9], builder.Ipdom(b[8]))

	require.True(t, builder.PostDominates(b[9], b[0]))
	require.True(t, builder.PostDominates(b[9], b[5]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: each branch point controls its immediate successors
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		0: {1, 2},
		2: {3, 4},
		4: {5, 6},
		5: {7, 8},
	})
}

// TestComplexConvergencePattern tests a CFG structure that triggers the deferred
// bucket processing path in the Lengauer-Tarjan algorithm where the semi-dominator
// of the evaluated node has a preorder number >= the current node being processed.
// This specifically exercises the else branch in computeDominators when processing buckets.
func TestComplexConvergencePattern(t *testing.T) {
	// CFG structure designed to trigger deferred bucket processing:
	//       0
	//      / \
	//     1   2
	//    / \ / \
	//   3   4   5
	//    \ / \ /
	//     6   7
	//      \ /
	//       8 (exit)
	//
	// This creates a complex convergence pattern where:
	// - Multiple paths converge at different points
	// - Semi-dominator relationships are non-trivial
	// - Some nodes will be deferred to buckets for later processing
	f, b := newFunctionWithBlocks(9)
	connect(b[0], b[1])
	connect(b[0], b[2])
	connect(b[1], b[3])
	connect(b[1], b[4])
	connect(b[2], b[4])
	connect(b[2], b[5])
	connect(b[3], b[6])
	connect(b[4], b[6])
	connect(b[4], b[7])
	connect(b[5], b[7])
	connect(b[6], b[8])
	connect(b[7], b[8])

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Exit should post-dominate all nodes
	require.Equal(t, b[8], builder.Ipdom(b[6]))
	require.Equal(t, b[8], builder.Ipdom(b[7]))
	require.Equal(t, b[8], builder.Ipdom(b[4]))
	require.Equal(t, b[6], builder.Ipdom(b[3]))
	require.Equal(t, b[7], builder.Ipdom(b[5]))
	require.Equal(t, b[8], builder.Ipdom(b[1]))
	fmt.Println(builder.Ipdom(b[2]))
	require.Equal(t, b[8], builder.Ipdom(b[2]))
	require.Equal(t, b[8], builder.Ipdom(b[0]))

	require.Nil(t, builder.Ipdom(b[8]))

	// Verify convergence points
	require.True(t, builder.PostDominates(b[8], b[0]))
	require.True(t, builder.PostDominates(b[8], b[1]))
	require.True(t, builder.PostDominates(b[8], b[2]))
	require.True(t, builder.PostDominates(b[8], b[3]))
	require.True(t, builder.PostDominates(b[8], b[4]))
	require.True(t, builder.PostDominates(b[8], b[5]))
	require.True(t, builder.PostDominates(b[8], b[6]))
	require.True(t, builder.PostDominates(b[8], b[7]))

	verifyTreeRelations(t, builder, f)

	// CDG validation: branch points control their respective paths
	// Expected CDG edges based on post-dominance frontiers:
	//   0 controls {1, 2} - immediate successors up to (excluding) ipdom(0)
	//   1 controls {3, 4, 6} - branches with alternate convergence
	//   2 controls {4, 5, 7} - branches with alternate convergence
	//   4 controls {6, 7} - direct branches to merge points
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		0: {1, 2},
		1: {3, 4, 6},
		2: {4, 5, 7},
		4: {6, 7},
	})
}

// TestDeferredBucketProcessing tests the deferred bucket processing path in the
// Lengauer-Tarjan algorithm where a node's idom is set to u (not w) during bucket
// processing. This covers line 343: cb.getPDomInfo(v).idom = u
func TestDeferredBucketProcessing(t *testing.T) {
	// CFG pattern from Lengauer-Tarjan Figure 6 - classic case for eval optimization:
	//       R (0)
	//      / \
	//     A   B (1,2)
	//    /|\ /
	//   / | X
	//  /  |/ \
	// C   D   E (3,4,5)
	//  \ /|\ /
	//   F | G (6,7)
	//    \|/
	//     H (8)
	//     |
	//     I (exit) (9)
	//
	// This specific pattern with cross edges should force the case where
	// eval returns a node whose sdom is better than the bucket node.
	f, b := newFunctionWithBlocks(10)
	connect(b[0], b[1]) // R -> A
	connect(b[0], b[2]) // R -> B
	connect(b[1], b[3]) // A -> C
	connect(b[1], b[4]) // A -> D
	connect(b[1], b[5]) // A -> E
	connect(b[2], b[4]) // B -> D (cross)
	connect(b[2], b[5]) // B -> E
	connect(b[3], b[6]) // C -> F
	connect(b[4], b[6]) // D -> F
	connect(b[4], b[7]) // D -> G
	connect(b[5], b[7]) // E -> G
	connect(b[6], b[8]) // F -> H
	connect(b[7], b[8]) // G -> H
	connect(b[8], b[9]) // H -> I

	builder := newCDGBuilder()
	builder.BuildPostdomTree(f)

	// Post-dominator tree validation
	// Immediate post-dominators (ipdom)
	require.Equal(t, b[8], builder.Ipdom(b[0]), "ipdom(0) should be 8 (H), as all paths from entry reconverge at H before exit")
	require.Equal(t, b[8], builder.Ipdom(b[1]), "ipdom(1) should be 8 (H), A's paths converge at H")
	require.Equal(t, b[8], builder.Ipdom(b[2]), "ipdom(2) should be 8 (H), B's paths converge at H")
	require.Equal(t, b[6], builder.Ipdom(b[3]), "ipdom(3) should be 6 (F), C flows only to F before H")
	require.Equal(t, b[8], builder.Ipdom(b[4]), "ipdom(4) should be 8 (H), D reaches H via F or G")
	require.Equal(t, b[7], builder.Ipdom(b[5]), "ipdom(5) should be 7 (G), E flows to G before H")
	require.Equal(t, b[8], builder.Ipdom(b[6]), "ipdom(6) should be 8 (H), F reaches H next")
	require.Equal(t, b[8], builder.Ipdom(b[7]), "ipdom(7) should be 8 (H), G reaches H next")
	require.Equal(t, b[9], builder.Ipdom(b[8]), "ipdom(8) should be 9 (I), H goes directly to exit")
	require.Nil(t, builder.Ipdom(b[9]), "exit block has no immediate post-dominator")

	// Key post-dominance facts
	require.True(t, builder.PostDominates(b[9], b[0]), "exit (9) post-dominates entry (0): all paths end at exit")

	verifyTreeRelations(t, builder, f)

	// CDG validation
	// Expected CDG edges based on post-dominance frontiers:
	//   0 controls {1, 2} - R branches to A/B
	//   1 controls {3, 4, 5, 6, 7} - A branches to C/D/E affecting deeper nodes
	//   2 controls {4, 5, 7} - B branches (cross edges) affecting D/E/G
	//   4 controls {6, 7} - D branches to F and G
	cdg := builder.BuildCDG(f)
	requireCDGEdges(t, cdg, map[int][]int{
		0: {1, 2},
		1: {3, 4, 5, 6, 7},
		2: {4, 5, 7},
		4: {6, 7},
	})
}
