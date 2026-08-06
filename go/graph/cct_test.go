package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/mapper"
	"github.com/uber-research/rpcshield/go/finder"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	"github.com/uber-research/rpcshield/go/grail"
	"github.com/uber-research/rpcshield/go/rpcshieldtest"
	"go.uber.org/zap"
)

func TestCCT(t *testing.T) {
	t.Parallel()
	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	entryPkg, allPkgs, ssaProg, ssaPkgs := testPkg.Entry, testPkg.All, testPkg.SSAProg, testPkg.SSAPkgs
	require.NotNil(t, entryPkg)
	require.Greater(t, len(allPkgs), 0)

	SSAASTBidirMap := mapper.SSAToASTMap(allPkgs, ssaPkgs, ssaProg)
	SigASTBidirMap := mapper.SignatureToASTMap(allPkgs)

	logger := zap.NewNop().Sugar()
	inboundService := grail.Service{Name: "rpcshielddemo", Tier: -1}
	lotteryService := grail.Service{Name: "lottery", Tier: 5}
	clientTypeToService := servicemapping.ServiceMapping{}
	clientTypeToService.AddMappings(
		servicemapping.FullyQualifiedType{PkgPath: "thriftrw/lottery/lotteryclient", Name: "Interface"},
		&servicemapping.ServiceWithIDLInfo{Service: lotteryService},
	)
	serviceMapper := servicemapping.New([]grail.Service{lotteryService}, clientTypeToService, logger)
	f := finder.New(inboundService, serviceMapper, logger, map[string]bool{} /* keepIBPkgs */, "" /* codebasePkgPrefix */, "" /* kafkaConsumerProxyPkgPath */, nil /* infraPkgPrefixes */, false /* enableGenericGRPCThriftDetection */)

	inbounds, err := f.Inbound(entryPkg, allPkgs, ssaProg)
	require.NoError(t, err)
	require.NotEmpty(t, inbounds)

	outbounds, _, err := f.Outbound(entryPkg, allPkgs)
	require.NoError(t, err)
	require.NotEmpty(t, outbounds)

	b := NewBuilder(logger, allPkgs, ssaProg, ssaPkgs, SSAASTBidirMap, SigASTBidirMap, true,
		entryPkg, inbounds, outbounds, true, DefaultSerialGraphConfig())
	paths, _, err := b.Build()
	require.NoError(t, err)

	if len(_rpcshieldDemoAllPaths) == 0 {
		for _, path := range paths {
			canonocalPath := path.Edges[0].Edge.Caller.Func.String()
			for _, edge := range path.Edges {
				if edge.Edge.Callee != nil {
					canonocalPath += edge.Edge.Callee.Func.String()
				}
			}
			t.Logf("DISCOVERY path=%q", canonocalPath)
		}
	}

	require.Len(t, paths, len(_rpcshieldDemoAllPaths))
	for _, path := range paths {
		canonocalPath := path.Edges[0].Edge.Caller.Func.String()
		for _, edge := range path.Edges {
			if edge.Edge.Callee != nil {
				canonocalPath += edge.Edge.Callee.Func.String()
			}
		}
		require.Contains(t, _rpcshieldDemoAllPaths, canonocalPath)
	}
}

func TestCCT_WithConvergence(t *testing.T) {
	t.Parallel()
	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	entryPkg, allPkgs, ssaProg, ssaPkgs := testPkg.Entry, testPkg.All, testPkg.SSAProg, testPkg.SSAPkgs
	require.NotNil(t, entryPkg)
	require.Greater(t, len(allPkgs), 0)

	SSAASTBidirMap := mapper.SSAToASTMap(allPkgs, ssaPkgs, ssaProg)
	SigASTBidirMap := mapper.SignatureToASTMap(allPkgs)

	logger := zap.NewNop().Sugar()
	inboundService := grail.Service{Name: "rpcshielddemo", Tier: -1}
	lotteryService := grail.Service{Name: "lottery", Tier: 5}
	clientTypeToService := servicemapping.ServiceMapping{}
	clientTypeToService.AddMappings(
		servicemapping.FullyQualifiedType{PkgPath: "thriftrw/lottery/lotteryclient", Name: "Interface"},
		&servicemapping.ServiceWithIDLInfo{Service: lotteryService},
	)
	serviceMapper := servicemapping.New([]grail.Service{lotteryService}, clientTypeToService, logger)
	f := finder.New(inboundService, serviceMapper, logger, map[string]bool{}, "" /* codebasePkgPrefix */, "" /* kafkaConsumerProxyPkgPath */, nil /* infraPkgPrefixes */, false /* enableGenericGRPCThriftDetection */)

	inbounds, err := f.Inbound(entryPkg, allPkgs, ssaProg)
	require.NoError(t, err)
	require.NotEmpty(t, inbounds)

	outbounds, _, err := f.Outbound(entryPkg, allPkgs)
	require.NoError(t, err)
	require.NotEmpty(t, outbounds)

	cfg := DefaultSerialGraphConfig()
	cfg.MaxFunctionVisits = 2
	b := NewBuilder(logger, allPkgs, ssaProg, ssaPkgs, SSAASTBidirMap, SigASTBidirMap, true,
		entryPkg, inbounds, outbounds, true, cfg)
	paths, buildResult, err := b.Build()
	require.NoError(t, err)
	require.NotNil(t, buildResult)
	require.Greater(t, len(paths), 0)

	hasConvergence := false
	for _, srcMetrics := range buildResult.PerSource {
		if srcMetrics.ConvergencePoints > 0 {
			hasConvergence = true
			break
		}
	}
	require.True(t, hasConvergence, "expected convergence detection with MaxFunctionVisits=2")
}
