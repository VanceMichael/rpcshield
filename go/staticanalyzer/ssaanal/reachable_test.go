package ssaanal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ssa"
)

// buildLinearCFG creates a simple SSA function with a linear chain of blocks:
// block0 -> block1 -> ... -> blockN-1.
func buildLinearCFG(n int) *ssa.Function {
	fn := &ssa.Function{}
	blocks := make([]*ssa.BasicBlock, n)
	for i := 0; i < n; i++ {
		blocks[i] = &ssa.BasicBlock{Index: i}
	}
	for i := 0; i < n-1; i++ {
		blocks[i].Succs = append(blocks[i].Succs, blocks[i+1])
		blocks[i+1].Preds = append(blocks[i+1].Preds, blocks[i])
	}
	fn.Blocks = blocks
	return fn
}

func TestComputeSrcReachable_NilSrcBlock(t *testing.T) {
	fn := buildLinearCFG(3)
	reachable, preds, succs := computeSrcReachable(fn, nil)

	for i := 0; i < 3; i++ {
		require.True(t, reachable[i], "block %d should be reachable", i)
		require.True(t, preds[i], "block %d should be in preds", i)
		require.True(t, succs[i], "block %d should be in succs", i)
	}
}

func TestComputeSrcReachable_MiddleBlock(t *testing.T) {
	fn := buildLinearCFG(3)
	reachable, preds, succs := computeSrcReachable(fn, fn.Blocks[1])

	for i := 0; i < 3; i++ {
		require.True(t, reachable[i], "block %d should be reachable", i)
	}

	require.True(t, preds[0], "block 0 should be a predecessor")
	require.True(t, preds[1], "srcBlock should be in preds")
	require.False(t, preds[2], "block 2 should not be a predecessor")

	require.False(t, succs[0], "block 0 should not be a successor")
	require.True(t, succs[1], "srcBlock should be in succs")
	require.True(t, succs[2], "block 2 should be a successor")
}

func TestComputeSrcReachable_FirstBlock(t *testing.T) {
	fn := buildLinearCFG(3)
	reachable, preds, succs := computeSrcReachable(fn, fn.Blocks[0])

	for i := 0; i < 3; i++ {
		require.True(t, reachable[i], "block %d should be reachable", i)
	}

	require.True(t, preds[0], "srcBlock should be in preds")
	require.False(t, preds[1])
	require.False(t, preds[2])

	require.True(t, succs[0])
	require.True(t, succs[1])
	require.True(t, succs[2])
}

func TestComputeSrcReachable_LastBlock(t *testing.T) {
	fn := buildLinearCFG(3)
	reachable, preds, succs := computeSrcReachable(fn, fn.Blocks[2])

	for i := 0; i < 3; i++ {
		require.True(t, reachable[i], "block %d should be reachable", i)
	}

	require.True(t, preds[0])
	require.True(t, preds[1])
	require.True(t, preds[2])

	require.False(t, succs[0])
	require.False(t, succs[1])
	require.True(t, succs[2])
}

// buildDiamondCFG creates a diamond-shaped CFG:
//
//	block0
//	 / \
//	v   v
//
// block1 block2
//
//	 \ /
//	  v
//	block3
func buildDiamondCFG() *ssa.Function {
	fn := &ssa.Function{}
	blocks := make([]*ssa.BasicBlock, 4)
	for i := range blocks {
		blocks[i] = &ssa.BasicBlock{Index: i}
	}
	// block0 -> block1, block2
	blocks[0].Succs = []*ssa.BasicBlock{blocks[1], blocks[2]}
	blocks[1].Preds = []*ssa.BasicBlock{blocks[0]}
	blocks[2].Preds = []*ssa.BasicBlock{blocks[0]}
	// block1, block2 -> block3
	blocks[1].Succs = []*ssa.BasicBlock{blocks[3]}
	blocks[2].Succs = []*ssa.BasicBlock{blocks[3]}
	blocks[3].Preds = []*ssa.BasicBlock{blocks[1], blocks[2]}
	fn.Blocks = blocks
	return fn
}

func TestGetReachableBlocks_AllFeasible(t *testing.T) {
	fn := buildDiamondCFG()

	a := &analyzer{
		blockState:    make(map[*ssa.BasicBlock]*blkState),
		feasibleEdges: make(map[cfgEdge]bool),
	}
	// Mark all blocks executable and all edges feasible.
	for _, b := range fn.Blocks {
		a.blockState[b] = &blkState{executable: true}
	}
	for _, b := range fn.Blocks {
		for _, s := range b.Succs {
			a.feasibleEdges[cfgEdge{from: b, to: s}] = true
		}
	}

	reachable := a.getReachableBlocks(fn.Blocks[0])
	for _, b := range fn.Blocks {
		require.True(t, reachable[b], "block %d should be reachable", b.Index)
	}
}

func TestGetReachableBlocks_OneBranchInfeasible(t *testing.T) {
	fn := buildDiamondCFG()

	a := &analyzer{
		blockState:    make(map[*ssa.BasicBlock]*blkState),
		feasibleEdges: make(map[cfgEdge]bool),
	}
	for _, b := range fn.Blocks {
		a.blockState[b] = &blkState{executable: true}
	}
	// Only mark the left branch (block0->block1->block3) as feasible.
	a.feasibleEdges[cfgEdge{from: fn.Blocks[0], to: fn.Blocks[1]}] = true
	a.feasibleEdges[cfgEdge{from: fn.Blocks[1], to: fn.Blocks[3]}] = true
	// block0->block2 is NOT feasible.

	reachable := a.getReachableBlocks(fn.Blocks[0])
	require.True(t, reachable[fn.Blocks[0]], "start block should be reachable")
	require.True(t, reachable[fn.Blocks[1]], "block1 should be reachable via feasible edge")
	require.False(t, reachable[fn.Blocks[2]], "block2 should NOT be reachable (infeasible edge)")
	require.True(t, reachable[fn.Blocks[3]], "block3 should be reachable via block1")
}

func TestGetReachableBlocks_NotExecutable(t *testing.T) {
	fn := buildLinearCFG(3)

	a := &analyzer{
		blockState:    make(map[*ssa.BasicBlock]*blkState),
		feasibleEdges: make(map[cfgEdge]bool),
	}
	a.blockState[fn.Blocks[0]] = &blkState{executable: true}
	a.blockState[fn.Blocks[1]] = &blkState{executable: false} // not executable
	a.blockState[fn.Blocks[2]] = &blkState{executable: true}
	a.feasibleEdges[cfgEdge{from: fn.Blocks[0], to: fn.Blocks[1]}] = true
	a.feasibleEdges[cfgEdge{from: fn.Blocks[1], to: fn.Blocks[2]}] = true

	reachable := a.getReachableBlocks(fn.Blocks[0])
	require.True(t, reachable[fn.Blocks[0]])
	require.False(t, reachable[fn.Blocks[1]], "non-executable block should not be reachable")
	require.False(t, reachable[fn.Blocks[2]], "block behind non-executable block should not be reachable")
}
