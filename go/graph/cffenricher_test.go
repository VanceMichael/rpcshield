package graph

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// findCFFFlowFuncs locates every *ssa.Function carrying CFF scheduling calls for the named
// top-level function baseName. Under the current inline go.uber.org/cff codegen, each flow/parallel
// body (including every "sched.Enqueue(ctx, cff.Job{...})" call) lives inside its own anonymous,
// immediately-invoked closure ("baseName$1", "baseName$2", ...) -- a function with multiple
// cff.Flow/cff.Parallel calls (see TestCFFMix) gets one such closure per call -- unlike the older,
// internal-fork-only codegen shape, where the generated "_cffFlow..."/"_cffParallel..." call sat
// directly in baseName's own body. Searching by cffEnqueueType/cffFlowType/cffParallelType
// callsites (rather than hardcoding "$1") keeps this working for both shapes.
func findCFFFlowFuncs(b *builderImpl, ssaProg *ssa.Program, baseName string) []*ssa.Function {
	var funcs []*ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if !strings.HasPrefix(fn.Name(), baseName) {
			continue
		}
		cffInstrs := b.getCFFCallsites(fn)
		if len(cffInstrs[cffEnqueueType]) > 0 || len(cffInstrs[cffFlowType]) > 0 || len(cffInstrs[cffParallelType]) > 0 {
			funcs = append(funcs, fn)
		}
	}
	return funcs
}

// cffEnqueueOwners maps every cffEnqueueType callsite found across funcs to the function it
// belongs to, so callers can process/verify callsites without caring which flow/parallel closure
// each one came from.
func cffEnqueueOwners(b *builderImpl, funcs []*ssa.Function) map[ssa.CallInstruction]*ssa.Function {
	owners := map[ssa.CallInstruction]*ssa.Function{}
	for _, fn := range funcs {
		for callsite := range b.getCFFCallsites(fn)[cffEnqueueType] {
			owners[callsite] = fn
		}
	}
	return owners
}

// cffEnqueueEdgeCallees returns the names of every callee reachable, after enrichWithCFFCall, via
// an edge whose Site is one of owners' callsites -- i.e. the task closures wired in as async
// callees of their owning flow/parallel closure.
func cffEnqueueEdgeCallees(cg *callgraph.Graph, owners map[ssa.CallInstruction]*ssa.Function) map[string]struct{} {
	found := map[string]struct{}{}
	for callsite, fn := range owners {
		node := cg.Nodes[fn]
		if node == nil {
			continue
		}
		for _, edge := range node.Out {
			if edge.Site == callsite {
				found[edge.Callee.Func.Name()] = struct{}{}
			}
		}
	}
	return found
}

func TestAddCGEdges(t *testing.T) {
	t.Parallel()

	// Create synthetic graph and nodes for testing
	cg := callgraph.New(nil)
	fA := &ssa.Function{}
	fB := &ssa.Function{}
	nodeA := cg.CreateNode(fA)
	nodeB := cg.CreateNode(fB)
	cg.CreateNode(fA)
	cg.CreateNode(fB)

	// Create synthetic callsites
	callInst := &mockCallInstruction{} // Replace with mock implementation of ssa.CallInstruction
	callsites := map[ssa.CallInstruction]map[*ssa.Function]struct{}{
		callInst: {nodeB.Func: {}},
	}

	// Create builder and invoke addCGEdges
	builder := &builderImpl{}
	result, _ := builder.addCGEdges(callsites, nodeA, cg, false)

	// Assertions
	require.NotNil(t, result)
	assert.Len(t, result, 1)    // Ensure one callsite is processed
	assert.Len(t, nodeA.Out, 1) // Ensure one outgoing edge from nodeA
	assert.Len(t, nodeB.In, 1)  // Ensure one incoming edge to nodeB

	// Check if the edge is correctly formed
	edge := nodeA.Out[0]
	assert.Equal(t, nodeA, edge.Caller)
	assert.Equal(t, nodeB, edge.Callee)
	assert.Equal(t, callInst, edge.Site)

	// Negative case: edge to a function without a node
	fC := &ssa.Function{}
	callsites = map[ssa.CallInstruction]map[*ssa.Function]struct{}{
		callInst: {fC: {}},
	}
	result, newEdges := builder.addCGEdges(callsites, nodeA, cg, false)
	assert.Len(t, result, 0)   // Ensure no callsites are processed
	assert.Len(t, newEdges, 0) // Ensure no new edges are added
}

func TestRemoveIncomingEdge(t *testing.T) {
	t.Parallel()

	// Create synthetic graph and nodes for testing
	fa := &ssa.Function{}
	fb := &ssa.Function{}
	fc := &ssa.Function{}
	abSite := &mockCallInstruction{}
	acSite := &mockCallInstruction{}
	cg := callgraph.New(nil)

	nodeA := cg.CreateNode(fa)
	nodeB := cg.CreateNode(fb)
	nodeC := cg.CreateNode(fc)
	callgraph.AddEdge(nodeA, abSite, nodeB)
	require.Len(t, nodeA.Out, 1)
	abEdge := nodeA.Out[0]
	var acEdge *callgraph.Edge
	callgraph.AddEdge(nodeA, acSite, nodeC)
	for _, edge := range nodeA.Out {
		if edge.Callee == nodeC {
			acEdge = edge
			break
		}
	}
	require.NotNil(t, acEdge)
	cbSite := &mockCallInstruction{}
	callgraph.AddEdge(nodeC, cbSite, nodeB)
	cbEdge := nodeC.Out[0]

	// Create builder
	builder := &builderImpl{}

	// Test: Remove a valid edge
	err := builder.removeIncomingEdge(nodeB, abEdge)
	require.NoError(t, err)
	assert.Len(t, nodeA.Out, 2)            // This does not change A's outgoing edges/
	assert.Len(t, nodeB.In, 1)             // B's incoming edge is removed.
	assert.Equal(t, &cbEdge, &nodeB.In[0]) // Ensure the correct edge is removed.

	// Test: Attempt to remove a nil edge
	err = builder.removeIncomingEdge(nodeA, nil)
	require.Error(t, err)

	// Test: Attempt to remove edge from nil node
	err = builder.removeIncomingEdge(nil, cbEdge)
	require.Error(t, err)

	// Test: Removing edge not present in the list
	err = builder.removeIncomingEdge(nodeA, abEdge)
	require.Error(t, err)
}

func TestDeleteCGEdges(t *testing.T) {
	t.Parallel()

	// Create synthetic graph and nodes for testing
	cg := callgraph.New(nil)
	fA := &ssa.Function{}
	fB := &ssa.Function{}
	fC := &ssa.Function{}
	callInst1 := &mockCallInstruction{} // Callsite for A -> B
	callInst2 := &mockCallInstruction{} // Callsite for A -> C
	callInst3 := &mockCallInstruction{} // Callsite for C -> B

	nodeA := cg.CreateNode(fA)
	nodeB := cg.CreateNode(fB)
	nodeC := cg.CreateNode(fC)

	// Add edges: A -> B, A -> C, and C -> B
	callgraph.AddEdge(nodeA, callInst1, nodeB)
	edgeAB := nodeA.Out[0]
	callgraph.AddEdge(nodeA, callInst2, nodeC)
	edgeAC := nodeA.Out[1]
	callgraph.AddEdge(nodeC, callInst3, nodeB)
	edgeCB := nodeC.Out[0]

	require.NotNil(t, edgeAB)
	require.NotNil(t, edgeAC)
	require.NotNil(t, edgeCB)

	// Initial state assertions
	assert.Len(t, nodeA.Out, 2) // A should have two outgoing edges: A -> B, A -> C
	assert.Len(t, nodeB.In, 2)  // B should have two incoming edges: A -> B, C -> B
	assert.Len(t, nodeC.Out, 1) // C should have one outgoing edge: C -> B
	assert.Len(t, nodeC.In, 1)  // C should have one incoming edge: A -> C

	// Create a callsite map to delete edge corresponding to A -> B
	callsites := map[ssa.CallInstruction]map[*ssa.Function]struct{}{
		callInst1: {fB: {}}, // We want to remove the edge A -> B
	}

	// Create builder and invoke deleteCGEdges
	builder := &builderImpl{}
	builder.deleteCGEdges(callsites, nodeA, cg)

	// Assertions after removing A -> B
	assert.Len(t, nodeA.Out, 1)           // Only one outgoing edge (A -> C) should remain
	assert.Equal(t, edgeAC, nodeA.Out[0]) // Ensure A -> C edge remains

	assert.Len(t, nodeB.In, 1)           // B should have one incoming edge (C -> B)
	assert.Equal(t, edgeCB, nodeB.In[0]) // Ensure C -> B edge remains

	assert.Len(t, nodeC.Out, 1)           // C should still have one outgoing edge (C -> B)
	assert.Equal(t, edgeCB, nodeC.Out[0]) // Ensure C -> B remains

	// Test edge deletion with no match (try to remove edge C -> A which doesn't exist)
	callsites = map[ssa.CallInstruction]map[*ssa.Function]struct{}{
		&mockCallInstruction{}: {fA: {}}, // No matching callsite in A's outgoing edges
	}
	builder.deleteCGEdges(callsites, nodeA, cg)

	// Ensure no changes are made since there's no matching edge
	assert.Len(t, nodeA.Out, 1) // Still only one outgoing edge
	assert.Equal(t, edgeAC, nodeA.Out[0])

	// Test deleting edge A -> C
	callsites = map[ssa.CallInstruction]map[*ssa.Function]struct{}{
		callInst2: {fC: {}}, // We want to remove the edge A -> C
	}
	builder.deleteCGEdges(callsites, nodeA, cg)

	// Assertions after removing A -> C
	assert.Len(t, nodeA.Out, 0) // No outgoing edges should remain for A
	assert.Len(t, nodeC.In, 0)  // C should have no incoming edges now

	// Ensure C -> B still remains after the other deletions
	assert.Len(t, nodeC.Out, 1)           // C should still have one outgoing edge (C -> B)
	assert.Equal(t, edgeCB, nodeC.Out[0]) // Ensure C -> B remains
	assert.Len(t, nodeB.In, 1)            // B should still have one incoming edge (C -> B)
	assert.Equal(t, edgeCB, nodeB.In[0])  // Ensure C -> B is intact
}

// mockCallInstruction is a mock implementation of ssa.CallInstruction for testing.
type mockCallInstruction struct {
	ssa.CallInstruction
}

func TestAddRelevantStoreTypeRefs(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "storeinstructions", "storeinstructions-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/storeinstructions"

	_, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "CodeUnderTest" {
			f = fn
			break
		}
	}
	require.NotNil(t, f, "Failed to find function CodeUnderTest")

	// Extract SSA instructions from the first block of the function
	block := f.Blocks[0]
	var storeInstr *ssa.Store
	var indexAddrInstr *ssa.IndexAddr
	var fieldAddrInstr *ssa.FieldAddr

	// Identify the instructions we want to test
	for _, instr := range block.Instrs {
		switch i := instr.(type) {
		case *ssa.Store:
			storeInstr = i
		case *ssa.IndexAddr:
			indexAddrInstr = i
		case *ssa.FieldAddr:
			fieldAddrInstr = i
		}
	}

	require.NotNil(t, storeInstr, "Expected a store instruction")
	require.NotNil(t, indexAddrInstr, "Expected an IndexAddr instruction")
	require.NotNil(t, fieldAddrInstr, "Expected a FieldAddr instruction")

	// Test: Add only Store instructions to stack
	refs := []ssa.Instruction{storeInstr}
	stack := []ssa.Value{}
	stack = addRelevantStoreTypeRefs(&refs, stack, f)

	require.Len(t, stack, 1)
	assert.Equal(t, storeInstr.Val, stack[0]) // Ensure the value stored (Val) is added to the stack

	// Case 2: Test adding IndexAddr instructions to stack
	refs = []ssa.Instruction{indexAddrInstr}
	stack = []ssa.Value{}
	stack = addRelevantStoreTypeRefs(&refs, stack, f)

	require.Len(t, stack, 1)
	assert.Equal(t, indexAddrInstr, stack[0]) // Ensure the IndexAddr instruction is added

	// Case 3: Test adding FieldAddr instructions to stack
	refs = []ssa.Instruction{fieldAddrInstr}
	stack = []ssa.Value{}
	stack = addRelevantStoreTypeRefs(&refs, stack, f)

	require.Len(t, stack, 1)
	assert.Equal(t, fieldAddrInstr, stack[0]) // Ensure the FieldAddr instruction is added

	// Case 4: Combination of Store, IndexAddr, and FieldAddr
	refs = []ssa.Instruction{storeInstr, indexAddrInstr, fieldAddrInstr}
	stack = []ssa.Value{}
	stack = addRelevantStoreTypeRefs(&refs, stack, f)

	require.Len(t, stack, 3)
	assert.Equal(t, storeInstr.Val, stack[0]) // Ensure Store.Val is added
	assert.Equal(t, indexAddrInstr, stack[1]) // Ensure IndexAddr is added
	assert.Equal(t, fieldAddrInstr, stack[2]) // Ensure FieldAddr is added

	// Negative case not a parent function
	refs = []ssa.Instruction{storeInstr}
	stack = []ssa.Value{}
	stack = addRelevantStoreTypeRefs(&refs, stack, &ssa.Function{})
	require.Len(t, stack, 0)
}

func TestCFFFullFlow(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffapp", "cffapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find the function(s) carrying the CFF scheduling calls (see findCFFFlowFuncs)
	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")
	expected := map[string]struct{}{
		"AFunctionWithUniqueName$1$7":  {},
		"AFunctionWithUniqueName$1$8":  {},
		"AFunctionWithUniqueName$1$9":  {},
		"AFunctionWithUniqueName$1$10": {},
		"AFunctionWithUniqueName$1$11": {},
		"AFunctionWithUniqueName$1$12": {},
		"AFunctionWithUniqueName$1$13": {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	owners := cffEnqueueOwners(b, funcs)
	require.Len(t, owners, len(expected))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}

	for callsite, fn := range owners {
		re, err := b.processCFFEnqueueCall(callsite.Common(), fn)
		require.NoError(t, err)
		require.Len(t, re, 1)
		for r := range re {
			found[r.Name()] = struct{}{}
		}
	}
	assert.Equal(t, len(expected), len(found))
	for k := range expected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}

	// Finally run the full flow.
	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)
	// Check the enqueue callsites' edges in the callgraph.
	found = cffEnqueueEdgeCallees(cg, owners)
	assert.Equal(t, len(expected), len(found))
	for k := range expected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestCFFPara(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffapppara", "cffapppara-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffapppara"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find the function(s) carrying the CFF scheduling calls (see findCFFFlowFuncs)
	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")
	expected := map[string]struct{}{
		"AFunctionWithUniqueName$1$6": {},
		"AFunctionWithUniqueName$1$7": {},
		"AFunctionWithUniqueName$1$8": {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	owners := cffEnqueueOwners(b, funcs)
	require.Len(t, owners, len(expected))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}

	for callsite, fn := range owners {
		re, err := b.processCFFEnqueueCall(callsite.Common(), fn)
		require.NoError(t, err)
		require.Len(t, re, 1)
		for r := range re {
			found[r.Name()] = struct{}{}
		}
	}
	assert.Equal(t, len(expected), len(found))
	for k := range expected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}

	// Finally run the full flow.
	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)
	// Check the enqueue callsites' edges in the callgraph.
	found = cffEnqueueEdgeCallees(cg, owners)
	assert.Equal(t, len(expected), len(found))
	for k := range expected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestCFFMix(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffappmix", "cffappmix-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffappmix"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find the function(s) carrying the CFF scheduling calls (see findCFFFlowFuncs)
	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")
	expected := map[string]struct{}{
		"AFunctionWithUniqueName$1$6": {},
		"AFunctionWithUniqueName$1$7": {},
		"AFunctionWithUniqueName$1$8": {},
		"AFunctionWithUniqueName$2$5": {},
		"AFunctionWithUniqueName$2$6": {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	owners := cffEnqueueOwners(b, funcs)
	require.Len(t, owners, len(expected))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}

	for callsite, fn := range owners {
		re, err := b.processCFFEnqueueCall(callsite.Common(), fn)
		require.NoError(t, err)
		require.Len(t, re, 1)
		for r := range re {
			found[r.Name()] = struct{}{}
		}
	}
	assert.Equal(t, len(expected), len(found))
	for k := range expected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}

	// Finally run the full flow.
	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)
	// Check the enqueue callsites' edges in the callgraph.
	found = cffEnqueueEdgeCallees(cg, owners)
	assert.Equal(t, len(expected), len(found))
	for k := range expected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestCFFSlice(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffappslice", "cffappslice-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffappslice"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true

	//find the function(s) carrying the CFF scheduling calls (see findCFFFlowFuncs)
	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")
	expectedTask := map[string]struct{}{
		"AFunctionWithUniqueName$1$6": {},
		"AFunctionWithUniqueName$1$7": {},
	}
	expectedSlice := map[string]struct{}{
		"AFunctionWithUniqueName$1$8": {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	owners := cffEnqueueOwners(b, funcs)
	require.Len(t, owners, len(expectedTask)+len(expectedSlice))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}
	for callsite, fn := range owners {
		re, err := b.processCFFEnqueueCall(callsite.Common(), fn)
		require.NoError(t, err)
		require.Len(t, re, 1)
		for r := range re {
			found[r.Name()] = struct{}{}
		}
	}
	assert.Equal(t, len(expectedSlice)+len(expectedTask), len(found))
	for k := range expectedTask {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
	for k := range expectedSlice {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}

	// Finally run the full flow.
	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)
	// Check the enqueue callsites' edges in the callgraph.
	found = cffEnqueueEdgeCallees(cg, owners)
	assert.Equal(t, len(expectedSlice)+len(expectedTask), len(found))
	for k := range expectedSlice {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
	for k := range expectedTask {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestCFFFallbackWith(t *testing.T) {
	t.Parallel()
	pathSlice := []string{"exampleapps", "cfffallback", "cfffallback-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cfffallback"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find the function(s) carrying the CFF scheduling calls (see findCFFFlowFuncs)
	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")
	expected := map[string]struct{}{
		"AFunctionWithUniqueName$1$7":  {}, // task0, no fallback
		"AFunctionWithUniqueName$1$10": {}, // task2, no fallback
		"AFunctionWithUniqueName$1$11": {}, // task4, no fallback
		// task3 (generalCall) and task1 are fallback-guarded (cff.FallbackWith) -- excluded, see
		// hasCFFFallback.
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)
	owners := cffEnqueueOwners(b, funcs)
	require.NotEmpty(t, owners)

	cgChecker(cg, true, true)

	// Finally run the full flow.
	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)
	// Check the enqueue callsites' edges in the callgraph.
	found := cffEnqueueEdgeCallees(cg, owners)
	assert.Equal(t, len(expected), len(found))
	for k := range expected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

// TestCFFBoundSliceCapture guards against a regression where a CFF task closure's captured
// bindings (here, a slice of bound methods invoked internally in an ordinary loop -- never
// scheduled as CFF tasks themselves) get spuriously treated as additional CFF task functions.
// This is doubly relevant under the inline-codegen shape: the generated "cff.Job{Run: ...}"
// wrapper closure itself captures the user's real task closure as a binding (see the
// *ssa.MakeClosure case comment in ssaValueVisitor), so a correct fix must not leak through that
// extra layer of indirection either. The bound methods (ValidateA$bound, ValidateB$bound) must
// NOT show up as found task functions; only the real task closure itself may.
func TestCFFBoundSliceCapture(t *testing.T) {
	t.Parallel()
	pathSlice := []string{"exampleapps", "cffappboundslice", "cffappboundslice-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffappboundslice"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find the function(s) carrying the CFF scheduling calls (see findCFFFlowFuncs)
	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")
	expected := map[string]struct{}{
		"AFunctionWithUniqueName$1$4": {},
		// "ValidateA$bound": {}, --> must NOT be found: it's captured data, not a CFF task
		// "ValidateB$bound": {}, --> must NOT be found: it's captured data, not a CFF task
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)
	owners := cffEnqueueOwners(b, funcs)
	require.NotEmpty(t, owners)

	cgChecker(cg, true, true)

	for callsite, fn := range owners {
		re, err := b.processCFFEnqueueCall(callsite.Common(), fn)
		require.NoError(t, err)
		require.Len(t, re, 1, "captured bound methods must not be misclassified as additional CFF tasks")
	}

	// Finally run the full flow.
	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)
	// Check the enqueue callsites' edges in the callgraph.
	found := cffEnqueueEdgeCallees(cg, owners)
	assert.Equal(t, len(expected), len(found))
	for k := range expected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
	_, ok := found["ValidateA$bound"]
	assert.False(t, ok, "ValidateA$bound must not be found as a CFF task; it is data captured by the task closure")
	_, ok = found["ValidateB$bound"]
	assert.False(t, ok, "ValidateB$bound must not be found as a CFF task; it is data captured by the task closure")
}

// TestCFFCtxlessSliceReachable is a regression guard for a fixed bug: a CFF task closure which
// does not take context.Context as its first parameter -- a shape cff.Slice's own doc comment
// documents as valid ("Optionally, a slice index of type int can be provided as a first ...
// parameter: func(idx int, item SomeType) { ... }"), and the exact shape ads-campaign-manager's
// real ValidateAdGroups uses -- used to be silently dropped from the call graph entirely, even
// though it unconditionally reaches an outbound (Sink). cffContextCallingFilter now accepts
// ctx-less task functions (excluding CFF's own synthesized helper functions by name), so this
// closure -- and Sink through it -- must be found/reachable.
//
// The fixture (exampleapps/cffappctxlessslice/app_gen.go) is not the older internal-fork shape
// nor the current OSS inline-codegen shape that cffenricher.go's other tests exercise: it is the
// checked-in ground truth of what the CURRENT internal `cff` codegen tool emits for this
// signature -- a hybrid shape where the original call site is rewritten to call a separately
// synthesized, uniquely-named top-level generic function (_cffParallel..._<line>_<col>), whose
// second argument is itself a call to another synthesized generic function
// (_cffSlice..._<line>_<col>) that takes the user's RAW original closure directly. That raw
// closure is exactly what processCFFTaskCall's fn.Args[0] extraction hands to
// cffContextCallingFilter.
func TestCFFCtxlessSliceReachable(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffappctxlessslice", "cffappctxlessslice-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffappctxlessslice"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	b.hasMain = true

	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")

	cg, err := b.getCG()
	require.NoError(t, err)
	cgChecker(cg, true, true)

	// Directly exercise processCFFFlowCall/processCFFTaskCall on the cffParallelType callsite,
	// exactly as processCFFInFunc would: confirm it finds the ctx-less slice closure.
	foundAny := false
	for _, fn := range funcs {
		cffInstrs := b.getCFFCallsites(fn)
		for callsite := range cffInstrs[cffParallelType] {
			funcsFound, err := b.processCFFFlowCall(callsite.Common(), fn, cffInstrs[cffSliceType], cg)
			require.NoError(t, err)
			if len(funcsFound) > 0 {
				foundAny = true
			}
			for f := range funcsFound {
				assert.Equal(t, "AFunctionWithUniqueName$1", f.Name(), "expected the real ctx-less slice closure, not a CFF-synthesized helper")
			}
		}
	}
	assert.True(t, foundAny, "expected the ctx-less cff.Slice closure to be found, not dropped")

	// Now confirm end-to-end: after full CFF enrichment, Sink must be reachable from
	// AFunctionWithUniqueName, and the new edge(s) must be marked don't-prune so they survive
	// CCT/SCCP path construction.
	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)

	entry := ssautil.AllFunctions(ssaProg)
	var entryFn *ssa.Function
	for fn := range entry {
		if fn.Name() == "AFunctionWithUniqueName" {
			entryFn = fn
		}
	}
	require.NotNil(t, entryFn, "could not find AFunctionWithUniqueName in SSA program")

	reachable := map[*ssa.Function]bool{}
	var dontPruneEdgeSeen bool
	var visit func(fn *ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil || reachable[fn] {
			return
		}
		reachable[fn] = true
		node := cg.Nodes[fn]
		if node == nil {
			return
		}
		for _, edge := range node.Out {
			if _, ok := b.dontPruneEdges[edge]; ok {
				dontPruneEdgeSeen = true
			}
			if edge.Callee != nil {
				visit(edge.Callee.Func)
			}
		}
	}
	visit(entryFn)

	foundSink := false
	for fn := range reachable {
		if fn.Name() == "Sink" {
			foundSink = true
		}
	}
	assert.True(t, foundSink, "Sink must be reachable from AFunctionWithUniqueName now that the ctx-less cff.Slice closure is recovered")
	assert.True(t, dontPruneEdgeSeen, "expected at least one CFF-added edge on the path to be marked don't-prune")
}

// TestCFFMixedParallelHybridShape stress-tests the hybrid codegen shape (see
// TestCFFCtxlessSliceReachable's doc comment for background) against variations not covered by
// the ctxlessslice fixture:
//
//   - AFunctionWithUniqueName mixes a plain cff.Task, a cff.Slice, and a cff.Task passed as a
//     BOUND METHOD (not a literal closure) in a single cff.Parallel, exercising whether the
//     per-argument generic seed-function wrapping (_cffTask.../_cffSlice...) generalizes when
//     multiple such wrappers appear together, and whether the bound-method's *ssa.MakeClosure
//     wrapper is still recovered correctly (mirroring cffappboundslice's guard, but for a bound
//     method passed DIRECTLY as the task argument rather than captured as data in a slice).
//   - AFunctionWithUniqueNameTwoSlices has two cff.Slice arguments over the SAME element type,
//     i.e. two structurally-identical _cffSliceXXX generic instantiations, to check for
//     cross-contamination between the two closures.
//
// The fixture (exampleapps/cffappmixedparallel/app_gen.go) is checked-in ground truth captured
// from the current internal `cff` codegen tool the same way as cffappctxlessslice's.
func TestCFFMixedParallelHybridShape(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffappmixedparallel", "cffappmixedparallel-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffappmixedparallel"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	b.hasMain = true

	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function(s) carrying CFF scheduling calls")

	var mixedFn, twoSlicesFn *ssa.Function
	for _, fn := range funcs {
		switch fn.Name() {
		case "AFunctionWithUniqueName":
			mixedFn = fn
		case "AFunctionWithUniqueNameTwoSlices":
			twoSlicesFn = fn
		}
	}
	require.NotNil(t, mixedFn, "expected to find AFunctionWithUniqueName")
	require.NotNil(t, twoSlicesFn, "expected to find AFunctionWithUniqueNameTwoSlices")

	cg, err := b.getCG()
	require.NoError(t, err)
	cgChecker(cg, true, true)

	// --- Scenario 1+2: mixed cff.Task/cff.Slice/bound-method-cff.Task in one cff.Parallel. ---
	mixedCffInstrs := b.getCFFCallsites(mixedFn)
	require.NotEmpty(t, mixedCffInstrs[cffParallelType], "expected a cff.Parallel callsite in AFunctionWithUniqueName")
	for callsite := range mixedCffInstrs[cffParallelType] {
		taskFuncs, err := b.processCFFFlowCall(callsite.Common(), mixedFn, mixedCffInstrs[cffTaskType], cg)
		require.NoError(t, err)
		sliceFuncs, err := b.processCFFFlowCall(callsite.Common(), mixedFn, mixedCffInstrs[cffSliceType], cg)
		require.NoError(t, err)

		// Two cff.Task arguments (literal closure + bound method) must both be recovered, plus one
		// cff.Slice closure -- three total, none of them a synthesized CFF helper.
		assert.Len(t, taskFuncs, 2, "expected both the literal closure and the bound-method task to be recovered from cff.Task")
		assert.Len(t, sliceFuncs, 1, "expected exactly one closure recovered from cff.Slice")

		for f := range taskFuncs {
			assert.NotRegexp(t, `_cff[A-Z].+_\d+`, f.Name(), "must not surface a synthesized CFF helper as a task function: %s", f.Name())
		}
		for f := range sliceFuncs {
			assert.NotRegexp(t, `_cff[A-Z].+_\d+`, f.Name(), "must not surface a synthesized CFF helper as a task function: %s", f.Name())
		}
	}

	// --- Scenario 3: two cff.Slice arguments over the same element type; must not cross-contaminate. ---
	twoSlicesCffInstrs := b.getCFFCallsites(twoSlicesFn)
	require.NotEmpty(t, twoSlicesCffInstrs[cffParallelType], "expected a cff.Parallel callsite in AFunctionWithUniqueNameTwoSlices")
	for callsite := range twoSlicesCffInstrs[cffParallelType] {
		sliceFuncs, err := b.processCFFFlowCall(callsite.Common(), twoSlicesFn, twoSlicesCffInstrs[cffSliceType], cg)
		require.NoError(t, err)
		assert.Len(t, sliceFuncs, 2, "expected both distinct cff.Slice closures to be recovered, not merged/duplicated/dropped")

		seen := map[string]struct{}{}
		for f := range sliceFuncs {
			seen[f.Name()] = struct{}{}
		}
		assert.Len(t, seen, 2, "the two same-element-type cff.Slice closures must be kept distinct, not cross-contaminated into a single function")
	}

	// --- End-to-end reachability: after enrichment, every Sink must be reachable exactly through
	// its own closure, with no cross-contamination collapsing distinct outbound paths. ---
	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)

	entry := ssautil.AllFunctions(ssaProg)
	findEntry := func(name string) *ssa.Function {
		for fn := range entry {
			if fn.Name() == name {
				return fn
			}
		}
		return nil
	}

	reachableFrom := func(start *ssa.Function) map[string]bool {
		reachable := map[*ssa.Function]bool{}
		names := map[string]bool{}
		var visit func(fn *ssa.Function)
		visit = func(fn *ssa.Function) {
			if fn == nil || reachable[fn] {
				return
			}
			reachable[fn] = true
			names[fn.Name()] = true
			node := cg.Nodes[fn]
			if node == nil {
				return
			}
			for _, edge := range node.Out {
				if edge.Callee != nil {
					visit(edge.Callee.Func)
				}
			}
		}
		visit(start)
		return names
	}

	mixedEntry := findEntry("AFunctionWithUniqueName")
	require.NotNil(t, mixedEntry)
	mixedReachable := reachableFrom(mixedEntry)
	assert.True(t, mixedReachable["SinkOne"], "SinkOne must be reachable via the literal cff.Task closure")
	assert.True(t, mixedReachable["SinkTwo"], "SinkTwo must be reachable via the cff.Slice closure")
	assert.True(t, mixedReachable["SinkThree"], "SinkThree must be reachable via the bound-method cff.Task")

	twoSlicesEntry := findEntry("AFunctionWithUniqueNameTwoSlices")
	require.NotNil(t, twoSlicesEntry)
	twoSlicesReachable := reachableFrom(twoSlicesEntry)
	assert.True(t, twoSlicesReachable["SinkOne"], "SinkOne must be reachable via the first cff.Slice closure")
	assert.True(t, twoSlicesReachable["SinkTwo"], "SinkTwo must be reachable via the second cff.Slice closure")
}

// reachableFuncNames returns the names of every *ssa.Function reachable from entry in cg.
func reachableFuncNames(cg *callgraph.Graph, entry *ssa.Function) map[string]bool {
	names := map[string]bool{}
	visited := map[*ssa.Function]bool{}
	var visit func(fn *ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil || visited[fn] {
			return
		}
		visited[fn] = true
		names[fn.Name()] = true
		node := cg.Nodes[fn]
		if node == nil {
			return
		}
		for _, edge := range node.Out {
			if edge.Callee != nil {
				visit(edge.Callee.Func)
			}
		}
	}
	visit(entry)
	return names
}

// findFuncByName returns the *ssa.Function named name in ssaProg, or nil.
func findFuncByName(ssaProg *ssa.Program, name string) *ssa.Function {
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == name {
			return fn
		}
	}
	return nil
}

// TestCFFMapReachable is a regression guard for a fixed bug: cff.Map had no dedicated regex
// pattern in cffenricher.go at all (unlike cff.Slice/cff.Task), so a cff.Map call site always
// fell into the generic cffRemainingType bucket, which is only used for edge deletion, never for
// task-closure recovery -- every cff.Map closure (ctx-first or ctx-less) was silently dropped
// from the call graph unconditionally. cffMapType/_cffMapPattern now classify it correctly and
// processCFFInFunc's cffParallelType handling recovers it the same way as cff.Slice.
func TestCFFMapReachable(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffappmap", "cffappmap-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffappmap"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	b.hasMain = true

	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")

	cg, err := b.getCG()
	require.NoError(t, err)
	cgChecker(cg, true, true)

	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)

	entry := findFuncByName(ssaProg, "AFunctionWithUniqueName")
	require.NotNil(t, entry, "could not find AFunctionWithUniqueName in SSA program")

	reachable := reachableFuncNames(cg, entry)
	assert.True(t, reachable["Sink"], "Sink must be reachable through the ctx-less cff.Map closure")
}

// TestCFFSliceEndReachable is a regression guard for a fixed bug: cff.SliceEnd's closure is
// wrapped (under the current internal codegen's hybrid shape) in a synthesized
// _cffSliceEnd..._<line>_<col> call nested as a trailing argument to the
// _cffSlice..._<line>_<col> wrapper call -- processCFFTaskCall only ever inspected the wrapper's
// first argument (the main per-item closure), so the SliceEnd closure (and anything it reaches)
// was never found. findCFFEndFuncArg now recovers it explicitly.
func TestCFFSliceEndReachable(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffappsliceend", "cffappsliceend-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffappsliceend"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	b.hasMain = true

	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")

	cg, err := b.getCG()
	require.NoError(t, err)
	cgChecker(cg, true, true)

	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)

	entry := findFuncByName(ssaProg, "AFunctionWithUniqueName")
	require.NotNil(t, entry, "could not find AFunctionWithUniqueName in SSA program")

	reachable := reachableFuncNames(cg, entry)
	assert.True(t, reachable["Sink"], "Sink must be reachable through the cff.SliceEnd closure")
}

// TestCFFMapEndReachable mirrors TestCFFSliceEndReachable for cff.MapEnd.
func TestCFFMapEndReachable(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "cffappmapend", "cffappmapend-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/cffappmapend"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	b.hasMain = true

	funcs := findCFFFlowFuncs(b, ssaProg, "AFunctionWithUniqueName")
	require.NotEmpty(t, funcs, "Failed to find function carrying CFF scheduling calls")

	cg, err := b.getCG()
	require.NoError(t, err)
	cgChecker(cg, true, true)

	err = b.enrichWithCFFCall(cg)
	require.NoError(t, err)

	entry := findFuncByName(ssaProg, "AFunctionWithUniqueName")
	require.NotNil(t, entry, "could not find AFunctionWithUniqueName in SSA program")

	reachable := reachableFuncNames(cg, entry)
	assert.True(t, reachable["Sink"], "Sink must be reachable through the cff.MapEnd closure")
}
