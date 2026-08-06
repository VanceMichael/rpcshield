package graph

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/syntheticfunc"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestCalleeIsAClosureInCallchain(t *testing.T) {
	t.Parallel()

	g, _, ssaProg := loadAndInitBuilder(t, []string{"exampleapps", "lambdaincaller", "lambdaincaller-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/lambdaincaller")

	// Setup the source code for the test.
	funcs := make(map[string]*ssa.Function)
	mainFunc := ssaProg.AllPackages()[0].Func("main")

	for fn := range ssautil.AllFunctions(ssaProg) {
		funcs[fn.Name()] = fn
	}

	// Build the call graph using RTA (Rapid Type Analysis)
	rtaResult := rta.Analyze([]*ssa.Function{mainFunc}, true)
	cg := rtaResult.CallGraph

	// Fetch the call graph node for function B
	bFunc := funcs["B"]
	require.NotNil(t, bFunc, "Function B not found")
	bNode := cg.Nodes[bFunc]
	require.NotNil(t, bNode, "Call graph node for function B not found")

	// Verify that B has multiple outgoing edges before applying calleeIsAClosureInCallchain
	initialEdges := bNode.Out
	for _, edge := range initialEdges {
		fmt.Printf("\n initial edges: %s\n", edge.Callee.Func.Name())
	}
	assert.GreaterOrEqual(t, len(initialEdges), 2, "Function B should have multiple outgoing edges before pruning")

	// Collect the names of the callees before pruning
	initialCalleeNames := map[string]struct{}{
		"StopHere$1": {},
		"main$1":     {},
		"f1":         {},
		"f2":         {},
		"f3":         {},
		"f4":         {},
		"f5":         {},
		"f6":         {},
		"f7":         {},
		"f8":         {},
	}

	assert.GreaterOrEqual(t, len(initialEdges), len(initialCalleeNames), "Expected at least 9 callees in B before pruning")
	for name := range initialCalleeNames {
		bFunc, ok := funcs[name]
		require.True(t, ok, "Function %s not found", name)
		// bFunc should be in initialEdges.Callee
		found := false
		for _, edge := range initialEdges {
			fmt.Printf("\n initial edges checking: %s with %s\n", edge.Callee.Func.Name(), name)
			if edge.Callee.Func == bFunc {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected function %s to be a callee of B", name)
	}

	csMap := g.buildCallsiteMap(cg)
	// Apply the function under test
	toDelete := g.calleeIsAClosureInCallchain(bNode, csMap, 5)
	for edge := range toDelete {
		fmt.Printf("\n got: %s\n", edge.Callee.Func.Name())
	}
	// Should prune the edges to f1-8 and main$1
	assert.Equal(t, len(initialCalleeNames)-1, len(toDelete), "Expected 9 edges to be pruned")
	// expect everything except A$1 to be in toDelete.Callee.Func
	for name := range initialCalleeNames {
		if name != "StopHere$1" {
			bFunc, ok := funcs[name]
			require.True(t, ok, "Function %s not found", name)
			// bFunc should be in toDelete.Callee
			found := false
			for edge := range toDelete {
				if edge.Callee.Func == bFunc {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected function %s to be pruned", name)
		}
	}

	// Now
	// Delete the edges.
	g.makeEdgesNilInGraph(cg, toDelete)
	g.cleanupNilsInGraph(cg)
	// Check that the edges have been pruned in bNode (there will be 4 remaining: f, P, Q, and r)
	assert.Equal(t, 4, len(bNode.Out), "Expected 4 outgoing edge from B after pruning")
	// Check that the incoming edges for f1-8 and main$1 have been pruned
	for name := range initialCalleeNames {
		if name != "StopHere$1" {
			f, ok := funcs[name]
			require.True(t, ok, "Function %s not found", name)
			// Incoming edges should be one.
			node := cg.Nodes[f]
			assert.Equal(t, 1, len(node.In), "Expected 1 incoming edge for %s", name)
		}
	}

}

func TestInvokeProbs(t *testing.T) {
	t.Parallel()

	g, _, ssaProg := loadAndInitBuilder(t, []string{"exampleapps", "invokeprobability", "invokeprobability-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/invokeprobability")

	// Setup the source code for the test.
	funcs := make(map[string]*ssa.Function)
	// find any function names main in the ssaProg
	var mainFunc []*ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		funcs[fn.String()] = fn
		fmt.Printf("\n fn: %s\n", fn.String())
		if fn.Name() == "main" {
			mainFunc = append(mainFunc, fn)
		}
	}

	// get the github.com/uber-research/rpcshield/go/exampleapps/invokeprobability.dCaller
	dCaller := funcs["github.com/uber-research/rpcshield/go/exampleapps/invokeprobability.dCaller"]
	require.NotNil(t, dCaller)
	// Build the call graph using RTA (Rapid Type Analysis)
	rtaResult := rta.Analyze(mainFunc, true)
	cg := rtaResult.CallGraph

	// create csMap
	csMap := g.buildCallsiteMap(cg)
	// There should be only one callsite for dCaller, get it and check that it is not nil
	var dCallsite ssa.CallInstruction
	for cs := range csMap {
		if cs.Parent() == dCaller {
			dCallsite = cs
			break
		}
	}
	require.NotNil(t, dCallsite, "Callsite for dCaller not found")
	// Print all the callees of dCaller
	outeEdges := csMap[dCallsite]
	// We should see >1 outgoing edges from dCaller at this point
	require.Greater(t, len(outeEdges), 1, "Expected >1 outgoing edges from dCaller")

	// We want to retain only the edges with probability > 0.99 and drop the rest.
	g.pruneLowProbabilityInvokes(cg, csMap, .99)

	// Now, we should see only one outgoing edge from dCaller
	require.Equal(t, 1, len(csMap[dCallsite]), "Expected 1 outgoing edge from dCaller after pruning")
	// The edge must be to github.com/uber-research/rpcshield/go/exampleapps/invokeprobability/a/b/c/d.F1
	edge := csMap[dCallsite][0]
	require.Equal(t, "(*github.com/uber-research/rpcshield/go/exampleapps/invokeprobability/a/b/c/d.controllerImpl).F1", edge.Callee.Func.String())
	// Also ensure the Node's edges are updated
	dCallerNode := cg.Nodes[dCaller]
	require.Equal(t, 1, len(dCallerNode.Out), "Expected 1 outgoing edge from dCaller after pruning")
	require.Equal(t, "(*github.com/uber-research/rpcshield/go/exampleapps/invokeprobability/a/b/c/d.controllerImpl).F1", dCallerNode.Out[0].Callee.Func.String())
}

func TestHandleSyntheticFunc(t *testing.T) {
	t.Parallel()

	g, _, prog := loadAndInitBuilder(t, []string{"exampleapps", "syntheticcallee", "syntheticcallee-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee")

	// find any function names main in the ssaProg
	var mainFunc []*ssa.Function
	for fn := range ssautil.AllFunctions(prog) {
		if fn.Name() == "main" {
			mainFunc = append(mainFunc, fn)
		}
	}

	// Build the call graph using RTA (Rapid Type Analysis)
	rtaResult := rta.Analyze(mainFunc, true)
	cg := rtaResult.CallGraph

	filtEdge := func(edge *callgraph.Edge) bool {
		return edge.Callee.Func.Synthetic != "" && strings.Contains(edge.Callee.Func.String(), "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee")
	}

	csMap := g.buildCallsiteMap(cg)

	before := make(map[string]int)
	for site, edges := range csMap {
		for _, edge := range edges {
			if filtEdge(edge) {
				before[site.String()]++
			}
		}
	}

	// populate unwrapper and prune low probability invokes
	g.populateUnwrapper(cg)
	g.pruneLowProbabilityInvokes(cg, csMap, .99)

	after := make(map[string]int)
	for site, edges := range csMap {
		for _, edge := range edges {
			if filtEdge(edge) {
				after[site.String()]++
			}
		}
	}

	assert.Equal(t, before, after, "Expected before and after to be equal")
}

// Test unwrap failure by not populating the unwrapper
func TestSyntheticFuncUnwrapFailure(t *testing.T) {
	g, _, prog := loadAndInitBuilder(t, []string{"exampleapps", "syntheticcallee", "syntheticcallee-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee")

	// find any function names main in the ssaProg
	var mainFunc []*ssa.Function
	for fn := range ssautil.AllFunctions(prog) {
		if fn.Name() == "main" {
			mainFunc = append(mainFunc, fn)
		}
	}

	// Build the call graph using RTA (Rapid Type Analysis)
	rtaResult := rta.Analyze(mainFunc, true)
	cg := rtaResult.CallGraph

	filtEdge := func(edge *callgraph.Edge) bool {
		return edge.Callee.Func.Synthetic != "" && strings.Contains(edge.Callee.Func.String(), "github.com/uber-research/rpcshield/go/exampleapps/syntheticcallee")
	}

	csMap := g.buildCallsiteMap(cg)

	// Manually set package to nil for functions that can be unwrapped without
	// using callgraph. This should induce failure for those nodes as well.
	unwrapper := syntheticfunc.NewUnwrapper(g.logger)
	for site, edges := range csMap {
		if !site.Common().IsInvoke() {
			continue
		}
		for _, edge := range edges {
			if filtEdge(edge) {
				unwrapped, err := unwrapper.Unwrap(edge.Callee.Func)
				if err != nil {
					continue
				}
				unwrapped.Pkg = nil
			}
		}
	}

	for site, edges := range csMap {
		if !site.Common().IsInvoke() {
			continue
		}

		edgeProbs := g.getProbabilitiesAtCallsite(cg, site, edges)

		for i, edge := range edges {
			if filtEdge(edge) {
				require.Equal(t, float64(0), edgeProbs[i])
			}
		}
	}
}
