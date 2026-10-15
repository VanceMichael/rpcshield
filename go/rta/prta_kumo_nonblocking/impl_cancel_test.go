package prta_kumo_nonblocking

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

// syntheticProgram builds a tiny in-memory SSA program with a handful of
// functions, so cancellation tests do not depend on the Bazel-built
// rpcshielddemo go_path fixture (which only exists under `bazel test`).
func syntheticProgram(t *testing.T) []*ssa.Function {
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

// TestAnalyze_PreCanceledContextReturnsNil verifies the parallel RTA honors
// cancellation: with a context already canceled, no work units are accepted
// (Push no-ops), workers are woken and converge, and Analyze returns nil
// instead of a partial call graph. The graph layer maps nil result + ctx
// error to its own cancellation error.
func TestAnalyze_PreCanceledContextReturnsNil(t *testing.T) {
	entry := syntheticProgram(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := New().Analyze(entry, true,
		rtalib.WithNumWorkers(8),
		rtalib.WithContext(ctx),
	)
	require.Nil(t, res, "canceled RTA must return nil, not a partial graph")
}

// TestAnalyze_UncanceledContextStillSucceeds guards against the cancellation
// plumbing changing normal behavior when ctx is not canceled.
func TestAnalyze_UncanceledContextStillSucceeds(t *testing.T) {
	entry := syntheticProgram(t)

	res := New().Analyze(entry, true,
		rtalib.WithNumWorkers(4),
		rtalib.WithContext(context.Background()),
	)
	require.NotNil(t, res)
	require.NotNil(t, res.GetCallGraph())
}
