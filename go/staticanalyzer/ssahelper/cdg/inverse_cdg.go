package cdg

import "golang.org/x/tools/go/ssa"

// =============================================================================
// Inverse Control Dependence Graph
// =============================================================================
//
// This file provides utilities for traversing a Control Dependence Graph (CDG)
// in reverse - from controlled blocks back to their controlling blocks.
//
// The CDG maps: controller -> {dependents}
// The inverse CDG maps: dependent -> {(controller, succIndex)}
//
// This is useful for determining which conditions control a given block,
// and which branch of those conditions was taken to reach the block.
//
// =============================================================================

// ControllerEdge represents a controlling block and which successor edge
// leads toward the dependent block.
type ControllerEdge struct {
	Controller *ssa.BasicBlock
	SuccIndex  int // 0 for true branch, 1 for false branch (for If terminators)
}

// InverseCDG provides efficient lookup of controllers for any block.
// It inverts the CDG relationship and caches the result.
type InverseCDG struct {
	// inverse maps each block to its immediate controllers with edge info
	inverse map[*ssa.BasicBlock][]ControllerEdge
}

// NewInverseCDG builds an inverse CDG from a forward CDG.
// For each (controller -> dependent) relationship in the CDG, it determines
// which successor edge of the controller leads toward the dependent.
func NewInverseCDG(forwardCDG CDG) *InverseCDG {
	inverse := make(map[*ssa.BasicBlock][]ControllerEdge)

	for controller, dependents := range forwardCDG {
		for dependent := range dependents {
			succIdx := findSuccessorIndexToward(controller, dependent)
			inverse[dependent] = append(inverse[dependent], ControllerEdge{
				Controller: controller,
				SuccIndex:  succIdx,
			})
		}
	}

	return &InverseCDG{inverse: inverse}
}

// defaultMaxPaths is the default limit on CDG path enumeration to prevent
// exponential blow-up in functions with complex control flow.
const defaultMaxPaths = 10000

// GetControllerPaths returns all non-cyclic paths from the given block to its
// transitive controllers. Each path is ordered from innermost (closest to
// the given block) to outermost controller.
//
// This properly handles CDGs that form DAGs (multiple control paths to a block).
// Path enumeration is capped at 10,000 paths by default.
// Returns the paths and whether the limit was hit.
func (inv *InverseCDG) GetControllerPaths(block *ssa.BasicBlock) ([][]ControllerEdge, bool) {
	return inv.GetControllerPathsWithLimit(block, defaultMaxPaths)
}

// GetControllerPathsWithLimit is like GetControllerPaths but stops enumerating
// once maxPaths paths have been collected. A maxPaths of 0 means no limit.
// This prevents exponential blow-up in functions with deeply nested or
// diamond-shaped control flow.
// Returns the paths and whether the limit was hit.
func (inv *InverseCDG) GetControllerPathsWithLimit(block *ssa.BasicBlock, maxPaths int) ([][]ControllerEdge, bool) {
	var paths [][]ControllerEdge
	limited := false

	// visited and currentPath are reused across the whole traversal using
	// backtracking (mark on descent, unmark on return). This avoids copying
	// the visited set on every recursive call, which was O(depth) per node and
	// caused quadratic time and memory blow-up on large control-flow graphs.
	visited := make(map[*ssa.BasicBlock]bool)
	var currentPath []ControllerEdge

	// savePath records a copy of the current path and updates the limit flag.
	savePath := func() {
		if len(currentPath) == 0 {
			return
		}
		pathCopy := make([]ControllerEdge, len(currentPath))
		copy(pathCopy, currentPath)
		paths = append(paths, pathCopy)
		if maxPaths > 0 && len(paths) >= maxPaths {
			limited = true
		}
	}

	var findPaths func(*ssa.BasicBlock)
	findPaths = func(b *ssa.BasicBlock) {
		immediateControllers := inv.inverse[b]

		if len(immediateControllers) == 0 {
			// Reached a root (no controllers) - save this path if non-empty.
			savePath()
			return
		}

		for _, edge := range immediateControllers {
			if limited {
				return
			}
			// Check for cycles.
			if visited[edge.Controller] {
				// Found a cycle - save current path and stop extending.
				savePath()
				continue
			}

			// Mark this controller as visited, extend the path, and recurse.
			// Undo both on return so siblings see the correct state.
			visited[edge.Controller] = true
			currentPath = append(currentPath, edge)

			findPaths(edge.Controller)

			currentPath = currentPath[:len(currentPath)-1]
			delete(visited, edge.Controller)
		}
	}

	findPaths(block)
	return paths, limited
}

// GetImmediateControllers returns the immediate controllers of a block
// (not transitive). Returns nil if the block has no controllers.
func (inv *InverseCDG) GetImmediateControllers(block *ssa.BasicBlock) []ControllerEdge {
	return inv.inverse[block]
}

// findSuccessorIndexToward determines which successor of controller leads toward target.
// Returns the successor index (0 for first successor, 1 for second, etc.)
func findSuccessorIndexToward(controller *ssa.BasicBlock, target *ssa.BasicBlock) int {
	// Check if target is a direct successor
	for i, succ := range controller.Succs {
		if succ == target {
			return i
		}
	}

	// Target is not a direct successor, use dominance to determine path
	for i, succ := range controller.Succs {
		if succ.Dominates(target) {
			return i
		}
	}

	// Fallback: return 0 (shouldn't happen in well-formed CDG)
	return 0
}
