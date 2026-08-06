//go:build manual
// +build manual

package ssaanal

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// TestModeComparison runs all permutation tests in both MustBe and Maybe NonNil modes
// and displays a side-by-side comparison of the results.
func TestModeComparison(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)

	// Collect all test functions and their calls
	type testCase struct {
		name       string
		fn         *ssa.Function
		srcCall    *ssa.Call
		otherCalls []*ssa.Call
	}

	var tests []testCase

	for fn := range ssautil.AllFunctions(ssaProg) {
		if !strings.HasPrefix(fn.Name(), "analyzeError") {
			continue
		}

		var srcCall *ssa.Call
		var otherCalls []*ssa.Call

		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if call, ok := instr.(*ssa.Call); ok {
					callStr := call.Call.Value.String()
					if strings.Contains(callStr, "anRPCCall") {
						srcCall = call
					} else if strings.Contains(callStr, "someCall") {
						otherCalls = append(otherCalls, call)
					}
				}
			}
		}

		if srcCall != nil {
			tests = append(tests, testCase{
				name:       fn.Name(),
				fn:         fn,
				srcCall:    srcCall,
				otherCalls: otherCalls,
			})
		}
	}

	// Sort tests by name for consistent output
	sort.Slice(tests, func(i, j int) bool {
		return tests[i].name < tests[j].name
	})

	// Run all tests in both modes
	type result struct {
		mustBe constant.Failness // MustBe NonNil mode
		maybe  constant.Failness // Maybe NonNil mode
	}

	results := make(map[string]result)
	var mustBeErrors, maybeErrors []string

	for _, tc := range tests {
		// MustBe NonNil mode
		mustBeOpts := SCCPOptions{
			MaybeAnalysisMode: false, // MustBe NonNil
			OtherSources:      tc.otherCalls,
		}
		mustBeRes, err := AnalyzeCallBySCCPWithOptions(tc.fn, tc.srcCall, mustBeOpts)
		if err != nil {
			mustBeErrors = append(mustBeErrors, fmt.Sprintf("%s: %v", tc.name, err))
			mustBeRes = "ERROR"
		}

		// Maybe NonNil mode
		maybeOpts := SCCPOptions{
			MaybeAnalysisMode: true, // Maybe NonNil
			OtherSources:      tc.otherCalls,
		}
		maybeRes, err := AnalyzeCallBySCCPWithOptions(tc.fn, tc.srcCall, maybeOpts)
		if err != nil {
			maybeErrors = append(maybeErrors, fmt.Sprintf("%s: %v", tc.name, err))
			maybeRes = "ERROR"
		}

		results[tc.name] = result{
			mustBe: mustBeRes,
			maybe:  maybeRes,
		}
	}

	// Display results
	fmt.Println("\n" + strings.Repeat("=", 120))
	fmt.Println("MODE COMPARISON: MustBe NonNil vs Maybe NonNil")
	fmt.Println(strings.Repeat("=", 120))
	fmt.Printf("%-70s | %-15s | %-15s | %s\n", "Test Function", "MustBe NonNil", "Maybe NonNil", "Diff")
	fmt.Println(strings.Repeat("-", 120))

	// Track differences
	var different []string
	var same []string

	for _, tc := range tests {
		res := results[tc.name]
		diff := ""
		if res.mustBe != res.maybe {
			diff = "DIFFERS"
			different = append(different, tc.name)
		} else {
			diff = "-"
			same = append(same, tc.name)
		}

		fmt.Printf("%-70s | %-15s | %-15s | %s\n", tc.name, res.mustBe, res.maybe, diff)
	}

	fmt.Println(strings.Repeat("=", 120))
	fmt.Printf("Total: %d tests | Same: %d | Different: %d\n", len(tests), len(same), len(different))
	fmt.Println(strings.Repeat("=", 120))

	if len(different) > 0 {
		fmt.Println("\nTESTS WITH DIFFERENT RESULTS:")
		fmt.Println(strings.Repeat("-", 120))
		for _, name := range different {
			res := results[name]
			fmt.Printf("  %-68s: MustBe=%-10s Maybe=%-10s\n", name, res.mustBe, res.maybe)
		}
		fmt.Println(strings.Repeat("-", 120))
	}

	if len(mustBeErrors) > 0 {
		fmt.Println("\nMUSTBE NONNIL MODE ERRORS:")
		for _, e := range mustBeErrors {
			fmt.Printf("  %s\n", e)
		}
	}

	if len(maybeErrors) > 0 {
		fmt.Println("\nMAYBE NONNIL MODE ERRORS:")
		for _, e := range maybeErrors {
			fmt.Printf("  %s\n", e)
		}
	}

	// Summary by result type
	fmt.Println("\n" + strings.Repeat("=", 120))
	fmt.Println("SUMMARY BY RESULT TYPE")
	fmt.Println(strings.Repeat("=", 120))

	mustBeCounts := make(map[constant.Failness]int)
	maybeCounts := make(map[constant.Failness]int)

	for _, res := range results {
		mustBeCounts[res.mustBe]++
		maybeCounts[res.maybe]++
	}

	fmt.Printf("%-20s | MustBe Mode | Maybe Mode\n", "Result")
	fmt.Println(strings.Repeat("-", 50))
	for _, failness := range []constant.Failness{constant.FailClose, constant.FailOpen, constant.FailUnknown, "ERROR"} {
		fmt.Printf("%-20s | %11d | %10d\n", failness, mustBeCounts[failness], maybeCounts[failness])
	}
	fmt.Println(strings.Repeat("=", 120))
}
