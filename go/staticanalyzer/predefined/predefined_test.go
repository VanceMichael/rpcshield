package predefined

import (
	"go/token"
	"go/types"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
)

func TestAnalyzeCall(t *testing.T) {
	callsite := &ssa.Call{}

	fset := token.NewFileSet()
	prog := ssa.NewProgram(fset, ssa.SanityCheckFunctions)

	// Build SSA representation
	prog.Build()

	t.Run("NilFunction", func(t *testing.T) {
		failness, err := AnalyzeCall(nil, callsite)
		require.Equal(t, constant.FailUnknown, failness)
		require.Error(t, err)
		require.Equal(t, "nil function", err.Error())
	})

	t.Run("SyntheticFunction", func(t *testing.T) {
		fn := &ssa.Function{Synthetic: "some_synthetic_function"}
		failness, err := AnalyzeCall(fn, callsite)
		require.NoError(t, err)
		require.Equal(t, constant.FailClose, failness)
	})

	t.Run("FunctionNameMatchesFailCloseRegex", func(t *testing.T) {
		fn := prog.NewFunction("foo$bound", &types.Signature{}, "")
		failness, err := AnalyzeCall(fn, callsite)
		require.NoError(t, err)
		require.Equal(t, constant.FailClose, failness)
	})

	t.Run("FunctionNameDoesNotMatchAnyRegex", func(t *testing.T) {
		fn := prog.NewFunction("somefunction", &types.Signature{}, "")
		failness, err := AnalyzeCall(fn, callsite)
		require.NoError(t, err)
		require.Equal(t, constant.FailUnknown, failness)
	})
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
