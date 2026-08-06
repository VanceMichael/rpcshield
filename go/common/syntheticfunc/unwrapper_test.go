package syntheticfunc

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/loader"
	"go.uber.org/zap"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func loadSSAProgram(t *testing.T, pathSlice []string, loadPath string) (*ssa.Program, []*ssa.Package) {
	// "rpcshield" is this workspace's declared Bazel workspace name (see WORKSPACE), which is the
	// top-level ancestor directory name inside the runfiles tree during `bazel test` -- the local
	// equivalent of the monorepo's loader.DefaultRootDirName ("example.internal").
	_, allPkgs, err := loader.LoadWithGoPath("rpcshield", pathSlice, loadPath)
	require.NoError(t, err)

	// Build SSA representation.
	prog, allSSAPkgs := ssautil.AllPackages(allPkgs, ssa.SanityCheckFunctions|ssa.GlobalDebug|ssa.InstantiateGenerics)
	prog.Build()

	return prog, allSSAPkgs
}

func TestUnwrapper(t *testing.T) {
	t.Parallel()

	ssaProg, _ := loadSSAProgram(t, []string{"exampleapps", "syntheticcallee", "syntheticcallee-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee")

	// Setup the source code for the test.
	funcs := make(map[string]*ssa.Function)
	// find any function names main in the ssaProg
	var mainFunc []*ssa.Function
	var syntheticFuncs []*ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		funcs[fn.String()] = fn
		if fn.Synthetic != "" {
			syntheticFuncs = append(syntheticFuncs, fn)
		}
		if fn.Name() == "main" {
			mainFunc = append(mainFunc, fn)
		}
	}

	// Sort syntheticFuncs by name
	sort.Slice(syntheticFuncs, func(i, j int) bool {
		return syntheticFuncs[i].String() < syntheticFuncs[j].String()
	})

	// Build the call graph using RTA (Rapid Type Analysis)
	rtaResult := rta.Analyze(mainFunc, true)
	cg := rtaResult.CallGraph

	logger := zap.NewNop().Sugar()
	unwrapper := NewUnwrapper(logger).WithCallgraph(cg, ssaProg)

	type result struct {
		fn  string
		err error
	}

	var results []result
	for _, fn := range syntheticFuncs {
		r, err := unwrapper.Unwrap(fn)

		var fn string
		if r != nil {
			fn = r.String()
			if !strings.Contains(fn, "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee") {
				continue
			}
			results = append(results, result{
				fn:  fn,
				err: err,
			})
		}
	}

	expected := []result{
		{
			fn: "(github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.Impl1).Foo",
		},
		{
			fn: "(github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.Impl2).Foo",
		},
		{
			fn: "(*github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.Impl3[T]).FooPtr",
		},
		{
			fn: "(github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.Impl3[T]).Foo",
		},
		{
			fn: "(*github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.Impl3[T]).FooPtr",
		},
		{
			fn: "(github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.Impl3[T]).Foo",
		},
		{
			fn: "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.work3",
		},
		{
			fn: "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.work3",
		},
		{
			fn: "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee.work4",
		},
	}

	sort.Slice(expected, func(i, j int) bool {
		return expected[i].fn < expected[j].fn
	})
	sort.Slice(results, func(i, j int) bool {
		return results[i].fn < results[j].fn
	})

	require.Equal(t, expected, results)
}

// goSDKBinDir returns the directory containing the real `go` binary. Under `bazel test`,
// runtime.GOROOT() points at a stripped-down stdlib-only tree with no `bin/go` (see
// https://github.com/bazel-contrib/rules_go/issues/2370). rules_go's go_test sets RUNFILES_DIR
// (or TEST_SRCDIR/TEST_WORKSPACE) pointing at the runfiles tree, where the go_sdk's real bin/go
// is reachable if declared as a `data` dep; outside Bazel, runtime.GOROOT() already has one.
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

// loader.LoadWithGoPath shells out to `go list` via golang.org/x/tools/go/packages, which
// resolves the `go` binary via the process's PATH. Under bazel test sandboxing the inherited
// PATH won't contain a working `go`, so TestMain fixes it up before any test runs.
func TestMain(m *testing.M) {
	goBin := goSDKBinDir()
	os.Setenv("PATH", goBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if os.Getenv("GOCACHE") == "" {
		if dir, err := os.MkdirTemp("", "syntheticfunc-gocache"); err == nil {
			os.Setenv("GOCACHE", dir)
		}
	}
	os.Exit(m.Run())
}
