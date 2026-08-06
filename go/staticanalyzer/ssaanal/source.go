package ssaanal

import "golang.org/x/tools/go/ssa"

// =============================================================================
// Source Tracking
// =============================================================================
//
// This file defines types for tracking error sources during data flow analysis.
// Error sources are call instructions that return error values. By tracking
// which calls an error value is derived from, we can perform causal attribution
// to determine whether a returned error is attributed to a specific source.
//
// Example: In the code below, we track that wrappedErr is derived from both
// RPC1 and RPC2 calls:
//
//	_, err1 := RPC1()
//	_, err2 := RPC2()
//	wrappedErr := fmt.Errorf("wrap: %w", err1)  // Sources: {RPC1}
//	combinedErr := combineErrors(wrappedErr, err2)  // Sources: {RPC1, RPC2}
//
// =============================================================================

// sourceID uniquely identifies an error source (the error value from a call).
// This is the actual ssa.Value being tracked (Extract instruction for multi-return,
// or Call instruction itself for single-return error).
type sourceID ssa.Value

// sourceSet is a set of source IDs used to track which calls an error is derived from.
type sourceSet map[sourceID]struct{}

// newSourceSet creates an empty source set.
func newSourceSet() sourceSet {
	return make(sourceSet)
}

// add adds a source ID to the set.
func (s sourceSet) add(id sourceID) {
	s[id] = struct{}{}
}

// contains checks if a source ID is in the set.
func (s sourceSet) contains(id sourceID) bool {
	_, ok := s[id]
	return ok
}

// union adds all sources from another set to this set.
func (s sourceSet) union(other sourceSet) {
	for id := range other {
		s[id] = struct{}{}
	}
}

// isEmpty returns true if the set contains no sources.
func (s sourceSet) isEmpty() bool {
	return len(s) == 0
}

// len returns the number of sources in the set.
func (s sourceSet) len() int {
	return len(s)
}
