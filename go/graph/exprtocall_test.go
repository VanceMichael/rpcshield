package graph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Utility function to generate SSA from Go source code
func generateSSA(t *testing.T, src string) (*ssa.Package, *ast.CallExpr, *token.FileSet) {
	fset := token.NewFileSet()
	conf := loader.Config{
		Fset:       fset,
		ParserMode: parser.AllErrors,
	}

	// Parse the source code
	file, err := parser.ParseFile(fset, "test.go", src, parser.AllErrors)
	require.NoError(t, err)

	// Load the program
	conf.CreateFromFiles("testpkg", file)
	prog, err := conf.Load()
	require.NoError(t, err)

	// Build SSA from the loaded program
	ssaProg := ssautil.CreateProgram(prog, ssa.SanityCheckFunctions|ssa.GlobalDebug)
	ssaPkg := ssaProg.Package(prog.Created[0].Pkg)
	ssaPkg.Build()

	// Find the first call expression in the AST
	var callExpr *ast.CallExpr
	ast.Inspect(file, func(n ast.Node) bool {
		if ce, ok := n.(*ast.CallExpr); ok {
			callExpr = ce
			return false // Stop after finding the first CallExpr
		}
		return true
	})

	require.NotNil(t, callExpr, "Expected a call expression in the source code")
	return ssaPkg, callExpr, fset
}

func TestGetViaValForExpr_ValidCallInstruction(t *testing.T) {
	t.Parallel()

	src := `
		package testpkg
		func Add(x int, y int) int { return x + y }
		func Test() {
			_ = Add(1, 2)
		}
	`
	ssaPkg, callExpr, fset := generateSSA(t, src)

	// Get the SSA function for Test
	ssaFunc := ssaPkg.Func("Test")
	require.NotNil(t, ssaFunc)

	// Test the getViaValForExpr
	ci, err := getViaValForExpr(ssaFunc, callExpr, fset, 5)
	require.NoError(t, err)
	assert.NotNil(t, ci)
}

func TestFindTopNClosestInstructions(t *testing.T) {
	t.Parallel()

	src := `
		package testpkg
		func A() {}
		func B() {}
		func C() {}
		func Test() {
			A()
			B()
			C()
		}
	`
	ssaPkg, callExpr, fset := generateSSA(t, src)

	// Get the SSA function for Test
	ssaFunc := ssaPkg.Func("Test")
	require.NotNil(t, ssaFunc)

	// Target position should be around the second function call
	targetPos := fset.Position(callExpr.Pos())
	blocks := ssaFunc.Blocks
	instructions := findTopNClosestInstructions(blocks, targetPos, fset, 2)
	require.Len(t, instructions, 2)
	assert.NotNil(t, instructions[0])
	assert.NotNil(t, instructions[1])
}

func TestFindNearestCallInBlock(t *testing.T) {
	t.Parallel()

	src := `
		package testpkg
		func A() {}
		func Test() {
			A()
		}
	`
	ssaPkg, _, fset := generateSSA(t, src)

	// Get the SSA function for Test
	ssaFunc := ssaPkg.Func("Test")
	require.NotNil(t, ssaFunc)

	// Find the nearest call instruction in the first block
	block := ssaFunc.Blocks[0]
	callInstr, err := findNearestCallInBlock(block, fset)
	require.NoError(t, err)
	assert.NotNil(t, callInstr)
}

func TestGetClosestCallInstruction_TopNFallback(t *testing.T) {
	t.Parallel()

	src := `
		package testpkg
		func A() {}
		func Test() {
			A()
		}
	`
	ssaPkg, callExpr, fset := generateSSA(t, src)

	// Get the SSA function for Test
	ssaFunc := ssaPkg.Func("Test")
	require.NotNil(t, ssaFunc)

	// Test the fallback scenario for getClosestCallInstruction
	ci, err := getClosestCallInstruction(ssaFunc, callExpr, fset, 5)
	require.NoError(t, err)
	assert.NotNil(t, ci)

	// Test nil callexpr
	ci, err = getClosestCallInstruction(ssaFunc, nil, fset, 0)
	require.Error(t, err)
	assert.Nil(t, ci)

	callExpr = &ast.CallExpr{
		Fun:    &ast.Ident{Name: "NonExisting"},
		Lparen: token.NoPos,
		Rparen: token.NoPos,
	}
	ci, err = getClosestCallInstruction(ssaFunc, callExpr, fset, 5)
	require.NoError(t, err)
	assert.NotNil(t, ci)

	ci, err = getClosestCallInstruction(ssaFunc, callExpr, fset, 0)
	require.Error(t, err)
	assert.Nil(t, ci)

}

func TestAbsFunction(t *testing.T) {
	assert.Equal(t, 5, abs(5))
	assert.Equal(t, 5, abs(-5))
	assert.Equal(t, 0, abs(0))
}
