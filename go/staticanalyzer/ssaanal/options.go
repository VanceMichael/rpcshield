package ssaanal

import (
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
)

// SCCPOptions controls the behavior of the SCCP-based fail-close analyzer.
type SCCPOptions struct {
	// MaybeAnalysisMode controls how unknown (Top) nilness values are handled
	// in both forward and inverse analysis:
	//   - false (strict): Requires proof of the analysis condition.
	//     Unknown nilness fails the check.
	//   - true (lenient): Unknown nilness is treated as satisfying the
	//     analysis condition (maybe non-nil in forward, maybe nil in inverse).
	// Default: true (lenient mode)
	MaybeAnalysisMode bool

	// InverseCheck controls whether Phase 4 (Inverse Plausibility) runs.
	// When true:
	//   1. Seed Src as Nil (instead of NonNil)
	//   2. Rerun SCCP from scratch
	//   3. Check if at least one return path has MaybeNil error
	//   4. Declare FailClose only when both forward and inverse analyses pass
	// Phase 4 confirms that Src genuinely controls the error outcome.
	// Default: false
	InverseCheck bool

	// ForwardPathCheck, when true, adds path reachability checking in forward mode.
	// Only considers returns reachable from Src via feasible edges, filtering out
	// returns in executable blocks that are not on any feasible path from Src.
	// Default: false
	ForwardPathCheck bool

	// OtherSources are other error-producing calls in the function that are
	// not the source being analyzed. Their error values are extracted via
	// getErrorValue and registered as root sources with Top nilness. During
	// attribution, data sources on the returned error are checked: if an
	// OtherSource contributed, its nilness fact in the return block must be
	// Nil to rule it out. Error creators (errors.New, fmt.Errorf) are
	// registered separately as synthetics and need not be listed here.
	OtherSources []*ssa.Call

	// Logger is an optional zap logger for debug-level logging of SCCP analysis steps.
	// When nil, no logging is performed.
	Logger *zap.SugaredLogger

	// Metrics is an optional aggregate metrics collector. When non-nil, each
	// AnalyzeCallBySCCPWithOptions call updates it in place. Nil-safe: all
	// increment methods are no-ops on nil receiver.
	Metrics *SCCPAggregateMetrics

	// MaxCDGPaths limits the number of inverse CDG paths enumerated per block.
	// Functions with deeply nested or diamond-shaped control flow can cause
	// exponential path explosion. When set to a positive value, path enumeration
	// stops after collecting this many paths. 0 means no limit.
	// Default: 10000
	MaxCDGPaths int

	// NilOtherSourcesAfterSrc, when true, sets OtherSources to Nil
	// (instead of Top) when the OtherSource appears after Src in the CFG.
	// Dominance is used as a fast path; CFG reachability is the fallback.
	//
	// The formal definition of fail-close requires that all RPCs invoked
	// after the one under analysis succeed. Setting OtherSources after Src
	// to Nil simulates this by assuming their error returns are nil
	// (i.e., they succeed).
	// Default: false
	NilOtherSourcesAfterSrc bool

	// KnownErrorConstructorFuncs lists fully qualified function names (e.g. "pkgpath.FuncName")
	// treated as definitely constructing a non-nil error regardless of the nilness of their
	// error-typed arguments, on top of the built-in "fmt.Errorf" detection. Empty adds nothing.
	KnownErrorConstructorFuncs []string
}

// DefaultSCCPOptions returns the default options for SCCP analysis.
func DefaultSCCPOptions() SCCPOptions {
	return SCCPOptions{
		MaybeAnalysisMode: true,
		InverseCheck:      false,
		ForwardPathCheck:  false,
		OtherSources:      nil,
		MaxCDGPaths:       10000,
	}
}
