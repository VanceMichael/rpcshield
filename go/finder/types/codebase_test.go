package types

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/rpcshieldtest"
	"go.uber.org/zap"
)

func TestFindCodebasePkgs(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop().Sugar()

	pkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	codebase, err := NewCodebase(logger, pkg.Entry, pkg.All, map[string]bool{})
	require.NoError(t, err)
	require.NotNil(t, codebase)
}

// Tests requiring Go SDK in runtime need a real `go` binary on PATH -- see
// finder/inbound_test.go's goSDKBinDir for the pattern this mirrors.
func TestMain(m *testing.M) {
	goBin := goSDKBinDir()
	os.Setenv("PATH", goBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if os.Getenv("GOCACHE") == "" {
		if dir, err := os.MkdirTemp("", "finder-types-gocache"); err == nil {
			os.Setenv("GOCACHE", dir)
		}
	}
	os.Exit(m.Run())
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
