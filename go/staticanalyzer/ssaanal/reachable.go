package ssaanal

import (
	"golang.org/x/tools/go/ssa"
)

// computeSrcReachable computes the structural reachability set for srcBlock.
// Returns:
//   - reachable: R = transitive predecessors ∪ transitive successors ∪ {srcBlock}
//   - preds: transitive predecessors of srcBlock (including srcBlock itself)
//   - succs: transitive successors of srcBlock (including srcBlock itself)
//
// Any block not in R cannot participate in the Src's error flow.
// The preds set is used to force branch conditions toward srcBlock at blocks
// that are strictly before srcBlock (not in succs). In loops, srcBlock is its
// own successor via back-edges, so forcing is skipped and SCCP resolves
// conditions normally.
func computeSrcReachable(fn *ssa.Function, srcBlock *ssa.BasicBlock) (reachable []bool, preds []bool, succs []bool) {
	n := len(fn.Blocks)
	reachable = make([]bool, n)
	preds = make([]bool, n)
	succs = make([]bool, n)
	if srcBlock == nil {
		// No Src → all blocks allowed (set all true)
		for i := range reachable {
			reachable[i] = true
			preds[i] = true
			succs[i] = true
		}
		return reachable, preds, succs
	}
	reachable[srcBlock.Index] = true
	preds[srcBlock.Index] = true
	succs[srcBlock.Index] = true

	// BFS backward: transitive predecessors
	queue := []*ssa.BasicBlock{srcBlock}
	for len(queue) > 0 {
		b := queue[0]
		queue = queue[1:]
		for _, pred := range b.Preds {
			if !preds[pred.Index] {
				preds[pred.Index] = true
				reachable[pred.Index] = true
				queue = append(queue, pred)
			}
		}
	}

	// BFS forward: transitive successors
	queue = []*ssa.BasicBlock{srcBlock}
	for len(queue) > 0 {
		b := queue[0]
		queue = queue[1:]
		for _, succ := range b.Succs {
			if !succs[succ.Index] {
				succs[succ.Index] = true
				reachable[succ.Index] = true
				queue = append(queue, succ)
			}
		}
	}
	return reachable, preds, succs
}

// getReachableBlocks computes all blocks reachable from startBlock via feasible edges.
// Uses BFS over CFG successors, restricted to edges marked feasible by SCCP.
func (a *analyzer) getReachableBlocks(startBlock *ssa.BasicBlock) map[*ssa.BasicBlock]bool {
	reachable := make(map[*ssa.BasicBlock]bool)
	worklist := []*ssa.BasicBlock{startBlock}
	reachable[startBlock] = true

	for len(worklist) > 0 {
		block := worklist[0]
		worklist = worklist[1:]

		for _, succ := range block.Succs {
			if reachable[succ] || !a.isEdgeFeasible(block, succ) || !a.getBlockState(succ).executable {
				continue
			}
			reachable[succ] = true
			worklist = append(worklist, succ)
		}
	}
	return reachable
}
