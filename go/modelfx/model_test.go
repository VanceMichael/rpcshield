package model

import (
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/loader"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestNewAnalyzer(t *testing.T) {
	t.Run("Nil packages", func(t *testing.T) {
		analyzer, err := NewAnalyzer(nil)
		// No error should be returned
		require.NoError(t, err)
		// Analyzer should be created
		require.NotNil(t, analyzer)
	})
}

func TestImportsFx(t *testing.T) {
	t.Run("Package is nil", func(t *testing.T) {
		var pkg *ssa.Package = nil
		assert.False(t, importsFx(pkg))
	})

	t.Run("Package Pkg is nil", func(t *testing.T) {
		pkg := &ssa.Package{Pkg: nil}
		assert.False(t, importsFx(pkg))
	})

	t.Run("Package imports fx", func(t *testing.T) {
		fxPkg := types.NewPackage(_fxPackagePath, "fx")
		myPkg := types.NewPackage("my/package", "mypkg")
		myPkg.SetImports([]*types.Package{fxPkg})
		pkg := &ssa.Package{Pkg: myPkg}
		assert.True(t, importsFx(pkg))
	})

	t.Run("Package does not import fx", func(t *testing.T) {
		otherPkg := types.NewPackage("other/package", "otherpkg")
		myPkg := types.NewPackage("my/package", "mypkg")
		myPkg.SetImports([]*types.Package{otherPkg})
		pkg := &ssa.Package{Pkg: myPkg}
		assert.False(t, importsFx(pkg))
	})

	t.Run("Package imports list is empty", func(t *testing.T) {
		myPkg := types.NewPackage("my/package", "mypkg")
		myPkg.SetImports([]*types.Package{})
		pkg := &ssa.Package{Pkg: myPkg}
		assert.False(t, importsFx(pkg))
	})

	t.Run("Package imports is nil", func(t *testing.T) {
		myPkg := types.NewPackage("my/package", "mypkg")
		// Do not set imports to simulate nil imports
		pkg := &ssa.Package{Pkg: myPkg}
		assert.False(t, importsFx(pkg))
	})
}

// loadAnal finds the fixture app compiled by exampleapps/simplefx1's go_path rule and loads it.
// "rpcshield" is this workspace's declared Bazel workspace name (see WORKSPACE), which is the
// top-level ancestor directory name inside the runfiles tree during `bazel test` -- the local
// equivalent of the monorepo's loader.DefaultRootDirName ("example.internal").
func loadAnal(analPath []string, loadPath string) (entryPkg *packages.Package, allPkgs []*packages.Package) {
	entryPkg, allPkgs, err := loader.LoadWithGoPath("rpcshield", analPath, loadPath)
	if err != nil {
		panic(err)
	}
	return entryPkg, allPkgs
}

func TestExtractFxProvideAndInvoke(t *testing.T) {

	_, allPkgs := loadAnal([]string{"exampleapps", "simplefx1", "simplefx1-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/simplefx1")

	// Build SSA representation.
	prog, _ := ssautil.AllPackages(allPkgs, ssa.SanityCheckFunctions|ssa.GlobalDebug)
	prog.Build()
	allSSAPkgs := prog.AllPackages()

	// Create the analyzer.
	logger, _ := zap.NewDevelopment()
	a := &Analyzer{
		Logger: logger.Sugar(),
	}

	providers, invokes := a.ExtractFxProvideAndInvoke(allSSAPkgs)

	expectedProviders := map[string]struct{}{
		"NewHTTPServer": {},
		"NewDatabase":   {},
		"NewLogger":     {},
		"main$1":        {},
	}
	expectedInvokes := map[string]struct{}{
		"StartHTTPServer": {},
	}

	assert.Equal(t, len(expectedProviders), len(providers))
	assert.Equal(t, len(expectedInvokes), len(invokes))

	for k := range providers {
		name := k.Name()
		assert.Contains(t, expectedProviders, name)
	}
	for k := range invokes {
		name := k.Name()
		assert.Contains(t, expectedInvokes, name)
	}
}

func TestExtractParams(t *testing.T) {
	_, allPkgs := loadAnal([]string{"exampleapps", "simplefx1", "simplefx1-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/simplefx1")

	// Build SSA representation.
	prog, _ := ssautil.AllPackages(allPkgs, ssa.SanityCheckFunctions|ssa.GlobalDebug)
	prog.Build()
	allSSAPkgs := prog.AllPackages()

	// Create the analyzer.
	logger, _ := zap.NewDevelopment()
	a := &Analyzer{
		Logger: logger.Sugar(),
	}

	// Extract the parameters.
	infx, outfx := a.ExtractParams(allSSAPkgs)

	expectedIns := map[string]struct{}{
		"github.com/uber-research/rpcshield/go/exampleapps/simplefx1.HandlerInParams": {},
	}
	expectedOuts := map[string]struct{}{
		"github.com/uber-research/rpcshield/go/exampleapps/simplefx1.HandlerOutParams": {},
	}

	assert.Equal(t, len(expectedIns), len(infx))
	assert.Equal(t, len(expectedOuts), len(outfx))

	for k := range infx {
		assert.Contains(t, expectedIns, k.String())
	}
	for k := range outfx {
		assert.Contains(t, expectedOuts, k.String())
	}

}

func TestAllAddrTaken(t *testing.T) {
	_, allPkgs := loadAnal([]string{"exampleapps", "simplefx1", "simplefx1-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/simplefx1")

	// Build SSA representation.
	prog, _ := ssautil.AllPackages(allPkgs, ssa.SanityCheckFunctions|ssa.GlobalDebug)
	prog.Build()
	allSSAPkgs := prog.AllPackages()

	// Create the analyzer.
	logger, _ := zap.NewDevelopment()
	a := &Analyzer{
		Logger: logger.Sugar(),
	}
	// For now nothing more than being able to run.
	addressTakenFuncs := a.AllAddrTaken(allSSAPkgs, prog)
	expectedValues := map[string]bool{
		"NewHTTPServer":   false,
		"NewDatabase":     false,
		"NewLogger":       false,
		"StartHTTPServer": false,
		// "main$1" will not be present since this is not public
	}
	// These expected values should be there but there can be more.
	for k := range addressTakenFuncs {
		if _, ok := expectedValues[k.Name()]; ok {
			expectedValues[k.Name()] = true
		}
	}

	for k, v := range expectedValues {
		assert.True(t, v, "missing function: "+k)
	}

}

func TestMain(m *testing.M) {
	goBin := goSDKBinDir()
	os.Setenv("PATH", goBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if os.Getenv("GOCACHE") == "" {
		if dir, err := os.MkdirTemp("", "modelfx-gocache"); err == nil {
			os.Setenv("GOCACHE", dir)
		}
	}
	os.Exit(m.Run())
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
