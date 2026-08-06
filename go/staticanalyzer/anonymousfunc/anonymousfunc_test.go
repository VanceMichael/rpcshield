package anonymousfunc

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/uber-research/rpcshield/go/loader"
)

func TestCollectAnonymousFuncs(t *testing.T) {
	t.Parallel()

	testdataPaths := []string{
		"../function/testdata",              // reuse the testdata written for CFG tests
		"../../testdata/src/contrivedtests", // reuse the contrived tests
	}

	for _, testdataPath := range testdataPaths {
		// Load packages explicitly to get the ASTs for computing the actual CFGs
		pkgs, err := loader.LoadAll(testdataPath, nil /* cdeps */)
		require.NoErrorf(t, err, "Failed to load packages: %v", err)
		require.Greaterf(t, len(pkgs), 0, "Expected at least one package to be loaded")

		// Check if there are any errors in the packages
		for _, pkg := range pkgs {
			require.Nilf(t, pkg.Errors, "Expected no errors in package '%s'. Error: %s", pkg.Name, pkg.Errors)
		}

		for _, pkg := range pkgs {
			for _, file := range pkg.Syntax {
				var funcNodes []*ast.FuncDecl
				var expectedFuncLits []*ast.FuncLit

				ast.Inspect(file, func(node ast.Node) bool {
					if n, ok := node.(*ast.FuncDecl); ok {
						funcNodes = append(funcNodes, n)
					}
					return true
				})

				for _, f := range funcNodes {
					expectedFuncLits = getAllFuncLitsInFunc(f)
					if len(expectedFuncLits) == 0 {
						// This is a function with no func lits, so we can skip it.
						continue
					}

					funcLitFuncMap, identFuncLitMap := CollectAnonymousFuncs(pkg, f)
					require.Equalf(t, len(expectedFuncLits), len(funcLitFuncMap), "Mismatch in the number of FuncLits in the funcLitFuncMap and the expected FuncLits of func `%s`", f.Name.Name)
					require.Equalf(t, len(expectedFuncLits), len(identFuncLitMap), "Mismatch in the number of FuncLits in the identFuncLitMap and the expected FuncLits of func `%s`", f.Name.Name)

					for _, funcLit := range expectedFuncLits {
						pos := pkg.Fset.Position(funcLit.Pos())
						info := funcLitFuncMap[funcLit]
						require.NotNilf(t, info, "Expected to find the funcLit at line `%d` in the funcLitFuncMap", pos.Line)
						require.NotNilf(t, identFuncLitMap[info.FakeFuncDecl.Name], "Expected to find the fake func decl in the identFuncLitMap")
					}

					for _, info := range funcLitFuncMap {
						// Check if the fake func decl nodes are properly created.
						require.NotNil(t, info.FakeFuncDecl)
						// The fake func name should be an illegal identifier to avoid collisions, it should
						// also be unexported just to avoid accidental uses of such variables in the rest of
						// the system.
						require.True(t, strings.HasPrefix(info.FakeFuncDecl.Name.Name, _fakeFuncDeclPrefix))
						require.False(t, token.IsIdentifier(info.FakeFuncDecl.Name.Name))
						require.False(t, token.IsExported(info.FakeFuncDecl.Name.Name))

						// The func lit body and the fake func decl body should be the same.
						require.Equalf(t, identFuncLitMap[info.FakeFuncDecl.Name].Body, info.FakeFuncDecl.Body, "Mismatch in the body of the fake func decl `%s` and the original func lit", info.FakeFuncDecl.Name.Name)

						// Params and results should be the same.
						if info.FakeFuncDecl.Type.Params != nil {
							for i, param := range info.FakeFuncDecl.Type.Params.List {
								require.Equal(t, param, identFuncLitMap[info.FakeFuncDecl.Name].Type.Params.List[i])
							}
						}
						if info.FakeFuncDecl.Type.Results != nil {
							for i, result := range info.FakeFuncDecl.Type.Results.List {
								require.Equal(t, result, identFuncLitMap[info.FakeFuncDecl.Name].Type.Results.List[i])
							}
						}

						// The fake type should be properly populated.
						require.False(t, info.FakeFuncObj.Exported())
						require.Equal(t, info.FakeFuncObj.Name(), info.FakeFuncDecl.Name.Name)
					}
				}
			}
		}
	}
}

func getAllFuncLitsInFunc(enclosingFunc *ast.FuncDecl) []*ast.FuncLit {
	var funcLits []*ast.FuncLit
	ast.Inspect(enclosingFunc, func(node ast.Node) bool {
		if funcLit, ok := node.(*ast.FuncLit); ok {
			funcLits = append(funcLits, funcLit)
		}
		return true
	})
	return funcLits
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

// loader.LoadAll shells out to `go list` via golang.org/x/tools/go/packages, which resolves the
// `go` binary via the process's PATH. Under bazel test sandboxing the inherited PATH won't
// contain a working `go`, so TestMain fixes it up before any test runs (see goSDKBinDir above).
func TestMain(m *testing.M) {
	goBin := goSDKBinDir()
	pathOverride := goBin + string(os.PathListSeparator) + os.Getenv("PATH")
	os.Setenv("PATH", pathOverride)
	if os.Getenv("GOCACHE") == "" {
		if dir, err := os.MkdirTemp("", "anonymousfunc-gocache"); err == nil {
			os.Setenv("GOCACHE", dir)
		}
	}
	os.Exit(m.Run())
}
