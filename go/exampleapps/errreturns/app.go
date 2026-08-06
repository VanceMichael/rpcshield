package main

import (
	"errors"
	"fmt"
	"strings"
)

//go:noinline
func anRPCCall() (string, error) {
	return "fail", nil
}

//go:noinline
func someCall() (string, error) {
	return "fail", nil
}

//go:noinline
func manyRetRPCCall() (string, string, string, string, error) {
	return "fail", "fail", "fail", "fail", nil
}

//go:noinline
func someProcessing() error {
	return nil
}

// ErrorWrapper is an interface for wrapping errors, used to test
// interface method call (invoke) paths in SCCP analysis.
type ErrorWrapper interface {
	Wrap(err error) error
}

type simpleWrapper struct{}

//go:noinline
func (simpleWrapper) Wrap(err error) error {
	if err != nil {
		return fmt.Errorf("wrapped: %w", err)
	}
	return nil
}

// joinErrors joins multiple errors, returning the first non-nil one.
// Used to test collectVarargsSources path in SCCP analysis.
//
//go:noinline
func joinErrors(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// Validator is an interface with a method that returns error but does NOT
// take an error param (not wrap-like). Used to test the invoke non-wrap-like
// path in evalCall (lines 1567-1577 of sccp_analyze.go).
type Validator interface {
	Validate(s string) error
}

type simpleValidator struct{}

// myErrorType is a named type based on string that implements error.
// Used to test ChangeType passthrough in SCCP analysis.
type myErrorType string

func (e myErrorType) Error() string { return string(e) }

//go:noinline
func (simpleValidator) Validate(s string) error {
	if s == "" {
		return errors.New("empty")
	}
	return nil
}

//go:noinline
func analyzeErrorSimpleFailClose() (string, error) {
	// Error: "fail" is not closed
	return anRPCCall()
}

//go:noinline
func analyzeErrorSimpleFailOpen() (string, error) {
	_, _ = anRPCCall()
	return "", nil
}

//go:noinline
func analyzeErrorCond1FailClose() (string, error) {
	s, e := anRPCCall()
	if e != nil {
		return s, e
	}
	return s, nil
}

//go:noinline
func analyzeErrorCond2FailOpen() (string, error) {
	s, e := anRPCCall()
	if e == nil {
		return s, e
	}
	return s, nil
}

//go:noinline
func analyzeErrorCond3FailClose() (string, error) {
	s, e := anRPCCall()
	if e == nil {
		return s, e
	}
	return s, e
}

//go:noinline
func analyzeErrorCond4FailOpen() (string, error) {
	s, e := anRPCCall()
	if e != nil {
		return s, nil
	}
	return s, nil
}

//go:noinline
func analyzeErrorCond5FailOpen() (string, error) {
	s, e := anRPCCall()
	if e == nil {
		return s, nil
	}
	return s, nil
}

//go:noinline
func analyzeErrorCondNested1FailUnknown() (string, error) {
	s, e := anRPCCall()
	if e != nil {
		s1, e := someCall()
		return s1, e // this is by design at SSA level for now.
	}
	return s, nil
}

//go:noinline
func analyzeErrorCondNested2FailClose() (string, error) {
	s, e := anRPCCall()
	if e != nil {
		s1, _ := someCall()
		return s1, e
	}
	return s, nil
}

//go:noinline
func analyzeErrorCondNested3FailOpen() (string, error) {
	s, e := anRPCCall()
	if e == nil {
		s1, e := someCall()
		return s1, e
	}
	return s, nil
}

//go:noinline
func analyzeErrorCondNested4FailOpen() (string, error) {
	s, e := anRPCCall()
	if e == nil {
		s1, _ := someCall()
		return s1, e
	}
	return s, nil
}

//go:noinline
func analyzeErrorCondNested5FailClose() (string, error) {
	s, e := anRPCCall()
	if e == nil {
		s1, e := someCall()
		return s1, e
	}
	return s, e
}

//go:noinline
func analyzeErrorCondNested6FailClose() (string, error) {
	s, e := anRPCCall()
	if e == nil {
		s1, _ := someCall()
		return s1, e
	}
	return s, e
}

func anRPCCall2() (error, error) {
	return nil, nil
}

// metricsTimer simulates a metrics timer whose Stop method takes a *error
// argument (like metrics.New(...).Stop(&err)). This is NOT a closure pattern —
// the address is passed as a function argument, not captured as a FreeVar.
type metricsTimer struct{}

//go:noinline
func newMetricsTimer() *metricsTimer { return &metricsTimer{} }

//go:noinline
func (m *metricsTimer) Stop(err *error) {
	// In real code this records the error for metrics.
	// From SCCP's perspective, this could write to *err.
	_ = err
}

type resolverError struct {
	msg string
}

func (e *resolverError) Error() string { return e.msg }

func anRPCCall3() int {
	return 42
}

//go:noinline
func analyzeErrorCorner1Unknown() (string, error) {
	_, _ = anRPCCall2()
	return "", nil
}

//go:noinline
func analyzeErrorCorner2Unknown() (string, error) {
	_ = anRPCCall3()
	return "", nil
}

//go:noinline
func analyzeErrorCorner3MultiBinOpUnknown() (string, error) {
	_, e := anRPCCall()
	v1 := e != nil
	v2 := e == nil
	x := v1 && v2
	_ = x
	return "", nil
}

var myError error

//go:noinline
func analyzeErrorCorner4NonNilComparisonUnknown() (string, error) {
	_, e := anRPCCall()
	if e != myError {
		return "", nil
	}
	return "", nil
}

//go:noinline
func analyzeErrorCorner5OtherInstructionsUnknown() (string, error) {
	_, e := anRPCCall()
	other := e
	_ = other
	if e != myError {
		return "", nil
	}
	return "", nil
}

//go:noinline
func analyzeErrorCorner6MultiIfUnknown() (string, error) {
	_, e := anRPCCall()
	if e != nil {
		return "", nil
	}
	if e == nil {
		return "", nil
	}
	return "", nil
}

//go:noinline
func analyzeErrorPredicatedReturn1FailClose() (string, error) {
	s, e := anRPCCall()
	if e != nil {
		// This is fail-closed because the non-error return is unprotected in the block.
		return "", errors.New("error")
	}
	return s, nil
}

//go:noinline
func analyzeErrorPredicatedReturn2FailOpen() (string, error) {
	s, e := anRPCCall()
	if e == nil {
		// This is fail-open since the block guard is  e == nil.
		return "", errors.New("error")
	}
	return s, nil
}

//go:noinline
func analyzeErrorPredicatedReturn3FailOpen(a, b int) (string, error) {
	s, e := anRPCCall()
	if e != nil {
		if a > b {
			return "", errors.New("error")
		} else if b == 21 {
			return s, nil
		}
	}
	return s, nil
}

//go:noinline
func analyzeErrorPredicatedReturn4FailOpen(a, b int) (string, error) {
	s, e := anRPCCall()
	if e != nil {
		if a > b {
			return "", errors.New("error")
		} else if b == 21 {
			return s, nil
		}
		return "", errors.New("error") // This is not guaranteed to be reached. Hence, fail-open.
	}
	return s, nil
}

//go:noinline
func analyzeErrorPredicatedReturn5FailClose(a, b int) (string, error) {
	s, e := anRPCCall()
	if e != nil {
		return "", errors.New("error") // This is guaranteed to be reached. Hence, fail-closed.
		if a > b {
			return "", errors.New("error")
		} else if b == 21 {
			return s, nil
		}
	}
	return s, nil
}

//go:noinline
func analyzeErrorPredicatedReturn6FailClose() (string, error) {
	s, e := anRPCCall()
	if e != nil {
		return "", errors.New("error") // This is guaranteed to be reached. Hence, fail-closed.
		return "", nil                 // This is is not reachable.
		return "", e                   // This is is not reachable.
	}
	return s, nil
}

//go:noinline
func analyzeErrorPredicatedReturn7FailOpen() (string, error) {
	s, e := anRPCCall()
	if e != nil {
		return "", nil                 // This is guaranteed to be reached. Hence, fail-open.
		return "", errors.New("error") // This is is not reachable.
		return "", e                   // This is is not reachable.
	}
	return s, nil
}

//go:noinline
func analyzeErrorPredicatedReturn8FailClose() (string, error) {
	s, e := anRPCCall()
	if e != nil {
		return "", e                   // This is guaranteed to be reached. Hence, fail-close.
		return "", nil                 // This is is not reachable.
		return "", errors.New("error") // This is is not reachable.
	}
	return s, nil
}

//go:noinline
func analyzeErrorPredicatedReturn9Unknown(f func() error) (string, error) {
	s, e := anRPCCall()
	if e != nil {
		return "", f() // non-static callee
	}
	return s, nil
}

//go:noinline
func analyzeErrorSimpleButFailOpen() (string, error) {
	s, _ := anRPCCall()
	return s, errors.New("error")
}

//go:noinline
func analyzeErrorNoerrorFailOpen() {
	anRPCCall()
	return
}

//go:noinline
func analyzeErrorPredicatedReturnOffIndexTest1FailClose() error {
	// Regression test for a fail-close/fail-open classification edge case involving predicated returns.
	_, e := anRPCCall()
	if e != nil {
		// This is fail-closed because the last non-error return is unprotected in the block.
		return errors.New("error")
	}
	return nil
}

//go:noinline
func analyzeErrorPredicatedReturnOffIndexTest2FailClose() (error, int, error) {
	// Regression test for a fail-close/fail-open classification edge case involving predicated returns.
	_, _, _, _, e := manyRetRPCCall()
	if e != nil {
		// This is fail-closed because the last non-error return is unprotected in the block.
		return errors.New("error"), 42, errors.New("error")
	}
	return nil, 42, nil
}

//go:noinline
func analyzeErrorPredicatedReturnOffIndexTest3FailOpen() (error, int, error) {
	// Regression test for a fail-close/fail-open classification edge case involving predicated returns.
	_, _, _, _, e := manyRetRPCCall()
	if e != nil {
		// This is fail-closed because the non-error return is unprotected in the block.
		// Only the last error return matters, hence fail-open.
		return errors.New("error"), 42, nil
	}
	return nil, 42, nil
}

//go:noinline
func analyzeErrorPredicatedReturnOffIndexTest4FailClose() (error, int, error) {
	// Regression test for a fail-close/fail-open classification edge case involving predicated returns.
	_, _, _, _, e := manyRetRPCCall()
	if e != nil {
		// This is fail-closed because the last non-error return is unprotected in the block.
		return nil, 42, errors.New("error")
	}
	return nil, 42, nil
}

//go:noinline
func wrapError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("wrapped: %w", err)
}

// fail_close.md Example 5: err == nil guard, return errors.New unconditionally.
// simple: FailClose (errors.New in errNeqNil merge block), path: FailClose
//
//go:noinline
func analyzeErrorSCCPMd5EqNilRetNewErrFailOpen() (string, error) {
	s, err := anRPCCall()
	if err == nil {
		_, _ = someCall() // non-trivial work to prevent SSA optimization of the branch
	}
	return s, errors.New("new error")
}

// fail_close.md Example 6: Wrap(err) return under err != nil guard.
// simple: FailUnknown (err used in call arg), path: FailClose
//
//go:noinline
func analyzeErrorSCCPMd6WrapRetFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, wrapError(err)
	}
	return s, err
}

// fail_close.md Example 7: OR condition with two error sources, return err2.
// simple: FailUnknown, path: FailOpen (attribution fails for anRPCCall since err2 is from someCall)
//
//go:noinline
func analyzeErrorSCCPMd7OrCondRetErr2FailOpen() (string, error) {
	_, err := anRPCCall()
	_, err2 := someCall()
	if err != nil || err2 != nil {
		return "", err2
	}
	return "", nil
}

// fail_close.md Example 8: AND condition with two error sources, return err.
// simple: FailClose (err returned not dominated by errEqNil), path: FailOpen (C-sufficiency fails)
//
//go:noinline
func analyzeErrorSCCPMd8AndCondRetErrFailClose() (string, error) {
	_, err := anRPCCall()
	_, err2 := someCall()
	if err != nil && err2 != nil {
		return "", err
	}
	_ = err2
	return "", nil
}

// fail_close.md Example 11: Always fails - err != nil returns err, else returns errors.New.
// simple: FailClose, path: FailClose
//
//go:noinline
func analyzeErrorSCCPMd11AlwaysFailsFailOpen() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, err
	}
	return s, errors.New("some error")
}

// fail_close.md Example 13: Nested G call with guard on err2, return err2. NOT FC on C.
// simple: FailOpen, path: FailOpen
//
//go:noinline
func analyzeErrorSCCPMd13NestedGuardRetErr2FailOpen() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		s2, err2 := someCall()
		if err2 == nil {
			return s, nil
		}
		return s2, err2
	}
	return s, nil
}

// fail_close.md Example 14: Nested G call with guard on err2, return C's err.
// simple: FailClose (err returned not dominated by errEqNil), path: FailOpen (C-sufficiency fails - last gate is err2)
//
//go:noinline
func analyzeErrorSCCPMd14NestedGuardRetErrFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		_, err2 := someCall()
		if err2 == nil {
			return s, nil
		}
		return s, err
	}
	return s, nil
}

// fail_close.md Example 15: Nested G call with guard on err2, return Wrap(err).
// simple: FailUnknown (err used in call arg), path: FailOpen (C-sufficiency fails - last gate is err2)
//
//go:noinline
func analyzeErrorSCCPMd15NestedGuardWrapFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		_, err2 := someCall()
		if err2 == nil {
			return s, nil
		}
		return s, wrapError(err)
	}
	return s, nil
}

// fail_close.md Example 16: derivedErr = Wrap(err); return derivedErr. FC.
// simple: FailUnknown (err used in call arg), path: FailClose
//
//go:noinline
func analyzeErrorSCCPMd16DerivedRetFailClose() (string, error) {
	s, err := anRPCCall()
	derivedErr := wrapError(err)
	return s, derivedErr
}

// fail_close.md Example 17: derivedErr guard, return derivedErr. FC.
// simple: FailUnknown (err used in call arg), path: FailClose
//
//go:noinline
func analyzeErrorSCCPMd17DerivedGuardRetDerivedFailClose() (string, error) {
	s, err := anRPCCall()
	derivedErr := wrapError(err)
	if derivedErr != nil {
		return s, derivedErr
	}
	return s, err
}

// fail_close.md Example 18: derivedErr guard, return err. FC.
// simple: FailUnknown (err used in call arg), path: FailClose
//
//go:noinline
func analyzeErrorSCCPMd18DerivedGuardRetErrFailClose() (string, error) {
	s, err := anRPCCall()
	derivedErr := wrapError(err)
	if derivedErr != nil {
		return s, err
	}
	return s, err
}

// fail_close.md Example 19: derivedErr guard, return Wrap(derivedErr). FC.
// simple: FailUnknown (err used in call arg), path: FailClose
//
//go:noinline
func analyzeErrorSCCPMd19DerivedGuardRetWrapFailClose() (string, error) {
	s, err := anRPCCall()
	derivedErr := wrapError(err)
	if derivedErr != nil {
		return s, wrapError(derivedErr)
	}
	return s, err
}

// TODO: Milind: Fix code and test to set correct expected results.
// fail_close.md Example 20: derivedErr guard, G call, return err2. NOT FC on C.
// simple: FailUnknown (err used in call arg), path: FailOpen (attribution fails)
//
//go:noinline
func analyzeErrorSCCPMd20DerivedGuardGRetErr2FailOpen() (string, error) {
	s, err := anRPCCall()
	derivedErr := wrapError(err)
	if derivedErr != nil {
		s2, err2 := someCall()
		return s2, err2
	}
	return s, err
}

// Bool alias condition: e1 := (err == nil); if !e1 { return err }.
// SSA normalizes this to direct branch on the compare, so simple/path are both FailClose.
//
//go:noinline
func analyzeErrorSCCPMd21BoolAliasNegationFailClose() (string, error) {
	s, err := anRPCCall()
	e1 := err == nil
	if !e1 {
		return s, err
	}
	return s, nil
}

// Bool condition through control-flow merge (phi-style):
// c is either (err==nil) or !(err==nil) depending on an unrelated branch.
// simple: FailUnknown (err compare result not used directly by a single If), path: FailClose.
//
//go:noinline
func analyzeErrorSCCPMd22BoolPhiAliasFailClose() (string, error) {
	s, err := anRPCCall()
	e1 := err == nil
	var c bool
	if len(s)%2 == 0 {
		c = e1
	} else {
		c = !e1
	}
	if !c {
		return s, err
	}
	return s, nil
}

// Additional test cases for claiming model

// CASE 7: err1 guarded, then return unrelated err2
// Expected: FailOpen (err2 is Top, non-suppression fails)
//
//go:noinline
func analyzeErrorSCCPCase7UnrelatedReturnFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall()
	}
	_, err2 := someCall()
	return s, err2
}

// CASE 8: err1 guarded, then return fresh error unconditionally
// Dataflow: FailClose (fresh error, only path under Src!=nil goes through if-block)
// Path enum: FailOpen (err3 is unconditional, not inside if-block)
//
//go:noinline
func analyzeErrorSCCPCase8FreshUnconditionalFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall()
	}
	_, _ = someCall()
	err3 := errors.New("unconditional error")
	return s, err3
}

// CASE 9a: err1 guarded, err2 checked, return fresh in err2's error branch
// Expected: FailOpen (err2 claims the return, not err1)
//
//go:noinline
func analyzeErrorSCCPCase9aFreshInErr2BranchFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall()
	}
	_, err2 := someCall()
	err3 := errors.New("fresh error")
	if err2 != nil {
		return s, err3
	}
	return s, nil
}

// CASE 9b: err1 guarded, err2 checked, return err1 in err2's success branch
// Dataflow: FailClose (err2's success branch doesn't claim, err1 has data attr)
// Path enum: FailOpen (err2 is in scope, fails C-Sufficiency)
//
//go:noinline
func analyzeErrorSCCPCase9bReturnErr1SuccessBranchFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall()
	}
	_, err2 := someCall()
	err3 := errors.New("fresh error")
	if err2 != nil {
		return s, err3
	}
	return s, err1
}

// CASE 10: err1 guarded, err2 checked, return err2 in err2's success branch
// Expected: FailOpen (err2 nilness is Top, non-suppression fails)
//
//go:noinline
func analyzeErrorSCCPCase10ReturnErr2SuccessBranchFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall()
	}
	_, err2 := someCall()
	err3 := errors.New("fresh error")
	if err2 != nil {
		return s, err3
	}
	return s, err2
}

// =============================================================================
// SCCP Tests with Multiple Sources (OtherSources awareness)
// =============================================================================
// These functions are named by what the SIMPLE analyzer returns.
// The SCCP test has its own expectations that differ due to OtherSources awareness.

// SCCP1: Simple case - err1 checked, return err1
// Simple: FailClose, SCCP: FailClose
//
//go:noinline
func analyzeErrorSCCP1SimpleDirectFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		return s, err1
	}
	return s, nil
}

// SCCP2: err1 checked, inside err1's error branch: err2 checked, return err1
// err2 is OtherSource, we're on err2's SUCCESS branch for return err1
// Simple: FailClose (err1 returned under err1!=nil), SCCP: FailClose (err2's success doesn't claim)
//
//go:noinline
func analyzeErrorSCCP2NestedSuccessBranchFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err2 := someCall()
		if err2 != nil {
			return s, errors.New("err2 failed")
		}
		// err2 == nil (success branch of err2), return err1
		return s, err1
	}
	return s, nil
}

// SCCP3: err1 checked, inside err1's error branch: err2 checked, return fresh error
// err2 is OtherSource, we're on err2's ERROR branch
// Simple: FailOpen (predicated on err2), SCCP: FailOpen (err2 claims this return)
//
//go:noinline
func analyzeErrorSCCP3NestedErrorBranchFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err2 := someCall()
		if err2 != nil {
			// err2's error branch claims this synthetic error
			return s, errors.New("nested error")
		}
		return s, nil
	}
	return s, nil
}

// SCCP4: Deeply nested - err1 -> err2 -> err3 success branches, return err1
// Simple: FailClose (err1 returned under err1!=nil), SCCP: FailClose (all inner on success)
//
//go:noinline
func analyzeErrorSCCP4DeepNestedSuccessFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err2 := someCall()
		if err2 == nil {
			_, err3 := someCall()
			if err3 == nil {
				// All nested sources succeeded, return err1
				return s, err1
			}
			return s, errors.New("err3 failed")
		}
		return s, errors.New("err2 failed")
	}
	return s, nil
}

// SCCP5: err1 checked, err2 checked sequentially (not nested), return err1 in err2's success
// Simple: FailOpen (err1 returned outside err1!=nil guard), SCCP: FailClose (data attr)
//
//go:noinline
func analyzeErrorSCCP5SequentialSuccessFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall() // do something
	}
	_, err2 := someCall()
	if err2 != nil {
		return s, errors.New("err2 error")
	}
	// err2 == nil, return err1 (which might be nil or non-nil)
	return s, err1
}

// SCCP6: Both sources fail, nested, return synthetic in innermost error branch
// Simple: FailOpen (predicated on err2), SCCP: FailOpen (err2 claims)
//
//go:noinline
func analyzeErrorSCCP6BothFailInnerClaimsFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err2 := someCall()
		if err2 != nil {
			// Both failed, but err2 is the proximate controller
			return s, errors.New("both failed")
		}
	}
	return s, nil
}

// SCCP7: err1 error branch, err2 success branch, return wrap(err1)
// Simple: FailUnknown (wrap call), SCCP: FailClose (wrap has data attr)
//
//go:noinline
func analyzeErrorSCCP7NestedSuccessWrapFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err2 := someCall()
		if err2 != nil {
			return s, errors.New("err2 failed")
		}
		// err2 == nil, wrap and return err1
		return s, wrapError(err1)
	}
	return s, nil
}

// SCCP8: Triple nesting - err1 error, err2 success, err3 success, return err1
// Simple: FailClose (err1 under err1!=nil), SCCP: FailClose
//
//go:noinline
func analyzeErrorSCCP8TripleNestedSuccessFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err2 := someCall()
		if err2 == nil {
			_, err3 := someCall()
			if err3 == nil {
				return s, err1
			}
		}
	}
	return s, nil
}

// SCCP9: err1 success branch (err1 == nil), then err2, return err2
// Simple: FailOpen (err1==nil guard), SCCP: FailOpen
//
//go:noinline
func analyzeErrorSCCP9SrcSuccessBranchFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 == nil {
		_, err2 := someCall()
		if err2 != nil {
			return s, err2
		}
	}
	return s, nil
}

// SCCP10: err1 error branch, then unconditional synthetic (no err2 guard)
// Simple: FailClose, SCCP: FailClose
//
//go:noinline
func analyzeErrorSCCP10UnconditionalSyntheticFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall() // do something but ignore error
		return s, errors.New("unconditional inside err1 branch")
	}
	return s, nil
}

// =============================================================================
// Additional SCCP Tests - Patterns from AST-based tests
// =============================================================================

// SCCP11: Chained wrapping - wrap(wrap(wrap(err)))
// Tests data attribution through multiple wrap calls
// Simple: FailUnknown (wrap call), SCCP: FailClose (data attr through chain)
//
//go:noinline
func analyzeErrorSCCP11ChainedWrapFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		return s, wrapError(wrapError(wrapError(err1)))
	}
	return s, nil
}

// SCCP12: Wrap synthetic error inside err1 branch
// Tests that synthetic wrapped in wrap still has control attribution
// Simple: FailUnknown (wrap call), SCCP: FailClose (synthetic under err1 guard)
//
//go:noinline
func analyzeErrorSCCP12WrapSyntheticFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall() // avoid optimization
		return s, wrapError(wrapError(errors.New("synthetic")))
	}
	return s, nil
}

// SCCP13: err1 checked, then err2 inside loop, return err1 in loop
// Tests loop doesn't break attribution
// Simple: FailClose (err1 under err1!=nil), SCCP: FailClose
//
//go:noinline
func analyzeErrorSCCP13LoopWithErrorCheckFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		for i := 0; i < 3; i++ {
			_, err2 := someCall()
			if err2 != nil {
				continue
			}
		}
		return s, err1
	}
	return s, nil
}

// SCCP14: Three sequential sources, return err1 in err3's success
// Tests multiple OtherSources
// Simple: FailOpen (err1 outside guard), SCCP: FailClose (data attr, err2/err3 are others)
//
//go:noinline
func analyzeErrorSCCP14ThreeSequentialFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, _ = someCall()
	}
	_, err2 := someCall()
	if err2 != nil {
		return s, errors.New("err2 failed")
	}
	_, err3 := someCall()
	if err3 != nil {
		return s, errors.New("err3 failed")
	}
	return s, err1
}

// SCCP15: err1 checked, err2 claims then err1 returned later
// Tests that err2's error branch claiming doesn't affect err1's later return
// Simple: FailOpen (predicated by err2), SCCP: FailOpen (err2 claims)
//
//go:noinline
func analyzeErrorSCCP15Err2ClaimsThenErr1FailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err2 := someCall()
		if err2 != nil {
			// err2 claims this return
			return s, errors.New("err2 error")
		}
		// This is on err2's success branch, but inside err1's error branch
		// Need to check if err1 is still attributed here
	}
	return s, nil
}

// SCCP16: Return synthetic in nested else branch
// Tests else branch control flow
// FailOpen because err2 is the nested controller.
//
//go:noinline
func analyzeErrorSCCP16ElseBranchSyntheticFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err2 := someCall()
		if err2 == nil {
			return s, nil
		} else {
			// Inside err1's error branch AND err2's error branch
			// err2 should claim this since it's the innermost controller
			return s, errors.New("nested else synthetic")
		}
	}
	return s, nil
}

// SCCP17: Double check same error - err1 != nil twice
// Tests idempotent condition
// Simple: FailClose, SCCP: FailClose
//
//go:noinline
func analyzeErrorSCCP17DoubleCheckFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		if err1 != nil {
			return s, err1
		}
	}
	return s, nil
}

// SCCP18: Check err1, assign to err2, return err2
// Tests error variable aliasing
// Simple: FailClose (err2 is err1), SCCP: FailClose (alias tracking)
//
//go:noinline
func analyzeErrorSCCP18ErrorAliasFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		err2 := err1
		return s, err2
	}
	return s, nil
}

// SCCP19: Complex - err1 guard with OR condition
// Tests OR condition handling
// Simple: FailClose (err returned under err!=nil || cond), SCCP: FailClose
//
//go:noinline
func analyzeErrorSCCP19OrConditionFailClose() (string, error) {
	s, err1 := anRPCCall()
	cond := len(s) > 5
	if err1 != nil || cond {
		_, _ = someCall()
		return s, err1
	}
	return s, nil
}

// SCCP20: Reassign err1 to another source, return reassigned
// Tests that reassignment breaks attribution
// Simple: FailOpen (err1 reassigned), SCCP: FailOpen
//
//go:noinline
func analyzeErrorSCCP20ReassignmentFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		_, err1 = someCall() // reassign err1
		return s, err1
	}
	return s, nil
}

// SCCP21: err1 checked first, then err2 is source, return err1 in err2's error branch
// Key insight: err1 is KNOWN NIL after its check, so returning it is FailOpen
// This tests that returning a nil value (err1) fails non-suppression
// Simple: FailOpen, SCCP: FailOpen
//
//go:noinline
func analyzeErrorSCCP21ReturnNilErr1InErr2BranchFailOpen() (string, error) {
	s, err1 := someCall() // not the source we're analyzing
	if err1 != nil {
		return s, err1
	}
	// err1 == nil here

	_, err2 := anRPCCall() // SOURCE - this is what we're analyzing
	if err2 != nil {
		return s, err1 // FAIL-OPEN: err1 is nil, AND not from err2
	}
	return s, err2 // err2 is nil here (but unreachable under err2!=nil assumption)
}

// SCCP22: Tests assertion propagation proving NonNil on error branch
// Inside `if err2 != nil`, the OTHER error (err1) checked earlier is Nil.
// But err2 itself is proven NonNil by the branch predicate.
// This tests that assertion propagation correctly refines err2 to NonNil.
// Simple: FailClose, SCCP: FailClose
//
//go:noinline
func analyzeErrorSCCP22AssertionNonNilFailClose() (string, error) {
	s, err1 := someCall()
	if err1 != nil {
		return s, err1
	}
	// err1 == nil (assertion propagation knows this)

	_, err2 := anRPCCall() // SOURCE
	if err2 != nil {
		// err2 is NonNil here (assertion propagation knows this)
		// err1 is Nil here (assertion propagation knows this)
		return s, err2 // FAIL-CLOSE: err2 is NonNil and from source
	}
	return s, nil
}

// SCCP23: Tests short-circuit OR condition lowered to CFG
// `if err1 != nil || cond` becomes separate blocks
// In the final false-false path, err1 is known Nil
// Simple: FailClose, SCCP: FailClose
//
//go:noinline
func analyzeErrorSCCP23ShortCircuitOrFailClose() (string, error) {
	s, err1 := anRPCCall() // SOURCE
	cond := len(s) > 10
	if err1 != nil || cond {
		// Either err1 != nil OR cond is true
		// err1 could be nil or non-nil here
		return s, err1
	}
	// err1 == nil AND !cond (both conditions false)
	return s, nil
}

// =============================================================================
// Inverse Check Test Cases
// =============================================================================
// These tests expose differences between forward-only and inverse-check modes

// InverseTest1: Both branches return non-nil errors
// Forward: FailClose (err branch returns SRC)
// Inverse: FailOpen (success branch returns synthetic non-nil)
//
//go:noinline
func analyzeErrorInverse01_BothBranchesNonNilFailOpen() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, err // Returns SRC error
	}
	return s, errors.New("default error") // Returns synthetic error
}

// InverseTest2: Err2 dominates - always returns err2
// Forward: FailOpen (attribution fails - err2 not from SRC)
// Inverse: FailOpen (err2 actually controls the outcome)
//
//go:noinline
func analyzeErrorInverse02_Err2DominatesFailOpen() (string, error) {
	s, err1 := anRPCCall()
	_, err2 := someCall()
	if err1 != nil || err2 != nil {
		return s, err2 // Always returns err2, not err1
	}
	return s, nil
}

// InverseTest3: Unconditional wrap after check
// Forward: FailOpen (fmt.Errorf creates non-nil even when input is nil)
// Inverse: FailOpen (always returns wrapped error)
//
//go:noinline
func analyzeErrorInverse03_UnconditionalWrapFailOpen() (string, error) {
	s, err := anRPCCall()
	result := err // Could be nil or non-nil
	if result == nil {
		result = errors.New("something")
	}
	return s, fmt.Errorf("wrapped: %w", result) // Always non-nil
}

// InverseTest4: Three-way merge all non-nil
// Forward: FailClose (SRC controls which branch)
// Inverse: FailOpen (all branches return non-nil)
//
//go:noinline
func analyzeErrorInverse04_ThreeWayMergeFailOpen() (string, error) {
	s, err := anRPCCall()
	var result error
	if err != nil {
		result = err
	} else if len(s) > 5 {
		result = errors.New("too long")
	} else {
		result = errors.New("too short")
	}
	return s, result // Always non-nil
}

// InverseTest5: Defensive pattern - validate then fail
// Forward: FailClose (SRC checked first)
// Inverse: FailOpen (validation always returns error)
//
//go:noinline
func analyzeErrorInverse05_DefensiveValidationFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, err
	}
	// Defensive validation always fails
	if len(s) >= 0 { // Always true
		return s, errors.New("validation failed")
	}
	return s, nil
}

// InverseTest6: Masked by wrapper
// Forward: FailOpen (fmt.Errorf(%w, nil) creates non-nil)
// Inverse: FailOpen (wrapper always returns non-nil)
//
//go:noinline
func analyzeErrorInverse06_MaskedByWrapperFailOpen() (string, error) {
	s, err := anRPCCall()
	wrapped := fmt.Errorf("context: %w", err) // Non-nil even if err is nil
	if wrapped != nil {                       // Always true
		return s, wrapped
	}
	return s, nil
}

// InverseTest7: Fallback chain
// Forward: FailClose (SRC is first in chain)
// Inverse: FailOpen (chain always produces error)
//
//go:noinline
func analyzeErrorInverse07_FallbackChainFailClose() (string, error) {
	s, err1 := anRPCCall()
	if err1 != nil {
		return s, err1
	}
	_, err2 := someCall()
	if err2 != nil {
		return s, err2
	}
	return s, errors.New("fallback error") // Always reached
}

// InverseTest8: Both conditions return same synthetic
// Forward: FailClose (SRC is checked)
// Inverse: FailOpen (same error always returned)
//
//go:noinline
func analyzeErrorInverse08_SameSyntheticFailOpen() (string, error) {
	s, err := anRPCCall()
	defaultErr := errors.New("default")
	if err != nil {
		return s, defaultErr
	}
	return s, defaultErr
}

// InverseTest9: Error override pattern
// Forward: FailClose (SRC triggers override)
// Inverse: FailOpen (override produces non-nil)
//
//go:noinline
func analyzeErrorInverse09_ErrorOverrideFailOpen() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		err = errors.New("overridden: " + err.Error()) // New error
	} else {
		err = errors.New("default error")
	}
	return s, err // Always non-nil
}

// InverseTest10: Merge with err2 that's always non-nil
// Forward: FailClose (SRC is in condition)
// Inverse: FailOpen (err2 is always set)
//
//go:noinline
func analyzeErrorInverse10_Err2AlwaysSetFailOpen() (string, error) {
	s, err1 := anRPCCall()
	err2 := errors.New("always set")
	if err1 != nil {
		return s, err1
	}
	return s, err2 // Always non-nil
}

// InverseTest11: Guard with else-if chain
// Forward: FailClose (SRC guards first return)
// Inverse: FailOpen (else-if always produces error)
//
//go:noinline
func analyzeErrorInverse11_GuardElseIfFailOpen() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, err
	} else if s == "" {
		return s, errors.New("empty")
	} else if len(s) > 100 {
		return s, errors.New("too long")
	}
	return s, errors.New("other error")
}

// InverseTest12: Double wrap
// Forward: FailOpen (wrapping nil creates non-nil)
// Inverse: FailOpen (wrapping creates non-nil)
//
//go:noinline
func analyzeErrorInverse12_DoubleWrapFailOpen() (string, error) {
	s, err := anRPCCall()
	wrapped := fmt.Errorf("wrap1: %w", err)
	doubleWrapped := fmt.Errorf("wrap2: %w", wrapped)
	if doubleWrapped != nil { // Always true
		return s, doubleWrapped
	}
	return s, nil
}

// InverseTest13: Priority error selection
// Forward: FailClose (SRC has priority)
// Inverse: FailOpen (one of them is always non-nil)
//
//go:noinline
func analyzeErrorInverse13_PrioritySelectionFailClose() (string, error) {
	s, err1 := anRPCCall()
	_, err2 := someCall()
	var result error
	if err1 != nil {
		result = err1 // Priority
	} else if err2 != nil {
		result = err2
	} else {
		result = errors.New("both nil")
	}
	return s, result // Always non-nil
}

// InverseTest14: Conditional synthetic in both branches
// Forward: FailClose (SRC determines branch)
// Inverse: FailOpen (both branches create new error)
//
//go:noinline
func analyzeErrorInverse14_ConditionalSyntheticFailOpen() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, errors.New("error occurred")
	}
	return s, errors.New("no error occurred")
}

// InverseTest15: Accumulator pattern
// Forward: FailOpen (errs always set to new error)
// Inverse: FailOpen (accumulator always non-nil)
//
//go:noinline
func analyzeErrorInverse15_AccumulatorFailOpen() (string, error) {
	s, err := anRPCCall()
	var errs error
	if err != nil {
		errs = err
	}
	// Add more errors
	errs = errors.New("additional error")
	return s, errs
}

// SCCP24: Loop Phi widening — tests SSA worklist terminator re-evaluation.
//
// The loop reassigns localErr to an untracked call result, creating a Phi at
// the loop header: Phi(err[NonNil], someProcessing()[Top]) = Top.
//
// Without the fix, the Phi widens from NonNil to Top via the SSA worklist,
// but the If terminator for `localErr != nil` is never re-evaluated because
// terminators were filtered from propagateToUses. The false branch stays
// incorrectly unreachable, hiding the FailClose return path.
//
// Forward: FailClose (return s, err is reachable when localErr is nil after loop)
//
//go:noinline
func analyzeErrorSCCP24LoopPhiWideningFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		localErr := err
		for i := 0; i < 3; i++ {
			localErr = someProcessing()
		}
		if localErr != nil {
			return s, nil // Swallow: localErr is non-nil but we return nil
		}
		// OLD: unreachable (localErr stuck at NonNil, If never re-evaluated)
		// NEW: reachable (localErr widened to Top → boolTop → both branches)
		return s, err // Return original error → FailClose
	}
	return s, nil
}

// SCCP25: == nil widening — tests boolFalse → boolTop path.
//
// SCCP24 tests `!= nil` (boolTrue → boolTop). This tests `== nil`
// (boolFalse → boolTop). The FailClose path is in the TRUE branch of
// `== nil`, which is initially infeasible because NonNil == nil is boolFalse.
//
//go:noinline
func analyzeErrorSCCP25EqNilLoopWideningFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		localErr := err
		for i := 0; i < 3; i++ {
			localErr = someProcessing()
		}
		if localErr == nil {
			// OLD: unreachable (NonNil == nil → boolFalse → false branch only)
			// NEW: reachable (Top == nil → boolTop → both branches)
			return s, err
		}
		return s, nil
	}
	return s, nil
}

// SCCP26: Control flow widening — If re-evaluation makes a new block
// executable, cascading into a Phi merge that flips the result.
//
// The loop widens localErr. The `if localErr != nil` controls which value
// flows into retErr's Phi. Initially only the nil assignment is reachable.
// After widening, the else branch (retErr = err) becomes executable,
// the merge block re-processes, and retErr widens from Nil to Top.
//
// This tests the full cascade: SSA worklist → If → cfgWorklist → new block
// → Phi merge change → classification flip.
//
//go:noinline
func analyzeErrorSCCP26ControlFlowWideningFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		localErr := err
		for i := 0; i < 3; i++ {
			localErr = someProcessing()
		}
		var retErr error
		if localErr != nil {
			retErr = nil
		} else {
			retErr = err
		}
		// OLD: retErr = Phi(nil) = Nil — only true branch was feasible → FailOpen
		// NEW: retErr = Phi(nil, err) = Top with err sources → FailClose
		return s, retErr
	}
	return s, nil
}

// SCCP27: Sequential widening checks — two independent loops each widen
// a different error, and both must widen before the combined condition
// resolves. Tests that the SSA worklist correctly propagates through
// multiple independent widening chains.
//
//go:noinline
func analyzeErrorSCCP27SequentialWideningFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		a := err
		for i := 0; i < 3; i++ {
			a = someProcessing()
		}
		// a widened to Top

		if a != nil {
			// OLD: always taken (a stuck at NonNil)
			b := err
			for j := 0; j < 3; j++ {
				b = someProcessing()
			}
			// b widened to Top
			if b != nil {
				return s, nil // Swallow when both non-nil
			}
			// b nil path — reachable regardless, since b widens within this block
			return s, err // FailClose path 1
		}
		// OLD: unreachable (a stuck at NonNil, outer If never re-evaluated)
		// NEW: reachable (a widened to Top → boolTop)
		return s, err // FailClose path 2
	}
	return s, nil
}

// =============================================================================
// Short-Circuit AND/OR Test Cases
// =============================================================================
// These test the detectShortCircuitPhi logic for compound nil checks.

// SCCP28: AND condition — both err1 and err2 checked, return err1 in true branch
// Go SSA decomposes this into control flow + Phi. With detectShortCircuitPhi,
// the true branch knows both err1 and err2 are non-nil.
//
//go:noinline
func analyzeErrorSCCP28AndShortCircuitFailClose() (string, error) {
	s, err1 := anRPCCall()
	_, err2 := someCall()
	if err1 != nil && err2 != nil {
		return s, err1
	}
	return s, nil
}

// SCCP29: OR condition false branch — both err1 and err2 are nil on false branch
// On the false branch of (err1 != nil || err2 != nil), De Morgan tells us
// both are nil. So returning err1 on that branch should be FailOpen.
//
//go:noinline
func analyzeErrorSCCP29OrShortCircuitFalseBranchFailOpen() (string, error) {
	s, err1 := anRPCCall()
	_, err2 := someCall()
	if err1 != nil || err2 != nil {
		return s, nil
	}
	// Both err1 and err2 are nil here (De Morgan)
	return s, err1
}

// SCCP30: Nested AND — three error checks combined with &&
// err1 != nil && err2 != nil && err3 != nil → true branch: all non-nil
//
//go:noinline
func analyzeErrorSCCP30NestedAndFailClose() (string, error) {
	s, err1 := anRPCCall()
	_, err2 := someCall()
	err3 := someProcessing()
	if err1 != nil && err2 != nil && err3 != nil {
		return s, err1
	}
	return s, nil
}

// SCCP31: OR condition true branch — at least one is non-nil, return err1
// On the true branch of (err1 != nil || err2 != nil), we don't know which one.
// But err1 may still be non-nil → FailClose with MaybeAnalysisMode.
//
//go:noinline
func analyzeErrorSCCP31OrShortCircuitTrueBranchFailClose() (string, error) {
	s, err1 := anRPCCall()
	_, err2 := someCall()
	if err1 != nil || err2 != nil {
		return s, err1
	}
	return s, nil
}

//go:noinline
func noop() {}

// Defer causes Go SSA to use allocs for return values (store-then-load pattern).
// When err is non-nil, the function returns nil error → fail-open.
//
//go:noinline
func analyzeErrorSCCP32DeferSwallowFailOpen() (string, error) {
	defer noop()
	s, err := anRPCCall()
	if err != nil {
		return "", nil
	}
	return s, nil
}

// Defer with error propagation → fail-close.
//
//go:noinline
func analyzeErrorSCCP33DeferPropagateFailClose() (string, error) {
	defer noop()
	s, err := anRPCCall()
	if err != nil {
		return s, err
	}
	return s, nil
}

// Defer with error wrapping → fail-close.
//
//go:noinline
func analyzeErrorSCCP34DeferWrapFailClose() (string, error) {
	defer noop()
	s, err := anRPCCall()
	if err != nil {
		return s, fmt.Errorf("wrapped: %w", err)
	}
	return s, nil
}

// Defer with multiple error checks, first swallowed, second propagated.
// The callee is anRPCCall (first call). Its error is swallowed → fail-open.
//
//go:noinline
func analyzeErrorSCCP35DeferMultiCallSwallowFirstFailOpen() (string, error) {
	defer noop()
	_, err := anRPCCall()
	if err != nil {
		return "", nil
	}
	s, err2 := someCall()
	if err2 != nil {
		return s, err2
	}
	return s, nil
}

// Defer with unconditional error return → fail-close.
//
//go:noinline
func analyzeErrorSCCP36DeferUnconditionalFailClose() (string, error) {
	defer noop()
	s, err := anRPCCall()
	return s, err
}

// SCCP37: OtherSource CDG walk — terrablob pattern.
// someCall() (ProvisionDatastore) is called first, its error is checked and returned.
// anRPCCall() (GetDatastoreInfo, the source) is called after on the success path.
// The source's error (err2) is never returned — all paths return nil or someCall's error.
// Without OtherSources: CDG walk from `return "", err1` passes the untracked someCall
// nil check and incorrectly reaches anRPCCall's err2==nil check → false FailClose.
// With OtherSources: someCall is tracked, its nil check stops the CDG walk → FailOpen.
//
//go:noinline
func analyzeErrorSCCP37OtherSourceCDGWalkFailOpen() (string, error) {
	_, err1 := someCall() // ProvisionDatastore analog
	if err1 != nil {
		return "", err1 // returns someCall's error → FailOpen w.r.t. anRPCCall
	}
	s, err2 := anRPCCall() // GetDatastoreInfo analog (source)
	if err2 == nil {
		return s, nil
	}
	return "", nil // source error not propagated → FailOpen
}

//go:noinline
func errorConstructor(code int, format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}

//go:noinline
func analyzeErrorSCCP38VarargIfaceWrapFailClose() (string, error) {
	s, err := anRPCCall()
	if err != nil {
		return s, errorConstructor(1, "wrapped: %v", err)
	}
	return s, nil
}

// SCCP39: err == nil guard with non-nil error creator — terrablob ProvisionDatastore pattern.
// anRPCCall() (GetDatastoreInfo) is checked with `err == nil` (inverted nil check).
// On the success path (err == nil), a synthetic non-nil error is returned.
// Forward: CDG walk attributes to Src through the `err == nil` else branch.
// Inverse (Src=Nil): `err == nil` is true, so only the if-body is reachable.
// The if-body returns errors.New(...) which is proven non-nil → inverse fails → FailOpen.
//
//go:noinline
func analyzeErrorSCCP39EqNilNonNilCreatorFailOpen() (string, error) {
	s, err := anRPCCall()
	if err == nil {
		return s, errors.New("already exists")
	}
	return s, nil
}

//go:noinline
func skipCheck() bool {
	return false
}

// SCCP40: Mimics freight-money-kafka ProcessPayerRequirements → checkLineItems pattern.
// Multiple error-returning calls before the Src, each checked with if err != nil { return }.
// Then Src is inside a nested conditional: if met && cond { if skipCheck() { ... } else { src } }
// The error from Src is wrapped with fmt.Errorf and returned.
// This should be FailClose.
//
//go:noinline
func analyzeErrorSCCP40NestedConditionalSrcFailClose() (string, error) {
	met := true

	// First error call (OtherSource) - like isPayerOrderStatusReady
	s1, err := someCall()
	if err != nil {
		return "", fmt.Errorf("call1 failed: %w", err)
	}
	met = met && (s1 != "")

	// Second error call (OtherSource) - like getAccessorialStatus
	s2, err := someCall()
	if err != nil {
		return "", fmt.Errorf("call2 failed: %w", err)
	}
	met = met && (s2 != "")

	// Third error call (OtherSource) - like baseRateAccurate
	s3, err := someCall()
	if err != nil {
		return "", err
	}
	met = met && (s3 != "")

	// Nested conditional with skip check — like the TMSLineItemsMatchReq pattern
	if met && s1 > "a" {
		if skipCheck() {
			// skip case — no error
		} else {
			// Src call — like checkLineItems
			s1, err = anRPCCall()
			if err != nil {
				return "", fmt.Errorf("check failed: %w", err)
			}
			met = met && (s1 != "")
		}
	}

	_ = s3
	return s1, nil
}

// SCCP41: Same as SCCP40 but uses wrapError (which matches isWrapLike generic heuristic)
// instead of fmt.Errorf. This tests that generic wrap-like detection works for
// the nested conditional pattern.
//
//go:noinline
func analyzeErrorSCCP41NestedConditionalWrapFailClose() (string, error) {
	met := true

	// First error call (OtherSource)
	s1, err := someCall()
	if err != nil {
		return "", wrapError(err)
	}
	met = met && (s1 != "")

	// Second error call (OtherSource)
	s2, err := someCall()
	if err != nil {
		return "", wrapError(err)
	}
	met = met && (s2 != "")

	// Third error call (OtherSource)
	s3, err := someCall()
	if err != nil {
		return "", err
	}
	met = met && (s3 != "")

	// Nested conditional with skip check
	if met && s1 > "a" {
		if skipCheck() {
			// skip case
		} else {
			// Src call
			s1, err = anRPCCall()
			if err != nil {
				return "", wrapError(err)
			}
			met = met && (s1 != "")
		}
	}

	_ = s3
	return s1, nil
}

// evaluator mimics a controller struct with methods
type evaluator struct{}

//go:noinline
func (e *evaluator) rpcCall() (bool, error) { return true, nil }

//go:noinline
func (e *evaluator) otherRPCCall() (bool, error) { return true, nil }

//go:noinline
func (e *evaluator) pureCheck() bool { return true }

//go:noinline
func (e *evaluator) skipReq() bool { return false }

type result struct {
	Met  bool
	Data string
}

// SCCP42: Mimics the exact pattern of ProcessPayerRequirements → checkLineItems.
// - Method receiver (struct, not standalone)
// - 3 OtherSource method calls with error checks
// - 8 `met = met && x` short-circuit evaluations between calls
// - Nested conditional: if met && cond { if skip { } else { src } }
// - Returns struct, not string
// - Uses wrapError (generic wrap-like) not fmt.Errorf
//
//go:noinline
func (e *evaluator) analyzeErrorSCCP42RealisticMethodFailClose(flag1, flag2 bool, status string) (result, error) {
	met := true

	// First conditional (like TMSLoadCompletedReq) - non-error check
	if flag1 && status != "" {
		if e.skipReq() {
			// skip
		} else {
			completed := status == "COMPLETED"
			met = met && completed
		}
	}

	// OtherSource 1: like isPayerOrderStatusReady
	ready, err := e.otherRPCCall()
	if err != nil {
		return result{}, wrapError(err)
	}
	met = met && ready

	// Pure check (no error)
	met = met && flag2

	// Non-error check (like OrderRecencyReq)
	met = met && e.pureCheck()

	// Non-error check (like HasBusinessFacilitiesReq)
	met = met && e.pureCheck()

	// OtherSource 2: like getAccessorialStatus
	ok2, err := e.otherRPCCall()
	if err != nil {
		return result{}, wrapError(err)
	}
	met = met && ok2

	// OtherSource 3: like baseRateAccurate (returns err directly, not wrapped)
	ok3, err := e.otherRPCCall()
	if err != nil {
		return result{}, err
	}
	met = met && ok3

	// Non-error check (like UnlockedReq)
	met = met && e.pureCheck()

	// Non-error check (like InvoiceNotOnHoldReq)
	met = met && e.pureCheck()

	// Nested conditional with skip check — like TMSLineItemsMatchReq
	if met && flag1 && status != "" {
		if e.skipReq() {
			// skip case
		} else {
			// Src call — like checkLineItems
			met, err = e.rpcCall()
			if err != nil {
				return result{}, wrapError(err)
			}
		}
	}

	return result{Met: met, Data: "done"}, nil
}

func main() {
	analyzeErrorSimpleFailClose()
	analyzeErrorSimpleFailOpen()
	analyzeErrorCond1FailClose()
	analyzeErrorCond2FailOpen()
	analyzeErrorCond3FailClose()
	analyzeErrorCond4FailOpen()
	analyzeErrorCond5FailOpen()
	analyzeErrorCondNested1FailUnknown()
	analyzeErrorCondNested2FailClose()
	analyzeErrorCondNested3FailOpen()
	analyzeErrorCondNested4FailOpen()
	analyzeErrorCondNested5FailClose()
	analyzeErrorCondNested6FailClose()
	analyzeErrorCorner1Unknown()
	analyzeErrorCorner2Unknown()
	analyzeErrorCorner3MultiBinOpUnknown()
	analyzeErrorCorner4NonNilComparisonUnknown()
	analyzeErrorCorner5OtherInstructionsUnknown()
	analyzeErrorCorner6MultiIfUnknown()
	analyzeErrorPredicatedReturn1FailClose()
	analyzeErrorPredicatedReturn2FailOpen()
	analyzeErrorPredicatedReturn3FailOpen(1, 2)
	analyzeErrorPredicatedReturn4FailOpen(1, 10)
	analyzeErrorPredicatedReturn5FailClose(1, 10)
	analyzeErrorPredicatedReturn6FailClose()
	analyzeErrorPredicatedReturn7FailOpen()
	analyzeErrorPredicatedReturn8FailClose()
	analyzeErrorPredicatedReturn9Unknown(func() error { return nil })
	analyzeErrorSimpleButFailOpen()
	analyzeErrorNoerrorFailOpen()
	analyzeErrorPredicatedReturnOffIndexTest1FailClose()
	analyzeErrorPredicatedReturnOffIndexTest2FailClose()
	analyzeErrorPredicatedReturnOffIndexTest3FailOpen()
	analyzeErrorPredicatedReturnOffIndexTest4FailClose()
	analyzeErrorSCCPMd5EqNilRetNewErrFailOpen()
	analyzeErrorSCCPMd6WrapRetFailClose()
	analyzeErrorSCCPMd7OrCondRetErr2FailOpen()
	analyzeErrorSCCPMd8AndCondRetErrFailClose()
	analyzeErrorSCCPMd11AlwaysFailsFailOpen()
	analyzeErrorSCCPMd13NestedGuardRetErr2FailOpen()
	analyzeErrorSCCPMd14NestedGuardRetErrFailClose()
	analyzeErrorSCCPMd15NestedGuardWrapFailClose()
	analyzeErrorSCCPMd16DerivedRetFailClose()
	analyzeErrorSCCPMd17DerivedGuardRetDerivedFailClose()
	analyzeErrorSCCPMd18DerivedGuardRetErrFailClose()
	analyzeErrorSCCPMd19DerivedGuardRetWrapFailClose()
	analyzeErrorSCCPMd20DerivedGuardGRetErr2FailOpen()
	analyzeErrorSCCPMd21BoolAliasNegationFailClose()
	analyzeErrorSCCPMd22BoolPhiAliasFailClose()
	// SCCP tests
	analyzeErrorSCCP1SimpleDirectFailClose()
	analyzeErrorSCCP2NestedSuccessBranchFailClose()
	analyzeErrorSCCP3NestedErrorBranchFailOpen()
	analyzeErrorSCCP4DeepNestedSuccessFailClose()
	analyzeErrorSCCP5SequentialSuccessFailClose()
	analyzeErrorSCCP6BothFailInnerClaimsFailOpen()
	analyzeErrorSCCP7NestedSuccessWrapFailClose()
	analyzeErrorSCCP8TripleNestedSuccessFailClose()
	analyzeErrorSCCP9SrcSuccessBranchFailOpen()
	analyzeErrorSCCP10UnconditionalSyntheticFailClose()
	analyzeErrorSCCP11ChainedWrapFailClose()
	analyzeErrorSCCP12WrapSyntheticFailClose()
	analyzeErrorSCCP13LoopWithErrorCheckFailClose()
	analyzeErrorSCCP14ThreeSequentialFailClose()
	analyzeErrorSCCP15Err2ClaimsThenErr1FailOpen()
	analyzeErrorSCCP16ElseBranchSyntheticFailOpen()
	analyzeErrorSCCP17DoubleCheckFailClose()
	analyzeErrorSCCP18ErrorAliasFailClose()
	analyzeErrorSCCP19OrConditionFailClose()
	analyzeErrorSCCP20ReassignmentFailOpen()
	analyzeErrorSCCP21ReturnNilErr1InErr2BranchFailOpen()
	analyzeErrorSCCP22AssertionNonNilFailClose()
	analyzeErrorSCCP23ShortCircuitOrFailClose()
	// Inverse check tests
	analyzeErrorInverse01_BothBranchesNonNilFailOpen()
	analyzeErrorInverse02_Err2DominatesFailOpen()
	analyzeErrorInverse03_UnconditionalWrapFailOpen()
	analyzeErrorInverse04_ThreeWayMergeFailOpen()
	analyzeErrorInverse05_DefensiveValidationFailClose()
	analyzeErrorInverse06_MaskedByWrapperFailOpen()
	analyzeErrorInverse07_FallbackChainFailClose()
	analyzeErrorInverse08_SameSyntheticFailOpen()
	analyzeErrorInverse09_ErrorOverrideFailOpen()
	analyzeErrorInverse10_Err2AlwaysSetFailOpen()
	analyzeErrorInverse11_GuardElseIfFailOpen()
	analyzeErrorInverse12_DoubleWrapFailOpen()
	analyzeErrorInverse13_PrioritySelectionFailClose()
	analyzeErrorInverse14_ConditionalSyntheticFailOpen()
	analyzeErrorInverse15_AccumulatorFailOpen()
	// Loop Phi widening test
	analyzeErrorSCCP24LoopPhiWideningFailClose()
	analyzeErrorSCCP25EqNilLoopWideningFailClose()
	analyzeErrorSCCP26ControlFlowWideningFailClose()
	analyzeErrorSCCP27SequentialWideningFailClose()
	// Short-circuit AND/OR tests
	analyzeErrorSCCP28AndShortCircuitFailClose()
	analyzeErrorSCCP29OrShortCircuitFalseBranchFailOpen()
	analyzeErrorSCCP30NestedAndFailClose()
	analyzeErrorSCCP31OrShortCircuitTrueBranchFailClose()
	// Defer store-load tests
	analyzeErrorSCCP32DeferSwallowFailOpen()
	analyzeErrorSCCP33DeferPropagateFailClose()
	analyzeErrorSCCP34DeferWrapFailClose()
	analyzeErrorSCCP35DeferMultiCallSwallowFirstFailOpen()
	analyzeErrorSCCP36DeferUnconditionalFailClose()
	analyzeErrorSCCP37OtherSourceCDGWalkFailOpen()
	// ChangeInterface + varargs interface{} boxing test
	analyzeErrorSCCP38VarargIfaceWrapFailClose()
	// err == nil guard with non-nil creator (terrablob pattern)
	analyzeErrorSCCP39EqNilNonNilCreatorFailOpen()
	analyzeErrorSCCP40NestedConditionalSrcFailClose()
	analyzeErrorSCCP41NestedConditionalWrapFailClose()
	ev := &evaluator{}
	ev.analyzeErrorSCCP42RealisticMethodFailClose(true, true, "COMPLETED")
	analyzeErrorSCCP43LoopSrcFailClose(map[string]int{"a": 1})
	analyzeErrorSCCP44GenericWrapperFailClose()
	analyzeErrorSCCP45NonGenericWrapperFailClose()
	// Additional permutations for coverage
	analyzeErrorPermute66_ShortCircuitOrFailClose()
	analyzeErrorPermute67_ShortCircuitAndBothNilCheckFailClose()
	analyzeErrorPermute68_NegatedNilCheckFailClose()
	analyzeErrorPermute69_FmtErrorfDirectFailClose()
	analyzeErrorPermute70_InterfaceWrapFailClose()
	analyzeErrorPermute71_ThreeSourcesAllCheckedFailClose()
	analyzeErrorPermute72_VarargsJoinFailClose()
	analyzeErrorPermute73_DeadCodeBranchFailClose()
	analyzeErrorPermute74_NonWrapInvokeFailClose()
	analyzeErrorPermute75_OtherSourceNilAtReturnFailClose()
	analyzeErrorPermute76_MultiOtherSourceAllCheckedFailClose()
	analyzeErrorPermute77_StrictCDGProofFailClose()
	analyzeErrorPermute78_ThreeWayOrTrueBranchFailClose()
	analyzeErrorPermute79_ThreeWayAndFalseBranchFailClose()
	analyzeErrorPermute80_TypeAssertPassthroughFailClose()
	analyzeErrorPermute81_MapLookupBoolFailClose()
	analyzeErrorPermute82_ChangeTypeFailClose()
	analyzeErrorPermute83_SingleReturnSourceFailClose()
	analyzeErrorPermute84_StoreLoadFailClose()
	analyzeErrorPermute85_NamedReturnDeferFailClose()
	analyzeErrorPermute86_NamedReturnMultiPathFailClose()
}

// SCCP43: Src call inside a for-range loop. Error from anRPCCall is checked
// and returned immediately. Tests that loop back-edges don't cause false FailOpen.
//
//go:noinline
func analyzeErrorSCCP43LoopSrcFailClose(items map[string]int) (string, error) {
	var results []string
	for key := range items {
		val, err := anRPCCall()
		if err != nil {
			return "", err
		}
		results = append(results, key+val)
	}
	return strings.Join(results, ","), nil
}

// transformResponse mimics the generic helper pattern from service-0060.
// Signature: takes (*T, error), returns (*T, error) — wrap-like by generic heuristic.
//
//go:noinline
func transformResponse[T any](resp *T, err error) (*T, error) {
	if resp == nil {
		resp = new(T)
	}
	return resp, err
}

// SCCP44: Return through a generic wrapper function.
// `transformResponse` takes (resp, err) and returns (resp, err) — wrap-like.
// The error flows through the wrapper and should be attributed to Src.
//
//go:noinline
func analyzeErrorSCCP44GenericWrapperFailClose() (*string, error) {
	s, err := anRPCCall()
	return transformResponse(&s, err)
}

// SCCP45: Same as SCCP44 but with a non-generic wrapper (control case).
//
//go:noinline
func passthroughResponse(s string, err error) (string, error) {
	return s, err
}

//go:noinline
func analyzeErrorSCCP45NonGenericWrapperFailClose() (string, error) {
	s, err := anRPCCall()
	return passthroughResponse(s, err)
}

// NilOtherSrc01: Src (anRPCCall) comes before OtherSrc (someCall) in the same
// straight-line code. With NilOtherSourcesAfterSrc, someCall's error is Nil
// (it appears after Src), so "err2 != nil && err1 != nil" is always false
// on err2 → FailOpen. Without NilOtherSourcesAfterSrc, err2 is Top → guard
// can fire → FailClose.
//
//go:noinline
func analyzeErrorSCCPNilOtherSrc01_SrcThenOtherGuardBothFailOpen() (string, error) {
	_, err1 := anRPCCall()
	_, err2 := someCall()
	if err2 != nil && err1 != nil {
		return "", errors.New("both failed")
	}
	return "", nil
}

// NilOtherSrc02: OtherSrc (someCall) comes before Src (anRPCCall).
// OtherSrc is before Src in the CFG → OtherSrc stays Top regardless of
// NilOtherSourcesAfterSrc → FailClose in both modes.
//
//go:noinline
func analyzeErrorSCCPNilOtherSrc02_OtherThenSrcGuardBothFailClose() (string, error) {
	_, err2 := someCall()
	_, err1 := anRPCCall()
	if err2 != nil && err1 != nil {
		return "", errors.New("both failed")
	}
	return "", nil
}

// NilOtherSrc03: Src first, then OtherSrc. Return checks only OtherSrc error.
// With NilOtherSourcesAfterSrc: err2 is Nil → nil-check always false →
// return nil → FailOpen. Without it: err2 is Top → can be non-nil → FailClose.
//
//go:noinline
func analyzeErrorSCCPNilOtherSrc03_SrcThenOtherReturnOtherErrFailOpen() (string, error) {
	_, _ = anRPCCall()
	_, err2 := someCall()
	if err2 != nil {
		return "", err2
	}
	return "", nil
}

// NilOtherSrc04: Src first, return src error directly (OtherSrc irrelevant).
// FailClose regardless of NilOtherSourcesAfterSrc — the src error propagates
// independently of OtherSrc's nilness.
//
//go:noinline
func analyzeErrorSCCPNilOtherSrc04_SrcThenOtherReturnSrcErrFailClose() (string, error) {
	_, err1 := anRPCCall()
	_, _ = someCall()
	if err1 != nil {
		return "", err1
	}
	return "", nil
}

// NilOtherSrc05: Src in entry block, OtherSrc after a branch on a non-src
// condition (different block). Src's block dominates OtherSrc's block →
// Nil with NilOtherSourcesAfterSrc. Guard uses OtherSrc's error →
// FailClose without flag (err2 is Top), FailOpen with flag (err2 is Nil).
//
//go:noinline
func analyzeErrorSCCPNilOtherSrc05_SrcDominatesOtherBranchFailOpen() (string, error) {
	s, err1 := anRPCCall()
	if len(s) > 0 {
		_, err2 := someCall()
		if err2 != nil && err1 != nil {
			return "", errors.New("both failed")
		}
	}
	return "", nil
}

// NilOtherSrc06: OtherSrc in entry block, Src after a branch on a non-src
// condition (different block). OtherSrc's block dominates Src's block →
// Top regardless of NilOtherSourcesAfterSrc → FailClose in both modes.
//
//go:noinline
func analyzeErrorSCCPNilOtherSrc06_OtherDominatesSrcBranchFailClose() (string, error) {
	s, err2 := someCall()
	if len(s) > 0 {
		_, err1 := anRPCCall()
		if err2 != nil && err1 != nil {
			return "", errors.New("both failed")
		}
	}
	return "", nil
}

// NilOtherSrc07: Src in a conditional branch, OtherSrc in a join block
// reachable from both branches. Neither block dominates the other, but
// OtherSrc is a transitive successor of Src → Nil with NilOtherSourcesAfterSrc.
// Guard uses OtherSrc's error →
// FailClose without flag (err2 is Top), FailOpen with flag (err2 is Nil).
// Exercises the CFG reachability fallback in classifyOtherSource.
//
//go:noinline
func analyzeErrorSCCPNilOtherSrc07_ReachableNotDominatedFailOpen() (string, error) {
	s, _ := someCall()
	var err1 error
	if len(s) > 5 {
		_, err1 = anRPCCall()
	}
	// Join block: reachable from both branches, so Src's block does not dominate.
	_, err2 := someCall()
	if err2 != nil && err1 != nil {
		return "", errors.New("both failed")
	}
	return "", nil
}

// NilOtherSrc08: Src and OtherSrc in separate branches of an if-else.
// Neither block dominates the other and neither is reachable from the other →
// Top regardless of NilOtherSourcesAfterSrc → FailClose in both modes.
// Exercises the mutual-unreachability return in classifyOtherSource.
//
//go:noinline
func analyzeErrorSCCPNilOtherSrc08_MutuallyUnreachableFailClose() (string, error) {
	s, _ := someCall()
	var err1, err2 error
	if len(s) > 5 {
		_, err1 = anRPCCall()
	} else {
		_, err2 = someCall()
	}
	if err1 != nil || err2 != nil {
		return "", errors.New("one failed")
	}
	return "", nil
}
