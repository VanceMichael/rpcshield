package ssaanal

import (
	"golang.org/x/tools/go/ssa"
)

// =============================================================================
// Value State
// =============================================================================

// valState holds the abstract state for an SSA value.
type valState struct {
	// Unified lattice value: nilness for nillable types, constBool for booleans, nil for others.
	// A nil lattice means bottom (not yet computed) for any type.
	lattice latticeVal

	// For boolean values: tracks what nil checks (if any) this boolean MAY represent
	// and whether they form a conjunction (AND) or disjunction (OR).
	// Examples:
	//   - err != nil:                     {err: checkNeqNil}, conjunctive=true
	//   - err1 != nil && err2 != nil:     {err1: checkNeqNil, err2: checkNeqNil}, conjunctive=true
	//   - err1 != nil || err2 != nil:     {err1: checkNeqNil, err2: checkNeqNil}, conjunctive=false
	//   - !(err == nil):                  {err: checkNeqNil}, conjunctive=true
	// Zero value (nil checks map) means this boolean doesn't represent nil checks.
	nilInfo nilCheckInfo

	// Error sources this value is derived from (for error attribution).
	dataSources sourceSet

	// Tracks if created by errors.New/fmt.Errorf (NonNil, no sources).
	isSynthetic synthetic
}

// getNilness returns the nilness component of the lattice, or nlBottom if not a nilness value.
func (vs *valState) getNilness() nilness {
	if n, ok := vs.lattice.(nilness); ok {
		return n
	}
	return nlBottom
}

// getBoolState returns the constBool component of the lattice, or boolBottom if not a bool value.
func (vs *valState) getBoolState() constBool {
	if b, ok := vs.lattice.(constBool); ok {
		return b
	}
	return boolBottom
}

// newValState creates a new value state initialized to bottom (not yet computed).
func newValState() *valState {
	return &valState{
		dataSources: newSourceSet(),
		isSynthetic: synthBottom,
	}
}

// valSnapshot captures the parts of valState needed for change detection.
type valSnapshot struct {
	lattice     latticeVal
	srcCount    int
	isSynthetic synthetic
	nilChecks   map[ssa.Value]nilCheckType // raw map for change detection
	conjunctive bool                       // track conjunctive for exhaustive change detection
}

// snapshot captures the current state for later change detection.
func (vs *valState) snapshot() valSnapshot {
	return valSnapshot{
		lattice:     vs.lattice,
		srcCount:    vs.dataSources.len(),
		isSynthetic: vs.isSynthetic,
		nilChecks:   cloneNilChecks(vs.nilInfo.checks),
		conjunctive: vs.nilInfo.conjunctive,
	}
}

// changed reports whether the value state has changed since the snapshot.
func (vs *valState) changed(snap valSnapshot) bool {
	return vs.lattice != snap.lattice ||
		vs.dataSources.len() != snap.srcCount ||
		vs.isSynthetic != snap.isSynthetic ||
		vs.nilInfo.conjunctive != snap.conjunctive ||
		!nilChecksEqual(vs.nilInfo.checks, snap.nilChecks)
}

// =============================================================================
// Block State
// =============================================================================

// blkState holds the abstract state for a basic block.
type blkState struct {
	executable bool // Block is reachable under Src != nil assumption

	// nilFacts holds nilness facts that are true at the ENTRY of this block.
	// These are refined based on branch predicates from predecessors.
	// This enables assertion propagation: after `if err != nil { return }`,
	// the fall-through block knows `err` is Nil.
	nilFacts map[ssa.Value]nilness
}

// newBlkState creates a new block state initialized to not executable with no facts.
func newBlkState() *blkState {
	return &blkState{
		executable: false,
		nilFacts:   nil, // Lazily initialized
	}
}
