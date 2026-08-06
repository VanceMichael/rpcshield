package cdg

// This file implements post-dominator tree and control dependence graph (CDG) construction
// for SSA control flow graphs.
//
// Post-dominator tree construction
// --------------------------------
// We use the algorithm described in Lengauer & Tarjan. 1979. A fast algorithm for finding
// dominators in a flowgraph. http://doi.acm.org/10.1145/357062.357071
//
// We also apply the optimizations to SLT described in Georgiadis et al, Finding Dominators
// in Practice, JGAA 2006, https://jgaa.info/index.php/jgaa/article/view/paper119/2847
// to avoid the need for buckets of size > 1.
//
// Control Dependence Graph construction
// -------------------------------------
// CDG is computed via postdominance frontiers:
//   1. Compute postdominators of all nodes
//   2. Build the postdominator tree
//   3. Compute the postdominance frontier (PDF) for each node X:
//      - For each successor S of X, walk up the postdominator tree from S
//        until (but not including) ipdom(X)
//      - Add these nodes to PDF(X)
//   4. Construct the CDG: for each node X, add control dependence edge X → Y
//      for each Y in PDF(X)

import (
	"sort"

	"golang.org/x/tools/go/ssa"
)

type BasicBlock = ssa.BasicBlock
type Function = ssa.Function

type CDGBuilder interface {
	Ipdom(b *BasicBlock) *BasicBlock
	PostDominees(b *BasicBlock) []*BasicBlock
	PostDominates(b, c *BasicBlock) bool
	PostdomPreorder(f *Function) []*BasicBlock
	PostdomPostorder(f *Function) []*BasicBlock
	BuildPostdomTree(f *Function)
	BuildCDG(f *Function) CDG
}

// CDG is a map of basic blocks to their control dependence successors.
type CDG map[*BasicBlock]map[*BasicBlock]struct{}

// cdgBuilderImpl holds post-dominator tree information for a function's control flow graph.
type cdgBuilderImpl struct {
	pdom      map[*BasicBlock]*domInfo
	exitDummy *BasicBlock // dummy exit block connecting all actual exit blocks
}

// isExitBlock returns true if the block has no successors.
func isExitBlock(b *BasicBlock) bool {
	return len(b.Succs) == 0
}

// NewCDGBuilder creates and initializes a new CDGBuilder instance.
func NewCDGBuilder() CDGBuilder {
	return &cdgBuilderImpl{
		pdom:      make(map[*BasicBlock]*domInfo),
		exitDummy: &BasicBlock{},
	}
}

func (cb *cdgBuilderImpl) getPDomInfo(b *BasicBlock) *domInfo {
	if di, ok := cb.pdom[b]; ok && di != nil {
		return di
	}
	di := &domInfo{}
	cb.pdom[b] = di
	return di
}

// Ipdom returns the block that immediately post-dominates b:
// its parent in the post-dominator tree, if any.
// Exit blocks and recover blocks have no parent and return nil.
func (cb *cdgBuilderImpl) Ipdom(b *BasicBlock) *BasicBlock { return cb.getPDomInfo(b).idom }

func (cb *cdgBuilderImpl) getIndex(b *BasicBlock) int { return cb.getPDomInfo(b).index }

// PostDominees returns the list of blocks that b immediately post-dominates:
// its children in the post-dominator tree.
func (cb *cdgBuilderImpl) PostDominees(b *BasicBlock) []*BasicBlock {
	return cb.getPDomInfo(b).children
}

// PostDominates reports whether b post-dominates c.
// Block b post-dominates c if all paths from c to any exit must pass through b.
func (cb *cdgBuilderImpl) PostDominates(b, c *BasicBlock) bool {
	return cb.getPDomInfo(b).pre <= cb.getPDomInfo(c).pre && cb.getPDomInfo(c).post <= cb.getPDomInfo(b).post
}

// PostdomPreorder returns a new slice containing the blocks of f in
// post-dominator tree preorder.
func (cb *cdgBuilderImpl) PostdomPreorder(f *Function) []*BasicBlock {
	order := make([]*BasicBlock, len(f.Blocks))
	copy(order, f.Blocks)
	sort.Slice(order, func(i, j int) bool {
		return cb.getPDomInfo(order[i]).pre < cb.getPDomInfo(order[j]).pre
	})
	return order
}

// PostdomPostorder returns a new slice containing the blocks of f in
// post-dominator tree postorder.
func (cb *cdgBuilderImpl) PostdomPostorder(f *Function) []*BasicBlock {
	order := make([]*BasicBlock, len(f.Blocks))
	copy(order, f.Blocks)
	sort.Slice(order, func(i, j int) bool {
		return cb.getPDomInfo(order[i]).post < cb.getPDomInfo(order[j]).post
	})
	return order
}

// domInfo contains a BasicBlock's post-dominance information.
type domInfo struct {
	idom      *BasicBlock   // immediate post-dominator (parent in post-dominator tree)
	children  []*BasicBlock // nodes immediately post-dominated by this one
	pre, post int32         // pre- and post-order numbering within post-dominator tree
	index     int           // basic block index
}

// ltState holds the working state for the Lengauer-Tarjan algorithm.
// During execution, domInfo.pre is repurposed for CFG DFS preorder numbering.
type ltState struct {
	// Each slice is indexed by block index.
	sdom     []*BasicBlock // semi-dominator of each block
	parent   []*BasicBlock // parent in DFS traversal of CFG
	ancestor []*BasicBlock // ancestor with least semi-dominator
}

// dfsPostdom implements the depth-first search part of the Lengauer-Tarjan algorithm.
// It traverses the *reverse* CFG (following predecessor edges) starting from the dummy exit node.
//
// Why reverse CFG? Post-dominators are computed by running the dominator algorithm on the
// reverse CFG. A node X post-dominates Y if X dominates Y in the reverse graph.
//
// Parameters:
//   - v: current block being visited
//   - i: current preorder number (DFS discovery order)
//   - preorder: slice to store blocks in DFS preorder
//
// Returns: the next available preorder number after visiting v and its descendants
//
// Side effects:
//   - Assigns DFS preorder numbers to each visited block (stored in domInfo.pre)
//   - Records the DFS parent of each block in lt.parent
//   - Initializes lt.sdom[v] to v (will be refined later)
//   - Calls lt.link to initialize path compression structures
func (lt *ltState) dfsPostdom(cb *cdgBuilderImpl, v *BasicBlock, i int32, preorder []*BasicBlock) int32 {
	// Record this block in the preorder traversal
	preorder[i] = v
	cb.getPDomInfo(v).pre = i // For now: DFS preorder of spanning tree of CFG
	i++

	vIdx := cb.getIndex(v)
	// Initialize semi-dominator to self (will be refined in step 2)
	lt.sdom[vIdx] = v

	// Initialize path compression structure (ancestor = nil initially)
	lt.link(cb, nil, v)

	// Traverse predecessors (reverse CFG edges)
	for _, w := range v.Preds {
		wIdx := cb.getIndex(w)
		// If w hasn't been visited yet (sdom is nil)
		if lt.sdom[wIdx] == nil {
			// w's parent in the DFS tree is v
			lt.parent[wIdx] = v
			// Recursively visit w
			i = lt.dfsPostdom(cb, w, i, preorder)
		}
	}
	return i
}

// eval implements the EVAL part of the Lengauer-Tarjan algorithm with path compression.
//
// Purpose: Given a node v, find the ancestor of v (along the path to the DFS root) that has
// the smallest semi-dominator number. This is used to efficiently compute semi-dominators.
//
// How it works:
//   - Walk up the ancestor chain from v toward the DFS root
//   - Track the node u with the minimum semi-dominator preorder number along this path
//   - This implements the "simple" version of path compression from the LT paper
//
// The path compression optimization (applied via link()) ensures that repeated eval calls
// on the same subtree are efficient by flattening the ancestor chains.
//
// Returns: the ancestor node with the smallest semi-dominator preorder number
func (lt *ltState) eval(cb *cdgBuilderImpl, v *BasicBlock) *BasicBlock {
	// u tracks the node with the smallest semi-dominator seen so far
	u := v
	uIdx := cb.getIndex(u)

	// Walk up the ancestor chain until we reach a node with no ancestor (the DFS root)
	for vIdx := cb.getIndex(v); lt.ancestor[vIdx] != nil; v, vIdx = lt.ancestor[vIdx], cb.getIndex(lt.ancestor[vIdx]) {
		// If v's semi-dominator is smaller (has lower preorder number) than u's,
		// update u to v
		if cb.getPDomInfo(lt.sdom[vIdx]).pre < cb.getPDomInfo(lt.sdom[uIdx]).pre {
			u = v
			uIdx = vIdx
		}
	}
	return u
}

// link implements the LINK part of the LT algorithm.
func (lt *ltState) link(cb *cdgBuilderImpl, v, w *BasicBlock) {
	wIdx := cb.getIndex(w)
	lt.ancestor[wIdx] = v
}

// collectExitBlocks returns all exit blocks in the function (blocks with no successors).
func collectExitBlocks(f *Function) []*BasicBlock {
	exitBlocks := make([]*BasicBlock, 0)
	for _, b := range f.Blocks {
		if isExitBlock(b) {
			exitBlocks = append(exitBlocks, b)
		}
	}
	return exitBlocks
}

// assignDummyExitNode creates a dummy exit node that connects all actual exit blocks.
// This gives us a single root for the reverse CFG traversal.
func (cb *cdgBuilderImpl) assignDummyExitNode(f *Function) *BasicBlock {
	cb.exitDummy = &BasicBlock{
		Preds: collectExitBlocks(f),
		Index: len(f.Blocks),
	}
	return cb.exitDummy
}

// initializeDomInfo initializes domInfo for all blocks in the function.
func (cb *cdgBuilderImpl) initializeDomInfo(f *Function, root *BasicBlock) {
	for _, b := range f.Blocks {
		cb.pdom[b] = &domInfo{
			index: b.Index,
		}
	}
	cb.pdom[root] = &domInfo{
		index: len(f.Blocks),
	}
}

// reachabilityResult holds the result of ensuring all blocks are reachable.
type reachabilityResult struct {
	noPathToExit map[*BasicBlock]struct{}
	preorder     []*BasicBlock
	lt           ltState
	space        []*BasicBlock
}

// ensureAllBlocksReachable iteratively adds unreachable blocks to the dummy exit
// until all blocks are reachable from the dummy exit. This handles infinite loops
// and blocks that can't reach any natural exit.
func (cb *cdgBuilderImpl) ensureAllBlocksReachable(f *Function, root *BasicBlock, n int) *reachabilityResult {
	noPathToExit := make(map[*BasicBlock]struct{})
	var preorder []*BasicBlock
	var space []*BasicBlock
	var lt ltState

	for {
		cb.initializeDomInfo(f, root)

		// Allocate space for 5 contiguous [n]*BasicBlock slices used by LT algorithm:
		// [0:n]     = sdom:     semi-dominator of each block
		// [n:2n]    = parent:   parent in DFS spanning tree
		// [2n:3n]   = ancestor: ancestor chain for path compression
		// [3n:4n]   = preorder: blocks in DFS preorder
		// [4n:5n]   = buckets:  deferred processing lists (optimization from Georgiadis et al.)
		space = make([]*BasicBlock, 5*n)
		lt = ltState{
			sdom:     space[0:n],
			parent:   space[n : 2*n],
			ancestor: space[2*n : 3*n],
		}

		// STEP 1: Number vertices by depth-first preorder on the reverse CFG.
		// Start from the dummy exit and traverse backwards through predecessor edges.
		preorder = space[3*n : 4*n]
		prenum := lt.dfsPostdom(cb, root, 0, preorder)

		// If all blocks were reached, we're done
		if prenum == int32(len(preorder)) {
			break
		}

		// Some blocks weren't reached by DFS from the dummy exit.
		// These are blocks in infinite loops or otherwise disconnected from exits.
		reachable := make(map[*BasicBlock]struct{}, prenum)
		for _, b := range preorder[0:prenum] {
			reachable[b] = struct{}{}
		}

		// Find one unreachable block and add it to the dummy exit's predecessors.
		// We only add one at a time, then retry DFS. This ensures correctness.
		for _, b := range f.Blocks {
			if _, ok := reachable[b]; !ok {
				noPathToExit[b] = struct{}{}
				root.Preds = append(root.Preds, b)
				break
			}
		}
	}

	return &reachabilityResult{
		noPathToExit: noPathToExit,
		preorder:     preorder,
		lt:           lt,
		space:        space,
	}
}

// computeDominators computes semi-dominators and immediate dominators using the LT algorithm.
// This processes nodes in reverse preorder (bottom-up) to compute semi-dominators and
// determine immediate dominators using the bucket optimization.
func (cb *cdgBuilderImpl) computeDominators(n int, preorder []*BasicBlock, lt *ltState, buckets []*BasicBlock, noPathToExit map[*BasicBlock]struct{}, root *BasicBlock) {
	// Process nodes in REVERSE preorder (bottom-up in the DFS tree).
	// This ensures we process children before parents, which is required for the algorithm.
	for i := int32(n) - 1; i > 0; i-- {
		w := preorder[i]

		// STEP 3a: Process deferred nodes in w's bucket.
		// These are nodes whose semi-dominator is w, and now we can compute their idom.
		for v := buckets[i]; v != w; v = buckets[cb.getPDomInfo(v).pre] {
			u := lt.eval(cb, v)
			uIdx := cb.getIndex(u)
			// If u's semi-dominator is an ancestor of w (has smaller preorder number),
			// then u is v's immediate dominator.
			// Otherwise, w is v's immediate dominator (or an ancestor of it).
			if cb.getPDomInfo(lt.sdom[uIdx]).pre < i {
				cb.getPDomInfo(v).idom = u
			} else {
				cb.getPDomInfo(v).idom = w
			}
		}

		// STEP 2: Compute semi-dominator of w.
		cb.computeSemidominator(w, lt, root, noPathToExit)

		wIdx := cb.getIndex(w)
		// Update path compression structure: link w to its parent
		lt.link(cb, lt.parent[wIdx], w)

		// STEP 3b: Determine if we can compute w's idom immediately or defer it.
		if lt.parent[wIdx] == lt.sdom[wIdx] {
			// Simple case: parent == semi-dominator, so parent is the immediate dominator
			cb.getPDomInfo(w).idom = lt.parent[wIdx]
		} else {
			// Defer processing: add w to its semi-dominator's bucket
			// We'll finalize w's idom when we process its semi-dominator
			sdomPre := cb.getPDomInfo(lt.sdom[wIdx]).pre
			buckets[i] = buckets[sdomPre]
			buckets[sdomPre] = w
		}
	}

	// STEP 3c: Process the bucket for the root (dummy exit).
	// Any remaining nodes in this bucket have the root as their immediate dominator.
	for v := buckets[0]; v != root; v = buckets[cb.getPDomInfo(v).pre] {
		cb.getPDomInfo(v).idom = root
	}
}

// computeSemidominator computes the semi-dominator of w.
// The semi-dominator is the node with the smallest preorder number that can
// reach w via a path of nodes (excluding endpoints) with preorder > w's preorder.
func (cb *cdgBuilderImpl) computeSemidominator(w *BasicBlock, lt *ltState, root *BasicBlock, noPathToExit map[*BasicBlock]struct{}) {
	wIdx := cb.getIndex(w)
	// Start with w's parent in the DFS tree as a candidate
	lt.sdom[wIdx] = lt.parent[wIdx]

	// Check all of w's successors in the CFG (predecessors in reverse CFG)
	for _, v := range w.Succs {
		u := lt.eval(cb, v)
		uIdx := cb.getIndex(u)
		// If u's semi-dominator is smaller, update w's semi-dominator
		if cb.getPDomInfo(lt.sdom[uIdx]).pre < cb.getPDomInfo(lt.sdom[wIdx]).pre {
			lt.sdom[wIdx] = lt.sdom[uIdx]
		}
	}

	// Special case: If w was in an infinite loop (added to dummy exit artificially),
	// also consider the dummy exit as a successor.
	if _, ok := noPathToExit[w]; ok {
		v := root
		u := lt.eval(cb, v)
		uIdx := cb.getIndex(u)
		if cb.getPDomInfo(lt.sdom[uIdx]).pre < cb.getPDomInfo(lt.sdom[wIdx]).pre {
			lt.sdom[wIdx] = lt.sdom[uIdx]
		}
	}
}

// finalizeIdoms finalizes immediate dominators and builds the tree structure.
// Process nodes in preorder to ensure parents are finalized before children.
func (cb *cdgBuilderImpl) finalizeIdoms(preorder []*BasicBlock, lt *ltState, root *BasicBlock) {
	for _, w := range preorder[1:] {
		if w == root {
			// The root (dummy exit) has no parent
			cb.getPDomInfo(w).idom = nil
		} else {
			wIdx := cb.getIndex(w)
			wInfo := cb.getPDomInfo(w)

			// If w's idom was deferred and points to something other than its semi-dominator,
			// it means the idom we computed was actually an ancestor of the true idom.
			// Follow the chain to find the true immediate dominator.
			if wInfo.idom != lt.sdom[wIdx] {
				wInfo.idom = cb.getPDomInfo(wInfo.idom).idom
			}

			// Build the children relation (inverse of idom).
			// Each node's children are the nodes it immediately post-dominates.
			idomInfo := cb.getPDomInfo(wInfo.idom)
			idomInfo.children = append(idomInfo.children, w)
		}
	}
}

// BuildPostdomTree computes the post-dominator tree of f using the Lengauer-Tarjan algorithm.
//
// Overview:
// Post-dominators are computed by applying the dominator algorithm to the *reverse* CFG.
// Node X post-dominates node Y if all paths from Y to any exit must pass through X.
//
// Algorithm steps (Lengauer-Tarjan):
//  1. Create a dummy exit node that connects all actual exit blocks
//  2. Perform DFS on the reverse CFG from the dummy exit, numbering nodes in preorder
//  3. Compute semi-dominators for each node (in reverse preorder)
//  4. Compute immediate dominators from semi-dominators
//  5. Build the post-dominator tree from immediate dominator relationships
//  6. Number the tree with pre/post-order for efficient queries
//
// Special handling:
//   - Blocks in infinite loops (no path to exit) are connected to the dummy exit
//   - The algorithm iterates until all blocks are reachable from the dummy exit
//
// Precondition: all blocks are reachable (e.g. optimizeBlocks has been run).
func (cb *cdgBuilderImpl) BuildPostdomTree(f *Function) {
	root := cb.assignDummyExitNode(f)

	// If there are no exit blocks, the function never returns (infinite loop).
	// Nothing to compute.
	if len(root.Preds) == 0 {
		return
	}

	n := len(f.Blocks) + 1 // +1 for a dummy exit node we create
	result := cb.ensureAllBlocksReachable(f, root, n)

	noPathToExit := result.noPathToExit
	preorder := result.preorder
	lt := result.lt
	space := result.space

	// Initialize buckets for deferred processing (optimization from Georgiadis et al.)
	buckets := space[4*n : 5*n]
	copy(buckets, preorder)

	// STEP 2 & 3: Compute semi-dominators and immediate dominators
	cb.computeDominators(n, preorder, &lt, buckets, noPathToExit, root)

	// STEP 4: Finalize immediate dominators and build the tree structure
	cb.finalizeIdoms(preorder, &lt, root)

	// STEP 5: Number the post-dominator tree with pre/post-order traversal numbers.
	// These numbers enable O(1) post-dominance queries via the interval test:
	// b post-dominates c iff b.pre <= c.pre && c.post <= b.post
	numberPostdomTree(cb, root, 0, 0)

	// STEP 6: Clean up the dummy exit node references.
	cb.cleanupDummyExit(f)
}

// cleanupDummyExit removes references to the dummy exit node.
// Set ipdom to nil for blocks that point to the dummy exit.
// This ensures exit blocks correctly report nil as their immediate post-dominator.
func (cb *cdgBuilderImpl) cleanupDummyExit(f *Function) {
	for _, b := range f.Blocks {
		if cb.getPDomInfo(b).idom == cb.exitDummy {
			cb.getPDomInfo(b).idom = nil
		}
	}
}

// numberPostdomTree sets the pre- and post-order numbers of a depth-first
// traversal of the post-dominator tree rooted at v. These are used to
// answer post-dominance queries in constant time.
func numberPostdomTree(cb *cdgBuilderImpl, v *BasicBlock, pre, post int32) (int32, int32) {
	cb.getPDomInfo(v).pre = pre
	pre++
	for _, child := range cb.getPDomInfo(v).children {
		pre, post = numberPostdomTree(cb, child, pre, post)
	}
	cb.getPDomInfo(v).post = post
	post++
	return pre, post
}

// BuildCDG constructs a Control Dependence Graph (CDG) for function f.
// The CDG is computed via postdominance frontiers. For each block X with successor S,
// all blocks on the path from S up the post-dominator tree (excluding ipdom(X))
// are control-dependent on X.
func (cb *cdgBuilderImpl) BuildCDG(f *Function) CDG {
	cb.BuildPostdomTree(f)

	// Initialize CDG adjacency map.
	cdg := make(CDG)
	for _, x := range f.Blocks {
		cdg[x] = make(map[*BasicBlock]struct{})
	}

	// Compute post-dominance frontier for each block.
	// For each block X with successor S, walk up the post-dominator tree from S
	// until reaching ipdom(X). All blocks on this path are control-dependent on X.
	for _, x := range f.Blocks {
		ipdomX := cb.Ipdom(x)
		for _, s := range x.Succs {
			current := s
			for current != nil && current != ipdomX && current != cb.exitDummy {
				cdg[x][current] = struct{}{}
				current = cb.Ipdom(current)
			}
		}
	}
	return cdg
}
