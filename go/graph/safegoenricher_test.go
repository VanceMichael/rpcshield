package graph

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestSafeGo(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "AFunctionWithUniqueName" {
			f = fn
			break
		}
	}
	safeGoAnonyExpected := map[string]struct{}{
		"AFunctionWithUniqueName$1": {},
	}
	safeGoInnereExpected := map[string]struct{}{
		"generalFCall":    {},
		"generalNilECall": {},
		"notContextCall":  {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}
	for callsite := range resp[safeGoType] {
		varCallMap, err := b.processSafeGoCall(callsite.Common())
		assert.Equal(t, len(varCallMap), 2) //errChan and sharedErr
		require.NoError(t, err)
		for _, callInst := range varCallMap {
			for r := range callInst {
				found[r.String()] = struct{}{}
			}
		}
	}
	assert.Equal(t, len(safeGoInnereExpected), len(found))
	for k := range safeGoInnereExpected {
		hit := false
		for callInst := range found {
			if strings.Contains(callInst, k) {
				hit = true
				break
			}
		}
		assert.True(t, hit, fmt.Sprintf("Expected %s in found", k))
	}
	// Finally run the full flow.
	totalEdgeMetaInfo, err := b.enrichWithSafeGoCall(cg)
	require.NoError(t, err)

	node := cg.Nodes[f]
	require.NotNil(t, node)

	found = map[string]struct{}{}
	for _, edge := range node.Out {
		found[edge.Callee.Func.Name()] = struct{}{}

		//Iterate all the edge meta info and use the edge to match
		for _, edgeMetaInfo := range totalEdgeMetaInfo {
			if allVarOnEdge, ok := edgeMetaInfo[edge]; ok {
				assert.Equal(t, len(allVarOnEdge), 2)
			}
		}
	}
	for k := range safeGoAnonyExpected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestSafeGo_NoSafeGo(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "BFunctionWithUniqueName" {
			f = fn
			break
		}
	}
	safeGoAnonyExpected := map[string]struct{}{}
	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))
}

func TestSafeGo_NoErrorReturn(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "CFunctionWithUniqueName" {
			f = fn
			break
		}
	}

	safeGoAnonyExpected := map[string]struct{}{
		"CFunctionWithUniqueName$1": {},
	}
	safeGoInnereExpected := map[string]struct{}{}
	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))

	found := map[string]struct{}{}
	for callsite := range resp[safeGoType] {
		varCallMap, err := b.processSafeGoCall(callsite.Common())
		require.NoError(t, err)
		for _, v := range varCallMap {
			for r := range v {
				found[r.String()] = struct{}{}
			}
		}
	}
	assert.Equal(t, len(safeGoInnereExpected), len(found))

}

func TestSafeGo_NoQualiBinding(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "DFunctionWithUniqueName" {
			f = fn
			break
		}
	}

	safeGoAnonyExpected := map[string]struct{}{
		"DFunctionWithUniqueName$1": {},
	}
	safeGoInnereExpected := map[string]struct{}{}
	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))

	found := map[string]struct{}{}
	for callsite := range resp[safeGoType] {
		rt, err := b.processSafeGoCall(callsite.Common())
		require.Nil(t, rt)
		require.NoError(t, err)
	}
	assert.Equal(t, len(safeGoInnereExpected), len(found))
}

func TestSafeGo_Functor(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "EFunctionWithUniqueName" {
			f = fn
			break
		}
	}
	safeGoAnonyExpected := map[string]struct{}{
		"EFunctionWithUniqueName$1": {},
	}

	safeGoInnereExpected := map[string]struct{}{
		"generalNilECall": {},
		"generalFCall":    {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}
	for callsite := range resp[safeGoType] {
		varCallMap, err := b.processSafeGoCall(callsite.Common())
		require.NoError(t, err)
		for _, v := range varCallMap {
			for r := range v {
				found[r.String()] = struct{}{}
			}
		}
	}
	assert.Equal(t, len(safeGoInnereExpected), len(found))
	for k := range safeGoInnereExpected {
		hit := false
		for callInst := range found {
			if strings.Contains(callInst, k) {
				hit = true
				break
			}
		}
		assert.True(t, hit, fmt.Sprintf("Expected %s in found", k))
	}

	// Finally run the full flow.
	_, err = b.enrichWithSafeGoCall(cg)
	require.NoError(t, err)

	node := cg.Nodes[f]
	require.NotNil(t, node)

	found = map[string]struct{}{}
	for _, edge := range node.Out {
		found[edge.Callee.Func.Name()] = struct{}{}
	}
	for k := range safeGoAnonyExpected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestSafeGo_FunctorRewrite(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "GFunctionWithUniqueName" {
			f = fn
			break
		}
	}
	safeGoAnonyExpected := map[string]struct{}{
		"GFunctionWithUniqueName$1": {},
	}
	safeGoInnereExpected := map[string]struct{}{
		"generalNilECall": {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}
	for callsite := range resp[safeGoType] {
		varCallMap, err := b.processSafeGoCall(callsite.Common())
		require.NoError(t, err)
		for _, v := range varCallMap {
			for r := range v {
				found[r.String()] = struct{}{}
			}
		}
	}
	assert.Equal(t, len(safeGoInnereExpected), len(found))
	for k := range safeGoInnereExpected {
		hit := false
		for callInst := range found {
			if strings.Contains(callInst, k) {
				hit = true
				break
			}
		}
		assert.True(t, hit, fmt.Sprintf("Expected %s in found", k))
	}

	// Finally run the full flow.
	_, err = b.enrichWithSafeGoCall(cg)
	require.NoError(t, err)

	node := cg.Nodes[f]
	require.NotNil(t, node)

	found = map[string]struct{}{}
	for _, edge := range node.Out {
		found[edge.Callee.Func.Name()] = struct{}{}
	}
	for k := range safeGoAnonyExpected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestSafeGo_TwoGood(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "HFunctionWithUniqueName" {
			f = fn
			break
		}
	}
	safeGoAnonyExpected := map[string]struct{}{
		"HFunctionWithUniqueName$1": {},
	}
	safeGoInnereExpected := map[string]struct{}{
		"generalNilECall": {},
		"notContextCall":  {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}
	for callsite := range resp[safeGoType] {
		varCallMap, err := b.processSafeGoCall(callsite.Common())
		require.NoError(t, err)
		for _, v := range varCallMap {
			for r := range v {
				found[r.String()] = struct{}{}
			}
		}
	}
	assert.Equal(t, len(safeGoInnereExpected), len(found))
	for k := range safeGoInnereExpected {
		hit := false
		for callInst := range found {
			if strings.Contains(callInst, k) {
				hit = true
				break
			}
		}
		assert.True(t, hit, fmt.Sprintf("Expected %s in found", k))
	}

	// Finally run the full flow.
	_, err = b.enrichWithSafeGoCall(cg)
	require.NoError(t, err)

	node := cg.Nodes[f]
	require.NotNil(t, node)

	found = map[string]struct{}{}
	for _, edge := range node.Out {
		found[edge.Callee.Func.Name()] = struct{}{}
	}
	for k := range safeGoAnonyExpected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestSafeGo_M_N(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "JFunctionWithUniqueName" {
			f = fn
			break
		}
	}
	safeGoAnonyExpected := map[string]struct{}{
		"JFunctionWithUniqueName$1": {},
	}
	safeGoInnereExpected := map[string]struct{}{
		"generalFCall":  {},
		"generalFCall(": {}, // test trick to avoid dup name
		"generalTCall":  {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))

	cgChecker(cg, true, true)

	found := map[string]struct{}{}
	for callsite := range resp[safeGoType] {
		varCallMap, err := b.processSafeGoCall(callsite.Common())
		assert.Equal(t, len(varCallMap), 2) //errChan and errChan1

		require.NoError(t, err)
		for _, v := range varCallMap {
			for r := range v {
				found[r.String()] = struct{}{}
			}
		}
	}
	assert.Equal(t, len(safeGoInnereExpected), len(found))
	for k := range safeGoInnereExpected {
		hit := false
		for callInst := range found {
			if strings.Contains(callInst, k) {
				hit = true
				break
			}
		}
		assert.True(t, hit, fmt.Sprintf("Expected %s in found", k))
	}
}

func TestSafeGo_StaticFunc(t *testing.T) {
	t.Parallel()

	pathSlice := []string{"exampleapps", "safegoapp", "safegoapp-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/safegoapp"

	b, _, ssaProg := loadAndInitBuilder(t, pathSlice, loadPath)
	// Set b.hasMain = true to get RTA graph.
	b.hasMain = true
	// find CodeUnderTest function using ssautil
	var f *ssa.Function
	for fn := range ssautil.AllFunctions(ssaProg) {
		if fn.Name() == "KFunctionWithUniqueName" {
			f = fn
			break
		}
	}
	safeGoAnonyExpected := map[string]struct{}{
		"staticFunc": {},
	}
	safeGoInnereExpected := map[string]struct{}{
		"generalNilECall": {},
	}

	// Get callgraph
	cg, err := b.getCG()
	require.NoError(t, err)

	require.NotNil(t, f, "Failed to find function CodeUnderTest")
	resp := b.getSafeGoCallsites(f)
	require.Len(t, resp[safeGoType], len(safeGoAnonyExpected))

	// Finally run the full flow.
	totalEdgeMetaInfo, err := b.enrichWithSafeGoCall(cg)
	require.NoError(t, err)

	node := cg.Nodes[f]
	require.NotNil(t, node)

	found := map[string]struct{}{}
	for _, edge := range node.Out {
		found[edge.Callee.Func.Name()] = struct{}{}

		//Iterate all the edge meta info and use the edge to match
		for _, edgeMetaInfo := range totalEdgeMetaInfo {
			if allVarOnEdge, ok := edgeMetaInfo[edge]; ok {
				assert.Equal(t, len(allVarOnEdge), len(safeGoInnereExpected))

			}
		}
	}
	for k := range safeGoAnonyExpected {
		_, ok := found[k]
		assert.True(t, ok, fmt.Sprintf("Expected %s in found", k))
	}
}
