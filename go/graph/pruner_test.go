package graph

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/mapper"
	"github.com/uber-research/rpcshield/go/finder"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/grail"
	"github.com/uber-research/rpcshield/go/rpcshieldtest"
	"go.uber.org/zap"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestBlockListPruning(t *testing.T) {
	t.Parallel()

	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	entryPkg, allPkgs, ssaProg, ssaPkgs := testPkg.Entry, testPkg.All, testPkg.SSAProg, testPkg.SSAPkgs
	// Produce SSA to AST mapping.
	SSAASTBidirMap := mapper.SSAToASTMap(allPkgs, ssaPkgs, ssaProg)
	SigASTBidirMap := mapper.SignatureToASTMap(allPkgs)
	isGlue := true
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
	var filteredOutbounds []fty.OutboundPoint
	for _, ob := range outbounds {
		if ob.Type != fty.TypeHTTP {
			filteredOutbounds = append(filteredOutbounds, ob)
		}
	}
	outbounds = filteredOutbounds
	require.NoError(t, err)
	require.NotEmpty(t, outbounds)

	// Test both parallel and serial call tree computation
	testCases := []struct {
		name                    string
		disableParallelCallTree bool
	}{
		{name: "parallel", disableParallelCallTree: false},
		{name: "serial", disableParallelCallTree: true},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			// Now, create a graph builder with RTA
			cfg := DefaultSerialGraphConfig()
			cfg.DisableParallelCallTree = tc.disableParallelCallTree
			b := NewBuilder(logger, allPkgs, ssaProg, ssaPkgs, SSAASTBidirMap, SigASTBidirMap, isGlue, entryPkg, inbounds, outbounds, true, cfg)
			require.NotNil(t, b)
			rtaBuilder := b.(*builderImpl)
			err = rtaBuilder.setSSAInOutBounds()
			require.NoError(t, err)

			// Lengths of SSA inbounds and outbounds should be the same the ones given to the builder.
			require.Equal(t, len(inbounds), len(rtaBuilder.inboundsSSA))
			require.Equal(t, len(outbounds), len(rtaBuilder.outboundsSSA))
			// Set up the list of blocked packages.
			rtaBuilder.blockedPkgs, err = rtaBuilder.getBlockListPackages(rtaBuilder.ssaProg.AllPackages())
			require.NoError(t, err)
			// Populate needed types.
			rtaBuilder.populateNeededTypes(rtaBuilder.ssaProg)
			cg, err := rtaBuilder.getCG()
			require.NoError(t, err)
			err = rtaBuilder.pruneGraph(cg)
			require.NoError(t, err)
			// All inbounds and outbounds should be present in the pruned graph.
			for _, in := range rtaBuilder.inboundsSSA {
				assert.NotNil(t, cg.Nodes[in.function])
			}
			for _, out := range rtaBuilder.outboundsSSA {
				assert.NotNil(t, cg.Nodes[out.function])
			}
			// cgChecker should not panic.
			cgChecker(cg, true, true)
			cct, err := rtaBuilder.buildCallTree(cg)
			fmt.Printf("cct (%s): %+v\n", tc.name, cct)
			require.NoError(t, err)
			// cct should not be nil.
			assert.NotNil(t, cct)
			// get call paths.
			callPaths, err := rtaBuilder.collectPaths(cct)
			require.NoError(t, err)

			handlerget := "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler.handler).Get"
			controllerget := "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller.controller).Get"
			controllerfailOpenDemo := "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller.controller).failOpenDemo"
			controllerfailCloseDemo := "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller.controller).failCloseDemo"
			lotteryPlay := "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/lottery.gateway).Play"

			neededFuncs := map[string]*ssa.Function{
				handlerget:              nil,
				controllerget:           nil,
				controllerfailOpenDemo:  nil,
				controllerfailCloseDemo: nil,
				lotteryPlay:             nil,
			}
			// populate neededFuncs with the actual functions from ssautil.
			for f := range ssautil.AllFunctions(ssaProg) {
				if _, ok := neededFuncs[f.String()]; ok {
					neededFuncs[f.String()] = f
				}
			}
			// check if all the needed functions are present in the call paths.
			for _, f := range neededFuncs {
				assert.NotNil(t, f)
			}
			expectedPaths := [][]*ssa.Function{
				[]*ssa.Function{neededFuncs[handlerget], neededFuncs[controllerget], neededFuncs[controllerfailOpenDemo], neededFuncs[lotteryPlay]},
				[]*ssa.Function{neededFuncs[handlerget], neededFuncs[controllerget], neededFuncs[controllerfailCloseDemo], neededFuncs[lotteryPlay]},
			}

			// Ensure that the needed paths are present in the callPaths, order does not matter.
			for _, expectedPath := range expectedPaths {
				found := false
				for _, cp := range callPaths {
					if len(cp) != len(expectedPath) {
						continue
					}
					// Walk each edge
					for idx, e := range cp {
						require.NotNil(t, e)
						require.NotNil(t, e.Caller)
						if e.Caller.Func != expectedPath[idx] {
							break
						}
						if idx == len(cp)-1 {
							found = true
						}
					}
					if found == true {
						break
					}
				}
				assert.True(t, found)
			}

			// Print the call paths.
			for _, cp := range callPaths {
				assert.NotNil(t, cp)
				fmt.Printf("\n\n\n")
				for _, edge := range cp {
					assert.NotNil(t, edge.Caller)
					assert.NotNil(t, edge.Caller.Func)
					fmt.Printf("\n edge (%s): %s\n", tc.name, edge.Caller.Func.String())
				}
			}
		})
	}
}

func TestIsNecessaryNode(t *testing.T) {
	// Create a synthetic SSA function and node
	funcType := &ssa.Function{}
	node := &callgraph.Node{Func: funcType}

	// Create a builderImpl with the function in the inboundMap
	builder := &builderImpl{
		inboundMap: map[*ssa.Function]inbound{
			funcType: {},
		},
		outboundMap: map[*ssa.Function]outbound{},
	}

	result := builder.isNecessaryNode(node)
	assert.True(t, result, "Node should be considered necessary when found in inboundMap")

	// Create a synthetic SSA function and node
	funcType = &ssa.Function{}
	node = &callgraph.Node{Func: funcType}

	// Create a builderImpl with the function in the outboundMap
	builder = &builderImpl{
		inboundMap: map[*ssa.Function]inbound{},
		outboundMap: map[*ssa.Function]outbound{
			funcType: {},
		},
	}

	result = builder.isNecessaryNode(node)
	assert.True(t, result, "Node should be considered necessary when found in outboundMap")

	// Create a synthetic SSA function and node
	funcType = &ssa.Function{}
	node = &callgraph.Node{Func: funcType}

	// Create a builderImpl with the function not in either map
	builder = &builderImpl{
		inboundMap:  map[*ssa.Function]inbound{},
		outboundMap: map[*ssa.Function]outbound{},
	}

	result = builder.isNecessaryNode(node)
	assert.False(t, result, "Node should not be considered necessary when not found in either inboundMap or outboundMap")

	funcType = &ssa.Function{}
	node = &callgraph.Node{Func: funcType}
}
