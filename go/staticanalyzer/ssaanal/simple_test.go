package ssaanal

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/loader"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func loadAnal(analPath []string, loadPath string) (entryPkg *packages.Package, allPkgs []*packages.Package) {
	_, allPkgs, err := loader.LoadWithGoPath("rpcshield", analPath, loadPath)
	if err != nil {
		panic(err)
	}

	if len(allPkgs) == 0 {
		panic("no packages loaded for the app")
	}

	// Now find the entry package.
	for _, pkg := range allPkgs {
		if pkg.PkgPath == loadPath {
			return pkg, allPkgs
		}
	}

	panic("could not find the entry package for the app")
}

func loadAndInitBuilder(t *testing.T, pathSlice []string, loadPath string) ([]*ssa.Package, *ssa.Program) {
	_, allPkgs := loadAnal(pathSlice, loadPath)

	// Build SSA representation.
	prog, allSSAPkgs := ssautil.AllPackages(allPkgs, ssa.SanityCheckFunctions|ssa.GlobalDebug|ssa.InstantiateGenerics)
	prog.Build()
	return allSSAPkgs, prog
}

// getExpectedFromName extracts the expected Failness from a function name suffix.
func getExpectedFromName(name string) (constant.Failness, bool) {
	if strings.Contains(name, "FailClose") {
		return constant.FailClose, true
	}
	if strings.Contains(name, "FailOpen") {
		return constant.FailOpen, true
	}
	if strings.Contains(name, "Unknown") {
		return constant.FailUnknown, true
	}
	return "", false
}

// testFn holds a collected test function with its source call and other calls.
type testFn struct {
	fn         *ssa.Function
	srcCall    *ssa.Call
	otherCalls []*ssa.Call
}

// collectTestFunctions scans the SSA program for analyzeError* functions and
// extracts their anRPCCall (source) and someCall/RPC (other) calls.
func collectTestFunctions(ssaProg *ssa.Program) map[string]*testFn {
	collected := map[string]*testFn{}
	for fn := range ssautil.AllFunctions(ssaProg) {
		if !strings.HasPrefix(fn.Name(), "analyzeError") {
			continue
		}
		if _, ok := getExpectedFromName(fn.Name()); !ok {
			continue
		}
		pair := &testFn{fn: fn}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if call, ok := instr.(*ssa.Call); ok {
					callStr := call.Call.Value.String()
					if strings.Contains(callStr, "anRPCCall") && pair.srcCall == nil {
						pair.srcCall = call
					} else if (strings.Contains(callStr, "someCall") || strings.Contains(callStr, "RPC")) && !strings.Contains(callStr, "anRPCCall") {
						pair.otherCalls = append(pair.otherCalls, call)
					}
				}
			}
		}
		if pair.srcCall != nil {
			collected[fn.Name()] = pair
		}
	}
	return collected
}

// runSCCPTests runs AnalyzeCallBySCCPWithOptions on all collected functions using
// the given base options and overrides, and asserts the results as subtests.
func runSCCPTests(t *testing.T, collected map[string]*testFn, baseOpts SCCPOptions, overrides map[string]constant.Failness) {
	t.Helper()
	t.Logf("Collected %d test functions", len(collected))

	for fnName, pair := range collected {
		t.Run(fnName, func(t *testing.T) {
			opts := baseOpts
			opts.OtherSources = pair.otherCalls

			r, err := AnalyzeCallBySCCPWithOptions(pair.fn, pair.srcCall, opts)

			var expected constant.Failness
			if override, ok := overrides[fnName]; ok {
				expected = override
			} else {
				expected, _ = getExpectedFromName(fnName)
			}

			if expected == constant.FailUnknown {
				require.Equal(t, expected, r, "for %s", fnName)
				return
			}
			require.NoError(t, err, "for %s", fnName)
			require.Equal(t, expected, r, "for %s", fnName)
		})
	}
}

func TestErrorAnal(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	expected := map[*ssa.Function]ssa.CallInstruction{}

	for fn := range ssautil.AllFunctions(ssaProg) {
		if strings.HasPrefix(fn.Name(), "analyzeError") {
			// Skip SCCP-specific tests - they're tested separately in TestAnalyzeCallBySCCP.
			if strings.Contains(fn.Name(), "SCCP") || strings.Contains(fn.Name(), "Inverse") {
				continue
			}
			// Skip Permute tests - they're tested separately in TestAnalyzeCallBySCCP.
			if strings.Contains(fn.Name(), "Permute") {
				continue
			}
			fmt.Println(fn.Name())
			for _, block := range fn.Blocks {
				for _, instr := range block.Instrs {
					if call, ok := instr.(*ssa.Call); ok {
						if strings.Contains(call.Call.Value.String(), "anRPCCall") {
							expected[fn] = call
						}
					}
				}
			}
		}
	}

	for fn, call := range expected {
		fn.WriteTo(os.Stdout)
		r, err := AnalyzeCall(fn, call)
		switch {
		case strings.Contains(fn.Name(), "Unknown"):
			require.Equal(t, constant.FailUnknown, r)
		case strings.Contains(fn.Name(), "FailOpen"):
			require.NoError(t, err)
			require.Equal(t, constant.FailOpen, r)
		case strings.Contains(fn.Name(), "FailClose"):
			require.NoError(t, err)
			require.Equal(t, constant.FailClose, r)
		default:
			t.Fatalf("unexpected function name: %s", fn.Name())
		}
	}
}

func TestBoundaryCases(t *testing.T) {
	err := validateInput(nil, nil)
	require.Error(t, err)
}

// TestAnalyzeCall_NilInput exercises the early-return path in AnalyzeCall
// when validateInput fails (simple.go lines 16-17).
func TestAnalyzeCall_NilInput(t *testing.T) {
	r, err := AnalyzeCall(nil, nil)
	require.Error(t, err)
	require.Equal(t, constant.FailUnknown, r)
}

// TestAnalyzeCallBySCCPWithOptions_NilInput exercises the early-return path
// in AnalyzeCallBySCCPWithOptions when validateInput fails (sccp_analyze.go
// lines 2673-2676) and verifies IncFailUnknown is called.
func TestAnalyzeCallBySCCPWithOptions_NilInput(t *testing.T) {
	metrics := &SCCPAggregateMetrics{}
	r, err := AnalyzeCallBySCCPWithOptions(nil, nil, SCCPOptions{Metrics: metrics})
	require.Error(t, err)
	require.Equal(t, constant.FailUnknown, r)
	require.Equal(t, 1, metrics.TotalInvocations, "should increment TotalInvocations")
	require.Equal(t, 1, metrics.FailUnknownCount, "should increment FailUnknownCount")
}

func goSDKBinDir() string {
	runfilesRoot := os.Getenv("RUNFILES_DIR")
	if runfilesRoot == "" {
		runfilesRoot = os.Getenv("TEST_SRCDIR")
	}
	if runfilesRoot != "" {
		if goBin := filepath.Join(runfilesRoot, "go_sdk", "bin", "go"); fileExists(goBin) {
			return filepath.Dir(goBin)
		}
	}
	return filepath.Join(runtime.GOROOT(), "bin")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Tests requiring Go SDK in runtime need PATH fixed up due to
// https://github.com/bazelbuild/rules_go/issues/2370.
func TestMain(m *testing.M) {
	goBin := goSDKBinDir()
	pathOverride := goBin + string(os.PathListSeparator) + os.Getenv("PATH")
	os.Setenv("PATH", pathOverride)
	if os.Getenv("GOCACHE") == "" {
		if dir, err := os.MkdirTemp("", "ssaanal-gocache"); err == nil {
			os.Setenv("GOCACHE", dir)
		}
	}
	os.Exit(m.Run())
}

// TestAnalyzeCallBySCCP runs the SCCP-based analyzer (forward-only, no inverse check).
func TestAnalyzeCallBySCCP(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	collected := collectTestFunctions(ssaProg)

	sccpOverrides := map[string]constant.Failness{
		"analyzeErrorCorner1Unknown":                         constant.FailOpen,
		"analyzeErrorCorner2Unknown":                         constant.FailOpen,
		"analyzeErrorCondNested1FailUnknown":                 constant.FailOpen,
		"analyzeErrorCorner3MultiBinOpUnknown":               constant.FailOpen,
		"analyzeErrorCorner4NonNilComparisonUnknown":         constant.FailOpen,
		"analyzeErrorCorner5OtherInstructionsUnknown":        constant.FailOpen,
		"analyzeErrorCorner6MultiIfUnknown":                  constant.FailOpen,
		"analyzeErrorPredicatedReturn4FailOpen":              constant.FailClose,
		"analyzeErrorPredicatedReturn9Unknown":               constant.FailClose,
		"analyzeErrorPredicatedReturn3FailOpen":              constant.FailClose,
		"analyzeErrorSCCPMd11AlwaysFailsFailOpen":            constant.FailClose,
		"analyzeErrorInverse01_BothBranchesNonNilFailOpen":   constant.FailClose,
		"analyzeErrorInverse04_ThreeWayMergeFailOpen":        constant.FailClose,
		"analyzeErrorInverse08_SameSyntheticFailOpen":        constant.FailClose,
		"analyzeErrorInverse09_ErrorOverrideFailOpen":        constant.FailClose,
		"analyzeErrorInverse10_Err2AlwaysSetFailOpen":        constant.FailClose,
		"analyzeErrorInverse11_GuardElseIfFailOpen":          constant.FailClose,
		"analyzeErrorInverse14_ConditionalSyntheticFailOpen": constant.FailClose,
		"analyzeErrorInverse03_UnconditionalWrapFailOpen":    constant.FailClose,
		"analyzeErrorInverse06_MaskedByWrapperFailOpen":      constant.FailClose,
		"analyzeErrorInverse12_DoubleWrapFailOpen":           constant.FailClose,
		// NilOtherSrc tests: names encode NilOtherSourcesAfterSrc=true results; without it these differ.
		"analyzeErrorSCCPNilOtherSrc01_SrcThenOtherGuardBothFailOpen":   constant.FailClose,
		"analyzeErrorSCCPNilOtherSrc05_SrcDominatesOtherBranchFailOpen": constant.FailClose,
		"analyzeErrorSCCPNilOtherSrc07_ReachableNotDominatedFailOpen":   constant.FailClose,
		// Forward-only MaybeAnalysis says FailClose (error IS propagated when outbound fails),
		// but inverse check demotes to FailOpen (function always fails regardless of outbound).
		"analyzeErrorPermute101_NamedReturnStructErrBothBranchesFailOpen": constant.FailClose,
		"analyzeErrorPermute102_StructErrBothBranchesNoDeferFailOpen":     constant.FailClose,
	}

	metrics := &SCCPAggregateMetrics{}
	runSCCPTests(t, collected, SCCPOptions{
		MaybeAnalysisMode: true,
		Metrics:           metrics,
	}, sccpOverrides)

	// Verify metrics were populated by all invocations.
	require.Equal(t, len(collected), metrics.TotalInvocations, "TotalInvocations should match collected function count")
	require.True(t, metrics.FailCloseCount > 0, "should have at least one FailClose")
	require.True(t, metrics.FailOpenCount > 0, "should have at least one FailOpen")

	m := metrics.AsMap()
	require.NotNil(t, m)
	require.Equal(t, float64(metrics.TotalInvocations), m["sccp_total_invocations"])
	require.Equal(t, float64(metrics.FailCloseCount), m["sccp_failclose_count"])
	require.Equal(t, float64(metrics.FailOpenCount), m["sccp_failopen_count"])
}

// TestAnalyzeCallBySCCP_InverseCheck runs SCCP with InverseCheck enabled.
func TestAnalyzeCallBySCCP_InverseCheck(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	collected := collectTestFunctions(ssaProg)

	inverseOverrides := map[string]constant.Failness{
		"analyzeErrorCorner1Unknown":                  constant.FailOpen,
		"analyzeErrorCorner2Unknown":                  constant.FailOpen,
		"analyzeErrorCondNested1FailUnknown":          constant.FailOpen,
		"analyzeErrorCorner3MultiBinOpUnknown":        constant.FailOpen,
		"analyzeErrorCorner4NonNilComparisonUnknown":  constant.FailOpen,
		"analyzeErrorCorner5OtherInstructionsUnknown": constant.FailOpen,
		"analyzeErrorCorner6MultiIfUnknown":           constant.FailOpen,
		"analyzeErrorPredicatedReturn4FailOpen":       constant.FailClose,
		"analyzeErrorPredicatedReturn9Unknown":        constant.FailClose,
		"analyzeErrorPredicatedReturn3FailOpen":       constant.FailClose,
		// NilOtherSrc tests: names encode NilOtherSourcesAfterSrc=true results; without it these differ.
		"analyzeErrorSCCPNilOtherSrc01_SrcThenOtherGuardBothFailOpen":   constant.FailClose,
		"analyzeErrorSCCPNilOtherSrc05_SrcDominatesOtherBranchFailOpen": constant.FailClose,
		"analyzeErrorSCCPNilOtherSrc07_ReachableNotDominatedFailOpen":   constant.FailClose,
	}

	metrics := &SCCPAggregateMetrics{}
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	runSCCPTests(t, collected, SCCPOptions{
		MaybeAnalysisMode: true,
		InverseCheck:      true,
		Metrics:           metrics,
		Logger:            logger.Sugar(),
	}, inverseOverrides)

	// Verify inverse metrics were populated.
	require.Equal(t, len(collected), metrics.TotalInvocations, "TotalInvocations should match collected function count")
	require.True(t, metrics.InverseReached > 0, "inverse phase should have been reached at least once")

	m := metrics.AsMap()
	require.NotNil(t, m)
	require.True(t, m["sccp_inverse_reached"] > 0, "inverse_reached should be > 0")
}

// TestAnalyzeCallBySCCP_ForwardPathCheck runs SCCP with ForwardPathCheck enabled.
func TestAnalyzeCallBySCCP_ForwardPathCheck(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	collected := collectTestFunctions(ssaProg)

	// ForwardPathCheck should not change results for these functions
	// (it only filters unreachable returns).
	fpOverrides := map[string]constant.Failness{
		"analyzeErrorCorner1Unknown":                         constant.FailOpen,
		"analyzeErrorCorner2Unknown":                         constant.FailOpen,
		"analyzeErrorCondNested1FailUnknown":                 constant.FailOpen,
		"analyzeErrorCorner3MultiBinOpUnknown":               constant.FailOpen,
		"analyzeErrorCorner4NonNilComparisonUnknown":         constant.FailOpen,
		"analyzeErrorCorner5OtherInstructionsUnknown":        constant.FailOpen,
		"analyzeErrorCorner6MultiIfUnknown":                  constant.FailOpen,
		"analyzeErrorPredicatedReturn4FailOpen":              constant.FailClose,
		"analyzeErrorPredicatedReturn9Unknown":               constant.FailClose,
		"analyzeErrorPredicatedReturn3FailOpen":              constant.FailClose,
		"analyzeErrorSCCPMd11AlwaysFailsFailOpen":            constant.FailClose,
		"analyzeErrorInverse01_BothBranchesNonNilFailOpen":   constant.FailClose,
		"analyzeErrorInverse04_ThreeWayMergeFailOpen":        constant.FailClose,
		"analyzeErrorInverse08_SameSyntheticFailOpen":        constant.FailClose,
		"analyzeErrorInverse09_ErrorOverrideFailOpen":        constant.FailClose,
		"analyzeErrorInverse10_Err2AlwaysSetFailOpen":        constant.FailClose,
		"analyzeErrorInverse11_GuardElseIfFailOpen":          constant.FailClose,
		"analyzeErrorInverse14_ConditionalSyntheticFailOpen": constant.FailClose,
		"analyzeErrorInverse03_UnconditionalWrapFailOpen":    constant.FailClose,
		"analyzeErrorInverse06_MaskedByWrapperFailOpen":      constant.FailClose,
		"analyzeErrorInverse12_DoubleWrapFailOpen":           constant.FailClose,
		// NilOtherSrc tests: names encode NilOtherSourcesAfterSrc=true results; without it these differ.
		"analyzeErrorSCCPNilOtherSrc01_SrcThenOtherGuardBothFailOpen":   constant.FailClose,
		"analyzeErrorSCCPNilOtherSrc05_SrcDominatesOtherBranchFailOpen": constant.FailClose,
		"analyzeErrorSCCPNilOtherSrc07_ReachableNotDominatedFailOpen":   constant.FailClose,
		// Same as sccpOverrides: forward-only says FailClose, name says FailOpen.
		"analyzeErrorPermute101_NamedReturnStructErrBothBranchesFailOpen": constant.FailClose,
		"analyzeErrorPermute102_StructErrBothBranchesNoDeferFailOpen":     constant.FailClose,
	}

	runSCCPTests(t, collected, SCCPOptions{
		MaybeAnalysisMode: true,
		ForwardPathCheck:  true,
	}, fpOverrides)
}

// TestAnalyzeCallBySCCP_StrictModeSelected runs SCCP in strict (Must) mode on
// selected functions that exercise the proveNonNilByControlGuard CDG fallback.
// In strict mode, Top nilness fails non-suppression unless proven by CDG,
// covering lines ~1959-2011 in sccp_analyze.go.
func TestAnalyzeCallBySCCP_StrictModeSelected(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	collected := collectTestFunctions(ssaProg)

	// Functions with explicit expected values in strict mode.
	// Most still produce FailClose; named-return-defer patterns produce FailOpen
	// because the CDG can't prove nilness through the memory store/load.
	strictExpected := map[string]constant.Failness{
		"analyzeErrorSCCP1SimpleDirectFailClose":         constant.FailClose,
		"analyzeErrorSCCP22AssertionNonNilFailClose":     constant.FailClose,
		"analyzeErrorSCCP28AndShortCircuitFailClose":     constant.FailClose,
		"analyzeErrorPermute01_BasicFailClose":           constant.FailClose,
		"analyzeErrorPermute68_NegatedNilCheckFailClose": constant.FailClose,
		"analyzeErrorPermute69_FmtErrorfDirectFailClose": constant.FailClose,
		"analyzeErrorPermute77_StrictCDGProofFailClose":  constant.FailClose,
		// Named return + defer: SCCP resolves nilness through store/load
		// tracking in memState. Exercises defer/named-return SSA patterns.
		"analyzeErrorPermute85_NamedReturnDeferFailClose":     constant.FailClose,
		"analyzeErrorPermute86_NamedReturnMultiPathFailClose": constant.FailOpen,
		// Block-local store peephole tests: named return + defer variations.
		"analyzeErrorPermute87_NamedReturnDeferNoCaptureFailOpen":  constant.FailOpen,
		"analyzeErrorPermute88_NamedReturnDeferReadOnlyFailOpen":   constant.FailOpen,
		"analyzeErrorPermute89_NamedReturnDeferWriteFailClose":     constant.FailClose,
		"analyzeErrorPermute93_NamedReturnDirectNilFailOpen":       constant.FailOpen,
		"analyzeErrorPermute94_NamedReturnDeferNoCaptureFailClose": constant.FailClose,
		"analyzeErrorPermute95_NamedReturnDeferReadOnlyFailClose":  constant.FailClose,
		// Defer dominance tests: non-dominating defer should not block peephole.
		"analyzeErrorPermute96_NamedReturnDeferNonDominatingFailOpen":  constant.FailOpen,
		"analyzeErrorPermute97_NamedReturnDeferNonDominatingFailClose": constant.FailClose,
		// Deferred method call with &retErr as argument (not MakeClosure).
		"analyzeErrorPermute99_NamedReturnDeferMethodArgFailClose":        constant.FailClose,
		"analyzeErrorPermute100_NamedReturnDeferMethodArgShadowFailClose": constant.FailClose,
	}

	for name, expected := range strictExpected {
		pair, ok := collected[name]
		if !ok {
			continue
		}
		t.Run(name, func(t *testing.T) {
			opts := SCCPOptions{MaybeAnalysisMode: false, OtherSources: pair.otherCalls}
			r, err := AnalyzeCallBySCCPWithOptions(pair.fn, pair.srcCall, opts)
			require.NoError(t, err, "for %s", name)
			require.Equal(t, expected, r, "for %s", name)
		})
	}
}

// TestAnalyzeCallBySCCP_DefaultOptions exercises the AnalyzeCallBySCCP public API
// (no options variant) to cover line 2662-2664.
func TestAnalyzeCallBySCCP_DefaultOptions(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	collected := collectTestFunctions(ssaProg)

	// Run a few functions through the default-options API.
	selectedNames := []string{
		"analyzeErrorSCCP1SimpleDirectFailClose",
		"analyzeErrorSimpleFailOpen",
		"analyzeErrorCond1FailClose",
	}
	for _, name := range selectedNames {
		pair, ok := collected[name]
		if !ok {
			continue
		}
		t.Run(name, func(t *testing.T) {
			r, err := AnalyzeCallBySCCP(pair.fn, pair.srcCall)
			require.NoError(t, err)
			expected, _ := getExpectedFromName(name)
			require.Equal(t, expected, r, "for %s", name)
		})
	}
}

// TestAnalyzeCallBySCCP_NilOtherSourcesAfterSrc runs SCCP with
// NilOtherSourcesAfterSrc enabled, verifying that OtherSources
// after Src in the CFG are treated as Nil.
func TestAnalyzeCallBySCCP_NilOtherSourcesAfterSrc(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	collected := collectTestFunctions(ssaProg)

	// Filter to NilOtherSrc test functions only.
	nilOtherSrcTests := map[string]*testFn{}
	for name, pair := range collected {
		if strings.Contains(name, "NilOtherSrc") {
			nilOtherSrcTests[name] = pair
		}
	}
	require.True(t, len(nilOtherSrcTests) > 0, "should find NilOtherSrc test functions")

	// Function names encode the expected result with NilOtherSourcesAfterSrc enabled.
	// No overrides needed — the names are the truth for this configuration.
	runSCCPTests(t, nilOtherSrcTests, SCCPOptions{
		MaybeAnalysisMode:       true,
		NilOtherSourcesAfterSrc: true,
	}, nil)
}
