//go:build manual
// +build manual

package ssaanal

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// TestMaybeAnalysisMode tests the MaybeAnalysisMode option.
func TestMaybeAnalysisMode(t *testing.T) {
	pathSlice := []string{"exampleapps", "errreturns", "errreturns-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/errreturns"

	_, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)

	// Find Permute44
	var targetFn *ssa.Function
	var srcCall *ssa.Call
	var otherCalls []*ssa.Call

	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() != "analyzeErrorPermute44_ConditionalReassignmentFailOpen" {
			continue
		}
		targetFn = fn
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
		break
	}

	require.NotNil(t, targetFn, "Should find Permute44")
	require.NotNil(t, srcCall, "Should find anRPCCall")

	t.Run("MustBeNonNilMode", func(t *testing.T) {
		// Default MustBe NonNil mode
		opts := SCCPOptions{
			MaybeAnalysisMode: false, // MustBe NonNil
			OtherSources:      otherCalls,
		}

		result, err := AnalyzeCallBySCCPWithOptions(targetFn, srcCall, opts)
		require.NoError(t, err)
		// Should be FailOpen because non-suppression fails (Top nilness)
		require.Equal(t, constant.FailOpen, result, "MustBe NonNil mode: should fail at non-suppression")
	})

	t.Run("MaybeAnalysisMode", func(t *testing.T) {
		// Maybe NonNil mode - allows Top nilness
		opts := SCCPOptions{
			MaybeAnalysisMode: true, // Maybe NonNil
			OtherSources:      otherCalls,
		}

		result, err := AnalyzeCallBySCCPWithOptions(targetFn, srcCall, opts)
		require.NoError(t, err)
		// Should still be FailOpen because attribution fails
		// (PHI sources are err1 and err3, not anRPCCall)
		require.Equal(t, constant.FailOpen, result, "Maybe NonNil mode: should fail at attribution")
	})
}
