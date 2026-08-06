package rtatest

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/loader"
	rta "github.com/uber-research/rpcshield/go/rta"
	"golang.org/x/tools/go/callgraph"
	rtapkg "golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// SetupGoSDK arranges for the real `go` binary to be on PATH and for GOCACHE to be writable, so
// loadDemo's packages.Load-driven `go list` succeeds under `bazel test`. Call this from each
// flavor package's TestMain (see modelfx/model_test.go's goSDKBinDir for the pattern this
// mirrors) -- each such test's BUILD.bazel go_test target must declare
// data = ["//exampleapps/rpcshielddemo:rpcshielddemo-anal", "@go_sdk//:files"].
func SetupGoSDK() {
	goBin := goSDKBinDir()
	os.Setenv("PATH", goBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if os.Getenv("GOROOT") == "" {
		os.Setenv("GOROOT", filepath.Dir(goBin))
	}
	if os.Getenv("GOCACHE") == "" {
		if dir, err := os.MkdirTemp("", "rtatest-gocache"); err == nil {
			os.Setenv("GOCACHE", dir)
		}
	}
}

// RunWithGoSDK runs m under a corrected GOROOT/PATH/GOCACHE/GOPATH, for tests that (unlike plain
// SetupGoSDK callers) call APIs comparing `go env GOROOT` against runtime.GOROOT() directly (e.g.
// golang.org/x/tools/go/analysis/analysistest, go/importer.Default()). Under `bazel test`, Bazel
// stamps GOROOT to the literal constant string "GOROOT" baked into the binary
// (https://github.com/bazel-contrib/rules_go/issues/2370); unlike a `go` subprocess's environment
// (which SetupGoSDK's os.Setenv calls do correctly influence), this in-process constant cannot be
// fixed by os.Setenv, so the only fix is to detect it and re-exec the test binary with the
// corrected environment (mirrors the monorepo's example.internal/devexp/bazel/testutil.RunWithGoSDK).
// Call this directly from TestMain: `func TestMain(m *testing.M) { rtatest.RunWithGoSDK(m) }`.
func RunWithGoSDK(m *testing.M) {
	srcDir := os.Getenv("TEST_SRCDIR")
	if srcDir == "" || runtime.GOROOT() != "GOROOT" {
		os.Exit(m.Run())
	}
	tempDir := os.Getenv("TEST_TMPDIR")
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	goRoot := filepath.Join(srcDir, "go_sdk")
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(),
		"GOCACHE="+filepath.Join(tempDir, "cache"),
		"GOROOT="+goRoot,
		fmt.Sprintf("PATH=%s:%s", filepath.Join(goRoot, "bin"), os.Getenv("PATH")),
		"GOPATH="+srcDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err == nil {
		os.Exit(0)
	}
	if xerr, ok := err.(*exec.ExitError); ok {
		os.Exit(xerr.ExitCode())
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(6)
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

// loadDemo loads the rpcshielddemo whole-program SSA fixture once. "rpcshield" is this
// workspace's Bazel workspace name (see WORKSPACE), the local equivalent of the monorepo's
// loader.DefaultRootDirName ("example.internal") -- see exampleapps/rpcshielddemo/BUILD.bazel's
// "rpcshielddemo-anal" go_path target and modelfx/model_test.go's loadAnal for the established
// pattern this mirrors.
var loadDemo = sync.OnceValue(func() *ssa.Program {
	prog, _, _, err := loader.LoadSSAWithGoPath(
		"rpcshield",
		[]string{"exampleapps", "rpcshielddemo", "rpcshielddemo-anal"},
		"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo",
	)
	if err != nil {
		panic(err)
	}
	return prog
})

// graphStats holds summary statistics for a callgraph.
type graphStats struct {
	Nodes     int
	Edges     int
	GraphHash [32]byte
}

// computeStats counts nodes and edges and computes a neighbor-based hash
// (XOR of per-node SHA256) for graph equivalence checking.
func computeStats(g *callgraph.Graph) graphStats {
	var s graphStats
	s.Nodes = len(g.Nodes)
	for fn, node := range g.Nodes {
		s.Edges += len(node.Out)
		if fn == nil {
			continue
		}
		callees := make([]string, 0, len(node.Out))
		for _, edge := range node.Out {
			if edge.Callee != nil && edge.Callee.Func != nil {
				callees = append(callees, edge.Callee.Func.String())
			}
		}
		sort.Strings(callees)
		h := sha256.New()
		h.Write([]byte(fn.String()))
		for _, c := range callees {
			h.Write([]byte(c))
		}
		nodeHash := h.Sum(nil)
		for i := range s.GraphHash {
			s.GraphHash[i] ^= nodeHash[i]
		}
	}
	return s
}

// roots extracts main and init entry-point functions from the SSA program.
func roots(prog *ssa.Program) []*ssa.Function {
	var entry []*ssa.Function
	for f, ok := range ssautil.AllFunctions(prog) {
		if !ok || (f.Signature != nil && f.Signature.Recv() != nil) {
			continue
		}
		if f.Name() == "main" || f.Name() == "init" {
			entry = append(entry, f)
		}
	}
	return entry
}

// LoadProgramAndRoots returns the rpcshielddemo SSA program and root functions.
// Use this in per-flavor tests to call Analyze directly and exercise
// flavor-specific Result methods.
func LoadProgramAndRoots(t *testing.T) (*ssa.Program, []*ssa.Function) {
	t.Helper()
	prog := loadDemo()
	entry := roots(prog)
	require.NotEmpty(t, entry, "no root functions found")
	return prog, entry
}

// AssertMatchesStdlib runs the given RTA flavor on the rpcshielddemo app and
// asserts that the resulting callgraph has the same number of nodes, edges,
// and neighbor hash as the built-in golang.org/x/tools/go/callgraph/rta.
func AssertMatchesStdlib(t *testing.T, flavor rta.RTA, opts ...rta.AnalyzeOption) {
	t.Helper()

	prog := loadDemo()
	entry := roots(prog)
	require.NotEmpty(t, entry, "no root functions found")

	// Reference: built-in RTA.
	refResult := rtapkg.Analyze(entry, true)
	require.NotNil(t, refResult, "stdlib RTA returned nil")
	refStats := computeStats(refResult.CallGraph)

	// Flavor under test.
	flavorResult := flavor.Analyze(entry, true, opts...)
	require.NotNil(t, flavorResult, "flavor Analyze returned nil")
	flavorCG := flavorResult.GetCallGraph()
	require.NotNil(t, flavorCG, "flavor callgraph is nil")
	flavorStats := computeStats(flavorCG)

	require.Equal(t, refStats.Nodes, flavorStats.Nodes, "node count mismatch")
	require.Equal(t, refStats.Edges, flavorStats.Edges, "edge count mismatch")
	require.Equal(t, refStats.GraphHash, flavorStats.GraphHash, "graph hash mismatch")

	// Exercise remaining Result interface methods for coverage.
	reachable := flavorResult.GetReachable()
	require.NotNil(t, reachable, "GetReachable returned nil")
	require.NotEmpty(t, reachable, "GetReachable returned empty map")

	runtimeTypes := flavorResult.GetRuntimeTypes()
	require.Greater(t, runtimeTypes.Len(), 0, "GetRuntimeTypes returned empty map")

	materialized := flavorResult.Materialize()
	require.NotNil(t, materialized, "Materialize returned nil")
	require.NotNil(t, materialized.CallGraph, "Materialized CallGraph is nil")
	require.NotNil(t, materialized.Reachable, "Materialized Reachable is nil")
	require.Greater(t, materialized.RuntimeTypes.Len(), 0, "Materialized RuntimeTypes is empty")
}
