package graph

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// loadExampleApp loads a specific example app from the constantpropagation directory
// and returns the SSA program, source functions, sink functions, and call graph.
func loadExampleApp(t *testing.T, appName string) (*ssa.Program, map[*ssa.Function]struct{}, map[*ssa.Function]struct{}, *callgraph.Graph) {
	// Load the example app packages
	analPath := []string{"exampleapps", "constantpropagation", appName, appName + "-anal"}
	loadPath := "github.com/uber-research/rpcshield/go/exampleapps/constantpropagation/" + appName

	_, _, ssaProg := loadAndInitBuilder(t, analPath, loadPath)

	// Setup the source code for the test.
	funcs := make(map[string]*ssa.Function)
	// Find functions named 'main', 'source', and 'sink' in the SSA program
	var mainFunc []*ssa.Function
	sourceFuncs := make(map[*ssa.Function]struct{})
	sinkFuncs := make(map[*ssa.Function]struct{})
	for fn := range ssautil.AllFunctions(ssaProg) {
		funcs[fn.String()] = fn
		if fn.Name() == "main" {
			mainFunc = append(mainFunc, fn)
		}
		if fn.Name() == "source" && fn.Pkg != nil && fn.Pkg.Pkg != nil && fn.Pkg.Pkg.Path() == loadPath {
			sourceFuncs[fn] = struct{}{}
		} else if fn.Name() == "sink" && fn.Pkg != nil && fn.Pkg.Pkg != nil && fn.Pkg.Pkg.Path() == loadPath {
			sinkFuncs[fn] = struct{}{}
		}
	}

	// Build the call graph using RTA (Rapid Type Analysis)
	rtaResult := rta.Analyze(mainFunc, true)
	cg := rtaResult.CallGraph

	require.NotEmpty(t, sourceFuncs, "No source functions found in %s", appName)
	require.NotEmpty(t, sinkFuncs, "No sink functions found in %s", appName)

	return ssaProg, sourceFuncs, sinkFuncs, cg
}

func hasStaticFunctionInvocation(block *ssa.BasicBlock, functionName string) bool {
	for _, instr := range block.Instrs {
		if call, ok := instr.(*ssa.Call); ok {
			if call.Call.StaticCallee() != nil && call.Call.StaticCallee().Name() == functionName {
				return true
			}
		}
	}
	return false
}

// A path is feasible if there is atleast one static function invocation with name "infeasible" in a the same block as the callsite.

func isInfeasiblePath(path *callPath) bool {
	for _, edge := range path.path {
		if edge == nil || edge.Site == nil {
			continue
		}
		// There should be no call site with name "infeasible" in the same block as the call site.
		if hasStaticFunctionInvocation(edge.Site.Block(), "infeasible") {
			return true
		}
	}
	return false
}

// findPathsFromSourceToSink finds all paths from source functions to sink functions
func findPathsFromSourceToSink(cg *callgraph.Graph, sourceFuncs, sinkFuncs map[*ssa.Function]struct{}) []callPath {
	var paths []callPath

	// For each source function, find paths to sinks
	for source := range sourceFuncs {
		sourceNode := cg.Nodes[source]
		if sourceNode == nil {
			continue
		}

		// Use DFS to find all paths from source to sinks
		visited := make(map[*callgraph.Node]bool)
		var currentPath []*callgraph.Edge
		findPathsDFS(sourceNode, sinkFuncs, visited, currentPath, &paths)
	}

	// Print the paths
	for _, path := range paths {
		fmt.Printf("Path: %v\n", path.path)
	}
	return paths
}

// callPath represents a path in the call graph from source to sink functions.
type callPath struct {
	path []*callgraph.Edge // The sequence of call graph edges forming the path
}

// findPathsDFS performs DFS to find all paths from current node to sink functions
func findPathsDFS(node *callgraph.Node, sinkFuncs map[*ssa.Function]struct{}, visited map[*callgraph.Node]bool, currentPath []*callgraph.Edge, paths *[]callPath) {
	if visited[node] {
		return
	}

	visited[node] = true
	defer func() { visited[node] = false }()
	// Check if current node is a sink
	if _, ok := sinkFuncs[node.Func]; ok {
		// Found a path to sink
		pathCopy := make([]*callgraph.Edge, len(currentPath))
		copy(pathCopy, currentPath)
		// Append the final edge to the path, which by convention is nil to mark the end.
		pathCopy = append(pathCopy, &callgraph.Edge{
			Caller: node,
			Callee: nil,
			Site:   nil,
		})
		*paths = append(*paths, callPath{path: pathCopy})
	}

	// Explore all outgoing edges
	for _, edge := range node.Out {
		if edge.Callee != nil {
			currentPath = append(currentPath, edge)
			findPathsDFS(edge.Callee, sinkFuncs, visited, currentPath, paths)
			currentPath = currentPath[:len(currentPath)-1]
		}
	}
}

// hasInfeasibleInSameBlock checks if a call to "infeasible" function is in the same basic block as the call site.
func hasInfeasibleInSameBlock(edge *callgraph.Edge) bool {
	if edge.Site == nil {
		return false
	}

	callBlock := edge.Site.Block()
	if callBlock == nil {
		return false
	}

	// Check if there's a call to infeasible function in the same block
	for _, instr := range callBlock.Instrs {
		if call, ok := instr.(*ssa.Call); ok {
			if call.Call.StaticCallee() != nil && strings.Contains(call.Call.StaticCallee().Name(), "infeasible") {
				return true
			}
		}
	}

	return false
}

// TestSCCPAnalyzerWithExampleApps tests the SCCPAnalyzer with various example apps
func TestSCCPAnalyzerWithExampleApps(t *testing.T) {
	tests := []struct {
		name    string
		appName string
	}{
		{
			name:    "switch app",
			appName: "switch",
		},
		{
			name:    "ifelse app",
			appName: "ifelse",
		},
		{
			name:    "interfaceresolution app",
			appName: "interfaceresolution",
		},
		{
			name:    "interfaceresolutionwithbinding app",
			appName: "interfaceresolutionwithbinding",
		},
		{
			name:    "phis app",
			appName: "phis",
		},
		{
			name:    "coverage app",
			appName: "coverage",
		},
		{
			name:    "lambdapassing app",
			appName: "lambdapassing",
		},
		{
			name:    "functionpassing app",
			appName: "functionpassing",
		},
		{
			name:    "nilvsfunc app",
			appName: "nilvsfunc",
		},
		{
			name:    "nilvsnonnil app",
			appName: "nilvsnonnil",
		},
		{
			name:    "typeassertions app",
			appName: "typeassertions",
		},
		{
			name:    "freevar app",
			appName: "freevar",
		},
		{
			name:    "channelreceive app",
			appName: "channelreceive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the example app
			prog, sourceFuncs, sinkFuncs, cg := loadExampleApp(t, tt.appName)

			// Build RTA call graph
			require.NotNil(t, cg)

			// Find paths from source to sink
			paths := findPathsFromSourceToSink(cg, sourceFuncs, sinkFuncs)
			require.NotEmpty(t, paths, "No paths found from source to sink in %s", tt.appName)

			logger1, err := zap.NewDevelopment()
			require.NoError(t, err)
			logger := logger1.Sugar()

			// Test each path with SCCPAnalyzer
			for pathIdx, pathPair := range paths {
				expectedInfeasibility := isInfeasiblePath(&pathPair)

				t.Logf("Testing path %d in %s: %d edges", pathIdx, tt.appName, len(pathPair.path))

				fmt.Printf("Processing path: %v\n", pathPair.path)
				// Create SCCPAnalyzer
				analyzer := NewSCCPAnalyzer(prog, pathPair.path, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})
				require.NotNil(t, analyzer)

				// Analyze the path.
				analyzer.Analyze()

				observedInfeasibility := false
				// Check if the path is feasible
				for _, edge := range pathPair.path {
					callsite := edge.Site
					if callsite == nil {
						continue
					}

					functionName := callsite.Parent().String()
					blockID := callsite.Block().String()
					if !analyzer.isBlockReachable(callsite.Block()) {
						observedInfeasibility = true
						t.Logf("Path %d in %s is infeasible (expected): %s:%s", pathIdx, tt.appName, functionName, blockID)
					} else {
						t.Logf("Path %d in %s is feasible: %s:%s", pathIdx, tt.appName, functionName, blockID)
					}
				}
				t.Logf("Path %d %s: expectedInfeasibility: %v, observedInfeasibility: %v", pathIdx, tt.appName, expectedInfeasibility, observedInfeasibility)
				assert.Equal(t, expectedInfeasibility, observedInfeasibility, "Path %d in %s: expected infeasibility %v, observed %v", pathIdx, tt.appName, expectedInfeasibility, observedInfeasibility)
			}
		})
	}
}

func TestSCCPVisitWithNoReferers(t *testing.T) {
	prog, _, _, cg := loadExampleApp(t, "switch")
	// Build RTA call graph
	require.NotNil(t, cg)

	logger1, err := zap.NewDevelopment()
	require.NoError(t, err)
	logger := logger1.Sugar()

	// Create SCCPAnalyzer
	analyzer := NewSCCPAnalyzer(prog, nil, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})
	require.NotNil(t, analyzer)

	// Create a dummy topValue
	topValue := &ssa.Const{Value: constant.MakeInt64(42)}

	//This covers the case where the referrers is empty.
	analyzer.visitValue(topValue)

	// Cover createUnknownConstant case for a tuple type.
	tupleType := &types.Tuple{}
	analyzer.createUnknownConstant(tupleType)

	// exercise evalBinOp with a real BinOp instruction to avoid nil deref in String()
	var nonValInstr *ssa.Instruction
OUT:
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil || fn.Blocks == nil {
			continue
		}
		for _, blk := range fn.Blocks {
			for _, ins := range blk.Instrs {
				if _, ok := ins.(ssa.Value); !ok {
					nonValInstr = &ins
					break OUT
				}
			}
		}
	}
	require.NotNil(t, nonValInstr, "expected to find at least one *ssa.BinOp in program")

	// non value modelInstructionAndUpdateValueLattice
	// It should simply return w/o panicing.
	analyzer.modelInstructionAndUpdateValueLattice(*nonValInstr)

}

func TestSCCPTopBranchCondition(t *testing.T) {
	// Covers the case where the branch condition is a TopVal.
	prog, _, _, cg := loadExampleApp(t, "switch")
	require.NotNil(t, cg)

	logger1, err := zap.NewDevelopment()
	require.NoError(t, err)
	logger := logger1.Sugar()
	// Find an If instruction.
	var ifInstr *ssa.If
OUT:
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil || fn.Blocks == nil {
			continue
		}
		for _, blk := range fn.Blocks {
			for _, ins := range blk.Instrs {
				var ok bool
				if ifInstr, ok = ins.(*ssa.If); ok {
					break OUT
				}
			}
		}
	}

	require.NotNil(t, ifInstr, "expected to find at least one *ssa.If in program")

	analyzer := NewSCCPAnalyzer(prog, nil, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})
	require.NotNil(t, analyzer)
	// Force the value state for Cond to TopVal.
	analyzer.valueState[ifInstr.Cond] = createSingletonLatticeElement(TopVal{}, ConcreteType{T: ifInstr.Cond.Type()})

	assert.Equal(t, 0, int(analyzer.metrics[_topAsIfCond]))
	analyzer.visitInstr(ifInstr)
	assert.Equal(t, 1, int(analyzer.metrics[_topAsIfCond]))
}

func TestEvalBinopFuncval_Branches(t *testing.T) {
	logger1, err := zap.NewDevelopment()
	require.NoError(t, err)
	logger := logger1.Sugar()

	analyzer := NewSCCPAnalyzer(nil, nil, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})

	// Get a real *ssa.BinOp from an example program so that BinOp.String() is safe in logs
	prog, _, _, _ := loadExampleApp(t, "switch")
	var binOp *ssa.BinOp
OUT:
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil || fn.Blocks == nil {
			continue
		}
		for _, blk := range fn.Blocks {
			for _, ins := range blk.Instrs {
				if bo, ok := ins.(*ssa.BinOp); ok {
					binOp = bo
					break OUT
				}
			}
		}
	}
	require.NotNil(t, binOp, "expected to find at least one *ssa.BinOp in program")

	setOp := func(op token.Token) *ssa.BinOp {
		binOp.Op = op
		return binOp
	}

	// Helpers to assert results
	assertIsBottom := func(tb testing.TB, v ValueLattice) {
		tb.Helper()
		_, isBottom := v.(BottomVal)
		assert.True(tb, isBottom, "expected BottomVal, got %T (%v)", v, v)
	}
	assertIsBool := func(tb testing.TB, v ValueLattice, want bool) {
		tb.Helper()
		cv, ok := v.(ConstantVal)
		require.True(tb, ok, "expected ConstantVal, got %T (%v)", v, v)
		require.Equal(tb, constant.Bool, cv.Value.Value.Kind())
		got := constant.BoolVal(cv.Value.Value)
		assert.Equal(tb, want, got)
	}

	// Prepare FunctionVals
	sameFn := &ssa.Function{}
	diffFn := &ssa.Function{}
	funcVal := FunctionVal{fn: sameFn}

	t.Run("FunctionVal_op_ConstantVal(nil)", func(t *testing.T) {
		// EQL => false
		funcNilType := types.NewSignatureType(nil, nil, nil, nil, nil, false)
		res := analyzer.evalBinopFuncval(setOp(token.EQL), funcVal, ConstantVal{Value: *ssa.NewConst(nil, funcNilType)})
		assertIsBool(t, res, false)
		// NEQ => true
		res = analyzer.evalBinopFuncval(setOp(token.NEQ), funcVal, ConstantVal{Value: *ssa.NewConst(nil, funcNilType)})
		assertIsBool(t, res, true)
		// Other op => bottom
		res = analyzer.evalBinopFuncval(setOp(token.ADD), funcVal, ConstantVal{Value: *ssa.NewConst(nil, funcNilType)})
		assertIsBottom(t, res)
	})

	t.Run("FunctionVal_op_ConstantVal(non-nil)", func(t *testing.T) {
		res := analyzer.evalBinopFuncval(setOp(token.EQL), funcVal, createConstantVal(*ssa.NewConst(constant.MakeInt64(0), types.Typ[types.Int])))
		assertIsBottom(t, res)
	})

	t.Run("FunctionVal_op_FunctionVal(same)", func(t *testing.T) {
		// EQL => true
		res := analyzer.evalBinopFuncval(setOp(token.EQL), funcVal, FunctionVal{fn: sameFn})
		assertIsBool(t, res, true)
		// NEQ => false
		res = analyzer.evalBinopFuncval(setOp(token.NEQ), funcVal, FunctionVal{fn: sameFn})
		assertIsBool(t, res, false)
		// Other op => bottom
		res = analyzer.evalBinopFuncval(setOp(token.ADD), funcVal, FunctionVal{fn: sameFn})
		assertIsBottom(t, res)
	})

	t.Run("FunctionVal_op_FunctionVal(different)", func(t *testing.T) {
		// EQL => false
		res := analyzer.evalBinopFuncval(setOp(token.EQL), funcVal, FunctionVal{fn: diffFn})
		assertIsBool(t, res, false)
		// NEQ => true
		res = analyzer.evalBinopFuncval(setOp(token.NEQ), funcVal, FunctionVal{fn: diffFn})
		assertIsBool(t, res, true)
		// Other op => bottom
		res = analyzer.evalBinopFuncval(setOp(token.ADD), funcVal, FunctionVal{fn: diffFn})
		assertIsBottom(t, res)
	})

	t.Run("FunctionVal_op_BottomVal", func(t *testing.T) {
		res := analyzer.evalBinopFuncval(setOp(token.EQL), funcVal, BottomVal{})
		assertIsBottom(t, res)
	})

	t.Run("FunctionVal_op_TopVal", func(t *testing.T) {
		res := analyzer.evalBinopFuncval(setOp(token.EQL), funcVal, TopVal{})
		assertIsBottom(t, res)
	})
}

// TestEvalBinopConstantval_StructAndArrayZeroValue is a regression test for a nil-pointer panic:
// doTypeCheck synthesizes a struct/array-typed ConstantVal for a statically-failed type assertion via
// ssa.NewConst(nil, asserted), which leaves Const.Value == nil while Const.IsNil() is false (structs
// and arrays aren't nillable types). evalBinopConstantval used to gate on IsNil() before calling
// isUnknownConst on the nil constant.Value, panicking on constant.Value.Kind(). Both operands being
// this same zero representation should fold EQL/NEQ correctly instead of crashing or falling into the
// "one nil, one not" mismatch branch.
func TestEvalBinopConstantval_StructAndArrayZeroValue(t *testing.T) {
	logger1, err := zap.NewDevelopment()
	require.NoError(t, err)
	logger := logger1.Sugar()

	analyzer := NewSCCPAnalyzer(nil, nil, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})

	prog, _, _, _ := loadExampleApp(t, "switch")
	var binOp *ssa.BinOp
OUT:
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil || fn.Blocks == nil {
			continue
		}
		for _, blk := range fn.Blocks {
			for _, ins := range blk.Instrs {
				if bo, ok := ins.(*ssa.BinOp); ok {
					binOp = bo
					break OUT
				}
			}
		}
	}
	require.NotNil(t, binOp, "expected to find at least one *ssa.BinOp in program")

	setOp := func(op token.Token) *ssa.BinOp {
		binOp.Op = op
		return binOp
	}

	assertIsBool := func(tb testing.TB, v ValueLattice, want bool) {
		tb.Helper()
		cv, ok := v.(ConstantVal)
		require.True(tb, ok, "expected ConstantVal, got %T (%v)", v, v)
		require.Equal(tb, constant.Bool, cv.Value.Value.Kind())
		got := constant.BoolVal(cv.Value.Value)
		assert.Equal(tb, want, got)
	}

	structType := types.NewStruct([]*types.Var{
		types.NewField(0, nil, "X", types.Typ[types.Int], false),
	}, nil)
	arrayType := types.NewArray(types.Typ[types.Int], 3)

	for _, tc := range []struct {
		name string
		typ  types.Type
	}{
		{"struct", structType},
		{"array", arrayType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			zero := ConstantVal{Value: *ssa.NewConst(nil, tc.typ)}
			require.Nil(t, zero.Value.Value, "precondition: zero value should have no scalar constant.Value payload")
			require.False(t, zero.Value.IsNil(), "precondition: struct/array types are not nillable")

			require.NotPanics(t, func() {
				res := analyzer.evalBinopConstantval(setOp(token.EQL), zero, zero)
				assertIsBool(t, res, true)
			})
			require.NotPanics(t, func() {
				res := analyzer.evalBinopConstantval(setOp(token.NEQ), zero, zero)
				assertIsBool(t, res, false)
			})
		})
	}
}

func TestHandleTypeAssertion_Branches(t *testing.T) {
	// Load an example app that contains type assertions
	prog, _, _, _ := loadExampleApp(t, "typeassertions")

	// Find a real *ssa.TypeAssert instruction to avoid panic on String()
	var ta *ssa.TypeAssert
OUT:
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil || fn.Blocks == nil {
			continue
		}
		for _, blk := range fn.Blocks {
			for _, ins := range blk.Instrs {
				if x, ok := ins.(*ssa.TypeAssert); ok {
					ta = x
					break OUT
				}
			}
		}
	}
	require.NotNil(t, ta, "expected to find at least one *ssa.TypeAssert in program")

	logger1, err := zap.NewDevelopment()
	require.NoError(t, err)
	logger := logger1.Sugar()
	analyzer := NewSCCPAnalyzer(prog, nil, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})

	makeLE := func(val ValueLattice, ty TypeLattice) LatticeElement {
		return createSingletonLatticeElement(val, ty)
	}

	// Helper for expected results for doTypeCheck
	expectedDoTypeCheck := func(src types.Type, v ValueLattice) (LatticeElement, LatticeElement) {
		if types.AssignableTo(src, ta.AssertedType) {
			return createSingletonLatticeElement(v, ConcreteType{T: ta.AssertedType}), createConstantTrueLatticeElement()
		}
		return createSingletonLatticeElement(ConstantVal{Value: *ssa.NewConst(nil, ta.AssertedType)}, ConcreteType{T: ta.AssertedType}), createConstantFalseLatticeElement()
	}

	iface := types.NewInterfaceType(nil, nil)
	iface.Complete()
	// Also use empty interface in some assignable cases
	emptyIface := types.NewInterfaceType(nil, nil)
	emptyIface.Complete()

	// Reuse a real function from program for function-val cases
	var fnVal *ssa.Function
	for f := range ssautil.AllFunctions(prog) {
		if f != nil {
			fnVal = f
			break
		}
	}
	require.NotNil(t, fnVal)

	constInt1 := createConstantVal(*ssa.NewConst(constant.MakeInt64(1), types.Typ[types.Int]))
	constNilOf := func(tpe types.Type) ConstantVal { return ConstantVal{Value: *ssa.NewConst(nil, tpe)} }

	tests := []struct {
		name     string
		asserted types.Type
		setup    func()
		expect   func() (LatticeElement, LatticeElement)
	}{
		{
			name:     "infeasible_len!=1",
			asserted: types.Typ[types.Int],
			setup: func() {
				analyzer.valueState[ta.X] = LatticeElement{}
			},
			expect: func() (LatticeElement, LatticeElement) {
				return createTypedLatticeBottom(ta.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
			},
		},
		{
			name:     "const_nil_interface",
			asserted: types.Typ[types.Int],
			setup: func() {
				analyzer.valueState[ta.X] = makeLE(constNilOf(iface), ConcreteType{T: iface})
			},
			expect: func() (LatticeElement, LatticeElement) {
				return createSingletonLatticeElement(constNilOf(ta.AssertedType), ConcreteType{T: ta.AssertedType}), createConstantFalseLatticeElement()
			},
		},
		{
			name:     "const_non_nil_assignable",
			asserted: emptyIface,
			setup: func() {
				analyzer.valueState[ta.X] = makeLE(constInt1, ConcreteType{T: types.Typ[types.Int]})
			},
			expect: func() (LatticeElement, LatticeElement) {
				return expectedDoTypeCheck(types.Typ[types.Int], constInt1)
			},
		},
		{
			name:     "const_non_nil_not_assignable",
			asserted: types.Typ[types.Bool],
			setup: func() {
				analyzer.valueState[ta.X] = makeLE(constInt1, ConcreteType{T: types.Typ[types.Int]})
			},
			expect: func() (LatticeElement, LatticeElement) {
				return expectedDoTypeCheck(types.Typ[types.Int], constInt1)
			},
		},
		{
			name:     "function_val_assignable",
			asserted: emptyIface,
			setup: func() {
				analyzer.valueState[ta.X] = makeLE(FunctionVal{fn: fnVal}, ConcreteType{T: fnVal.Type()})
			},
			expect: func() (LatticeElement, LatticeElement) {
				return expectedDoTypeCheck(fnVal.Type(), FunctionVal{fn: fnVal})
			},
		},
		{
			name:     "function_val_not_assignable",
			asserted: types.Typ[types.Int],
			setup: func() {
				analyzer.valueState[ta.X] = makeLE(FunctionVal{fn: fnVal}, ConcreteType{T: fnVal.Type()})
			},
			expect: func() (LatticeElement, LatticeElement) {
				return expectedDoTypeCheck(fnVal.Type(), FunctionVal{fn: fnVal})
			},
		},
		{
			name:     "top_concrete_interface",
			asserted: types.Typ[types.Int],
			setup:    func() { analyzer.valueState[ta.X] = makeLE(TopVal{}, ConcreteType{T: iface}) },
			expect: func() (LatticeElement, LatticeElement) {
				return createTypedLatticeBottom(ta.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
			},
		},
		{
			name:     "top_concrete_non_interface",
			asserted: types.Typ[types.Int],
			setup:    func() { analyzer.valueState[ta.X] = makeLE(TopVal{}, ConcreteType{T: types.Typ[types.Int]}) },
			expect: func() (LatticeElement, LatticeElement) {
				return expectedDoTypeCheck(types.Typ[types.Int], TopVal{})
			},
		},
		{
			name:     "top_toptype",
			asserted: types.Typ[types.Int],
			setup:    func() { analyzer.valueState[ta.X] = makeLE(TopVal{}, TopType{}) },
			expect: func() (LatticeElement, LatticeElement) {
				return createTopLatticeElement(ta.AssertedType), createTopLatticeElement(types.Typ[types.Bool])
			},
		},
		{
			name:     "top_bottomtype",
			asserted: types.Typ[types.Int],
			setup:    func() { analyzer.valueState[ta.X] = makeLE(TopVal{}, BottomType{}) },
			expect: func() (LatticeElement, LatticeElement) {
				return createTypedLatticeBottom(ta.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
			},
		},
		{
			name:     "bottom_concrete_interface",
			asserted: types.Typ[types.Int],
			setup:    func() { analyzer.valueState[ta.X] = makeLE(BottomVal{}, ConcreteType{T: iface}) },
			expect: func() (LatticeElement, LatticeElement) {
				return createTypedLatticeBottom(ta.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
			},
		},
		{
			name:     "bottom_concrete_non_interface",
			asserted: types.Typ[types.Int],
			setup:    func() { analyzer.valueState[ta.X] = makeLE(BottomVal{}, ConcreteType{T: types.Typ[types.Int]}) },
			expect: func() (LatticeElement, LatticeElement) {
				return expectedDoTypeCheck(types.Typ[types.Int], BottomVal{})
			},
		},
		{
			name:     "bottom_toptype",
			asserted: types.Typ[types.Int],
			setup:    func() { analyzer.valueState[ta.X] = makeLE(BottomVal{}, TopType{}) },
			expect: func() (LatticeElement, LatticeElement) {
				return createTypedLatticeBottom(ta.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
			},
		},
		{
			name:     "bottom_bottomtype",
			asserted: types.Typ[types.Int],
			setup:    func() { analyzer.valueState[ta.X] = makeLE(BottomVal{}, BottomType{}) },
			expect: func() (LatticeElement, LatticeElement) {
				return createTypedLatticeBottom(ta.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Save/restore asserted type
			origAssert := ta.AssertedType
			ta.AssertedType = tc.asserted
			t.Cleanup(func() { ta.AssertedType = origAssert })

			// Save/restore value state
			orig, had := analyzer.valueState[ta.X]
			t.Cleanup(func() {
				if had {
					analyzer.valueState[ta.X] = orig
				} else {
					delete(analyzer.valueState, ta.X)
				}
			})

			// Setup case state
			tc.setup()

			// Execute and compare
			v, b := analyzer.handleTypeAssertion(ta)
			exV, exB := tc.expect()
			assert.Equal(t, exV, v)
			assert.Equal(t, exB, b)
		})
	}
}

func TestAssignParamsToArgsElseDefault_Branches(t *testing.T) {
	prog, _, _, _ := loadExampleApp(t, "coverage")

	logger1, err := zap.NewDevelopment()
	require.NoError(t, err)
	logger := logger1.Sugar()
	analyzer := NewSCCPAnalyzer(prog, nil, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})

	// Find candidate parameters from any function in the program
	var anyParams []*ssa.Parameter
	var nonIfaceParam *ssa.Parameter
	var intParam *ssa.Parameter
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil || fn.Blocks == nil {
			continue
		}
		if len(fn.Params) > 0 && anyParams == nil {
			anyParams = fn.Params
		}
		for _, p := range fn.Params {
			if nonIfaceParam == nil {
				if _, isIface := p.Type().Underlying().(*types.Interface); !isIface {
					nonIfaceParam = p
				}
			}
			if intParam == nil {
				if bt, ok := p.Type().Underlying().(*types.Basic); ok && bt.Kind() == types.Int {
					intParam = p
				}
			}
		}
		if anyParams != nil && nonIfaceParam != nil && intParam != nil {
			break
		}
	}
	require.NotNil(t, anyParams, "expected to find at least one parameter in the program")

	// Helper to create a typed constant value for a basic type
	makeBasicConst := func(tpe types.Type) *ssa.Const {
		if bt, ok := tpe.Underlying().(*types.Basic); ok {
			switch bt.Kind() {
			case types.Int:
				return ssa.NewConst(constant.MakeInt64(7), types.Typ[types.Int])
			case types.String:
				return ssa.NewConst(constant.MakeString("x"), types.Typ[types.String])
			case types.Bool:
				return ssa.NewConst(constant.MakeBool(true), types.Typ[types.Bool])
			case types.Float64:
				return ssa.NewConst(constant.MakeFloat64(3.14), types.Typ[types.Float64])
			}
		}
		return nil
	}

	// Pick param for success test (prefer int, else any nillable or basic)
	pickSuccessParam := func() *ssa.Parameter {
		if intParam != nil {
			return intParam
		}
		// fallback: any param
		return anyParams[0]
	}

	// Build table
	tests := []struct {
		name           string
		prepare        func() ([]ssa.Value, []*ssa.Parameter)
		expectErr      bool
		expectBindings func(bindings map[ssa.Value]LatticeElement)
	}{
		{
			name: "length_mismatch",
			prepare: func() ([]ssa.Value, []*ssa.Parameter) {
				params := anyParams
				// Provide fewer args than params
				var args []ssa.Value
				return args, params
			},
			expectErr: true,
			expectBindings: func(bindings map[ssa.Value]LatticeElement) {
				// All params bound to typed bottom
				for _, p := range anyParams {
					le, ok := bindings[p]
					require.True(t, ok)
					expected := createTypedLatticeBottom(p.Type())
					assert.Equal(t, expected, le)
				}
			},
		},
		{
			name: "type_mismatch",
			prepare: func() ([]ssa.Value, []*ssa.Parameter) {
				p := nonIfaceParam
				if p == nil {
					t.Skip("no non-interface param found for mismatch test")
				}
				params := []*ssa.Parameter{p}
				// Choose an arg of a different basic type to force mismatch
				var arg *ssa.Const
				if bt, ok := p.Type().Underlying().(*types.Basic); ok && bt.Kind() == types.Int {
					arg = makeBasicConst(types.Typ[types.String])
				} else {
					arg = makeBasicConst(types.Typ[types.Int])
				}
				if arg == nil {
					t.Skip("could not create mismatch arg for param type")
				}
				args := []ssa.Value{arg}
				return args, params
			},
			expectErr: true,
			expectBindings: func(bindings map[ssa.Value]LatticeElement) {
				p := nonIfaceParam
				require.NotNil(t, p)
				le, ok := bindings[p]
				require.True(t, ok)
				expected := createTypedLatticeBottom(p.Type())
				assert.Equal(t, expected, le)
			},
		},
		{
			name: "success_bindings",
			prepare: func() ([]ssa.Value, []*ssa.Parameter) {
				p := pickSuccessParam()
				params := []*ssa.Parameter{p}
				// Make an arg assignable to p.Type()
				var arg *ssa.Const
				if c := makeBasicConst(p.Type()); c != nil {
					arg = c
				} else {
					// try typed nil for nillable types
					switch p.Type().(type) {
					case *types.Signature, *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Interface:
						arg = ssa.NewConst(nil, p.Type())
					default:
						t.Skip("unsupported param type for success test")
					}
				}
				args := []ssa.Value{arg}
				// Pre-set lattice for arg to a known value to assert binding picks it
				analyzer.valueState[arg] = createSingletonLatticeElement(createConstantVal(*arg), ConcreteType{T: arg.Type()})
				return args, params
			},
			expectErr: false,
			expectBindings: func(bindings map[ssa.Value]LatticeElement) {
				p := pickSuccessParam()
				le, ok := bindings[p]
				require.True(t, ok)
				// Should equal analyzer.getLatticeForSSAValue(arg), which we set to constant of arg
				// Rebuild expected from param's arg: since we don't have arg here, compare Type and that Value is ConstantVal
				// Instead, assert it is not Bottom and type matches param
				_, ty := le.getSingletonLatticeElement()
				ct, ok2 := ty.(ConcreteType)
				require.True(t, ok2)
				assert.True(t, types.Identical(ct.T, p.Type()))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bindings := make(map[ssa.Value]LatticeElement)
			args, params := tc.prepare()
			err := analyzer.assignParamsToArgsElseDefault(args, params, bindings)
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			tc.expectBindings(bindings)
		})
	}
}

// TestDecoratorTopValFalseNegative exercises exampleapps/constantpropagation/decoratortopval
// end-to-end (real source -> callgraph -> checkPathFeasibility-equivalent
// SCCPAnalyzer.Analyze). It documents a *negative* result from this investigation: a generic
// decorator gating a single, statically-known callback behind an opaque (non-constant-
// foldable) capability check, whose eventual guard is a struct field derived from a method
// call on a different outer struct, is analyzed *correctly* -- the unresolved capability
// check's condition lattice is BottomVal (not TopVal), both successor blocks are marked
// reachable per constprop.go's "Both branches are reachable" BottomVal handling, and the sink
// is found feasible. This rules out "a single opaque decorator hop" as sufficient to
// reproduce a false negative; see TestDecoratorLoopSound, which adds the remaining
// structural element (ranging over a heterogeneous slice of closures) -- and confirms it is
// *also* analyzed correctly, not a false negative.
func TestDecoratorTopValFalseNegative(t *testing.T) {
	prog, sourceFuncs, sinkFuncs, cg := loadExampleApp(t, "decoratortopval")

	paths := findPathsFromSourceToSink(cg, sourceFuncs, sinkFuncs)
	require.NotEmpty(t, paths, "expected at least one call-graph path from source to sink")

	logger1, err := zap.NewDevelopment()
	require.NoError(t, err)
	logger := logger1.Sugar()

	for pathIdx, pathPair := range paths {
		analyzer := NewSCCPAnalyzer(prog, pathPair.path, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})
		require.NotNil(t, analyzer)
		analyzer.Analyze()

		sinkReachable := false
		for _, edge := range pathPair.path {
			if edge.Site == nil {
				continue
			}
			if analyzer.isBlockReachable(edge.Site.Block()) {
				sinkReachable = true
			}
		}
		t.Logf("path %d: sinkReachable=%v", pathIdx, sinkReachable)
		assert.True(t, sinkReachable,
			"path %d: a single, statically-known callback through the decorator is expected "+
				"to be analyzed correctly (BottomVal on the opaque gate, both branches reachable)", pathIdx)
	}
}

// TestDecoratorLoopSound exercises, end-to-end, the same shape as production
// (ads-campaign-manager's campaign.controller.ValidateAdGroups looping over a 53-element slice
// of heterogeneous validator closures, calling each one through decorator.MetricDecorator[T]).
// exampleapps/constantpropagation/decoratorloop mirrors this exact shape with two closures
// instead of 53.
//
// Program semantics: Validator2's sink() IS always reached on the loop's second iteration
// (Outer.IsInternal and opaqueGate.Allow both unconditionally return true) -- there is no
// infeasible() call anywhere in this program, unlike ifelse/functionpassing's fixtures.
//
// This was originally suspected to be a false negative (RTA's context-insensitive callgraph
// node for MetricDecorator[T] is shared across every caller that instantiates it, so a naive
// path-feasibility check might wrongly reject a genuinely-reachable sink). Direct investigation
// (see the ads-campaign-manager reachability-gap root-cause session) confirmed the opposite:
// SCCPAnalyzer resolves the concrete callee bound to the decorator's `next` parameter for this
// specific caller/path and correctly proves the sink reachable. Any caller/path combination
// where the resolved callee provably does NOT match is correctly rejected elsewhere (that
// rejection is sound RTA-over-approximation pruning, not a bug either) -- this fixture's path
// has a genuinely matching callee, so it must resolve as reachable.
func TestDecoratorLoopSound(t *testing.T) {
	prog, sourceFuncs, sinkFuncs, cg := loadExampleApp(t, "decoratorloop")

	paths := findPathsFromSourceToSink(cg, sourceFuncs, sinkFuncs)
	require.NotEmpty(t, paths, "expected at least one call-graph path from source to sink")

	logger1, err := zap.NewDevelopment()
	require.NoError(t, err)
	logger := logger1.Sugar()

	for pathIdx, pathPair := range paths {
		analyzer := NewSCCPAnalyzer(prog, pathPair.path, logger, map[*callgraph.Edge]struct{}{}, map[string]float64{})
		require.NotNil(t, analyzer)
		analyzer.Analyze()

		sinkReachable := false
		for _, edge := range pathPair.path {
			if edge.Site == nil {
				continue
			}
			if analyzer.isBlockReachable(edge.Site.Block()) {
				sinkReachable = true
			}
		}
		t.Logf("path %d: sinkReachable=%v", pathIdx, sinkReachable)

		assert.True(t, sinkReachable,
			"path %d: expected SCCP to prove the sink reachable when the decorator is called from "+
				"inside a loop over a heterogeneous closure slice -- this is sound, context-sensitive "+
				"resolution of the concrete callee for this specific caller, not a bug", pathIdx)
	}
}
