package instrtoast

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const testSrc = `
package main

//go:noinline
func localFunction() {

}

//go:noinline
func AFunctionWithUniqueName1() {
	localFunction() // must find this call
	fn := func() {
		localFunction() // must find this call
	}
	fn() // must find this call
}

//go:noinline
func AFunctionWithUniqueName2() {
	fn := func() {
		localFunction() // must find this call
	}
	fn() // must find this call
}

//go:noinline
func AFunctionWithUniqueName3() {
	func() {
		localFunction() // must find this call
	}()
}

//go:noinline
func AFunctionWithUniqueName4() {
	func(f func()) {
		f() // must find this call
	}(func() {
		localFunction() // must find this call
	})
}

type AStruct struct {
}

//go:noinline
func (s *AStruct) AFunctionWithUniqueName5() {
	localFunction() // must find this call
}

//go:noinline
func AFunctionWithUniqueName6_bound() {
	a := AStruct{}
	v := a.AFunctionWithUniqueName5
	func() {
		v() // must find this call
	}()
}

//go:noinline
func one() string {
	return "one"
}

//go:noinline
func two() string {
	return "two"
}

//go:noinline
func three(a, b string) string {
	return "three"
}

//go:noinline
func AFunctionWithUniqueName7() {
	three(one(), two()) // must find three calls
}

//go:noinline
func d(s string) string { return s }

//go:noinline
func c(s string) string { return s }

//go:noinline
func b(s string) string { return s }

//go:noinline
func a(s1 string, s2 string) string { return s1 + s2 }

//go:noinline
func AFunctionWithUniqueName8() {
	a(b(c(d("hi"))), d(c(b("hello")))) // must find seven calls
}

//go:noinline
func AFunctionWithUniqueName9() {
	go b("") // should find b()
}

func main() {
	AFunctionWithUniqueName1()
	AFunctionWithUniqueName2()
	AFunctionWithUniqueName3()
	AFunctionWithUniqueName4()
	AFunctionWithUniqueName6_bound()
	AFunctionWithUniqueName7()
	AFunctionWithUniqueName8()
}
`

// goSDKBinDir returns the directory containing the real `go` binary. Under `bazel test`,
// runtime.GOROOT() points at a stripped-down stdlib-only tree with no `bin/go` (see
// https://github.com/bazel-contrib/rules_go/issues/2370). rules_go's go_test sets RUNFILES_DIR
// (or TEST_SRCDIR/TEST_WORKSPACE) pointing at the runfiles tree, where the go_sdk's real bin/go
// is reachable if declared as a `data` dep; outside Bazel, runtime.GOROOT() already has one.
func goSDKBinDir(t *testing.T) string {
	t.Helper()

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

// setupTestPackage creates a temporary directory with a single Go source file,
// loads the package with syntax information, builds the SSA program, and returns them.
func setupTestPackage(t *testing.T) (prog *ssa.Program, pkgs []*packages.Package, tempDir string, cleanup func()) {
	t.Helper()

	// Create a temporary directory.
	dir, err := os.MkdirTemp("", "instrtoast")
	require.NoError(t, err, "Failed to create temp dir")

	// Write our test source code to a file.
	srcFile := filepath.Join(dir, "testpkg.go")
	err = os.WriteFile(srcFile, []byte(testSrc), 0644)
	require.NoError(t, err, "Failed to write test source file")

	// Make dir a self-contained module so `go list`/`go build` (shelled out to by go/packages)
	// resolve it in ordinary module mode instead of legacy, increasingly unsupported GOPATH mode.
	err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module instrtoasttestpkg\n\ngo 1.21\n"), 0644)
	require.NoError(t, err, "Failed to write go.mod")

	// Load the package using go/packages. Filter out any inherited GOFLAGS/GOPATH/PATH/GO111MODULE
	// so our overrides below take effect (duplicate env entries are unreliable across libc's, and
	// an inherited PATH could shadow the real `go` binary with a wrapper script).
	env := make([]string, 0, len(os.Environ())+3)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GOFLAGS=") || strings.HasPrefix(kv, "GOPATH=") ||
			strings.HasPrefix(kv, "PATH=") || strings.HasPrefix(kv, "GO111MODULE=") {
			continue
		}
		env = append(env, kv)
	}
	goBin := goSDKBinDir(t)
	pathOverride := goBin + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
	env = append(env,
		"GOFLAGS=-buildvcs=false -mod=mod",
		"GOPATH="+filepath.Join(dir, ".gopath"),
		"GOCACHE="+filepath.Join(dir, ".gocache"),
		"PATH="+pathOverride,
	)
	// go/packages resolves the `go` binary via the current process's PATH before it ever looks
	// at cfg.Env, so the override must apply here too or it'll find a wrapper script instead.
	t.Setenv("PATH", pathOverride)
	cfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:        dir,
		Env:        env,
		BuildFlags: []string{"-buildvcs=false"},
	}
	loaded, err := packages.Load(cfg, "./...")
	require.NoError(t, err, "Failed to load packages")

	// Check for any package errors.
	var errorsFound []string
	for _, pkg := range loaded {
		for _, err := range pkg.Errors {
			errorsFound = append(errorsFound, err.Error())
		}
	}
	require.Empty(t, errorsFound, "Packages had errors: %v", strings.Join(errorsFound, "\n"))

	// Build the SSA program.
	ssaProg, _ := ssautil.AllPackages(loaded, ssa.GlobalDebug|ssa.InstantiateGenerics)
	ssaProg.Build()

	// Return a cleanup function to remove the temporary directory.
	cleanup = func() {
		os.RemoveAll(dir)
	}

	return ssaProg, loaded, dir, cleanup
}

func TestInstrToast(t *testing.T) {
	prog, pkgs, _, cleanup := setupTestPackage(t)
	defer cleanup()
	converter := New(prog, pkgs)
	expected := []struct {
		name          string
		calls         int
		shouldSucceed bool
	}{
		{"AFunctionWithUniqueName1", 2, true},
		{"AFunctionWithUniqueName1$1", 1, true},
		{"AFunctionWithUniqueName2", 1, true},
		{"AFunctionWithUniqueName2$1", 1, true},
		{"AFunctionWithUniqueName3", 1, true},
		{"AFunctionWithUniqueName3$1", 1, true},
		{"AFunctionWithUniqueName4", 1, true},
		{"AFunctionWithUniqueName4$1", 1, true},
		{"AFunctionWithUniqueName4$2", 1, true},
		{"AFunctionWithUniqueName5", 1, true},
		{"AFunctionWithUniqueName6_bound", 1, true},
		{"AFunctionWithUniqueName7", 3, true},
		{"AFunctionWithUniqueName8", 7, true},
		{"AFunctionWithUniqueName5$bound", 0, false},
		{"AFunctionWithUniqueName9", 1, true},
	}

	for _, test := range expected {
		t.Run(test.name, func(t *testing.T) {
			fn := findFunction(t, prog, test.name)
			require.NotNil(t, fn, "Function %s not found", test.name)
			calls := findCalls(t, converter, fn, test.shouldSucceed)
			require.Len(t, calls, test.calls, "Expected %d calls, got %d", test.calls, len(calls))
		})
	}

	// Negative test cases
	_, _, err := converter.NodeForInstruction(nil)
	require.Error(t, err, "Expected error for nil instruction")

}

func findFunction(t *testing.T, prog *ssa.Program, name string) *ssa.Function {
	t.Helper()
	for fn := range ssautil.AllFunctions(prog) {
		if fn.Name() == name {
			return fn
		}
	}
	return nil
}

func findCalls(t *testing.T, converter *Converter, fn *ssa.Function, shouldSucceed bool) []*ssa.CallCommon {
	t.Helper()
	var calls []*ssa.CallCommon
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if call, ok := instr.(ssa.CallInstruction); ok {
				node, err := converter.CallInstrToCallExpr(call)
				// hit in the cache
				node2, err2 := converter.CallInstrToCallExpr(call)
				require.Equal(t, node, node2)
				require.Equal(t, err, err2)

				if shouldSucceed {
					require.NoError(t, err, "Failed to find AST node for instruction")
					// Not a nested/synthetic function.
					nodeString, _ := nodeToStr(node.Fun)
					// Make sure the node's format contains the function name from instr, if it's a known function.
					if call.Common().IsInvoke() {
						// Check if the target function is known and is not a synthetic function and not a lambda.
						if target := call.Common().Method; target != nil {
							methodName := target.Name()
							require.Contains(t, nodeString, methodName)
						}
					} else {
						// Check if the target function is known and is not a synthetic function and not a lambda.
						if target := call.Common().Value; target != nil {
							if fn, ok := target.(*ssa.Function); ok {
								if fn.Parent() == nil {
									require.Contains(t, nodeString, target.Name())
								}
							}
						}
					}
				} else {
					require.Error(t, err, "Expected to fail finding AST node for instruction")
				}
				if node != nil {
					calls = append(calls, call.Common())
				}
			}
		}
	}
	return calls
}

func nodeToStr(node ast.Node) (string, error) {
	fset := token.NewFileSet()

	var buf bytes.Buffer
	err := format.Node(&buf, fset, node)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
