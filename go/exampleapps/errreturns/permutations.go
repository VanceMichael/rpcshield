package main

import (
	"errors"
	"fmt"
)

// Helper variables for conditional logic
var cond, cond2, cond3 bool

// =============================================================================
// Comprehensive Test Suite for SCCP Variant Algorithm
// =============================================================================
//
// Goal: Test diverse control flow, nesting, wrapping, and nilness scenarios
// Focus: Complex cases that truly test the algorithm's edge cases
//
// Analyzing: err2 (anRPCCall) as SOURCE
// OtherSources: err1, err3 (someCall)

// =============================================================================
// Group 1: Basic FailClose - Sanity Checks
// =============================================================================

//go:noinline
func analyzeErrorPermute01_BasicFailClose() (string, error) {
	// Baseline: err2 checked and returned directly
	s, err2 := anRPCCall()
	if err2 != nil {
		return s, err2
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute02_WrappedErr2FailClose() (string, error) {
	// err2 wrapped in error branch
	s, err2 := anRPCCall()
	if err2 != nil {
		return s, fmt.Errorf("wrapped: %w", err2)
	}
	return s, nil
}

// =============================================================================
// Group 2: Success Branch Checks (== nil)
// =============================================================================

//go:noinline
func analyzeErrorPermute03_SuccessBranchReturnErr2FailClose() (string, error) {
	// Check err2 == nil in success branch, return err2 at end
	s, err2 := anRPCCall()
	if err2 == nil {
		// Success path, continue
		_ = s
	}
	return s, err2 // Could be nil or non-nil
}

//go:noinline
func analyzeErrorPermute04_SuccessBranchWithOtherSourceFailClose() (string, error) {
	// Success branch has other error, but err2 returned at end
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 == nil {
		// In success branch, check another error
		s, err3 := someCall()
		if err3 != nil {
			return s, err3
		}
	}
	return s, err2 // err1 is nil, err2 could be non-nil, err3 only exists if err2 was nil
}

// =============================================================================
// Group 3: Complex Nesting - FailClose
// =============================================================================

//go:noinline
func analyzeErrorPermute05_DeepNestingErr2OutermostFailClose() (string, error) {
	// err2 is outermost, controls everything inside
	s, err2 := anRPCCall()
	if err2 != nil {
		s, err1 := someCall()
		if err1 != nil {
			s, err3 := someCall()
			if err3 != nil {
				return s, err3 // Transitively controlled by err2
			}
			return s, err1 // Transitively controlled by err2
		}
		return s, err2
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute06_NestedMixedChecksFailClose() (string, error) {
	// Mix of != nil and == nil checks
	s, err1 := someCall()
	if err1 == nil { // Success branch
		var err2 error
		s, err2 = anRPCCall()
		if err2 != nil { // Error branch
			var err3 error
			s, err3 = someCall()
			if err3 == nil { // Success branch
				return s, err2 // err2 controls this
			}
			return s, err3
		}
	}
	return s, err1
}

// =============================================================================
// Group 4: Wrapped Errors with Multiple Sources - FailClose
// =============================================================================

//go:noinline
func analyzeErrorPermute07_WrapErr2WithNilOtherFailClose() (string, error) {
	// Wrap err2 with err1 that's proven nil
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		// err1 is nil here, wrapping with it still attributes to err2
		return s, fmt.Errorf("err1=%v err2=%w", err1, err2)
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute08_WrapBothErr2AndOtherFailClose() (string, error) {
	// Wrap both err2 and err3, but err3 is nil
	s, err2 := anRPCCall()
	if err2 != nil {
		s, err3 := someCall()
		if err3 != nil {
			return s, err3
		}
		// err3 is nil, wrapping both - err2 is the only source
		return s, fmt.Errorf("e2=%w e3=%v", err2, err3)
	}
	return s, nil
}

// =============================================================================
// Group 5: Multiple Sources - (Competing Claims)
// =============================================================================

//go:noinline
func analyzeErrorPermute09_WrapErr2WithNonNilOtherFailClose() (string, error) {
	// Wrap err2 with err1 that might be non-nil
	s, err1 := someCall()
	// No check for err1!
	s, err2 := anRPCCall()
	if err2 != nil {
		// err1 might be non-nil, wrapping both - ambiguous
		return s, fmt.Errorf("both: e1=%v e2=%w", err1, err2)
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute10_BothErrorsInConditionFailClose() (string, error) {
	// Condition depends on both err1 and err2
	s, err1 := someCall()
	s, err2 := anRPCCall()
	// Both unchecked, both in condition
	if err1 != nil || err2 != nil {
		return s, errors.New("one of them failed")
	}
	return s, nil
}

// =============================================================================
// Group 6: Suppression - FailOpen
// =============================================================================

//go:noinline
func analyzeErrorPermute11_ReturnNilWhenErr2NonNilFailOpen() (string, error) {
	// Suppress err2 by returning nil
	s, err2 := anRPCCall()
	if err2 != nil {
		return s, nil // Suppression!
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute12_ReturnOtherNilErrorFailOpen() (string, error) {
	// Return err1 (which is nil) when err2 != nil
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		return s, err1 // err1 is nil, this is suppression
	}
	return s, nil
}

// =============================================================================
// Group 7: Reassignment - FailOpen
// =============================================================================

//go:noinline
func analyzeErrorPermute13_ReassignErr2ToOtherFailOpen() (string, error) {
	// Reassign err2 variable, breaking tracking
	s, err2 := anRPCCall()
	s, err3 := someCall()
	err2 = err3 // Break tracking!
	if err2 != nil {
		return s, err2 // This is err3, not original err2
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute14_ReassignInBranchFailClose() (string, error) {
	// Reassign in a branch
	s, err2 := anRPCCall()
	if err2 != nil {
		err2 = errors.New("replaced") // Different error
		return s, err2
	}
	return s, nil
}

// =============================================================================
// Group 8: Complex Control Flow with Multiple Claimers
// =============================================================================

//go:noinline
func analyzeErrorPermute15_MultipleReturnPathsFailClose() (string, error) {
	// Multiple paths, all returning err2 or controlled by err2
	s, err1 := someCall()
	if err1 != nil {
		s, err2 := anRPCCall()
		if err2 != nil {
			return s, err2
		}
		// err2 is nil in this branch, but still under err1 control
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		return s, err2
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute16_NestedWithCompetingSourcesFailClose() (string, error) {
	// err1 and err2 both non-nil, return synthetic
	s, err1 := someCall()
	if err1 != nil {
		s, err2 := anRPCCall()
		if err2 != nil {
			// Both are non-nil, synthetic controlled by both
			return s, errors.New("both failed")
		}
	}
	return s, nil
}

// =============================================================================
// Group 9: Success Branch Complexity
// =============================================================================

//go:noinline
func analyzeErrorPermute17_SuccessThenErrorFailClose() (string, error) {
	// Success check first, then error check
	s, err2 := anRPCCall()
	if err2 == nil {
		s, err3 := someCall()
		if err3 != nil {
			return s, err3 // Not err2, but err2 is nil
		}
	} else {
		return s, err2 // Error branch
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute18_DoubleSuccessCheckFailClose() (string, error) {
	// Two success checks in a row
	s, err1 := someCall()
	if err1 == nil {
		s, err2 := anRPCCall()
		if err2 == nil {
			return s, nil
		}
		return s, err2 // err2 != nil, err1 == nil
	}
	return s, err1 // err1 != nil
}

// =============================================================================
// Group 10: Wrapped with Competing Sources
// =============================================================================

//go:noinline
func analyzeErrorPermute19_WrapWithUncheckedSourceFailClose() (string, error) {
	// Wrap err2 with err3 that's not checked for nil
	s, err2 := anRPCCall()
	if err2 != nil {
		_, err3 := someCall()
		// err3 not checked! Could be non-nil
		return s, fmt.Errorf("e2=%w e3=%v", err2, err3)
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute20_MultiWrapAmbiguousFailClose() (string, error) {
	// Multiple unchecked errors wrapped together
	s, err1 := someCall()
	s, err2 := anRPCCall()
	s, err3 := someCall()
	// All unchecked, all in wrap
	if err2 != nil {
		return s, fmt.Errorf("all: %v %w %v", err1, err2, err3)
	}
	return s, nil
}

// =============================================================================
// Group 11: Deep Nesting with Mixed Results
// =============================================================================

//go:noinline
func analyzeErrorPermute21_FourLevelNestingFailClose() (string, error) {
	// Four levels: err1 { err2 { err3 { err4 } } }
	s, err1 := someCall()
	if err1 != nil {
		s, err2 := anRPCCall()
		if err2 != nil {
			s, err3 := someCall()
			if err3 != nil {
				s, err4 := someCall()
				if err4 != nil {
					return s, err4 // Transitively controlled by err2
				}
				return s, err3
			}
			return s, err2
		}
		return s, err1
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute22_DiamondControlFlowFailClose() (string, error) {
	// Diamond: err2 branches, then merges, then return
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		s = s + "A"
	} else {
		s = s + "B"
	}
	return s, err2 // Could be nil or non-nil, but checked
}

// =============================================================================
// Group 12: Early Returns Breaking Attribution
// =============================================================================

//go:noinline
func analyzeErrorPermute23_EarlyReturnBeforeErr2FailClose() (string, error) {
	// Early return that prevents err2 check
	s, err1 := someCall()
	if err1 == nil {
		return s, nil // Early return, err2 not created
	}
	s, err2 := anRPCCall()
	// err2 never checked!
	return s, err2
}

//go:noinline
func analyzeErrorPermute24_ConditionalErr2CreationFailClose() (string, error) {
	// err2 only created in some paths
	s, err1 := someCall()
	if err1 != nil {
		s, err2 := anRPCCall()
		if err2 != nil {
			return s, err2
		}
	}
	return s, nil
}

// =============================================================================
// Group 13: Multiple Source Attribution
// =============================================================================

//go:noinline
func analyzeErrorPermute25_AllSourcesNilReturnSyntheticFailClose() (string, error) {
	// All sources proven nil, synthetic not controlled by any
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		return s, err2
	}
	s, err3 := someCall()
	if err3 != nil {
		return s, err3
	}
	// All nil, synthetic has no controller
	return s, errors.New("unexpected state")
}

//go:noinline
func analyzeErrorPermute26_OnlyErr2ControlsSyntheticFailClose() (string, error) {
	// err2 controls synthetic, others proven nil
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		s, err3 := someCall()
		if err3 != nil {
			return s, err3
		}
		// err3 nil, synthetic controlled by err2
		return s, errors.New("err2 active")
	}
	return s, nil
}

// =============================================================================
// Group 14: Mixed Success/Error Branches
// =============================================================================

//go:noinline
func analyzeErrorPermute27_AlternatingChecksFailClose() (string, error) {
	// != nil, == nil, != nil pattern
	s, err1 := someCall()
	if err1 != nil {
		s, err2 := anRPCCall()
		if err2 == nil {
			s, err3 := someCall()
			if err3 != nil {
				return s, err3 // err2 is nil
			}
		} else {
			return s, err2 // err2 is non-nil
		}
		return s, err1
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute28_SuccessBranchOnlyPathFailClose() (string, error) {
	// Only path to return is through success branch
	s, err2 := anRPCCall()
	if err2 == nil {
		s, err3 := someCall()
		if err3 != nil {
			return s, err3 // err2 is nil, err3 claims
		}
		return s, nil
	}
	// err2 != nil has no return!
	return s, errors.New("unreachable?")
}

// =============================================================================
// Group 15: Fall-through and Multiple Paths
// =============================================================================

//go:noinline
func analyzeErrorPermute29_FallThroughToErr2ReturnFailClose() (string, error) {
	// Fall through from checks, return err2 at end
	s, err1 := someCall()
	if err1 != nil {
		// Fall through
		_ = s
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		// Fall through
		_ = s
	}
	return s, err2 // Could be non-nil
}

//go:noinline
func analyzeErrorPermute30_PartialFallThroughFailOpen() (string, error) {
	// Some paths return, some fall through
	s, err1 := someCall()
	if err1 != nil {
		return s, err1 // Return
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		// Fall through! err2 escapes unchecked
		_ = s
	}
	return s, nil
}

// =============================================================================
// Group 16: Convoluted FailClose - Multi-Layer Wrapping
// =============================================================================

//go:noinline
func analyzeErrorPermute31_TripleWrapFailClose() (string, error) {
	// Wrap err2 multiple times in sequence
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		wrap1 := fmt.Errorf("layer1: %w", err2)
		wrap2 := fmt.Errorf("layer2: %w", wrap1)
		return s, fmt.Errorf("layer3: %w", wrap2)
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute32_ConditionalWrappingDepthFailClose() (string, error) {
	// Different wrapping depth based on other conditions
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		s, err3 := someCall()
		if err3 != nil {
			// Deep wrap when err3 is also non-nil
			return s, fmt.Errorf("deep: %w (context: %v)", err2, err3)
		}
		// Shallow wrap when err3 is nil
		return s, fmt.Errorf("shallow: %w", err2)
	}
	return s, nil
}

// =============================================================================
// Group 17: Convoluted FailClose - Complex PHI and Merges
// =============================================================================

//go:noinline
func analyzeErrorPermute33_MultiPathMergeFailClose() (string, error) {
	// Multiple paths merge before returning err2
	s, err1 := someCall()
	var marker string
	if err1 != nil {
		marker = "path1"
	} else {
		marker = "path2"
	}
	s, err2 := anRPCCall()
	if err2 != nil {
		if marker == "path1" {
			s = s + "A"
		} else {
			s = s + "B"
		}
		return s, err2 // err2 from merged control flow
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute34_NestedDiamondFailClose() (string, error) {
	// Diamond within diamond
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}

	s, err2 := anRPCCall()
	var temp string
	if err2 != nil {
		// Inner diamond
		s, err3 := someCall()
		if err3 != nil {
			temp = "inner-true"
		} else { //nolint:redundantelse
			temp = "inner-false"
		}
		return s, fmt.Errorf("%s: %w", temp, err2)
	} else {
		// Outer diamond false branch
		temp = "outer-false"
	}

	if temp != "" {
		s = s + temp
	}
	return s, err2
}

// =============================================================================
// Group 18: Convoluted FailClose - Asymmetric Nesting
// =============================================================================

//go:noinline
func analyzeErrorPermute35_AsymmetricDepthFailClose() (string, error) {
	// Different nesting depths in different branches
	s, err1 := someCall()
	if err1 != nil {
		// Shallow path
		s, err2 := anRPCCall()
		if err2 != nil {
			return s, err2
		}
		return s, err1
	}

	// Deep path
	s, err2 := anRPCCall()
	if err2 != nil {
		s, err3 := someCall()
		if err3 != nil {
			s, err4 := someCall()
			if err4 != nil {
				s, err5 := someCall()
				if err5 != nil {
					return s, err5
				}
				return s, fmt.Errorf("deep: %w", err2)
			}
			return s, err2
		}
		return s, err2
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute36_MultiExitPointsFailClose() (string, error) {
	// Many exit points, all controlled by err2
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}

	s, err2 := anRPCCall()
	if err2 != nil {
		s, err3 := someCall()
		switch {
		case err3 != nil:
			return s, err3
		case len(s) > 10:
			return s, fmt.Errorf("long: %w", err2)
		case len(s) > 5:
			return s, fmt.Errorf("medium: %w", err2)
		default:
			return s, err2
		}
	}
	return s, nil
}

// =============================================================================
// Group 19: Convoluted FailClose - Loop-like Patterns
// =============================================================================

//go:noinline
func analyzeErrorPermute37_SequentialChecksFailClose() (string, error) {
	// Sequential checks that look like a loop unrolling
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}

	s, err2 := anRPCCall()
	if err2 == nil {
		s, err3 := someCall()
		if err3 == nil {
			s, err4 := someCall()
			if err4 == nil {
				return s, nil
			}
			return s, err4
		}
		return s, err3
	}
	return s, err2
}

//go:noinline
func analyzeErrorPermute38_AccumulatingContextFailClose() (string, error) {
	// Build up context through multiple checks
	s, err1 := someCall()
	ctx := ""
	if err1 != nil {
		ctx = ctx + "[err1]"
		return s, fmt.Errorf("%s: %w", ctx, err1)
	}

	s, err2 := anRPCCall()
	if err2 != nil {
		ctx = ctx + "[err2]"
		s, err3 := someCall()
		if err3 != nil {
			ctx = ctx + "[err3]"
			return s, fmt.Errorf("%s: %w", ctx, err3)
		}
		return s, fmt.Errorf("%s: %w", ctx, err2)
	}
	return s, nil
}

// =============================================================================
// Group 20: Convoluted FailClose - Complex Assertions
// =============================================================================

//go:noinline
func analyzeErrorPermute39_DoubleNegationFailClose() (string, error) {
	// Use double negation for control flow
	s, err1 := someCall()
	if err1 == nil {
		if !(err1 != nil) {
			s, err2 := anRPCCall()
			if !(err2 == nil) {
				return s, err2
			}
		}
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute40_ChainedSuccessChecksFailClose() (string, error) {
	// Chain multiple == nil checks
	s, err1 := someCall()
	if err1 == nil {
		s, err2 := anRPCCall()
		if err2 == nil {
			s, err3 := someCall()
			if err3 == nil {
				return s, nil
			} else { //nolint:redundantelse
				return s, err3
			}
		} else { //nolint:redundantelse
			// err2 != nil, err1 == nil
			return s, err2
		}
	}
	return s, err1
}

// =============================================================================
// Group 21: Convoluted - Partial Suppression
// =============================================================================

//go:noinline
func analyzeErrorPermute41_ConditionalSuppressionFailClose() (string, error) {
	// Suppress err2 in some branches, not others
	s, err1 := someCall()
	s, err2 := anRPCCall()

	if err1 != nil {
		if err2 != nil {
			return s, nil // Suppression when both fail
		}
		return s, err1
	}

	if err2 != nil {
		return s, err2 // Return err2 when only err2 fails
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute42_PartialPathSuppressionFailClose() (string, error) {
	// One path suppresses, another doesn't
	s, err1 := someCall()
	s, err2 := anRPCCall()

	if err1 != nil {
		// Path 1: suppress err2
		if err2 != nil {
			s = s + "suppressed"
		}
		return s, err1
	}

	// Path 2: might return err2
	if err2 != nil {
		s, err3 := someCall()
		if err3 != nil {
			return s, nil // Suppress both
		}
		return s, err2
	}
	return s, nil
}

// =============================================================================
// Group 22: Convoluted FailOpen - Complex Reassignment
// =============================================================================

//go:noinline
func analyzeErrorPermute43_MultipleReassignmentsFailOpen() (string, error) {
	// Reassign err2 multiple times
	s, err2 := anRPCCall()
	s, err3 := someCall()
	s, err4 := someCall()

	if err2 != nil {
		err2 = err3 // First reassignment
		if err3 != nil {
			err2 = err4 // Second reassignment
		}
		return s, err2 // Completely lost track
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute44_ConditionalReassignmentFailOpen() (string, error) {
	// Reassign based on complex conditions
	s, err1 := someCall()
	s, err2 := anRPCCall()
	s, err3 := someCall()

	if err2 != nil {
		if len(s) > 5 {
			err2 = err1 // Reassign to err1
		} else {
			err2 = err3 // Reassign to err3
		}
		return s, err2 // Could be err1, err2, or err3
	}
	return s, nil
}

// =============================================================================
// Group 23: Convoluted - Scope and Shadowing
// =============================================================================

//go:noinline
func analyzeErrorPermute45_NestedScopeShadowingFailClose() (string, error) {
	// Shadow err2 in nested scope
	s, err2 := anRPCCall()

	if err2 != nil {
		s, err1 := someCall()
		if err1 != nil {
			// Shadow err2 with a new error
			err2 := errors.New("shadowed")
			return s, err2 // This is the shadowed err2, not original
		}
		return s, err2 // Original err2
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute46_MultipleScopeErr2FailClose() (string, error) {
	// Different err2 in different scopes
	s, err1 := someCall()

	if err1 != nil {
		s, err2 := anRPCCall() // err2 in scope 1
		if err2 != nil {
			return s, err2
		}
	} else {
		s, err2 := someCall() // Different err2 in scope 2
		if err2 != nil {
			return s, err2 // This is not anRPCCall!
		}
	}
	return s, nil
}

// =============================================================================
// Group 24: Convoluted - Delayed and Interleaved Checks
// =============================================================================

//go:noinline
func analyzeErrorPermute47_DelayedCheckFailClose() (string, error) {
	// Check err2 much later after creation
	s, err1 := someCall()
	s, err2 := anRPCCall()
	s, err3 := someCall()
	s, err4 := someCall()

	// Do lots of work before checking err2
	if err1 != nil {
		return s, err1
	}
	if err3 != nil {
		return s, err3
	}
	if err4 != nil {
		return s, err4
	}

	// Finally check err2, but it might have been affected by something
	if err2 != nil {
		return s, err2
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute48_InterleavedCreationCheckFailClose() (string, error) {
	// Create and check errors in interleaved pattern
	s, err1 := someCall()
	if err1 != nil {
		s, err2 := anRPCCall()
		s, err3 := someCall()
		if err3 != nil {
			// Check err3 before err2
			return s, err3
		}
		s, err4 := someCall()
		if err2 != nil {
			// Check err2 after creating err4
			s, err5 := someCall()
			if err5 != nil {
				return s, err5
			}
			return s, err2
		}
		return s, err4
	}
	return s, nil
}

// =============================================================================
// Group 25: Convoluted  - Multi-Source Ambiguity
// =============================================================================

//go:noinline
func analyzeErrorPermute49_ThreeWayAmbiguityFailClose() (string, error) {
	// Three unchecked sources in complex condition
	s, err1 := someCall()
	s, err2 := anRPCCall()
	s, err3 := someCall()

	// Complex condition with all three
	if (err1 != nil && err2 != nil) || err3 != nil {
		return s, errors.New("complex failure")
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute50_NestedAmbiguousWrappingFailClose() (string, error) {
	// Nested wrapping with ambiguous sources
	s, err1 := someCall()
	s, err2 := anRPCCall()

	if err1 != nil {
		s, err3 := someCall()
		if err2 != nil {
			// err1 is non-nil, err2 is non-nil, err3 unchecked
			wrap1 := fmt.Errorf("inner: e2=%w e3=%v", err2, err3)
			return s, fmt.Errorf("outer: e1=%v wrapped=%w", err1, wrap1)
		}
	}
	return s, nil
}

//go:noinline
func analyzeErrorPermute51_PhiMergeSuppressionFailOpen() (string, error) {
	// PHI merge where err2 is reassigned to nil in error branch
	// This should be FailOpen because when err2 != nil, we suppress it by returning nil
	resp2, err2 := anRPCCall()
	if err2 != nil {
		err2 = nil // Reassign to nil - suppression!
	}
	// Return statement has PHI: phi[original_err2_if_nil, nil_from_error_branch]
	// When err2 != nil originally, we return nil (suppression)
	return resp2, err2
}

//go:noinline
func analyzeErrorPermute52_UnconditionalReassignNilFailOpen() (string, error) {
	// Unconditional reassignment to nil - always suppresses error
	// Based on testErrorAssignment4
	resp, err := anRPCCall()
	err = nil // Always nil
	return resp, err
}

//go:noinline
func analyzeErrorPermute53_UnconditionalReassignNewErrFailOpen() (string, error) {
	// Unconditional reassignment to new error - loses original source
	// Based on testErrorAssignment5
	resp, err := anRPCCall()
	err = errors.New("new error") // Always different error - data source lost
	return resp, err
}

//go:noinline
func analyzeErrorPermute54_IfElseBothReassignFailOpen() (string, error) {
	// Both branches reassign err - both lose original source
	// Based on testErrorAssignment3
	resp, err := anRPCCall()
	if cond {
		err = nil
	} else {
		err = errors.New("error")
	}
	return resp, err
}

//go:noinline
func analyzeErrorPermute55_ErrBranchReassignNewErrFailClose() (string, error) {
	// When err != nil, reassign to new error (still non-nil)
	// Based on testErrorAssignment15
	resp, err := anRPCCall()
	if err != nil {
		err = errors.New("error")
	}
	return resp, err
}

//go:noinline
func analyzeErrorPermute56_DoubleAssignNewThenNilFailOpen() (string, error) {
	// Double assignment: first to error, then to nil
	// Based on testErrorAssignment18
	resp, err := anRPCCall()
	err = errors.New("error")
	err = nil // Final value is nil
	return resp, err
}

//go:noinline
func analyzeErrorPermute57_ConditionalNewErrThenUnconditionalNilFailOpen() (string, error) {
	// If err != nil: err = errors.New; then unconditionally err = nil
	// Based on testErrorAssignment19
	resp, err := anRPCCall()
	if err != nil {
		err = errors.New("error")
	}
	err = nil // Unconditional suppression
	return resp, err
}

//go:noinline
func analyzeErrorPermute58_NilBranchReassignNilFailClose() (string, error) {
	// When err == nil, reassign to nil (no-op)
	// Based on testErrorAssignment8
	resp, err := anRPCCall()
	if err == nil {
		err = nil
	}
	return resp, err
}

//go:noinline
func analyzeErrorPermute59_ConditionalReturnNewErrFailClose() (string, error) {
	// When err != nil, return new error instead of err
	// Based on testConditionalFuncReturn
	resp, err := anRPCCall()
	if err != nil {
		return "", errors.New("error")
	}
	return resp, nil
}

//go:noinline
func analyzeErrorPermute60_ReturnDefaultErrVarFailOpen() (string, error) {
	// Return uninitialized error variable (default nil)
	// Based on testAlwaysFailOpen3
	var err2 error
	resp, err := anRPCCall()
	_ = err // Use err to avoid unused variable
	return resp, err2
}

//go:noinline
func analyzeErrorPermute61_InvertedLogicFailOpen() (string, error) {
	// Inverted logic: if err == nil return, else return nil
	// Based on testNamedReturn3 pattern
	resp, err := anRPCCall()
	if err == nil {
		return resp, err // Returns nil
	}
	return "", nil // When err != nil, return nil - suppression!
}

//go:noinline
func analyzeErrorPermute62_NestedConditionalReassignFailClose() (string, error) {
	// Nested conditional reassignment
	// Based on control flow patterns
	resp, err := anRPCCall()
	if err != nil {
		if cond {
			err = errors.New("error")
		}
	}
	return resp, err
}

//go:noinline
func analyzeErrorPermute63_MultiplePathsAllReturnErrFailClose() (string, error) {
	// Multiple nested paths, all return errors when err != nil
	// Based on testConditionalNilabilityPropagation2
	resp, err := anRPCCall()
	if err != nil {
		if cond {
			return "", errors.New("error1")
		} else { //nolint:redundantelse
			return "", errors.New("error2")
		}
	}
	return resp, err
}

//go:noinline
func analyzeErrorPermute64_ConditionalGenericReassignNilFailClose() (string, error) {
	// Generic condition (not based on err) may reassign to nil - suppression path exists
	// Based on testErrorAssignment2
	resp, err := anRPCCall()
	if cond {
		err = nil
	}
	return resp, err
}

//go:noinline
func analyzeErrorPermute65_NestedIfSuppressFailClose() (string, error) {
	// Nested if with suppression in inner branch
	// When err != nil and cond is true, suppress
	resp, err := anRPCCall()
	if err != nil {
		if cond {
			err = nil
		}
	}
	return resp, err
}

// =============================================================================
// Group 30: Additional edge-case permutations for coverage
// =============================================================================

// Permute66: Short-circuit OR with two error sources
// Tests detectShortCircuitPhi OR path and refineFacts OR true/false.
//
//go:noinline
func analyzeErrorPermute66_ShortCircuitOrFailClose() (string, error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	if err != nil || err2 != nil {
		return s, err
	}
	return s, nil
}

// Permute67: Short-circuit AND with two nil checks
// Tests detectShortCircuitPhi AND path and refineFacts AND true/false.
//
//go:noinline
func analyzeErrorPermute67_ShortCircuitAndBothNilCheckFailClose() (string, error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	if err != nil && err2 != nil {
		return s, err
	}
	return s, nil
}

// Permute68: Negated nil check via !(err == nil)
// Tests evalUnOp NOT path for negated nil comparisons.
//
//go:noinline
func analyzeErrorPermute68_NegatedNilCheckFailClose() (string, error) {
	s, err := anRPCCall()
	isNil := err == nil
	if !isNil {
		return s, err
	}
	return s, nil
}

// Permute69: Direct fmt.Errorf return wrapping the source error
// Tests evalCall fmt.Errorf NonNil path.
//
//go:noinline
func analyzeErrorPermute69_FmtErrorfDirectFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, fmt.Errorf("rpc failed: %w", err)
	}
	return s, nil
}

// Permute70: Interface method call wrapping the error
// Tests methodIsWrapLike and evalCall invoke path.
//
//go:noinline
func analyzeErrorPermute70_InterfaceWrapFailClose() (string, error) {
	s, err := anRPCCall()
	var w ErrorWrapper = simpleWrapper{}
	if err != nil {
		return s, w.Wrap(err)
	}
	return s, nil
}

// Permute71: Three error sources, all nil-checked
// Tests preInitializeCall other-source and checkAttribution multi-source.
//
//go:noinline
func analyzeErrorPermute71_ThreeSourcesAllCheckedFailClose() (string, error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	_, err3 := someCall()
	if err != nil {
		return s, err
	}
	if err2 != nil {
		return s, err2
	}
	if err3 != nil {
		return s, err3
	}
	return s, nil
}

// Permute72: Variadic joinErrors call
// Tests collectVarargsSources path.
//
//go:noinline
func analyzeErrorPermute72_VarargsJoinFailClose() (string, error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	if err != nil || err2 != nil {
		return s, joinErrors(err, err2)
	}
	return s, nil
}

// Permute73: Dead code branch due to constant false
// Tests markExecutable skip and block-not-executable paths.
//
//go:noinline
func analyzeErrorPermute73_DeadCodeBranchFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, err
	}
	if false {
		return s, errors.New("unreachable")
	}
	return s, nil
}

// Permute74: Non-wrap-like interface method (Validator.Validate) with error
// propagation. Tests evalCall invoke path for methods without error param
// (lines 1567-1577 in sccp_analyze.go).
//
//go:noinline
func analyzeErrorPermute74_NonWrapInvokeFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, err
	}
	var v Validator = simpleValidator{}
	if err2 := v.Validate(s); err2 != nil {
		return s, err2
	}
	return s, nil
}

// Permute75: OtherSource is nil-proven at return block, CDG attribution fallback.
// Tests the allOtherSourcesNilInBlock → checkControlAttribution path
// (lines 2087-2096 in sccp_analyze.go).
//
//go:noinline
func analyzeErrorPermute75_OtherSourceNilAtReturnFailClose() (string, error) {
	_, err1 := someCall()
	if err1 != nil {
		return "", err1
	}
	// err1 is nil here (assertion propagation)
	s, err2 := anRPCCall()
	if err2 != nil {
		// err1 is nil in this block; err2's return includes err1's data sources
		// via Phi, but CDG attribution proves control via err2's nil check.
		return s, fmt.Errorf("e1=%v e2=%w", err1, err2)
	}
	return s, nil
}

// Permute76: Multiple OtherSources, all checked before source.
// Ensures preInitializeCall registers all OtherSources.
//
//go:noinline
func analyzeErrorPermute76_MultiOtherSourceAllCheckedFailClose() (string, error) {
	_, err1 := someCall()
	if err1 != nil {
		return "", err1
	}
	_, err2 := someCall()
	if err2 != nil {
		return "", err2
	}
	_, err3 := someCall()
	if err3 != nil {
		return "", err3
	}
	s, err := anRPCCall()
	if err != nil {
		return s, err
	}
	return s, nil
}

// Permute77: Return wrapped error from conditional branch where wrapper
// produces Top nilness. Strict mode must use CDG to prove non-nil.
// Tests proveNonNilByControlGuard (lines 1976-2012).
//
//go:noinline
func analyzeErrorPermute77_StrictCDGProofFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		wrapped := wrapError(err) // wrapError returns nil if err is nil, non-nil otherwise
		return s, wrapped         // CDG proves wrapped is non-nil via err!=nil guard
	}
	return s, nil
}

// Permute78: Return error from three-source OR condition true branch.
// Tests refineFacts multi-check uncertainty (lines 1079-1087).
//
//go:noinline
func analyzeErrorPermute78_ThreeWayOrTrueBranchFailClose() (string, error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	err3 := someProcessing()
	if err != nil || err2 != nil || err3 != nil {
		// OR true branch with 3 checks — uncertain which one is non-nil.
		return s, err
	}
	return s, nil
}

// Permute79: Return error from three-source AND condition false branch.
// Tests refineFacts multi-check AND-false → Top insertion path.
//
//go:noinline
func analyzeErrorPermute79_ThreeWayAndFalseBranchFailClose() (string, error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	err3 := someProcessing()
	if err != nil && err2 != nil && err3 != nil {
		return s, nil
	}
	// AND false with 3 checks — uncertain which one is nil.
	return s, err
}

// Permute80: TypeAssert passthrough on error.
// Error flows through ChangeInterface (error → interface{}) then TypeAssert
// (interface{} → error), exercising evalPassthrough for both instruction types.
//
//go:noinline
func analyzeErrorPermute80_TypeAssertPassthroughFailClose() (string, error) {
	s, err := anRPCCall()
	var iface any = err //nolint:all // intentional ChangeInterface for coverage
	if e, ok := iface.(error); ok && e != nil {
		return s, e
	}
	return s, nil
}

// Permute81: Map lookup producing a boolean via default instruction path.
// The map lookup produces a bool via an SSA instruction that hits the
// visitInstruction default case (line 1316-1317 in sccp_analyze.go).
//
//go:noinline
func analyzeErrorPermute81_MapLookupBoolFailClose() (string, error) {
	s, err := anRPCCall()
	m := map[string]bool{"fail": true}
	if m["fail"] {
		if err != nil {
			return s, err
		}
	}
	return s, nil
}

// Permute82: ChangeType passthrough for error.
// A named error type is converted to error via ChangeType, exercising
// evalPassthrough for ChangeType instructions.
//
//go:noinline
func analyzeErrorPermute82_ChangeTypeFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, myErrorType(err.Error())
	}
	return s, nil
}

// Permute83: Single-return error source (someProcessing returns just error).
// The source call returns only error, exercising different Extract paths.
//
//go:noinline
func analyzeErrorPermute83_SingleReturnSourceFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, err
	}
	if err2 := someProcessing(); err2 != nil {
		return s, err2
	}
	return s, nil
}

// Permute84: Named return without defer — error goes through store/load via
// the named return variable (SSA alloc + Store + UnOp MUL).
//
//go:noinline
func analyzeErrorPermute84_StoreLoadFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	if err != nil {
		retErr = err
		return s, retErr
	}
	return s, nil
}

// Permute85: Named return with defer — error goes through memory (store/load).
// The defer captures the named return variable, preventing SSA from resolving
// nilness through value flow. In strict mode, the return value stays at Top
// nilness, triggering proveNonNilByControlGuard (lines 1976-2012 in sccp_analyze.go).
//
//go:noinline
func analyzeErrorPermute85_NamedReturnDeferFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("deferred: %w", retErr)
		}
	}()
	if err != nil {
		retErr = err
		return s, retErr
	}
	return s, nil
}

// Permute86: Named return with defer and multiple error paths.
// Exercises CDG attribution with named-return store/load under control guards.
//
//go:noinline
func analyzeErrorPermute86_NamedReturnMultiPathFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("deferred: %w", retErr)
		}
	}()
	if err != nil {
		retErr = err
		return s, retErr
	}
	if err2 != nil {
		retErr = err2
		return s, retErr
	}
	return s, nil
}

// Permute87: Named return with defer that does NOT capture the error variable.
// Src error is suppressed (retErr = nil). The deferred closure only touches
// unrelated variables. Block-local store peephole resolves *retErr = nil
// through rundefers, proving the error return is nil → FailOpen.
//
//go:noinline
func analyzeErrorPermute87_NamedReturnDeferNoCaptureFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	counted := false
	defer func() {
		if !counted {
			_ = s
		}
	}()
	if err != nil {
		retErr = nil
		return s, retErr
	}
	counted = true
	return s, nil
}

// Permute88: Named return with defer that only READS the error variable (no store).
// Src error is suppressed (retErr = nil). The closure captures retErr but
// never writes to it, so rundefers cannot modify the alloc.
// Block-local store peephole fires → FailOpen.
//
//go:noinline
func analyzeErrorPermute88_NamedReturnDeferReadOnlyFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	defer func() {
		if retErr != nil {
			fmt.Println("error occurred:", retErr)
		}
	}()
	if err != nil {
		retErr = nil
		return s, retErr
	}
	return s, nil
}

// Permute89: Named return with defer that writes to the error variable.
// Src error is propagated (retErr = err). The deferred closure captures
// retErr and stores to it (wrapping). Block-local store peephole must NOT
// fire because rundefers could modify the value. FailClose because the
// err!=nil path stores err (NonNil from Src) and the defer wraps it.
//
//go:noinline
func analyzeErrorPermute89_NamedReturnDeferWriteFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("wrapped: %w", retErr)
		}
	}()
	if err != nil {
		retErr = err
		return s, retErr
	}
	return s, nil
}

// Permute90: Named return, no defer, multi-path with fail-open suppression.
// Src error is suppressed (return s, nil). Without any defer, there is no
// rundefers between store and load. Block-local store directly resolves.
//
//go:noinline
func analyzeErrorPermute90_NamedReturnNoDeferFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	if err != nil {
		return s, nil
	}
	if err2 != nil {
		retErr = err2
		return s, retErr
	}
	return s, nil
}

// Permute91: Named return with defer that writes to error, but Src error
// path returns nil directly (not through named return). The other error
// path (err2) propagates. Since Src's path doesn't store to retErr at all,
// SCCP should classify as FailOpen.
//
//go:noinline
func analyzeErrorPermute91_NamedReturnDeferWriteSrcNilFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("deferred: %w", retErr)
		}
	}()
	if err != nil {
		return s, nil
	}
	if err2 != nil {
		retErr = err2
		return s, retErr
	}
	return s, nil
}

// Permute92: Named return with defer that unconditionally nils the error.
// Src error is propagated in the body (retErr = err), but the defer always
// sets retErr = nil, swallowing any error at runtime. SCCP cannot prove
// the defer's nil-ification through memState, so it conservatively classifies
// as FailClose. The block-local peephole also skips (defer writes to retErr).
//
//go:noinline
func analyzeErrorPermute92_NamedReturnDeferNilifyFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	defer func() {
		retErr = nil
	}()
	if err != nil {
		retErr = err
		return s, retErr
	}
	return s, nil
}

// Permute93: Named return, no defer, direct nil store on error (fail-open).
// Src's error path stores nil to retErr — textbook fail-open.
// Block-local store resolves immediately (no rundefers).
//
//go:noinline
func analyzeErrorPermute93_NamedReturnDirectNilFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	if err != nil {
		retErr = nil
		return s, retErr
	}
	return s, nil
}

// Permute94: Named return with defer that does NOT capture the error variable.
// Src error is PROPAGATED (retErr = err) — FailClose. The defer is
// irrelevant because it doesn't touch retErr. Block-local peephole resolves
// the err!=nil path store to NonNil (from Src) → FailClose.
//
//go:noinline
func analyzeErrorPermute94_NamedReturnDeferNoCaptureFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	counted := false
	defer func() {
		if !counted {
			_ = s
		}
	}()
	if err != nil {
		retErr = err
		return s, retErr
	}
	counted = true
	return s, nil
}

// Permute95: Named return with defer that only READS the error variable.
// Src error is PROPAGATED (retErr = err) — FailClose. The closure captures
// retErr for logging but never writes to it. Block-local peephole fires
// on both paths: NonNil on err path, Nil on success path → FailClose.
//
//go:noinline
func analyzeErrorPermute95_NamedReturnDeferReadOnlyFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	defer func() {
		if retErr != nil {
			fmt.Println("error occurred:", retErr)
		}
	}()
	if err != nil {
		retErr = err
		return s, retErr
	}
	return s, nil
}

// Permute96: Named return with defer that writes to error, but the defer is
// inside a conditional branch (does NOT dominate the return blocks).
// Src error is suppressed (retErr = nil). Since the defer doesn't dominate,
// isAddrModifiableByRunDefers returns false even though the closure writes
// to retErr. Block-local peephole fires → FailOpen.
//
//go:noinline
func analyzeErrorPermute96_NamedReturnDeferNonDominatingFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	if err2 != nil {
		defer func() {
			retErr = fmt.Errorf("deferred: %w", retErr)
		}()
	}
	if err != nil {
		retErr = nil
		return s, retErr
	}
	return s, nil
}

// Permute97: Same as Permute96 but Src error is PROPAGATED (retErr = err).
// The defer writes to retErr but doesn't dominate the return blocks,
// so the peephole fires and resolves NonNil → FailClose.
//
//go:noinline
func analyzeErrorPermute97_NamedReturnDeferNonDominatingFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	if err2 != nil {
		defer func() {
			retErr = fmt.Errorf("deferred: %w", retErr)
		}()
	}
	if err != nil {
		retErr = err
		return s, retErr
	}
	return s, nil
}

// Permute98: Named return with TWO defers — one dominates (writes), one doesn't.
// Src is NonNil so only the nil-storing path is executable; memState resolves
// to nil directly (never reaches Top). The dominating defer guard never fires.
// SCCP correctly classifies as FailOpen.
//
//go:noinline
func analyzeErrorPermute98_NamedReturnDeferDominatingAndNotFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	_, err2 := someCall()
	defer func() {
		if retErr != nil {
			retErr = fmt.Errorf("always wrapped: %w", retErr)
		}
	}()
	if err2 != nil {
		defer func() {
			retErr = fmt.Errorf("conditional: %w", retErr)
		}()
	}
	if err != nil {
		retErr = nil
		return s, retErr
	}
	return s, nil
}

// Permute99: Named return with deferred METHOD call (not closure) that takes
// &err as argument. Reproduces the bliss-resolver IsAuthorized pattern:
//
//	func (c *controller) IsAuthorized(...) (err error) {
//	    defer metrics.New(...).Stop(&err)       // method call, &err is argument
//	    ...
//	    err := c.recordActionFailure(...)        // shadows named return
//	    if err != nil {
//	        return &entities.ResolverError{...}  // propagates error
//	    }
//	    return &entities.ResolverError{...}      // also non-nil
//	}
//
// The outbound call error IS propagated (return non-nil error on all exit
// paths after the call). This should be FailClose. However, the deferred
// method call passes &retErr as a function argument (not a FreeVar in a
// MakeClosure), so isAddrModifiableByRunDefers does not detect it.
// BUG: SCCP classifies this as FailOpen because the block-local store
// peephole resolves a nil store without seeing the defer's potential write.
//
//go:noinline
func analyzeErrorPermute99_NamedReturnDeferMethodArgFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	defer newMetricsTimer().Stop(&retErr)
	if err != nil {
		retErr = err
		return s, retErr
	}
	return s, nil
}

// Permute100: Same as Permute99 but with an inner variable shadowing the
// named return (`:=`), and multiple outbound calls. Closely mirrors the
// IsAuthorized pattern where the shadow err receives the outbound error
// and the named return gets a new error value.
//
//go:noinline
func analyzeErrorPermute100_NamedReturnDeferMethodArgShadowFailClose() (result string, retErr error) {
	s, err := anRPCCall()
	defer newMetricsTimer().Stop(&retErr)
	_, err2 := someCall()
	if err2 != nil {
		retErr = err2
		return s, retErr
	}
	if err != nil {
		retErr = fmt.Errorf("propagated: %w", err)
		return s, retErr
	}
	return s, nil
}

// Permute101: Named return with deferred method call and struct error return
// on BOTH branches (no nil path). Reproduces the IsAuthorized pattern where
// both err != nil and err == nil branches return &ResolverError{...}.
// Since &resolverError{...} is an Alloc (always non-nil), both paths
// return a non-nil error → should be FailClose.
// BUG: SCCP treats Alloc as Top nilness (falls to default case in
// visitInstruction), so MakeInterface(Alloc) is also Top.
//
//go:noinline
func analyzeErrorPermute101_NamedReturnStructErrBothBranchesFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	defer newMetricsTimer().Stop(&retErr)
	if err != nil {
		retErr = &resolverError{msg: err.Error()}
		return s, retErr
	}
	retErr = &resolverError{msg: "permission denied"}
	return s, retErr
}

// Permute102: Same as Permute101 but WITHOUT the defer method call.
// Tests the Alloc nilness issue in isolation (no defer complication).
// Both branches return &resolverError{} → always non-nil → FailClose.
//
//go:noinline
func analyzeErrorPermute102_StructErrBothBranchesNoDeferFailOpen() (result string, retErr error) {
	s, err := anRPCCall()
	if err != nil {
		return s, &resolverError{msg: err.Error()}
	}
	return s, &resolverError{msg: "permission denied"}
}
