package ssaanal

import "golang.org/x/tools/go/ssa"

// =============================================================================
// Nil Check Types and Operations
// =============================================================================
//
// This file defines the types and operations for tracking nil checks in
// boolean conditions. When SCCP encounters a condition like `err != nil`,
// the boolean value's valState records what nil check it represents. This
// enables assertion propagation: after branching on the condition, the true
// and false successors can refine their nilness facts for the checked value.
//
// For compound conditions (AND/OR), multiple nil checks can be tracked
// together, with isConjunctive indicating AND vs OR semantics.
//
// =============================================================================

// nilCheckType indicates what kind of nil comparison a condition represents.
type nilCheckType int

const (
	notNilCheck nilCheckType = iota
	checkNeqNil              // err != nil
	checkEqNil               // err == nil
)

// nilCheckInfo bundles a set of nil checks with their logical connective.
// A zero-value nilCheckInfo (nil map) means "no nil check info".
type nilCheckInfo struct {
	checks      map[ssa.Value]nilCheckType
	conjunctive bool // true = AND, false = OR; only meaningful when len(checks) > 0
}

// flipCheckType returns the opposite check type.
// Used when propagating through boolean negation.
func flipCheckType(ct nilCheckType) nilCheckType {
	switch ct {
	case checkEqNil:
		return checkNeqNil
	case checkNeqNil:
		return checkEqNil
	default:
		return notNilCheck
	}
}

// isErrorBranch determines if the given successor index represents the error
// branch (the branch where the checked value is non-nil) for the given check.
func isErrorBranch(succIndex int, checkType nilCheckType) bool {
	isTrueBranch := (succIndex == 0)
	switch checkType {
	case checkNeqNil:
		return isTrueBranch // err != nil: true branch is error branch
	case checkEqNil:
		return !isTrueBranch // err == nil: false branch is error branch
	default:
		return false
	}
}

// =============================================================================
// Nil Check Map Operations
// =============================================================================
//
// These functions operate on maps of ssa.Value → nilCheckType, which track
// which values are being nil-checked in a boolean condition.
//
// =============================================================================

// cloneNilChecks creates a deep copy of a nil check map.
func cloneNilChecks(m map[ssa.Value]nilCheckType) map[ssa.Value]nilCheckType {
	if len(m) == 0 {
		return nil
	}
	result := make(map[ssa.Value]nilCheckType, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// flipNilChecks returns a new nilCheckInfo with all check types flipped (for boolean negation).
func flipNilChecks(info nilCheckInfo) nilCheckInfo {
	if len(info.checks) == 0 {
		return nilCheckInfo{}
	}
	flipped := make(map[ssa.Value]nilCheckType, len(info.checks))
	for k, v := range info.checks {
		flipped[k] = flipCheckType(v)
	}
	return nilCheckInfo{checks: flipped, conjunctive: info.conjunctive}
}

// unionNilChecks merges two nil check maps (for AND/OR operations).
// If the same value is checked differently in both maps, returns nil (contradiction).
func unionNilChecks(m1, m2 map[ssa.Value]nilCheckType) map[ssa.Value]nilCheckType {
	if len(m1) == 0 && len(m2) == 0 {
		return nil
	}
	if len(m1) == 0 {
		return cloneNilChecks(m2)
	}
	if len(m2) == 0 {
		return cloneNilChecks(m1)
	}

	result := make(map[ssa.Value]nilCheckType, len(m1)+len(m2))
	for k, v := range m1 {
		result[k] = v
	}
	for k, v := range m2 {
		if existing, ok := result[k]; ok && existing != v {
			return nil // Contradiction: err != nil && err == nil
		}
		result[k] = v
	}
	return result
}

// intersectNilChecks returns checks present in ALL infos with the same type (for phi merges).
// Also resolves the conjunctive flag: if all inputs agree, keep it; if they disagree, clear checks.
func intersectNilChecks(infos []nilCheckInfo) nilCheckInfo {
	if len(infos) == 0 {
		return nilCheckInfo{}
	}
	if len(infos) == 1 {
		return nilCheckInfo{
			checks:      cloneNilChecks(infos[0].checks),
			conjunctive: infos[0].conjunctive,
		}
	}

	result := cloneNilChecks(infos[0].checks)
	if len(result) == 0 {
		return nilCheckInfo{}
	}

	// Intersect the check maps
	for _, info := range infos[1:] {
		for k, v := range result {
			if mv, ok := info.checks[k]; !ok || mv != v {
				delete(result, k)
			}
		}
		if len(result) == 0 {
			return nilCheckInfo{}
		}
	}

	// Resolve conjunctive: only if all inputs agree
	allAgree := true
	firstVal := infos[0].conjunctive
	for _, info := range infos[1:] {
		if info.conjunctive != firstVal {
			allAgree = false
			break
		}
	}
	if !allAgree {
		return nilCheckInfo{}
	}
	return nilCheckInfo{checks: result, conjunctive: firstVal}
}

// nilChecksEqual checks if two nil check maps are equal.
func nilChecksEqual(m1, m2 map[ssa.Value]nilCheckType) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v := range m1 {
		if m2[k] != v {
			return false
		}
	}
	return true
}

// =============================================================================
// Path Attribution Verdict
// =============================================================================

// pathVerdict represents the outcome of walking one control dependence path
// during causal attribution (Phase 3).
type pathVerdict int

const (
	noAttribution    pathVerdict = iota // No relevant source found on this path
	srcAttribution                      // Src controls this return via error branch
	otherAttribution                    // A different source controls this path
)
