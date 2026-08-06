package graph

import (
	"bytes"
	"fmt"
	"go/constant"
	"go/token"
	"go/types"

	"go.uber.org/zap"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// Based on Wegman, Mark N. and Zadeck, F. Kenneth. "Constant Propagation with Conditional Branches."
// ACM Transactions on Programming Languages and Systems, 13(2), April 1991, pages 181-210.
// The algorithm is extended to handle interprocedural analysis for a given call chain.
// TODO: model more instructions. For now, we conservatively model them to produce bottom.
// TODO: model memory state for local variables that don't escape.
// TODO: model global variables that are written only during initialization.

// cfgEdge represents a control-flow edge.
type cfgEdge struct {
	// Source block.
	srcBlk *ssa.BasicBlock
	// Destination block.
	dstBlk *ssa.BasicBlock
}

// cfgWorkItem is a basic block with a flag whether only its phi nodes should be visited.
type cfgWorkItem struct {
	blk           *ssa.BasicBlock
	visitPhisOnly bool
}

func (e cfgEdge) String() string {
	return fmt.Sprintf("edge: %s -> %s", e.srcBlk.String(), e.dstBlk.String())
}

// SCCPAnalyzer holds the state for the SCCP analysis of a call chain.
// It is used to analyze the program and determine the possible values of the variables in the call chain.
// SCCPAnalyzer should be created one for each call chain.
type SCCPAnalyzer struct {
	// Program being analyzed.
	prog *ssa.Program
	// Call chain being analyzed.
	callPath []*callgraph.Edge
	// Lattice for values.
	valueState map[ssa.Value]LatticeElement
	// Edges that have been visited (i.e., marked as executable)
	reachableEdges map[cfgEdge]struct{}
	// Blocks that have been visited (i.e., marked as executable)
	reachableBlocks map[*ssa.BasicBlock]struct{}
	// Worklist for SSA values.
	ssaWorklist []ssa.Value
	// Worklist for CFG blocks.
	cfgWorklist []cfgWorkItem
	// Logger for debugging.
	logger *zap.SugaredLogger
	// Call chain edges that should be assumed to be always reachable. These are often synthetic edges and their callsite signatures may not match.
	dontPruneEdges map[*callgraph.Edge]struct{}
	// Metrics for debugging.
	metrics map[string]float64
}

// NewSCCPAnalyzer creates a new analyzer for a call chain.
// It is used to analyze the program and determine the possible values of the variables in the call chain.
func NewSCCPAnalyzer(
	prog *ssa.Program,
	path []*callgraph.Edge,
	logger *zap.SugaredLogger,
	dontPruneEdges map[*callgraph.Edge]struct{},
	metrics map[string]float64,
) *SCCPAnalyzer {
	analyzer := &SCCPAnalyzer{
		prog:            prog,
		callPath:        path,
		valueState:      make(map[ssa.Value]LatticeElement),
		reachableEdges:  make(map[cfgEdge]struct{}),
		reachableBlocks: make(map[*ssa.BasicBlock]struct{}),
		logger:          logger,
		dontPruneEdges:  dontPruneEdges,
		metrics:         metrics,
	}
	return analyzer
}

// analyzeFunction performs the SCCP analysis for a single function in the context of the prefix of its call chain.
// bindings is a set of argument and free variable bindings for the function parameters/free variables.
// TODO: since free vars are captured by reference and since we don't model memory state, they bottom out for now.
func (a *SCCPAnalyzer) analyzeFunction(
	f *ssa.Function,
	bindings map[ssa.Value]LatticeElement,
) {
	// Initialize lattice for all values in the function (constants, functions, etc.)
	a.initializeLatticeForFunction(f)

	// Meet params with arguments.
	a.meetParamsWithArg(f.Params, bindings)

	// Meet free vars with bindings.
	a.meetFreeVarsWithBindings(f.FreeVars, bindings)

	// Initialize reachable blocks.
	a.initializeReachableBlocks(f)

	// Core SCCP analysis loop.
	// Until the CFG and SSA worklists are empty, we visit blocks and values.
	// Each block is visited at most once. O(N) time.
	// Each value is visited at most 3 times (top, constant, bottom). O(3M) time.
	// Total time: O(N + 3M).
	// Guaranteed to terminate.
	// TODO (milind): If we want to be very smart, we can make the lattice of set of values and limit the set size.
	// For example, Top to singleton to set of 2, to set of 3, ... N to bottom.
	// This will allow us to find more concrete values and types.
	// However, this will make the analysis more expensive, and the real gain is not clear.
	// A lot of values will simply be bottom, so, whether it is worth it is not clear.

	for len(a.ssaWorklist) > 0 || len(a.cfgWorklist) > 0 {
		// Visit blocks from the worklist.
		if len(a.cfgWorklist) > 0 {
			workItem := a.cfgWorklist[0]
			a.cfgWorklist = a.cfgWorklist[1:]
			a.logger.Debugw("analyzeFunction", zap.String("visiting_block", workItem.blk.String()))
			a.visitBlock(workItem)
		}

		// Visit values from the worklist.
		if len(a.ssaWorklist) > 0 {
			val := a.ssaWorklist[0]
			a.ssaWorklist = a.ssaWorklist[1:]
			a.logger.Debugw("analyzeFunction", zap.String("visiting_value", val.String()))
			a.visitValue(val)
		}
	}
}

// visitBlock visits a block and its instructions.
// Debug instructions are skipped.
// Phi nodes are handled separately in visitPhisInBlock to ensure correct Phi semantics.
func (a *SCCPAnalyzer) visitBlock(workItem cfgWorkItem) {
	a.logger.Debugw("visitBlock", zap.String("block", workItem.blk.String()))

	// Phi must be modeled as executing in parallel: that is reading all RHS before updating the LHS.
	a.visitPhisInBlock(workItem.blk)

	// If we are only visiting phi nodes, we are done.
	// This happens when a new edge is marked as reachable and the dst block was already reachable.
	if workItem.visitPhisOnly {
		return
	}

	// After all Phi values are updated, visit the remaining instructions in the block.
	for _, instr := range workItem.blk.Instrs {
		// Skip debug instructions.
		if _, ok := instr.(*ssa.DebugRef); ok {
			continue
		}
		// Visit non-Phi instructions.
		if _, ok := instr.(*ssa.Phi); !ok {
			a.visitInstr(instr)
		}
	}
}

// visitPhisInBlock models the evaluation of Phi nodes in a block.
// It is used to update the Phi nodes in the block.
// Note: visitPhisInBlock gets called from two flows:
//  1. When a new edge is marked as reachable and the dst block was already reachable.
//     In this case, the phi nodes may get updated because of a new value flowing into it that was not
//     previously inspected.
//     visitInstr()  -> propagateAlongControlFlow() -> cfgWorklist(cfgWorkItem{blk: dst, visitPhisOnly: true})
//  2. When a new block is marked as reachable for the first time.
//     visitBlock() -> visitPhisInBlock()
func (a *SCCPAnalyzer) visitPhisInBlock(b *ssa.BasicBlock) {
	// To correctly model the parallel semantics of Phi nodes, we must first
	// evaluate all of them using the operand values from before the block,
	// and only then update their lattice states.

	// 1. Read phase: Evaluate all phi nodes and store their new state temporarily.
	newPhiValues := make(map[*ssa.Phi]LatticeElement)
	for _, instr := range b.Instrs {
		if phi, ok := instr.(*ssa.Phi); ok {
			newPhiValues[phi] = a.eval(phi)
		}
	}

	// 2. Write phase: Update the lattice state for each phi node. If the state
	// changed, add the phi to the worklist.
	for phi, newVal := range newPhiValues {
		oldVal := a.getLatticeForSSAValue(phi)
		a.logger.Debugw("visitPhisInBlock",
			zap.String("phi", phi.String()),
			zap.String("oldVal", oldVal.String()),
			zap.String("newVal", newVal.String()),
		)
		if !oldVal.Equal(newVal) {
			a.valueState[phi] = meet(oldVal, newVal, a.logger)
			a.ssaWorklist = append(a.ssaWorklist, phi)
		}
	}
}

// visitValue visits a value and its user instructions.
// Debug instructions are skipped.
// Users are visited iff they are in reachable blocks.
func (a *SCCPAnalyzer) visitValue(v ssa.Value) {
	a.logger.Debugw("visitValue",
		zap.String("value_name", v.Name()),
		zap.String("value_string", v.String()),
	)
	if v.Referrers() != nil {
		a.logger.Debugw("visitValue",
			zap.String("value_name", v.Name()),
			zap.String("value_string", v.String()),
			zap.Int("referrers", len(*v.Referrers())),
		)
		for _, user := range *v.Referrers() {
			// Skip debug instructions.
			if _, ok := user.(*ssa.DebugRef); ok {
				continue
			}
			if a.isBlockReachable(user.Block()) {
				a.logger.Debugw("visitValue_user_reachable",
					zap.String("user", user.String()),
					zap.String("block", user.Block().String()),
				)
				a.visitInstr(user)
			} else {
				a.logger.Debugw("visitValue_user_unreachable",
					zap.String("user", user.String()),
					zap.String("block", user.Block().String()),
				)
			}
		}
	} else {
		a.logger.Debugw("visitValue_no_referrers",
			zap.String("value_name", v.Name()),
			zap.String("value_string", v.String()),
		)
	}
}

// visitInstr models the evaluation of an instruction.
// It is used to update the value lattice and add the value to the ssa worklist.
func (a *SCCPAnalyzer) visitInstr(instr ssa.Instruction) {
	if v, ok := instr.(ssa.Value); ok {
		a.logger.Debugw("visitInstr",
			zap.String("block", instr.Block().String()),
			zap.String("instruction", fmt.Sprintf("%s = %s", v.String(), instr.String())),
		)
	} else {
		a.logger.Debugw("visitInstr",
			zap.String("block", instr.Block().String()),
			zap.String("instruction", instr.String()),
		)
	}

	// Handle instructions that define a new SSA value.
	if _, ok := instr.(ssa.Value); ok {
		// Phi nodes are handled separately in visitBlock to ensure correct semantics.
		if _, isPhi := instr.(*ssa.Phi); !isPhi {
			a.modelInstructionAndUpdateValueLattice(instr)
		}
	}

	// TODO: Handle instructions that modify memory state.

	// If this was a control instruction check which successor(s) becomes reachable as a result of its execution.
	switch i := instr.(type) {
	case *ssa.If:

		// A tuple should never be a condition. If we ever see this, we should report bottom.
		condVal, _ := a.getLatticeForSSAValue(i.Cond).getSingletonLatticeElement()

		block := i.Block()
		a.logger.Debugw("visitInstr_if",
			zap.String("instruction", i.String()),
			zap.String("condVal", condVal.String()),
			zap.String("block", block.String()),
		)

		switch cv := condVal.(type) {
		case ConstantVal:
			if cv.Value.Value.Kind() == constant.Bool {
				a.metrics[_boolAsIfCond]++
				if constant.BoolVal(cv.Value.Value) {
					// True branch is reachable.
					a.logger.Debugw("visitInstr_if_true",
						zap.String("instruction", i.String()),
						zap.String("condVal", condVal.String()),
						zap.String("block", block.String()),
					)
					a.propagateAlongControlFlow(block, i.Block().Succs[0]) // True branch
				} else {
					// False branch is reachable.
					a.logger.Debugw("visitInstr_if_false",
						zap.String("instruction", i.String()),
						zap.String("condVal", condVal.String()),
						zap.String("block", block.String()),
					)
					a.propagateAlongControlFlow(block, i.Block().Succs[1]) // False branch
				}
			}
		case BottomVal:
			a.metrics[_bottomAsIfCond]++
			a.logger.Debugw("visitInstr_if_bottom",
				zap.String("instruction", i.String()),
				zap.String("condVal", condVal.String()),
				zap.String("block", block.String()),
			)
			// Both branches are reachable
			a.propagateAlongControlFlow(block, i.Block().Succs[0])
			a.propagateAlongControlFlow(block, i.Block().Succs[1])
		case TopVal:
			// Top branch result is awkward.
			// Collect metrics for this.
			// If this reaches too many, we should think whether we should make both branches reachable.
			a.metrics[_topAsIfCond]++
			a.logger.Debugw("visitInstr_if_top",
				zap.String("instruction", i.String()),
				zap.String("condVal", condVal.String()),
				zap.String("block", block.String()),
			)
		}
	case *ssa.Jump:
		// Jump is always reachable.
		a.logger.Debugw("visitInstr_jump",
			zap.String("instruction", i.String()),
			zap.String("block", i.Block().String()),
		)
		a.propagateAlongControlFlow(i.Block(), i.Block().Succs[0])
	}
}

// modelInstructionAndUpdateValueLattice models the evaluation of an instruction and updates the value lattice based on the result.
// It is used to update the value lattice and add the value to the ssa worklist.
func (a *SCCPAnalyzer) modelInstructionAndUpdateValueLattice(instr ssa.Instruction) {
	val, ok := instr.(ssa.Value)
	if !ok {
		// Non-value instructions produce no new values.
		return
	}

	// Get the old lattice value for the instruction.
	oldVal := a.getLatticeForSSAValue(val)
	// Evaluate the instruction to get the new lattice value.
	newVal := a.eval(instr)

	a.logger.Debugw("modelInstructionAndUpdateValueLattice",
		zap.String("instruction", fmt.Sprintf("%s = %s", val.Name(), val.String())),
		zap.String("oldVal", oldVal.String()),
		zap.String("newVal", newVal.String()),
	)
	// If the lattice value has changed, update the value and add the value to the ssa worklist.
	if !oldVal.Equal(newVal) {
		a.logger.Debugw("modelInstructionAndUpdateValueLattice:update",
			zap.String("instruction", fmt.Sprintf("%s = %s", val.Name(), val.String())),
			zap.String("newVal", newVal.String()),
		)
		// Update the lattice with the meet of the old and new values.
		a.valueState[val] = meet(oldVal, newVal, a.logger)
		// Since the value has changed, we need to re-visit it and its users.
		a.ssaWorklist = append(a.ssaWorklist, val)
	} else {
		a.logger.Debugw("modelInstructionAndUpdateValueLattice:no_update",
			zap.String("instruction", fmt.Sprintf("%s = %s", val.Name(), val.String())))
	}
}

// propagateAlongControlFlow models the propagation of a value along a control flow edge.
// It is used to mark an edge as reachable and add the dst block to the worklist.
// If the dst block is not reachable, it will be added to the worklist and the phi nodes in the block will be visited.
func (a *SCCPAnalyzer) propagateAlongControlFlow(src *ssa.BasicBlock, dst *ssa.BasicBlock) {
	e := cfgEdge{srcBlk: src, dstBlk: dst}
	a.logger.Debugw("propagateAlongControlFlow",
		zap.String("src", src.String()),
		zap.String("dst", dst.String()),
		zap.String("edge", e.String()),
	)
	if _, exists := a.reachableEdges[e]; !exists {
		a.reachableEdges[e] = struct{}{}
		a.logger.Debugw("propagateAlongControlFlow_reachable",
			zap.String("edge", e.String()),
		)
		if _, r := a.reachableBlocks[dst]; !r {
			// This will automatically trigger a visit of the phi nodes in the dst block.
			a.reachableBlocks[dst] = struct{}{}
			a.cfgWorklist = append(a.cfgWorklist, cfgWorkItem{blk: dst, visitPhisOnly: false})
			a.logger.Debugw("propagateAlongControlFlow_dst_reachable",
				zap.String("dst", dst.String()),
			)
		} else {
			a.logger.Debugw("propagateAlongControlFlow_dst_already_reachable",
				zap.String("dst", dst.String()),
			)
			// All phi nodes in the dst block should be revisited since they were not evaluated with this new edge.
			a.cfgWorklist = append(a.cfgWorklist, cfgWorkItem{blk: dst, visitPhisOnly: true})
		}
	}
}

// eval models the instruction execution taking into account values if operands and potentially producing new values.
// The fallback is to assume the instruction produces bottom.
func (a *SCCPAnalyzer) eval(instr ssa.Instruction) LatticeElement {
	if v, ok := instr.(ssa.Value); ok {
		a.logger.Debugw("eval",
			zap.String("block", instr.Block().String()),
			zap.String("instruction", fmt.Sprintf("%s = %s", v.String(), instr.String())),
		)
	} else {
		a.logger.Debugw("eval",
			zap.String("block", instr.Block().String()),
			zap.String("instruction", instr.String()),
		)
	}

	v := instr.(ssa.Value)
	defaultType := ConcreteType{T: v.Type()}

	switch i := instr.(type) {
	case *ssa.Phi:
		return a.evalPhi(i)
	case *ssa.BinOp:
		return a.evalBinOp(i)
	case *ssa.UnOp:
		return a.evalUnOp(i)
	case *ssa.MakeClosure:
		return createSingletonLatticeElement(FunctionVal{fn: i}, defaultType)
	case *ssa.MakeInterface:
		return a.evalMakeInterface(i)
	case *ssa.MakeChan:
		return a.evalMakeChan(i)
	case *ssa.MakeMap:
		return a.evalMakeMap(i)
	case *ssa.MakeSlice:
		return a.evalMakeSlice(i)
	case *ssa.Slice:
		return a.evalSlice(i)
	case *ssa.ChangeType, *ssa.Convert:
		a.logger.Debugw("eval_changetype_convert",
			zap.String("instruction", i.String()),
		)
		var operand ssa.Value
		switch op := i.(type) {
		case *ssa.ChangeType:
			operand = op.X
		case *ssa.Convert:
			operand = op.X
		}
		// We can only handle singleton lattice elements.
		operandLatticeValue, _ := a.getLatticeForSSAValue(operand).getSingletonLatticeElement()
		a.logger.Debugw("eval_changetype_convert_operand",
			zap.String("instruction", i.String()),
			zap.String("operand", operand.String()),
			zap.String("operandLattice", operandLatticeValue.String()),
		)
		return createSingletonLatticeElement(operandLatticeValue, defaultType)

	case *ssa.TypeAssert:
		return a.evalTypeAssert(i)

	case *ssa.Extract:
		return a.evalExtract(i)
	case *ssa.Alloc:
		return a.evalAlloc(i)
	default:
		a.logger.Debugw("eval_default",
			zap.String("instruction", i.String()),
		)
		// if the type is a tuple, create a lattice element for each element.
		if tupleType, ok := v.Type().(*types.Tuple); ok && tupleType.Len() > 0 {
			return createTupleLatticeBottom(*tupleType)
		}

		return createSingletonLatticeElement(BottomVal{}, defaultType)
	}
}

func (a *SCCPAnalyzer) createUnknownConstant(t types.Type) LatticeElement {
	// If type is a tuple, we can't handle it.
	if tupleType, ok := t.(*types.Tuple); ok {
		return createTupleLatticeBottom(*tupleType)
	}
	// The instruction creates a new non-nil value of a given type.
	// We can simulate it to be a constant unknown value of the given type.
	c := ssa.NewConst(constant.MakeUnknown(), t)
	return createSingletonLatticeElement(ConstantVal{Value: *c}, ConcreteType{T: t})

}

// evalAlloc models the evaluation of an allocation instruction.

func (a *SCCPAnalyzer) evalAlloc(alloc *ssa.Alloc) LatticeElement {
	return a.createUnknownConstant(alloc.Type())
}

// evalMakeChan models the evaluation of a make chan operation.
func (a *SCCPAnalyzer) evalMakeChan(makeChan *ssa.MakeChan) LatticeElement {
	return a.createUnknownConstant(makeChan.Type())

}

// evalMakeMap models the evaluation of a make map operation.
func (a *SCCPAnalyzer) evalMakeMap(makeMap *ssa.MakeMap) LatticeElement {
	return a.createUnknownConstant(makeMap.Type())
}

// evalMakeSlice models the evaluation of a make slice operation.
func (a *SCCPAnalyzer) evalMakeSlice(makeSlice *ssa.MakeSlice) LatticeElement {
	return a.createUnknownConstant(makeSlice.Type())
}

// evalSlice models the evaluation of a slice operation.
func (a *SCCPAnalyzer) evalSlice(slice *ssa.Slice) LatticeElement {
	// A slice produces a new value of a given type.
	// We can simulate it to be a constant unknown value of the given type.
	// If slice is nil, program panics.
	// Hence, only if the incoming value is a non-nil constant, we can produce an output.
	incomingLatticeValue := a.getLatticeForSSAValue(slice.X)
	valLattice, _ := incomingLatticeValue.getSingletonLatticeElement()
	if constVal, ok := valLattice.(ConstantVal); ok {
		if !constVal.Value.IsNil() {
			return a.createUnknownConstant(slice.Type())
		}
	}
	return createTypedLatticeBottom(slice.Type())
}

// evalPhi models the evaluation of a phi node.
// It looks at the incoming values from executable edges and meets them with the top value.
// The result is the meet of the incoming values.
func (a *SCCPAnalyzer) evalPhi(phi *ssa.Phi) LatticeElement {
	val := createTopLatticeElement(phi.Type()) // Top is the identity for meet.
	a.logger.Debugw("eval_phi",
		zap.String("block", phi.Block().String()),
		zap.String("instruction", phi.String()),
	)
	for predIndex, predBlock := range phi.Block().Preds {
		e := cfgEdge{srcBlk: predBlock, dstBlk: phi.Block()}
		a.logger.Debugw("eval_phi_edge",
			zap.String("instruction", phi.String()),
			zap.String("edge", e.srcBlk.String()),
		)
		// Meet incoming values from executable edges.
		if _, isReachable := a.reachableEdges[e]; isReachable {
			a.logger.Debugw("eval_phi_edge_reachable",
				zap.String("instruction", phi.String()),
				zap.String("edge", e.srcBlk.String()),
			)
			incomingValue := phi.Edges[predIndex]
			val = meet(val, a.getLatticeForSSAValue(incomingValue), a.logger)
		} else {
			a.logger.Debugw("eval_phi_edge_not_reachable",
				zap.String("instruction", phi.String()),
				zap.String("edge", e.srcBlk.String()),
			)
		}
	}
	return val
}

// checkAndRecordTupleAsOperand checks if the type is a tuple and records it in metrics.
func (a *SCCPAnalyzer) checkAndRecordTupleAsOperand(t types.Type, tag string) {
	if _, ok := t.(*types.Tuple); ok {
		a.metrics[tag]++
	}
}

// evalBinOp models the evaluation of a binary operation.
// It looks at the first operand (X) and checks its lattice value.
// Depending on the type of the operand, it will either evaluate the operation or return bottom.
// Only ConstantVal/FunctionVal can proceed with evaluation.
func (a *SCCPAnalyzer) evalBinOp(binOp *ssa.BinOp) LatticeElement {
	// If we ever accidentally see a tuple as an operand, we record it in metrics.
	a.checkAndRecordTupleAsOperand(binOp.Type(), _tupleAsBinopType)
	a.checkAndRecordTupleAsOperand(binOp.X.Type(), _tupleAsBinopArgX)
	a.checkAndRecordTupleAsOperand(binOp.Y.Type(), _tupleAsBinopArgY)

	defaultType := ConcreteType{T: binOp.Type()}

	x, _ := a.getLatticeForSSAValue(binOp.X).getSingletonLatticeElement()
	y, _ := a.getLatticeForSSAValue(binOp.Y).getSingletonLatticeElement()
	a.logger.Debugw("eval_binop",
		zap.String("instruction", binOp.String()),
		zap.String("x", x.String()),
		zap.String("y", y.String()),
	)
	// Set the default result to bottom. It will be overridden if the operation is supported.
	var resVal ValueLattice = BottomVal{}

	switch xc := x.(type) {
	case FunctionVal: // This is the case where we have result = FunctionVal BinOP *
		resVal = a.evalBinopFuncval(binOp, xc, y)
	case ConstantVal: // This is the case where we have result = ConstantVal BinOP *
		resVal = a.evalBinopConstantval(binOp, xc, y)
	case BottomVal: // This is the case where we have result = BottomVal BinOP *
		a.logger.Debugw("eval_binop_x_bottom",
			zap.String("instruction", binOp.String()),
		)
	case TopVal: // This is the case where we have result = TopVal BinOP *
		// In a valid Go program, this should not happen.
		a.logger.Debugw("eval_binop_x_top",
			zap.String("instruction", binOp.String()),
		)
		a.metrics[_binopXTop]++
		// bottom.
	}
	return createSingletonLatticeElement(resVal, defaultType)
}

func isUnknownConst(v constant.Value) bool {
	return v == nil || v.Kind() == constant.Unknown
}

// evalBinopConstantval models the evaluation of a binary operation with a constant operand.
// It handles the case where the first operand (X) is a constant.
// It looks at the second operand (Y) and checks its lattice value.
//
// Decision Flow:
//
//	      ┌─────────────────────────────────────┐
//	      │ evalBinopConstantval(binOp, xc, y)  │
//	      │ X = ConstantVal, Y = ?              │
//	      └─────────────────┬───────────────────┘
//	                        │
//	                        ▼
//	              ┌─────────────────--------┐
//	              │ Switch on Y ValueLattice|
//	              └─────┬───┬───┬──--------─┘
//	                    │   │   │
//	        ┌───────────┘   │   └───────────┐
//	        │               │               │
//	        ▼               ▼               ▼
//	┌───────────────┐ ┌─────────────┐  ┌─────────────┐
//	│  Y=BottomVal  │ │Y=ConstantVal│  │Y=FunctionVal│
//	│               │ │             │  │             │
//	│   → Bottom    │ └──────┬──────┘  │   X.nil?    │
//	└───────────────┘        │         │             │
//	                         ▼         │ → EQL/NEQ   │
//	                ┌─────────────────┐│   only      │
//	                │ X.nil? Y.nil?   │└─────────────┘
//	                └────┬──────┬─────┘
//	                     │      │
//	                Both │      │ One/Both
//	              non-nil│      │ nil
//	                     ▼      ▼
//	            ┌─────────────┐ ┌─────────────┐
//	            │ Evaluate Op:│ │ Handle nil  │
//	            │ • EQL/NEQ   │ │ comparisons │
//	            │ • SHL/SHR   │ │ (EQL/NEQ)   │
//	            │ • QUO/REM   │ └─────────────┘
//	            │ • Default   │
//	            └─────────────┘
//
// Operation Types Handled:
//
// 1. COMPARISON OPERATIONS (EQL, NEQ, LSS, GTR, LEQ, GEQ):
//   - Uses constant.Compare() to evaluate the comparison
//   - Returns true/false constant value based on comparison result
//   - Supports all Go comparison operators for constant values
//
// 2. SHIFT OPERATIONS (SHL, SHR):
//   - Converts right-hand side (Y) to uint64 for shift amount
//   - Uses constant.Shift() to perform bit shifting
//   - Returns bottom if RHS cannot be converted to uint64
//
// 3. DIVISION OPERATIONS (QUO, QUO_ASSIGN):
//   - Checks for division by zero (returns bottom if divisor is zero)
//   - Forces integer division for int/int operands using QUO_ASSIGN
//   - Uses constant.BinaryOp() for actual division computation
//
// 4. REMAINDER OPERATIONS (REM, REM_ASSIGN):
//   - Only supported for integer operands
//   - Checks for division by zero (returns bottom if divisor is zero)
//   - Uses constant.BinaryOp() for modulo computation
//
// 5. DEFAULT OPERATIONS:
//   - All other binary operations (ADD, SUB, MUL, AND, OR, XOR, etc.)
//   - Uses constant.BinaryOp() directly
//   - Assumes Go's constant package can handle the operation
//
// NIL HANDLING:
//   - When one or both operands are nil, only EQL and NEQ are supported
//   - nil == nil → true, nil != nil → false
//   - nil == non-nil → false, nil != non-nil → true
//   - All other operations with nil operands return bottom
func (a *SCCPAnalyzer) evalBinopConstantval(binOp *ssa.BinOp, xc ConstantVal, y ValueLattice) ValueLattice {
	var resVal ValueLattice = BottomVal{}
	switch yc := y.(type) {
	case ConstantVal: // If both are constants, we can evaluate the operation.
		if !constHasNoScalarValue(xc.Value) && !constHasNoScalarValue(yc.Value) && !isUnknownConst(xc.Value.Value) && !isUnknownConst(yc.Value.Value) {
			a.logger.Debugw("eval_binop_xc_yc",
				zap.String("instruction", binOp.String()),
				zap.String("xc", xc.String()),
				zap.String("yc", yc.String()),
			)
			switch binOp.Op {
			case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
				if constant.Compare(xc.Value.Value, binOp.Op, yc.Value.Value) {
					resVal = createTrueConstantVal()
				} else {
					resVal = createFalseConstantVal()
				}
			case token.SHL, token.SHR:
				// SHL and SHR are supported only on integers.
				// Make RHS into a uint64 constant.
				if rhs, ok := constant.Uint64Val(yc.Value.Value); ok {
					newConst := constant.Shift(xc.Value.Value, binOp.Op, uint(rhs))
					resVal = createConstantVal(*ssa.NewConst(newConst, binOp.Type()))
					a.logger.Debugw("eval_binop_xc_yc_resVal",
						zap.String("instruction", binOp.String()),
						zap.String("xc", xc.String()),
						zap.String("yc", yc.String()),
						zap.String("resVal", resVal.String()),
					)
				} // else bottom
			case token.QUO, token.QUO_ASSIGN: // Ensure division by zero is handled.
				// Integer, Float, and complex division by zero will panic.
				if !isZero(yc.Value.Value) {
					// if both xc and yc are integers, then the result is an integer.
					// However the constant.BinaryOp automatically uses rational numbers.
					// Force QUO_ASSIGN in such cases which will return an integer.
					op := binOp.Op
					if xc.Value.Value.Kind() == constant.Int && yc.Value.Value.Kind() == constant.Int {
						op = token.QUO_ASSIGN
					}
					newConst := constant.BinaryOp(xc.Value.Value, op, yc.Value.Value)
					resVal = createConstantVal(*ssa.NewConst(newConst, binOp.Type()))
				} // else bottom
			case token.REM, token.REM_ASSIGN:
				// Remainder is possible only on integers without zero divisor.
				if yc.Value.Value.Kind() == constant.Int && !isZero(yc.Value.Value) {
					a.logger.Debugw("eval_binop_xc_yc_rem",
						zap.String("instruction", binOp.String()),
						zap.String("xc", xc.String()),
						zap.String("yc", yc.String()),
						zap.String("xc.Value.Kind()", xc.Value.Value.Kind().String()),
						zap.String("yc.Value.Kind()", yc.Value.Value.Kind().String()),
					)
					newConst := constant.BinaryOp(xc.Value.Value, binOp.Op, yc.Value.Value)
					resVal = createConstantVal(*ssa.NewConst(newConst, binOp.Type()))
				} // else bottom
			default: // For all other operations, we can evaluate the operation.
				newConst := constant.BinaryOp(xc.Value.Value, binOp.Op, yc.Value.Value)
				resVal = createConstantVal(*ssa.NewConst(newConst, binOp.Type()))
			}
		} else if constHasNoScalarValue(xc.Value) && constHasNoScalarValue(yc.Value) {
			// we can support EQL and NEQ for nil values.
			if binOp.Op == token.EQL {
				resVal = createTrueConstantVal()
			} else if binOp.Op == token.NEQ {
				resVal = createFalseConstantVal()
			} else {
				a.logger.Debugw("eval_binop_x_constant_y_bottom",
					zap.String("instruction", binOp.String()),
				)
				// bottom.
			}
		} else {
			// one is nil and the other is not.
			// Hence they are not equal.
			// We can support EQL and NEQ for nil values.
			if binOp.Op == token.EQL {
				resVal = createFalseConstantVal()
			} else if binOp.Op == token.NEQ {
				resVal = createTrueConstantVal()
			} else {
				a.logger.Debugw("eval_binop_x_constant_y_bottom",
					zap.String("instruction", binOp.String()),
				)
				// bottom.
			}
		}
	case FunctionVal:
		// This is the case where we have result = ConstantVal BinOP FunctionVal.
		// We can only compare it with nil constant.
		a.logger.Debugw("eval_binop_x_function_y_function",
			zap.String("instruction", binOp.String()),
		)
		// If the constant is a nil, we can compare it with a function.
		if xc.Value.IsNil() {
			if binOp.Op == token.EQL {
				// A function is never equal to nil. Hence EQL is always false.
				resVal = createFalseConstantVal()
			} else if binOp.Op == token.NEQ {
				// A function is never equal to nil. Hence NEQ is always true.
				resVal = createTrueConstantVal()
			} else {
				// The only allowed operation is EQL or NEQ.
				a.logger.Debugw("eval_binop_x_function_y_bottom",
					zap.String("instruction", binOp.String()),
				)
				// bottom.
			}
		}
		// else bottom.
	case BottomVal:
		a.logger.Debugw("eval_binop_x_constant_y_bottom",
			zap.String("instruction", binOp.String()),
		)
		// bottom.

	case TopVal: // In a valid Go program, this should not happen.
		a.metrics[_binopYTop]++
		// bottom.
	}
	return resVal
}

// evalBinopFuncval models the evaluation of a binary operation with a function operand.
// Here the first operand (X) is a function.
// It looks at the second operand (Y) and checks its lattice value.
// Depending on the type of the operand, it will either evaluate the operation or return bottom.
// Only ConstantVal can proceed with evaluation.
//
// Decision Flow:
//
//	      ┌─────────────────────────────────────┐
//	      │ evalBinopFuncval(binOp, xc, y)      │
//	      │ X = FunctionVal, Y = ?              │
//	      └─────────────────┬───────────────────┘
//	                        │
//	                        ▼
//	              ┌─────────────────────────┐
//	              │ Switch on Y ValueLattice│
//	              └─────┬───┬───┬───┬───────┘
//	                    │   │   │   │
//	        ┌───────────┘   │   │   └───────────┐
//	        │               │   │               │
//	        ▼               ▼   ▼               ▼
//	┌───────────────┐ ┌─────────┐ ┌─────────────┐ ┌─────────────┐
//	│Y=ConstantVal  │ │Y=FuncVal│ │ Y=BottomVal │ │  Y=TopVal   │
//	│               │ │         │ │             │ │             │
//	│   Y.nil?      │ │ x==y?   │ │  → Bottom   │ │  → Bottom   │
//	└───────┬───────┘ └────┬────┘ └─────────────┘ └─────────────┘
//	        │              │
//	        ▼              ▼
//	┌───────────────┐ ┌─────────────┐
//	│ Y is nil:     │ │ Function    │
//	│ → EQL: false  │ │ Comparison: │
//	│ → NEQ: true   │ │ → EQL: same │
//	│ → Others: ⊥   │ │ → NEQ: !same│
//	└───────────────┘ │ → Others: ⊥ │
//	                  └─────────────┘
func (a *SCCPAnalyzer) evalBinopFuncval(binOp *ssa.BinOp, xc FunctionVal, y ValueLattice) ValueLattice {
	var resVal ValueLattice = BottomVal{}
	switch yc := y.(type) {
	case ConstantVal: // This is the case where we have result = FunctionVal BinOP ConstantVal.
		// We need to handle the case where the constant is nil.
		if yc.Value.IsNil() {
			a.logger.Debugw("eval_binop_x_function_y_constant_nil",
				zap.String("instruction", binOp.String()),
			)
			if binOp.Op == token.EQL {
				// FunctionVal is never equal to Nil. Hence EQL is always false.
				resVal = createFalseConstantVal()
			} else if binOp.Op == token.NEQ {
				// FunctionVal is never equal to Nil. Hence NEQ is always true.
				resVal = createTrueConstantVal()
			} else {
				// The only allowed operation is EQL or NEQ.
				a.logger.Debugw("eval_binop_x_function_y_bottom",
					zap.String("instruction", binOp.String()),
				)
				// bottom.
			}
		} // else bottom
	case FunctionVal: // This is the case where we have result = FunctionVal BinOP FunctionVal.
		a.logger.Debugw("eval_binop_x_function_y_function",
			zap.String("instruction", binOp.String()),
		)
		// .fn can either be a ssa.Function or a ssa.MakeClosure.
		// Directly compare whether the functions are the same.
		same := xc.fn == yc.fn
		if binOp.Op == token.EQL {
			resVal = createBoolConstantVal(same)
		} else if binOp.Op == token.NEQ {
			resVal = createBoolConstantVal(!same)
		} else {
			a.logger.Debugw("eval_binop_x_function_y_bottom",
				zap.String("instruction", binOp.String()),
			)
			// bottom.
		}
	case BottomVal:
		a.logger.Debugw("eval_binop_x_function_y_bottom",
			zap.String("instruction", binOp.String()),
		)
		// bottom.
	case TopVal: // In a valid Go program, this should not happen.
		a.metrics[_binopYTop]++
		// bottom.
	}
	return resVal
}

// evalExtract models the evaluation of an extract operation.
// Go source code such as myStruct, ok := myIface.(myStructType) creates a tuple of the value and the boolean.
// Subsequently an access to myStruct or ok is made. The code generation extracts the value or the boolean from the tuple.
// The corresponding SSA code is:
// t0 = typeassert(myIface, myStructType) # t0 is a tuple of the value and the boolean.
// t1 = extractIns (t0, #1) # t1 is of type myStruct type
func (a *SCCPAnalyzer) evalExtract(extractIns *ssa.Extract) LatticeElement {
	defaultType := ConcreteType{T: extractIns.Type()}

	// A word on tuple of tuples:
	// In go/types, a *types.Tuple is only a container for a list of variables (each a *types.Var with a single types.Type).
	// Tuples show up for:
	// - function/method parameter lists (Signature.Params()),
	// - result lists (Signature.Results()),
	// - multi-valued expressions like map index, receive, type assertion, and some builtins (internally represented as a tuple of results).
	// To the best of my knowlege (Milind), no Go source code can create a Tuple of Tuples.
	// Programatically we can synthesize a Tuple of Tuples, but is artificial.
	// Hence we can safely assume that a Tuple of Tuples is not possible as of Sep 2025 Go 1.25.
	// If we ever face this corner case, let's panic.
	if _, ok := extractIns.Type().(*types.Tuple); ok {
		a.logger.Debugw("eval_extract_tuple_of_tuples",
			zap.String("instruction", extractIns.String()),
			zap.String("tuple", extractIns.Tuple.String()),
		)
		// Let us panic rather than handling it softly.
		// If we hit this we'll know and can update the tool.
		panic("Tuple of Tuples is not supported")
	}

	a.logger.Debugw("eval_extract",
		zap.String("instruction", extractIns.String()),
	)
	if extractIns.Index < 0 {
		a.logger.Debugw("eval_extract_index_out_of_bounds",
			zap.String("instruction", extractIns.String()),
			zap.Int("index", extractIns.Index),
		)
		a.metrics[_extractIndexOutOfBounds]++
		return createSingletonLatticeElement(BottomVal{}, defaultType)
	}

	// Get lattice value for tupleVal
	tupleLatticeElement := a.getLatticeForSSAValue(extractIns.Tuple)
	// Make sure it has index elements.
	if len(tupleLatticeElement.vtPair) <= extractIns.Index {
		a.logger.Errorw("eval_extract_tuple_index_out_of_bounds",
			zap.String("instruction", extractIns.String()),
			zap.String("tuple", extractIns.Tuple.String()),
			zap.Int("index", extractIns.Index),
		)
		a.metrics[_extractIndexOutOfBounds]++
		return createSingletonLatticeElement(BottomVal{}, defaultType)
	}
	// Get the value and type of the extracted element.
	extractVal := tupleLatticeElement.vtPair[extractIns.Index].Value
	extractType := tupleLatticeElement.vtPair[extractIns.Index].Type
	a.logger.Debugw("eval_extract_extractVal_extractType",
		zap.String("instruction", extractIns.String()),
		zap.String("extractVal", extractVal.String()),
		zap.String("extractType", extractType.String()),
	)

	// Return the extracted value and type.
	return createSingletonLatticeElement(extractVal, extractType)
}

// evalMakeInterface models the evaluation of a make interface operation.
func (a *SCCPAnalyzer) evalMakeInterface(i *ssa.MakeInterface) LatticeElement {
	a.logger.Debugw("eval_makeinterface",
		zap.String("instruction", i.String()))
	// Since the interface only wraps the underlying type+value,
	// we can just return the lattice type+value for the underlying value.
	return a.getLatticeForSSAValue(i.X)
}

// evalTypeAssert models the evaluation of a type assertion operation.
// Go source code such as myStruct, ok := myIface.(myStructType) creates a tuple of the value and the boolean.
// The corresponding SSA code is:
// t0 = typeassert(myIface, myStructType, CommaOk)
// If CommaOk is true, t0 is a tuple of the value and the boolean.
// If CommaOk is false, t0 is a single value of type myStructType.
func (a *SCCPAnalyzer) evalTypeAssert(i *ssa.TypeAssert) LatticeElement {
	v1, v2 := a.handleTypeAssertion(i)
	if i.CommaOk {
		// If the type assertion is comma ok, we need to return a tuple of the value and the boolean.
		return LatticeElement{
			vtPair: []ValueTypePair{
				{Value: v1.vtPair[0].Value, Type: v1.vtPair[0].Type},
				{Value: v2.vtPair[0].Value, Type: v2.vtPair[0].Type},
			},
		}
	}
	// If the type assertion is not comma ok, we can just return the asserted value.
	return v1
}

// evalUnOp models the evaluation of a unary operation.
// Channel read <- is a unary operation but its output value cannot be modeled easily.
// We always return bottom for this case.
// Unary ops on nil are also not supported and we always return bottom for this case.
// All other unary ops are supported and we evaluate them.
func (a *SCCPAnalyzer) evalUnOp(unOp *ssa.UnOp) LatticeElement {
	defaultType := ConcreteType{T: unOp.Type()}
	var resVal ValueLattice = BottomVal{}

	// TODO: handle memory state for unary op i.Op == token.MUL .
	// Handle load from memory (*x)

	// Handle other unary ops
	x, _ := a.getLatticeForSSAValue(unOp.X).getSingletonLatticeElement()
	if _, ok := x.(BottomVal); ok {
		a.logger.Debugw("eval_unop_x_bottom",
			zap.String("instruction", unOp.String()),
			zap.String("op", unOp.Op.String()),
		)
		// bottom.
	} else if xc, ok := x.(ConstantVal); ok {
		if unOp.Op != token.ARROW { // Channel receive is not constant
			if !constHasNoScalarValue(xc.Value) && !isUnknownConst(xc.Value.Value) { // If the constant is not nil and not unknown we can evaluate the unary operation.
				newConst := constant.UnaryOp(unOp.Op, xc.Value.Value, 0)
				resVal = createConstantVal(*ssa.NewConst(newConst, unOp.Type()))
				a.logger.Debugw("eval_unop_xc_resVal",
					zap.String("instruction", unOp.String()),
					zap.String("op", unOp.Op.String()),
					zap.String("xc", xc.String()),
					zap.String("resVal", resVal.String()),
				)
			}
		} else {
			a.logger.Debugw("eval_unop_xc_bottom",
				zap.String("instruction", unOp.String()),
				zap.String("op", unOp.Op.String()),
			)
			// bottom.
		}
	}
	// If the result type is a tuple (e.g., channel receive with comma-ok),
	// create a tuple lattice element with bottom for each element.
	if tupleType, ok := unOp.Type().(*types.Tuple); ok && tupleType.Len() > 0 {
		return createTupleLatticeBottom(*tupleType)
	}
	return createSingletonLatticeElement(resVal, defaultType)
}

// handleTypeAssertion models the evaluation of a type assertion operation.
//
// FLOW CHART AND RATIONALE:
//
// This function analyzes Go type assertions (x.(T)) in SCCP static analysis to determine:
// 1. Result value: What value will be produced if the assertion succeeds
// 2. Success boolean: Whether the assertion will succeed or fail
// The key approach is to first check whether the value lattice is a constant/function value.
// If not check whether the type lattice is a concrete type.
//
// DECISION FLOW:
//  1. GET LATTICE ELEMENT for ins.X
//     └─→ If len(le.vtPair) != 1 → INFEASIBLE → Return (Bottom, Bottom)
//     Rationale: Multiple or zero values means we can't analyze precisely
//
// 2. EXTRACT FIRST VT PAIR: vt := le.vtPair[0]
//
// 3. SWITCH ON VALUE TYPE (vt.Value):
//
//	┌─ ConstantVal ────────────────────────────────────────────────────┐
//	│  └─→ If nil constant:                                            │
//	│      └─→ If interface type → Return (Constant(nil,T), False)     │
//	│          Rationale: Nil interface cannot be converted to concrete│
//	│      └─→ Else → doTypeCheck (precise analysis possible)          │
//	│  └─→ If non-nil → doTypeCheck (precise analysis possible)        │
//	└──────────────────────────────────────────────────────────-───────┘
//
//	┌─ FunctionVal ────────────────────────────────────────────────────┐
//	│  └─→ doTypeCheck(funcVal.fn.Type(), funcVal, ins)                │
//	│      Rationale: Functions have concrete types, precise analysis  │
//	└──────────────────────────────────────────────────────────-───────┘
//
//	┌─ TopVal (Unknown runtime values) -───────────────────────────────┐
//	│  └─→ Switch on vt.Type:                                          │
//	│      ├─ ConcreteType:                                            │
//	│      │   └─→ If interface → Return (Bottom, Bottom)              │
//	│      │       Rationale: Interfaces can hold any implementing type│
//	│      │   └─→ Else → doTypeCheck (can still check statically)     │
//	│      ├─ TopType → Return (Top, Top)                              │
//	│      │   Rationale: Most permissive case, could be anything      │
//	│      └─ BottomType → Return (Bottom, Bottom)                     │
//	│          Rationale: Unlikely mismatch, play safe                 │
//	└──────────────────────────────────────────────-───────────────────┘
//
//	┌─ BottomVal (Multiple  values) --------------─────────────────────┐
//	│  └─→ Switch on vt.Type:                                          │
//	│      ├─ ConcreteType:                                            │
//	│      │   └─→ If interface → Return (Bottom, Bottom)              │
//	│      │   └─→ Else → doTypeCheck (even bottom values can be typed)│
//	│      ├─ TopType → Return (Bottom, Bottom)                        │
//	│      └─ BottomType → Return (Bottom, Bottom)                     │
//	│          Rationale: All cases conservative for unreachable values│
//	└─────────────────────────────────────────────────────────-────────┘
//
// CONSERVATIVE ANALYSIS STRATEGY:
// - When uncertain (interfaces, type mismatches) → Return Bottom (unknown)
// - When confident (concrete types with constants/functions) → Delegate to doTypeCheck
// - This prevents false positives while maintaining soundness in static analysis
//
// RETURN VALUES:
// - First return: LatticeElement representing the result value of successful assertion
// - Second return: LatticeElement representing whether assertion will succeed (bool)
func (a *SCCPAnalyzer) handleTypeAssertion(ins *ssa.TypeAssert) (LatticeElement, LatticeElement) {
	a.logger.Debugw("handle_type_assertion",
		zap.String("instruction", ins.String()))
	le := a.getLatticeForSSAValue(ins.X)

	if len(le.vtPair) != 1 {
		a.logger.Debugw("handle_type_assertion_bottom",
			zap.String("instruction", ins.String()),
		)
		a.metrics[_typeAssertionInfeasible]++
		// infeasible, return bottom.
		return createTypedLatticeBottom(ins.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
	}

	vt := le.vtPair[0]

	switch vt.Value.(type) {
	case ConstantVal:
		constVal := vt.Value.(ConstantVal)
		if constVal.Value.IsNil() { // If the constant is nil, we can check if it is an interface.
			if types.IsInterface(constVal.Value.Type()) {
				// a nil interface cannot be converted to anything.
				return createSingletonLatticeElement(ConstantVal{Value: *ssa.NewConst(nil, ins.AssertedType)}, ConcreteType{T: ins.AssertedType}), createConstantFalseLatticeElement()
			} // else must check
		} // else  must check
		return doTypeCheck(constVal.Value.Type(), constVal, ins)
	case FunctionVal:
		funcVal := vt.Value.(FunctionVal)
		return doTypeCheck(funcVal.fn.Type(), funcVal, ins)
	case TopVal:
		switch vt.Type.(type) {
		case ConcreteType:
			concreteType := vt.Type.(ConcreteType)
			if types.IsInterface(concreteType.T) {
				// Bottom, Bottom : conservative; play safe.
				return createTypedLatticeBottom(ins.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
			}
			return doTypeCheck(concreteType.T, TopVal{}, ins)
		case TopType:
			// Top, Top
			return createTopLatticeElement(ins.AssertedType), createTopLatticeElement(types.Typ[types.Bool])
		case BottomType:
			// Bottom, Bottom : unlikely mismatch, play safe.
			return createTypedLatticeBottom(ins.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
		default:
			panic("NYI")
		}
	case BottomVal:
		switch vt.Type.(type) {
		case ConcreteType: // some code duplication here, but it is better to flow diagram clean.
			concreteType := vt.Type.(ConcreteType)
			if types.IsInterface(concreteType.T) {
				// Bottom, Bottom : conservative; play safe.
				return createTypedLatticeBottom(ins.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
			}
			return doTypeCheck(concreteType.T, BottomVal{}, ins)
		case TopType:
			// Bottom, Bottom : unlikely mismatch, play safe.
			return createTypedLatticeBottom(ins.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
		case BottomType:
			// Bottom, Bottom : conservative; play safe.
			return createTypedLatticeBottom(ins.AssertedType), createTypedLatticeBottom(types.Typ[types.Bool])
		default:
			panic("NYI")
		}
	default:
		panic("NYI")
	}
}

// doTypeCheck models the type check for a type assertion operation.
// It checks if the type t can be converted to ins.AssertedType.
// If it can, it returns a LatticeElement{v, assertedtype}, LatticeElelment{true, bool}
// If it cannot, it returns a LatticeElement{nilConstofassertedtype, assertedtype}, LatticeElelment{false, bool}
func doTypeCheck(t types.Type, v ValueLattice, ins *ssa.TypeAssert) (LatticeElement, LatticeElement) {
	// Check if t can be converted to ins.AssertedType.
	// 2 cases exist:
	// 1. It is assignable, in that case return a LatticeElement{v, assertedtype}, LatticeElelment{true, bool}
	// 2. It is not assignable, in that case 2 cases exist:
	// 2.1. It is a nillable type, return a LatticeElement{nilConstofassertedtype, assertedtype}, LatticeElelment{false, bool}
	// 2.2. It is not nillable, in that case return a LatticeElement{defaultOfassertedtype, assertedtype}, LatticeElelment{false, bool}

	asserted := ins.AssertedType

	// If a value of type t is assignable to the asserted type, the assertion can succeed.
	// This covers both concrete -> interface (implements) and interface -> interface cases,
	// and identical concrete types.
	if types.AssignableTo(t, asserted) {
		// Successful assertion: same value, but now with the asserted static type.
		return createSingletonLatticeElement(v, ConcreteType{T: asserted}), createConstantTrueLatticeElement()
	}

	// Failed assertion: return the typed zero of the asserted type and 'false'.
	// For nillable types (ptr/map/chan/func/slice/interface), this zero is nil.
	return createSingletonLatticeElement(ConstantVal{Value: *ssa.NewConst(nil, asserted)}, ConcreteType{T: asserted}), createConstantFalseLatticeElement()

}

// isZero checks if a constant value is zero.
// This is used to check if a constant value is zero.
// An integer constant value is zero if its uint64 value is 0.
// A float constant value is zero if its float64 value is 0.0.
// A complex constant value is zero if its real and imaginary parts are both 0.0.
func isZero(v constant.Value) bool {
	switch v.Kind() {
	case constant.Int:
		if val, ok := constant.Uint64Val(v); ok {
			return val == 0
		}
		return false
	case constant.Float:
		if val, ok := constant.Float64Val(v); ok {
			return val == 0.0
		}
		return false
	case constant.Complex:
		if real, ok := constant.Float64Val(constant.Real(v)); ok {
			if img, ok := constant.Float64Val(constant.Imag(v)); ok {
				return real == 0.0 && img == 0.0
			}
		}
		return false
	}
	return false
}

// getLatticeForSSAValue looks up the lattice for an SSA value.
// If the value is not in the valueState map, it returns top.
func (a *SCCPAnalyzer) getLatticeForSSAValue(v ssa.Value) LatticeElement {
	// For instruction results, look up the computed value.
	if val, ok := a.valueState[v]; ok {
		return val
	}
	// Should not be reached for a properly initialized analyzer.
	return createTopLatticeElement(v.Type())
}

// setLatticeInitialValue sets initial values for SSA values if not already set.
func (a *SCCPAnalyzer) setLatticeInitialValue(v ssa.Value) {
	if _, ok := a.valueState[v]; ok {
		return
	}
	// Known constants
	switch c := v.(type) {
	case *ssa.Const:
		a.valueState[v] = createSingletonLatticeElement(ConstantVal{Value: *c}, ConcreteType{T: c.Type()})
	case *ssa.Function:
		a.valueState[v] = createSingletonLatticeElement(FunctionVal{fn: c}, ConcreteType{T: c.Type()})
	default:
		if v == nil {
			a.logger.Debugw("setLatticeInitialValue",
				zap.String("value", fmt.Sprintf("%v", v)),
			)
			a.metrics[_nilOperand]++
			// Nil ssa.value will bottom the value state.
			// This is a conservative approach.
			// While we don't know when this exactly occurs, we have seen times
			// when the instr.Operands(nil) returns a a slice with a nil operand.
			// When the operand type is unknown and value is unknown, we bottom the value state.
			a.valueState[v] = createBottomLatticeElement()
		} else {
			a.valueState[v] = createTopLatticeElement(v.Type())
		}
		// TODO: handle Global variables that are not modified after initialization.
	}
}

// initializeLatticeForFunction initializes the lattice for a function.
func (a *SCCPAnalyzer) initializeLatticeForFunction(f *ssa.Function) {
	// Set the initial values for the parameters.
	for _, param := range f.Params {
		a.setLatticeInitialValue(param)
	}
	// Set the initial values for the free variables.
	for _, freeVar := range f.FreeVars {
		a.setLatticeInitialValue(freeVar)
	}
	// Set the initial values for all operands and instruction results.
	for _, block := range f.Blocks {
		for _, instr := range block.Instrs {
			// Skip debug instructions.
			if _, ok := instr.(*ssa.DebugRef); ok {
				continue
			}
			// Set the initial values for all operands.
			for _, op := range instr.Operands(nil) {
				if op == nil || *op == nil {
					a.logger.Debugw("setLatticeInitialValue",
						zap.String("instruction", instr.String()),
						zap.String("operand", fmt.Sprintf("%v", op)),
					)
				}
				a.setLatticeInitialValue(*op)
			}
			// Set the initial values for the instruction result.
			if v, ok := instr.(ssa.Value); ok {
				a.setLatticeInitialValue(v)
			}
		}
	}
}

// meetParamsWithArg meets the parameters of a function with the argument.
// It is used to initialize the lattice for the parameters of a function.
func (a *SCCPAnalyzer) meetParamsWithArg(params []*ssa.Parameter, args map[ssa.Value]LatticeElement) {
	for _, val := range params {
		if l, ok := args[val]; ok {
			a.valueState[val] = meet(a.valueState[val], l, a.logger)
		} else {
			// If not in bindings, set to bottom
			a.valueState[val] = createTypedLatticeBottom(val.Type())
		}
	}
}

// meetFreeVarsWithBindings meets the free variables of a function with the bindings.
// It is used to initialize the lattice for the free variables of a function.
func (a *SCCPAnalyzer) meetFreeVarsWithBindings(
	freeVars []*ssa.FreeVar,
	bindings map[ssa.Value]LatticeElement,
) {
	// TODO: This function is always going to set the free vars to bottom because the bindings are memory states and
	// we don't model memory state yet.
	for _, val := range freeVars {
		if l, ok := bindings[val]; ok {
			a.valueState[val] = meet(a.valueState[val], l, a.logger)
		} else {
			// If not in bindings, set to bottom
			a.valueState[val] = createTypedLatticeBottom(val.Type())
		}
	}
}

// initializeReachableBlocks initializes the reachable blocks for the function.
// Entry and recover blocks are always reachable.
func (a *SCCPAnalyzer) initializeReachableBlocks(f *ssa.Function) {
	// The entry block is always reachable.
	if len(f.Blocks) > 0 {
		entry := f.Blocks[0]
		a.reachableBlocks[entry] = struct{}{}
		a.cfgWorklist = append(a.cfgWorklist, cfgWorkItem{blk: entry, visitPhisOnly: false})
	}

	// The recover block is always reachable.
	if f.Recover != nil {
		a.reachableBlocks[f.Recover] = struct{}{}
		a.cfgWorklist = append(a.cfgWorklist, cfgWorkItem{blk: f.Recover, visitPhisOnly: false})
	}
}

// isBlockReachable checks if a block is reachable.
func (a *SCCPAnalyzer) isBlockReachable(b *ssa.BasicBlock) bool {
	_, ok := a.reachableBlocks[b]
	return ok
}

// getDefaultBinding sets the default bindings for a function.
// It is used to initialize the lattice for the parameters and free variables for the first function in the call path for which we don't have a binding.
func (a *SCCPAnalyzer) getDefaultBinding(f *ssa.Function) map[ssa.Value]LatticeElement {
	bindings := make(map[ssa.Value]LatticeElement)
	// Set each param to its bottom value and type.
	for _, param := range f.Params {
		bindings[param] = createTypedLatticeBottom(param.Type())
	}

	// Set each free var to its bottom value and type.
	for _, freeVar := range f.FreeVars {
		bindings[freeVar] = createTypedLatticeBottom(freeVar.Type())
	}
	return bindings
}

// Analyze performs the SCCP analysis.
// It populates the ReachableBlocks map and the valueState map.
// By itself it does not return anything.
// The user of the analyzer should check the reachableBlocks map to check if the leaf callsite is reachable by calling
// isBlockReachable(leafCallsite.Block()).
// The reason for keeping it this way is to avoid having to run the analyzer again for the same call chain where we already have the reachableBlocks map populated.
// This optimizes the case when multiple callsites are under the same leaf function for a given call chain.
func (a *SCCPAnalyzer) Analyze() {
	if len(a.callPath) == 0 {
		return
	}

	// Set the default bindings for the first function in the call path.
	envForNextCallee := a.getDefaultBinding(a.callPath[0].Caller.Func)

	for i, link := range a.callPath {
		// TODO: if this is a don't prune edge, skip the analysis and mark the caller's first and the Site blocks are reachable.
		a.logger.Debugw("Analyzing chain link",
			zap.Int("index", i),
			zap.String("caller", link.Caller.Func.String()),
		)

		// dump function ssa for debugging.
		if a.logger.Level() == zap.DebugLevel {
			buf := bytes.NewBuffer(nil)
			link.Caller.Func.WriteTo(buf)
			fmt.Printf("Analyzing caller %s\n", link.Caller.Func.String())
			fmt.Printf("Function SSA:\n%s\n", buf.String())
		}

		// Heavy lifting here.
		a.analyzeFunction(link.Caller.Func, envForNextCallee)
		// Dump the reachable blocks for debugging.
		for _, b := range link.Caller.Func.Blocks {
			if _, ok := a.reachableBlocks[b]; ok {
				a.logger.Debugw("reachable block",
					zap.String("block", b.String()))
			} else {
				a.logger.Debugw("unreachable block",
					zap.String("block", b.String()))
			}
		}

		// The callee function of the leaf Edge is nil.
		if link.Callee == nil {
			// This happens only for the last link in the call path.
			// Since there is nothing to prepare for the next callee, we can skip the rest of the analysis.
			continue // break would have worked too.
		}

		// If this is a dontPruneEdge, retain it and mark the Site block reachable.
		// A dontPruneEdge is a special edge that is used to retain edges that are not reachable from the entry block.
		if _, ok := a.dontPruneEdges[link]; ok {
			a.reachableBlocks[link.Site.Block()] = struct{}{}
			a.logger.Debugw("OK: Retained dontPruneEdge",
				zap.String("dontPruneEdge", link.String()))
			continue
		}

		// We reach here only for non-leaf links.
		callsiteBlock := link.Site.Block()
		if !a.isBlockReachable(callsiteBlock) {
			// We reached an unreachable callsite.
			// Since the callsite is unreachable and the path expects a valid callee,
			// the path is infeasible.
			if link.Callee == nil {
				a.logger.Infow("infeasible_call_in_unreachable_blk_no_callee",
					zap.String("callSite", link.Site.Common().String()),
					zap.String("callBlock", callsiteBlock.String()))
			} else {
				a.logger.Infow("infeasible_call_in_unreachable_blk",
					zap.String("function", link.Caller.Func.String()),
					zap.String("callSite", link.Site.Common().String()),
					zap.String("callBlock", callsiteBlock.String()),
					zap.String("expectedCallee", link.Callee.Func.String()),
				)
			}
			// Since the call site is unreachable, we can skip the rest of the analysis for this call path.
			// The caller is still responsible to check if the desired set of callsites (corresponding blocks) were marked reachable.
			return
		}
		a.logger.Debugw("OK: Call site is reachable",
			zap.String("callSite", link.Site.Common().String()))

		// The callsite is reachable by flow analysis. We need to determine whether the callee based on the lattice values resolved to the expected callee.
		// This is a conservative analysis.

		// Based on the lattice values check which actual function the callsite is calling (if possible).
		// getResolvedCallee conservatively returns the default callee if it cannot resolve the callsite.
		_, _, err := a.getResolvedCallee(link)
		// getResolvedCallee results in an error if the actual callee is guaranteed to be not the expected callee.
		if err != nil {
			a.logger.Debugw("infeasible_call_in_reachable_blk",
				zap.String("function", link.Caller.Func.String()),
				zap.String("callSite", link.Site.Common().String()),
				zap.Error(err),
			)
			// We found out that the actual callee is not the expected callee.
			// This infeasibility implies, the chain is infeasible.
			return
		}
		// We reach here when we conservatively resolved the callee to the expected callee.
		// We now need to prepare the environment for the next callee.

		// Get the necessary bindings for the next callee.
		envForNextCallee = a.setupEnvForCallee(link)
	}

	return
}

// getBindingForStaticCallee gets the bindings for a ssa.Function or a ssa.MakeClosure.
func (a *SCCPAnalyzer) getBindingForStaticCallee(c *ssa.CallCommon) (*ssa.Function, []ssa.Value, error) {
	switch val := c.Value.(type) {
	case *ssa.Function:
		// no binding.
		return val, nil, nil
	case *ssa.MakeClosure:
		// this is guaranteed to be a *ssa.Function.
		return val.Fn.(*ssa.Function), val.Bindings, nil
	default:
		return nil, nil, fmt.Errorf("not a static callee")
	}
}

// getResolvedCallee finds which actual ssa.Function the callsite resolves to based on the lattice state.
// It is used to find the actual callee for a callsite.
// If the resolved callee is guaranteed to be not the same as the expected callee, it returns an error.
// When not sure, it returns the callee provided in the link.
//
// Decision Flow:
//
//	┌─────────────────────────────────────────┐
//	│ getResolvedCallee(link)                 │
//	│ Resolve actual callee from callsite     │
//	└─────────────────┬───────────────────────┘
//	                  │
//	                  ▼
//	        ┌─────────────────────┐
//	        │ link.Callee.Func    │
//	        │ or link.Site nil?   │
//	        └─────┬───────────┬───┘
//	              │ Yes       │ No
//	              ▼           ▼
//	      ┌───────────────┐ ┌────────────────--─┐
//	      │ Return Error  │ │ Set defaultCallee │
//	      └───────────────┘ │ = link.Callee.Func│
//	                        └─────────┬──────--─┘
//	                                  │
//	                                  ▼
//	                        ┌─────────────────┐
//	                        │ Is Static Call? │
//	                        └─────┬───────┬───┘
//	                              │ Yes   │ No
//	                              ▼       ▼
//	                      ┌─────────────┐ ┌─────────────────┐
//	                      │Return       │ │ Check Method    │
//	                      │defaultCallee│ │ Invoke?         │
//	                      └─────────────┘ └─────┬───────┬───┘
//	                                            │ Yes   │ No
//	                                            ▼       ▼
//	                                    ┌─────────────┐ ┌─────────────┐
//	                                    │Get Receiver │ │ Indirect    │
//	                                    │Lattice Type │ │ Call        │
//	                                    └──────┬──────┘ └──────┬──────┘
//	                                           │               │
//	                                           ▼               ▼
//	                                  ┌────────────────┐ ┌─────────────┐
//	                                  │ Concrete Type? │ │Get Function │
//	                                  │ Not Interface? │ │Lattice Value│
//	                                  └────┬───────┬───┘ └──────┬──────┘
//	                                       │ Yes   │ No         │
//	                                       ▼       ▼            ▼
//	                              ┌─────────────┐ ┌───────┐ ┌─────────────┐
//	                              │Lookup Method│ │Return │ │ FunctionVal?│
//	                              │in Type      │ │default│ └──────┬──────┘
//	                              └──────┬──────┘ │Callee │        │ Yes
//	                                     │        └───────┘        ▼
//	                                     ▼                ┌───────────------──┐
//	                            ┌────────────────┐        │Switch on          │
//	                            │method ==       │        │fc.fn type         │
//	                            │link.Callee.Func│        └─────┬──------─┬───┘
//	                            └────┬───────┬───┘              │         │
//	                                 │ No    │ Yes              │         │
//	                                 ▼       ▼                  ▼         ▼
//	                         ┌─────────────┐ ┌─────────────┐ ┌────────┐ ┌────────────┐
//	                         │Return Error │ │Return       │ │*ssa.   │ │*ssa.Make   │
//	                         │(infeasible) │ │defaultCallee│ │Function│ │Closure     │
//	                         └─────────────┘ └─────────────┘ └───┬────┘ └─────┬──────┘
//	                                                             │            │
//	                                                             ▼            ▼
//	                                                    ┌─────────────┐ ┌─────────────┐
//	                                                    │fn ==        │ │fn.Fn ==     │
//	                                                    │link.Callee  │ │link.Callee  │
//	                                                    │.Func?       │ │.Func?       │
//	                                                    └─┬─────────┬─┘ └─┬─────────┬─┘
//	                                                      │ Yes     │ No  │ Yes     │ No
//	                                                      ▼         ▼     ▼         ▼
//	                                                ┌──────────┐ ┌─────┐ ┌─────────┐ ┌─────┐
//	                                                │Return fn │ │Error│ │Return fn│ │Error│
//	                                                └──────────┘ └─────┘ │+bindings│ └─────┘
//	                                                                     └─────────┘
func (a *SCCPAnalyzer) getResolvedCallee(link *callgraph.Edge) (*ssa.Function, []ssa.Value, error) {
	if link.Callee.Func == nil || link.Site == nil {
		// We should never call this with nil callee.
		return nil, nil, fmt.Errorf("callee is nil")
	}

	// This is the default callee if we cannot resolve the callsite.
	defaultCallee := link.Callee.Func

	common := link.Site.Common()
	// If the callsite is a static call, we can resolve the callee.
	if _, bindings, err := a.getBindingForStaticCallee(common); err == nil {
		// this is guaranteed to provide a valid ssa.Function, we never return an error.
		return defaultCallee, bindings, nil
	}

	// Is this a method invoke?
	if common.IsInvoke() {
		// Get the lattice for the receiver.
		_, recvLatticeType := a.getLatticeForSSAValue(common.Value).getSingletonLatticeElement()
		// If we have a concrete (and non-interface) receiver, we can resolve the method.
		if ct, ok := recvLatticeType.(ConcreteType); ok {
			// ct must be a concrete type. If the receiver is an interface, we cannot resolve the method.
			if !types.IsInterface(ct.T) {
				// Lookup the method in the concrete type.
				method := a.prog.LookupMethod(ct.T, common.Method.Pkg(), common.Method.Name())
				if method != link.Callee.Func {
					// We found out that the actual callee is not the expected callee.
					// This infeasibility implies, the chain is infeasible.
					return nil, nil, fmt.Errorf("method %s not found in resolved concrete type %s: expected %v found %v", common.Method.Name(), ct.T.String(), link.Callee.Func.String(), method)
				}
			}
		}
		// Unresolved. We don't know which function the callsite is calling.
		return defaultCallee, nil, nil
	}

	// This is an indirect call.
	funcLatticeValue, _ := a.getLatticeForSSAValue(common.Value).getSingletonLatticeElement()
	a.logger.Debugw("getResolvedCallee",
		"indirect_call", true,
		"common.Value", common.Value.String(),
		"funcLattice", funcLatticeValue.String(),
	)

	// If the lattice value is a FunctionVal, we can resolve the callee.
	if fc, ok := funcLatticeValue.(FunctionVal); ok {
		a.logger.Debugw("getResolvedCallee", "FunctionVal", true)
		switch fn := fc.fn.(type) {
		case *ssa.Function:
			a.logger.Debugw("getResolvedCallee",
				"*ssa.Function", true,
				"fn", fn.String())
			if fn == link.Callee.Func {
				return fn, nil, nil
			}
			return nil, nil, fmt.Errorf("actual function %s does not match expected function %s", fn.String(), link.Callee.Func.String())
		case *ssa.MakeClosure:
			// this is guaranteed to be a *ssa.Function.
			a.logger.Debugw("getResolvedCallee",
				"*ssa.MakeClosure", true,
				"fn", fn.Fn.(*ssa.Function).String(),
			)
			if fn.Fn.(*ssa.Function) == link.Callee.Func {
				return fn.Fn.(*ssa.Function), fn.Bindings, nil
			}
			return defaultCallee, nil, fmt.Errorf("function %s does not match expected function %s", fn.Fn.(*ssa.Function).String(), link.Callee.Func.String())
		default:
			// This should never happen.
			panic("must never happen")
		}
	}
	// If the lattice value is not a FunctionVal, we cannot resolve the callee.
	a.logger.Debugw("getResolvedCallee", "FunctionVal", false)
	// Unresolved. We don't know which function the callsite is calling.
	return defaultCallee, nil, nil
}

// setupEnvForCallee creates the environment for the next callee.
// It maps the lattice values of the arguments to the parameters of the next callee.
// It also binds the values of the free variables of the next callee.
func (a *SCCPAnalyzer) setupEnvForCallee(link *callgraph.Edge) map[ssa.Value]LatticeElement {
	envForNextCallee := make(map[ssa.Value]LatticeElement)
	// Map the lattice values of the arguments to the parameters of the next callee.
	a.setupLatticeValuesForParams(link, envForNextCallee)
	// Map the free variables of the next callee.
	a.setupLatticeValuesFreeVars(link, envForNextCallee)
	return envForNextCallee
}

// setupLatticeValuesForParams maps the lattice values of the arguments to the parameters of the next callee.
// It also binds the values of the free variables of the next callee.
func (a *SCCPAnalyzer) setupLatticeValuesForParams(link *callgraph.Edge, envForNextCallee map[ssa.Value]LatticeElement) {
	common := link.Site.Common()
	expectedCallee := link.Callee.Func
	// If this is a method invoke, we need to map the lattice of the receiver.
	if (common.StaticCallee() == nil) && common.IsInvoke() {
		// Get the lattice for the receiver.
		recvSeenAtCallsite := common.Value
		recvLattice := a.getLatticeForSSAValue(recvSeenAtCallsite)
		actualReceiver := expectedCallee.Params[0]
		// We are checking if the type the receiver at the signature (which is the concrete type) is assignable to the type of the receiver at the callsite.
		// The receiver used at the callsite is more generic (i.e. interface).
		// Hence the direction of check is from the receiver of the method signature to the receiver at the callsite.
		// If not, we bind all params to bottom.
		if !types.AssignableTo(actualReceiver.Type(), recvSeenAtCallsite.Type()) {
			// This can happen some times when we override the edge with synthetic edges.
			// We simply bind all params to bottom.
			a.logger.Debugw("infeasible_call_due_to_type_mismatch",
				zap.String("callSite", link.Site.Common().String()),
				zap.String("actualReceiver", actualReceiver.Type().String()),
				zap.String("recvSeenAtCallsite", recvSeenAtCallsite.Type().String()),
			)
			for _, param := range expectedCallee.Params {
				envForNextCallee[param] = createTypedLatticeBottom(param.Type())
			}
		} else {
			// Map the lattice of the arguments to the parameters of the next callee.
			err := a.assignParamsToArgsElseDefault(common.Args, expectedCallee.Params[1:], envForNextCallee)
			if err != nil {
				// Map the receiver to bottom.
				envForNextCallee[actualReceiver] = createTypedLatticeBottom(actualReceiver.Type())
				a.logger.Debugw("infeasible_call_due_mismatch",
					zap.String("callSite", link.Site.Common().String()), zap.Error(err),
				)
			} else {
				// Map the lattice of the receiver to the parameter of the next callee.
				envForNextCallee[actualReceiver] = recvLattice
				a.logger.Debugw("binding_recv",
					zap.String("actualReceiver", actualReceiver.Name()),
					zap.String("recvLattice", recvLattice.String()))
			}
		}
	} else {
		// Map the lattice of the arguments to the parameters of the next callee.
		_ = a.assignParamsToArgsElseDefault(common.Args, expectedCallee.Params, envForNextCallee)
	}

}

// setupLatticeValuesFreeVars maps the lattice values of the free variables of the next callee.
// It is used to initialize the lattice for the free variables of the next callee.
func (a *SCCPAnalyzer) setupLatticeValuesFreeVars(link *callgraph.Edge, newBindings map[ssa.Value]LatticeElement) {
	// binding is possible only if the call.Common().Value's entry in the valueState is a FunctionVal.
	funcLatticeValue, _ := a.getLatticeForSSAValue(link.Site.Common().Value).getSingletonLatticeElement()
	if fc, ok := funcLatticeValue.(FunctionVal); ok {
		switch fn := fc.fn.(type) {
		case *ssa.MakeClosure:
			// Check if the number of free vars matches the number of bindings.
			if len(link.Callee.Func.FreeVars) != len(fn.Bindings) {
				a.logger.Debugw("bindings_free_var_number_mismatch",
					zap.Int("numFreeVars", len(link.Callee.Func.FreeVars)),
					zap.Int("numBindings", len(fn.Bindings)),
				)
				return // leave the binding to bottom.
			}
			for idx, freeVar := range link.Callee.Func.FreeVars {
				b := a.getLatticeForSSAValue(fn.Bindings[idx])
				a.logger.Debugw("binding_free_var",
					zap.String("freeVar", freeVar.String()),
					zap.String("binding", b.String()))
				newBindings[freeVar] = b
			}
		default:
			// not a closure, leave the binding to bottom.
			return
		}
	} else {
		// not a function, leave the binding to bottom.
		return
	}
}

// assignParamsToArgsElseDefault maps the lattice values of the arguments to the parameters of the next callee.
func (a *SCCPAnalyzer) assignParamsToArgsElseDefault(args []ssa.Value, params []*ssa.Parameter, bindings map[ssa.Value]LatticeElement) error {
	var err error
	defer func() {
		if err != nil {
			// On error, bind all params to bottom.
			for _, param := range params {
				a.logger.Debugw("binding_param_default",
					zap.String("param", param.Name()),
					zap.String("type", param.Type().String()),
				)
				bindings[param] = createTypedLatticeBottom(param.Type())
			}
		}
	}()

	// If the number of args and params don't match, skip the bindings.
	if len(args) != len(params) {
		a.logger.Debugw("infeasible_call_due_to_number_of_args_mismatch",
			zap.Int("numArgs", len(args)),
			zap.Int("numParams", len(params)),
		)
		err = fmt.Errorf("number of args %d does not match number of params %d", len(args), len(params))
		return err
	}

	// Check if the types match
	for i, arg := range args {
		if !types.AssignableTo(arg.Type(), params[i].Type()) {
			a.logger.Debugw("infeasible_call_due_to_type_mismatch",
				zap.String("argType", arg.Type().String()),
				zap.String("paramType", params[i].Type().String()),
			)
			err = fmt.Errorf("type of arg %d (%s) does not match type of param %d (%s)", i, arg.Type().String(), i, params[i].Type().String())
			return err
		}
	}

	// Perform the bindings.
	for i, param := range params {
		bindings[param] = a.getLatticeForSSAValue(args[i])
		a.logger.Debugw("binding_param",
			zap.String("param", param.Name()),
			zap.String("arg", args[i].String()),
			zap.String("val", a.getLatticeForSSAValue(args[i]).String()),
		)
	}

	// If we get here, the bindings are valid.
	return nil
}
