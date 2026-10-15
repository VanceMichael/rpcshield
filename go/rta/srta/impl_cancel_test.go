package srta

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	rtalib "github.com/uber-research/rpcshield/go/rta"
	"golang.org/x/tools/go/ssa"
)

// syntheticSRTAProgram builds a tiny in-memory SSA program so the
// cancellation test does not depend on the Bazel-built rpcshielddemo
// go_path fixture (only present under `bazel test`).
func syntheticSRTAProgram(t *testing.T) []*ssa.Function {
	t.Helper()
	src := `package p

func a() {}
func b() {}
func c() {}
func main() {}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	require.NoError(t, err)

	tpkg := types.NewPackage("p", "p")
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:      make(map[ast.Node]*types.Scope),
	}
	conf := types.Config{}
	_, err = conf.Check("p", fset, []*ast.File{f}, info)
	require.NoError(t, err)

	prog := ssa.NewProgram(fset, ssa.InstantiateGenerics|ssa.GlobalDebug)
	pkg := prog.CreatePackage(tpkg, []*ast.File{f}, info, true)
	prog.Build()
	pkg.Build()

	var roots []*ssa.Function
	for _, name := range []string{"a", "b", "c", "main"} {
		fn, ok := pkg.Members[name].(*ssa.Function)
		require.True(t, ok, "missing function %s", name)
		roots = append(roots, fn)
	}
	return roots
}

// TestAnalyze_PreCanceledContextReturnsNil verifies the serial RTA worklist
// bails out on a canceled context and returns nil, instead of finishing the
// fixed-point iteration and returning a partial graph.
func TestAnalyze_PreCanceledContextReturnsNil(t *testing.T) {
	entry := syntheticSRTAProgram(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := New().Analyze(entry, true, rtalib.WithContext(ctx))
	require.Nil(t, res, "canceled serial RTA must return nil")
}
