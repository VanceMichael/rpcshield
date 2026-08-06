package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/mapper"
	"github.com/uber-research/rpcshield/go/rpcshieldtest"
	"go.uber.org/zap"
)

func TestGetBlockListPackages_CustomPatternsReplaceDefault(t *testing.T) {
	t.Parallel()

	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	allPkgs, prog, allSSAPkgs := testPkg.All, testPkg.SSAProg, testPkg.SSAPkgs

	l := zap.NewNop().Sugar()

	cfg := DefaultSerialGraphConfig()
	cfg.ExcludePackagePatterns = []string{"^doesnotmatchanything(/.*)?$"}
	gBase := NewBuilder(l, allPkgs, prog, allSSAPkgs, mapper.SSAASTMap{}, mapper.SignatureASTMap{}, false, nil, nil, nil, false, cfg)
	g := gBase.(*builderImpl)

	blockedPkgs, err := g.getBlockListPackages(prog.AllPackages())
	require.NoError(t, err)

	for _, pkg := range prog.AllPackages() {
		if pkg == nil || pkg.Pkg == nil {
			continue
		}
		switch pkg.Pkg.Path() {
		case "go.uber.org/yarpc":
			// go.uber.org/yarpc is in DefaultExcludePackagePatterns, but a non-nil
			// ExcludePackagePatterns should be used exactly as given, with no default merged in.
			require.False(t, blockedPkgs[pkg], "expected go.uber.org/yarpc not blocked when a custom pattern list omits it")
		case "context":
			// GOROOT stdlib auto-exclusion is unaffected by ExcludePackagePatterns.
			require.True(t, blockedPkgs[pkg], "expected context still blocked via automatic GOROOT stdlib exclusion")
		}
	}
}

func TestGetBlockListPackages_InvalidPatternReturnsError(t *testing.T) {
	t.Parallel()

	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	allPkgs, prog, allSSAPkgs := testPkg.All, testPkg.SSAProg, testPkg.SSAPkgs

	l := zap.NewNop().Sugar()

	cfg := DefaultSerialGraphConfig()
	cfg.ExcludePackagePatterns = []string{"("} // invalid regexp
	gBase := NewBuilder(l, allPkgs, prog, allSSAPkgs, mapper.SSAASTMap{}, mapper.SignatureASTMap{}, false, nil, nil, nil, false, cfg)
	g := gBase.(*builderImpl)

	_, err := g.getBlockListPackages(prog.AllPackages())
	require.Error(t, err)
}

func TestGetBlockListPackages_CachesAcrossCalls(t *testing.T) {
	t.Parallel()

	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	allPkgs, prog, allSSAPkgs := testPkg.All, testPkg.SSAProg, testPkg.SSAPkgs

	l := zap.NewNop().Sugar()

	gBase := NewBuilder(l, allPkgs, prog, allSSAPkgs, mapper.SSAASTMap{}, mapper.SignatureASTMap{}, false, nil, nil, nil, false, DefaultSerialGraphConfig())
	g := gBase.(*builderImpl)

	first, err := g.getBlockListPackages(prog.AllPackages())
	require.NoError(t, err)
	regexesAfterFirst, err := g.excludePackageRegexes()
	require.NoError(t, err)

	second, err := g.getBlockListPackages(prog.AllPackages())
	require.NoError(t, err)
	regexesAfterSecond, err := g.excludePackageRegexes()
	require.NoError(t, err)

	require.Equal(t, len(first), len(second))
	// Same backing slice reused across calls -- proves the sync.Once cache, not just equal results.
	require.Same(t, &regexesAfterFirst[0], &regexesAfterSecond[0])
}
