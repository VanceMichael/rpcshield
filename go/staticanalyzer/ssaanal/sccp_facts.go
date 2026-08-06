package ssaanal

import "golang.org/x/tools/go/ssa"

// =============================================================================
// Per-Block Nilness Facts (Assertion Propagation)
// =============================================================================
//
// This file contains functions for managing per-block nilness facts, which
// are used to track refined nilness information at different points in the CFG.
//
// At branch points (if err != nil), we propagate different facts to different
// successors:
//   - True branch: err is NonNil
//   - False branch: err is Nil
//
// At merge points (PHI nodes), we join facts from all predecessors using the
// nilness lattice join operation.
//
// =============================================================================

// getBlockFacts returns the nilness facts for a block, or nil if none.
func (a *analyzer) getBlockFacts(b *ssa.BasicBlock) map[ssa.Value]nilness {
	bs := a.getBlockState(b)
	return bs.nilFacts
}

// cloneFacts creates a deep copy of a fact map.
func (a *analyzer) cloneFacts(facts map[ssa.Value]nilness) map[ssa.Value]nilness {
	if facts == nil {
		return make(map[ssa.Value]nilness)
	}
	result := make(map[ssa.Value]nilness, len(facts))
	for k, v := range facts {
		result[k] = v
	}
	return result
}

// joinFactsInto merges incoming facts into existing in place and reports
// whether any entry was added or changed.
//
// Ownership is clear: the caller retains the same map throughout; no new map
// is allocated and no ambiguous "is this the same pointer?" question arises.
//
// existing must be non-nil (the caller initialises it on first visit via
// cloneFacts). incoming may be nil (no-op, returns false).
func (a *analyzer) joinFactsInto(existing, incoming map[ssa.Value]nilness) bool {
	if incoming == nil {
		return false
	}

	changed := false
	for v, factI := range incoming {
		if factE, ok := existing[v]; ok {
			joined := factE.join(factI)
			if joined != factE {
				existing[v] = joined
				changed = true
			}
		} else {
			existing[v] = factI
			changed = true
		}
	}
	return changed
}

// factsEqual checks if two fact maps are equal.
func (a *analyzer) factsEqual(facts1, facts2 map[ssa.Value]nilness) bool {
	if len(facts1) != len(facts2) {
		return false
	}
	for v, n1 := range facts1 {
		if n2, ok := facts2[v]; !ok || n1 != n2 {
			return false
		}
	}
	return true
}
