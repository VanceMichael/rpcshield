package rpcshieldtest

import (
	"go/ast"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/pkgdeps"
	"github.com/uber-research/rpcshield/go/loader"
	"github.com/uber-research/rpcshield/go/staticanalyzer/anonymousfunc"
	"github.com/uber-research/rpcshield/go/staticanalyzer/asthelper"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Constants for denoting prefixes of expected values in the comments.
const (
	OutboundCallSite       = "outbound_callsite: "
	OutboundCallSitePos    = "outbound_callsite_pos: "
	WantFuncValidityPrefix = "expect_func_validity: "
	WantFuncStatusPrefix   = "expect_status: "
)

// ExpectedValuesMap contains the expected values for a function declaration, where key is the prefix of the expected value
type ExpectedValuesMap map[string]string

// TestData contains the package and the expected values for each function declaration in the test package.
type TestData struct {
	Pkg                *packages.Package
	FuncExpectedValues map[*ast.FuncDecl]ExpectedValuesMap
}

// SetupTest loads the test packages and extracts the expected values from the comments in the test files.
func SetupTest(t *testing.T, testdataPath string, prefix ...string) []*TestData {
	pkgs, err := loader.LoadAll(testdataPath, nil /* cdeps */)
	require.NoError(t, err)
	require.Greaterf(t, len(pkgs), 0, "Expected at least one package to be loaded")

	// Check if there are any errors in the packages
	for _, pkg := range pkgs {
		require.Emptyf(t, pkg.Errors, "Expected no errors in package '%s'. Error: %s", pkg.Name, pkg.Errors)
	}

	var testDataList []*TestData

	for _, pkg := range pkgs {
		funcExpectedValues := make(map[*ast.FuncDecl]ExpectedValuesMap)
		for _, file := range pkg.Syntax {
			// Store a mapping between single comment's line number to its text.
			comments := make(map[int]string)
			for _, group := range file.Comments {
				for _, comment := range group.List {
					comments[pkg.Fset.Position(comment.Pos()).Line] = comment.Text
				}
			}

			// Inspect each function declaration in the file.
			for _, decl := range file.Decls {
				if funcDecl, ok := decl.(*ast.FuncDecl); ok {
					expectedValues := findExpectedValues(pkg, funcDecl, comments, prefix...)
					if len(expectedValues) > 0 {
						funcExpectedValues[funcDecl] = expectedValues
					}
				}
			}
		}

		testDataList = append(testDataList, &TestData{
			Pkg:                pkg,
			FuncExpectedValues: funcExpectedValues,
		})
	}
	require.Greaterf(t, len(testDataList), 0, "Expected at least one package to have test data")

	return testDataList
}

// findExpectedValues extracts the expected values from the comments in the function declaration
func findExpectedValues(pkg *packages.Package, funcDecl *ast.FuncDecl, comments map[int]string, prefix ...string) ExpectedValuesMap {
	result := make(ExpectedValuesMap)
	visited := make(map[int]bool)

	ast.Inspect(funcDecl, func(node ast.Node) bool {
		if node == nil {
			return false
		}

		nodeLine := pkg.Fset.Position(node.Pos()).Line
		text, ok := comments[nodeLine]
		if !ok || visited[nodeLine] {
			// It is ok to not leave annotations for a node
			return true

		}
		visited[nodeLine] = true

		// Trim the trailing slashes and extra spaces and extract the set of expected values.
		text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
		for _, p := range prefix {
			if strings.HasPrefix(text, p) {
				text = strings.TrimSpace(strings.TrimPrefix(text, p))
				result[p] = text
			}
		}
		return true
	})
	return result
}

// ParseOutboundSite returns the function object of the outbound sink site from the call expression at the given position
// in the enclosing function declaration
func ParseOutboundSite(currPkg *packages.Package, allPkgs map[string]*packages.Package, calleeName string, enclosingCallerFuncDecl *ast.FuncDecl) []*ast.CallExpr {
	var calleeList []*ast.CallExpr

	astContext := asthelper.New(pkgdeps.New(currPkg, allPkgs), enclosingCallerFuncDecl)
	astContext.FuncLitFuncMap, astContext.IdentFuncLitMap = anonymousfunc.CollectAnonymousFuncs(currPkg, enclosingCallerFuncDecl)

	ast.Inspect(enclosingCallerFuncDecl, func(node ast.Node) bool {
		if node == nil {
			return false
		}

		// Get the function object from the call expression of the outbound sink site
		if call, ok := node.(*ast.CallExpr); ok {
			if funcIdent := astContext.FuncIdentFromCallExpr(call); funcIdent != nil && funcIdent.Name == calleeName {
				calleeList = append(calleeList, call)
			}
		}
		return true
	})
	return calleeList
}

type TestPackage struct {
	Entry   *packages.Package
	All     []*packages.Package
	SSAProg *ssa.Program
	SSAPkgs []*ssa.Package
}

// LoadrpcshieldDemoPackages loads all packages from the rpcshielddemo fixture's lifted Go path
// (see exampleapps/rpcshielddemo/BUILD.bazel's "rpcshielddemo-anal" go_path target, and
// rta/rtatest/helper.go's loadDemo for the same pattern). This function is mostly designed to be
// used in test code such that we can load a real service's codebase for thorough testing.
//
// The loaded packages are also cached to avoid consuming too much RAM in parallel tests. This means that the caller
// _must_ not modify the returned packages (e.g., changing ASTs), or else it may affect other tests.
var LoadrpcshieldDemoPackages = sync.OnceValue(func() TestPackage {
	entry, all, err := loader.LoadWithGoPath(
		"rpcshield",
		[]string{"exampleapps", "rpcshielddemo", "rpcshielddemo-anal"},
		"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo",
	)
	if err != nil {
		panic(err)
	}
	ssaProg, ssaPkgs := ssautil.AllPackages(all, ssa.InstantiateGenerics|ssa.GlobalDebug)
	ssaProg.Build()
	return TestPackage{Entry: entry, All: all, SSAProg: ssaProg, SSAPkgs: ssaPkgs}
})
