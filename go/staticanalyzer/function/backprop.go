package function

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/staticanalyzer/asthelper"
	"go.uber.org/zap/zapcore"
	"golang.org/x/tools/go/cfg"
)

// ErrorReturnMetadata contains the metadata related to an error return statement in a function.
type ErrorReturnMetadata struct {
	ReturnStmt *ast.ReturnStmt
	// Error object associated with the return statement. This could also be an inferred error object such as in the
	// case of `if err != nil { return nil, err2 }, where ErrorObj is `err` and OriginalErrorObj is `err2`.
	ErrorObj         types.Object
	OriginalErrorObj types.Object // Original error object associated with the return statement.
	Value            constant.ErrorValue
	isValueAssigned  bool // Indicates if the error value is determined from an explicit assignment, e.g., `err = nil`.
}

func (e *ErrorReturnMetadata) copy() *ErrorReturnMetadata {
	return &ErrorReturnMetadata{
		ReturnStmt:       e.ReturnStmt,
		ErrorObj:         e.ErrorObj,
		OriginalErrorObj: e.OriginalErrorObj,
		Value:            e.Value,
		isValueAssigned:  e.isValueAssigned,
	}
}

// blockConditionTuple defines a tuple for storing the error variable used in the conditional check and its nilability information.
type blockConditionTuple struct {
	errVar *types.Var
	value  constant.ErrorValue
	// Stores nodes in the basic block that have a direct influence on the inferred value of the error variable.
	// Nodes after reassignment of the error variable are not stored here, since the conditional no longer influences
	// the error variable thereafter.
	nodesWithInferredValue map[ast.Node]bool
	isAssigned             bool
}

func (b *blockConditionTuple) copy() *blockConditionTuple {
	return &blockConditionTuple{
		errVar:                 b.errVar,
		value:                  b.value,
		nodesWithInferredValue: b.nodesWithInferredValue,
	}
}

// funcContext contains the contextual information for a function required for backpropagation analysis.
type funcContext struct {
	*funcAnalyzer
	blockNilabilityInfo map[int32]*blockConditionTuple
	allErrorReturnData  map[types.Object][]*ErrorReturnMetadata
	calleeReturnData    []*ErrorReturnMetadata
}

func newFuncContext(analyzer *funcAnalyzer) *funcContext {
	return &funcContext{
		funcAnalyzer:        analyzer,
		blockNilabilityInfo: make(map[int32]*blockConditionTuple),
		allErrorReturnData:  make(map[types.Object][]*ErrorReturnMetadata),
	}
}

// copy creates a deep copy of the function context.
func (f *funcContext) copy() *funcContext {
	newCtx := newFuncContext(f.funcAnalyzer)
	for k, v := range f.blockNilabilityInfo {
		newCtx.blockNilabilityInfo[k] = v.copy()
	}
	for k, v := range f.allErrorReturnData {
		newCtx.allErrorReturnData[k] = make([]*ErrorReturnMetadata, len(v))
		for i, e := range v {
			newCtx.allErrorReturnData[k][i] = e.copy()
		}
	}
	newCtx.calleeReturnData = make([]*ErrorReturnMetadata, len(f.calleeReturnData))
	for i, e := range f.calleeReturnData {
		newCtx.calleeReturnData[i] = e.copy()
	}
	return newCtx
}

// mergeFuncContextsInto merges the function context from the right branch into the left branch.
func (f *funcContext) mergeFuncContextsInto(leftCtx, rightCtx *funcContext) {
	mergeObjErrorData := func(left, right []*ErrorReturnMetadata) []*ErrorReturnMetadata {
		// Case 1: Error return on object `obj` in left is not present in right: Return the entries from left as is.
		if len(right) == 0 {
			return left
		}

		// Case 2: Error return on object `obj` in right is not present in left: Return the entries from right as is.
		if len(left) == 0 {
			return right
		}

		// Case 3: Error return on object `obj` is present in left and right: Merge the entries from right to left.
		leftMap := make(map[*ast.ReturnStmt]*ErrorReturnMetadata, len(left))
		for _, d1 := range left {
			leftMap[d1.ReturnStmt] = d1
		}
		for _, d2 := range right {
			if existing, found := leftMap[d2.ReturnStmt]; found {
				// Return statements are the same. Merge the error values.
				existing.Value |= d2.Value
			} else {
				// ErrorObj is the same, but the return statements are different. This is the case of multiple return
				// statements for the same error variable. In this case, we add `right`'s entry to `left`.
				left = append(left, d2)
				leftMap[d2.ReturnStmt] = d2
			}
		}

		return left
	}

	// Collect all error objects from both the branches.
	allObjs := make(map[types.Object]bool)
	for k := range leftCtx.allErrorReturnData {
		allObjs[k] = true
	}
	for k := range rightCtx.allErrorReturnData {
		if _, ok := allObjs[k]; !ok {
			allObjs[k] = true
		}
	}

	// Merge the function context for each object in `allObjs` and `calleeReturnData`
	for obj := range allObjs {
		leftCtx.allErrorReturnData[obj] = mergeObjErrorData(leftCtx.allErrorReturnData[obj], rightCtx.allErrorReturnData[obj])
	}
	leftCtx.calleeReturnData = mergeObjErrorData(leftCtx.calleeReturnData, rightCtx.calleeReturnData)
}

// processConditionalSuppression parses the CFG to determine if the function contains conditional suppression of
// fail-close behavior. For example, flipr uses the suppression function `IsDependencyFailureError()`:
// ```
//
//	func foo() (*S, error) {
//		err := RPC()
//		if err != nil {
//			if entity.IsDependencyFailureError(err) {
//				return &S{}, nil
//			}
//			return nil, err
//		}
//		return &S{}, nil
//	}
//
// ```
// If such a conditional suppression is detected, we remove the false branch from the CFG to avoid false positives.
// This is safe to do because the false branch is unreachable at runtime in the presence of the suppression function.
func (f *funcAnalyzer) processConditionalSuppression(graph *cfg.CFG) {
	for _, b := range graph.Blocks {
		if !b.Live || len(b.Nodes) == 0 {
			continue
		}
		// Check if the block is a conditional block
		if len(b.Succs) != 2 {
			continue
		}
		// Check if:
		// (1) the true branch (i.e., b.Succs[0]) ends with a return statement
		// (2) the condition expr is a call to a trusted suppression function
		if expr, ok := b.Nodes[len(b.Nodes)-1].(ast.Expr); ok && len(b.Succs[0].Nodes) > 0 {
			if _, ok := b.Succs[0].Nodes[len(b.Succs[0].Nodes)-1].(*ast.ReturnStmt); ok {
				if f.astContext.IsModeledForSuppression(expr) {
					// Remove the false branch, i.e., point b's successor to the exit block
					b.Succs[1] = graph.Blocks[len(graph.Blocks)-1]
				}
			}
		}
	}
}

// backpropAcrossFunc performs backpropagation analysis across the function to determine the fail-ness status of the function.
func (f *funcAnalyzer) backpropAcrossFunc() []*ErrorReturnMetadata {
	// Build the control flow graph (CFG) for the function
	var caller ast.Node
	if lit, ok := f.astContext.IdentFuncLitMap[f.caller.Name]; ok {
		caller = lit
	} else {
		caller = f.caller
	}

	graph := BuildAndTransformCFG(f.astContext, caller)
	f.processConditionalSuppression(graph)
	// Compute the postorder of the depth first search tree (DFST) of the live blocks of the CFG
	postOrder := computePostOrder(graph.Blocks)
	// Process conditionals to get nilability information related to error returns
	blockNilabilityInfo := f.getBlockNilabilityInfo(graph, postOrder)
	f.logger.Debugw("Block nilability information", "info", blockNilabilityInfo)

	// Create a map to store the function context for each block in the CFG.
	funcCtxByBlock := make(map[int32]*funcContext)

	// Initialize the function context for each block in the CFG.
	for _, b := range graph.Blocks {
		emptyFuncCtx := newFuncContext(f)
		emptyFuncCtx.allErrorReturnData = make(map[types.Object][]*ErrorReturnMetadata)
		emptyFuncCtx.blockNilabilityInfo = blockNilabilityInfo
		funcCtxByBlock[b.Index] = emptyFuncCtx
	}

	// Backpropagate the error return metadata across the function blocks
	for _, i := range postOrder {
		f.logger.Debugw("Processing block", "block", i, "num_succs", len(graph.Blocks[i].Succs))
		block := graph.Blocks[i]

		// There is no need to process non-live blocks.
		if !block.Live {
			continue
		}

		if len(block.Succs) > 2 {
			panic("assumptions about CFG shape violated - a block has >2 successors")
		}

		// Before doing actual back propagation for the current block, we need to first prepare its function context
		// based on its successors:
		// (1) create a copy of the function context from each successor,
		// (2) merge if there are two successors.
		succs := make([]*funcContext, 0, len(block.Succs))
		for _, s := range block.Succs {
			succCtx := funcCtxByBlock[s.Index].copy()
			succs = append(succs, succCtx)
		}

		// Merge the function contexts of the successors
		if len(succs) == 2 {
			succs[0].mergeFuncContextsInto(succs[0], succs[1])
		}

		// Store the function context for the current block
		if len(succs) > 0 {
			funcCtxByBlock[i] = succs[0]
		}

		// Iterate over all block nodes in _reverse_ order
		for j := len(block.Nodes) - 1; j >= 0; j-- {
			node := block.Nodes[j]

			switch n := node.(type) {
			case *ast.ReturnStmt:
				if f.logger.Level() == zapcore.DebugLevel {
					nodeStr, err := asthelper.FormatExpr(node, f.astContext.PkgInfo.Pkg.Fset)
					if err != nil {
						nodeStr = fmt.Sprintf("error formatting node: %v", err)
					}
					f.logger.Debugw("Processing ReturnStmt node", "node", nodeStr, "block", i)
				}
				backpropAcrossReturn(funcCtxByBlock[i], n, i)

			case *ast.AssignStmt:
				if f.logger.Level() == zapcore.DebugLevel {
					nodeStr, err := asthelper.FormatExpr(node, f.astContext.PkgInfo.Pkg.Fset)
					if err != nil {
						nodeStr = fmt.Sprintf("error formatting node: %v", err)
					}
					f.logger.Debugw("Processing AssignStmt node", "node", nodeStr, "block", i)
				}
				backpropAcrossAssignment(funcCtxByBlock[i], n.Lhs, n.Rhs, i, n)
			case *ast.ValueSpec:
				if f.logger.Level() == zapcore.DebugLevel {
					nodeStr, err := asthelper.FormatExpr(node, f.astContext.PkgInfo.Pkg.Fset)
					if err != nil {
						nodeStr = fmt.Sprintf("error formatting node: %v", err)
					}
					f.logger.Debugw("Processing ValueSpec node", "node", nodeStr, "block", i)
				}
				// Cast ValueSpec's LHS of type *ast.Ident to []ast.Expr
				lhs := make([]ast.Expr, len(n.Names))
				for k, name := range n.Names {
					lhs[k] = name
				}
				backpropAcrossAssignment(funcCtxByBlock[i], lhs, n.Values, i, n)
			default:
				// Do nothing
				f.logger.Debugw("Skipping node", "node_type", fmt.Sprintf("%T", node), "block", i)
			}
		}
	}

	// After backpropagation, the function context for the entry block contains the error return metadata for the function.
	return funcCtxByBlock[0].calleeReturnData
}

// backpropAcrossAssignment performs backpropagation analysis across the assignment statement. It checks if the assignment
// is a call site with RHS as an outbound sink site, and extracts the error return metadata for the call site.
func backpropAcrossAssignment(f *funcContext, lhs, rhs []ast.Expr, blockIndex int32, stmt ast.Node) {
	// Helper function to get the error return metadata for the given error object
	getErrRetDataForObj := func(errObj types.Object) []*ErrorReturnMetadata {
		// This is the case of direct error return, where the error object is the same as the error object used in the return statement.
		// For example, `resp, err := callee(); if err != nil { return nil, err }`.
		// In this case, we can directly extract the error return metadata for the error object.
		if e, ok := f.allErrorReturnData[errObj]; ok {
			return e
		}

		// Check if this is the case of indirect error return, where the original error object is different from the
		// error object used in the return statement. For example, `if err != nil { err2 := wrap(err); return nil, err2 }`.
		// In this case, we need to extract the error return metadata for the original error object from both the sources
		// of allErrorReturnData and calleeReturnData.
		errRetData := make([]*ErrorReturnMetadata, 0, len(f.allErrorReturnData)+len(f.calleeReturnData))
		for _, slice := range f.allErrorReturnData {
			errRetData = append(errRetData, slice...)
		}

		// Add data from `calleeReturnData` to `errRetData` if the error value is still undetermined.
		//
		// For example:
		// ```
		// err2 := errors.New("error") // err2 is determined to be non-nil here
		// resp, err := RPC() // return err2 is undetermined here
		// if err != nil {
		// 	return nil, err2
		// }
		// return resp, nil
		// ```
		// In this case, when we reach the RPC point, the error return metadata for `err` is undetermined, since err2's
		// nilness is yet to be determined. Therefore, at the RPC point, we add this entry to `calleeReturnData` with
		// ErrorValue as `undetermined`. Hence, we need to add this entry to `errRetData` to ensure that we update the ErrorValue
		// to non-nil once we reach err2's initialization point.
		//
		// However, if the error value is already determined, we do not need to add it to `errRetData`.
		// For example, in the case:
		// ```
		// var err error
		// err = foo()
		// if err != nil {
		// 	return nil, err
		// }
		// resp, err := RPC()
		// if err != nil {
		// 	return nil, err
		// }
		// return resp, nil
		// ```
		// In this case, when we reach the RPC point, the error return metadata for `err` is already determined to be non-nil.
		// Therefore, we do not need to add it to `errRetData`.
		for _, r := range f.calleeReturnData {
			if r.Value == constant.ErrorUndetermined {
				errRetData = append(errRetData, r)
			}
		}

		filteredErrRetData := make([]*ErrorReturnMetadata, 0, len(errRetData))
		for _, v := range errRetData {
			if v.OriginalErrorObj == errObj {
				filteredErrRetData = append(filteredErrRetData, v)
			}
		}
		return filteredErrRetData
	}

	// Check if the assignment is a call site with RHS as an error returning function call, e.g., `resp, err := f()`
	if len(rhs) == 1 {
		rhsNode := ast.Unparen(rhs[0])
		if call, ok := rhsNode.(*ast.CallExpr); ok {
			if !f.astContext.IsModeledFuncForErrNonnil(call) {
				callSiteErr := lhs[len(lhs)-1]
				callSiteErrType := f.astContext.ObjectOf(callSiteErr)

				// If the call site is the callee of interest, check if the position of the call site matches the callee site position.
				// If yes, extract the error return metadata for the call site. If not, skip the call site.
				if call == f.callee {
					// Mark all direct error returns from the callsite as non-nil. For example, in the case:
					// ```
					// 	resp, err := f()
					// 	return resp, err
					// ```
					// `err` at return statement site is initially marked as unknown. However, since `err` is directly
					// returned from the call site, we can safely mark it as non-nil.
					for _, v := range getErrRetDataForObj(callSiteErrType) {
						if v.Value&constant.ErrorUnknown != 0 {
							v.Value = constant.ErrorNonnil
						}
					}
					// Collect the error return metadata for the current call site
					extractErrRetForCallsite(f, callSiteErrType)
					return
				}
				// This is the case of a user-defined function which is not the callee of interest
				// For example, `if err != nil { err2 := foo() ; return nil, err2 }`, where `foo` is a user-defined function.
				// In this case, we update the error return metadata for the call site to mark the error value as nil.
				// This is to indicate a fail-open status, since the error return of the caller is not influenced by
				// the callee, but instead by foo.
				isFound := false
				for _, v := range getErrRetDataForObj(callSiteErrType) {
					if v.Value == constant.ErrorUndetermined {
						v.Value = constant.ErrorNil
						isFound = true
					}
				}
				if isFound {
					return
				}
				delete(f.allErrorReturnData, callSiteErrType)
			}
		}
	}

	// Process 1:1 assignments, e.g., `err = errors.Wrapf(err, "message")` or
	// only variable declarations, e.g., `var err error`, where `err` takes the default (zero) value `nil`.
	if len(lhs) == len(rhs) || len(rhs) == 0 {
		for i, lhsExpr := range lhs {
			// Check if the LHS is an error object
			if !f.astContext.IsError(lhsExpr) {
				continue
			}

			lhsObj := f.astContext.ObjectOf(lhsExpr)
			if lhsObj == nil {
				continue
			}

			// Check if the LHS error variable is directly or indirectly found in at least one of the return statements.
			allRetDataForLHS := getErrRetDataForObj(lhsObj)

			// Helper function to update the error return metadata for the LHS
			updateLHS := func(value constant.ErrorValue) {
				for _, v := range allRetDataForLHS {
					// Update the error return metadata for the LHS only if the value is not already assigned.
					// This is to avoid overwriting the value in case of multiple assignments to the same error variable.
					// For example, err = errors.New("error"); err = nil; return resp, err. In this case, value of err at
					// the return statement site should be `nil` and not `non-nil`.
					if !v.isValueAssigned {
						v.Value = value
						v.isValueAssigned = true
					}
				}
			}

			// Analyze the error value of the RHS
			var value constant.ErrorValue

			if len(rhs) == 0 {
				// This is the case of only error variable declaration (e.g., `var err error`).
				// Here we take the default (zero) value of error, which is `nil`.
				value = constant.ErrorNil
				updateLHS(value)
			} else {
				// This is the case of 1:1 assignments
				rhsNode := ast.Unparen(rhs[i])
				_, value := f.astContext.AnalyzeErrorValue(rhsNode)

				// Check if the error value can be determined from the conditional branch.
				// For example, `if err != nil { tempErr = err; return nil, tempErr }`.
				// Here, it can be inferred that `tempErr` is non-nil, since it is assigned from `err`, which in turn
				// is inferred to be non-nil from the conditional branch.
				if value == constant.ErrorUndetermined {
					if t, ok := f.blockNilabilityInfo[blockIndex]; ok {
						if rhsObj := f.astContext.ObjectOf(rhsNode); rhsObj != nil && t.errVar == rhsObj {
							if t.nodesWithInferredValue[stmt] {
								value = t.value
							}
						}
					}
				}

				if value != constant.ErrorUndetermined {
					// Error value is directly determinable from the RHS
					updateLHS(value)
				} else if _, ok := rhsNode.(*ast.CallExpr); ok {
					// This is the case of an unrecognized function, such as a user-defined, non-modeled function.
					// Hence, mark the error return metadata for the LHS as undetermined.
					updateLHS(constant.ErrorNil)
				} else {
					// Check if the RHS is trackable, i.e., a variable or struct field
					ident := f.astContext.IdentOf(rhsNode)
					if ident == nil {
						continue
					}
					obj := f.astContext.ObjectOf(ident)
					if obj == nil {
						continue
					}

					varObj, ok := obj.(*types.Var)
					if !ok {
						continue
					}

					// Update the error return metadata for LHS with RHS
					for _, v := range allRetDataForLHS {
						v.ErrorObj = varObj
					}
					f.allErrorReturnData[varObj] = allRetDataForLHS
					delete(f.allErrorReturnData, lhsObj)
				}
			}
		}
	}
}

func extractErrRetForCallsite(f *funcContext, callSiteErrType types.Object) {
	f.calleeReturnData = f.allErrorReturnData[callSiteErrType]

	// Remove corresponding entries from allErrorReturnData to avoid duplicate entries
	delete(f.allErrorReturnData, callSiteErrType)
}

// backpropAcrossReturn performs backpropagation analysis across the return statement. It extracts the error return metadata
// by considering different cases:
// (1) The error return is a simple variable, e.g., `return nil, err`
// (2) The error return is a determined call expression, e.g., `return nil, fmt.Sprintf("error")`
// (3) The error return is a wrapper error function, e.g., `return nil, errors.Wrapf(err, "message")`
// (4) The error return is an outbound sink site, e.g., `return OutboundRPC()`
//
// The function also infers the nilability information of the error return object, if:
// - directly determinable: e.g., `return nil, nil`
// - influenced by a conditional branch: e.g., `if err != nil { return nil, err }`
// - influenced by a wrapper function: e.g., `return nil, errors.Wrapf(err, "message")`
// - direct return of an outbound sink site, which by definition returns a non-nil error: e.g., `return OutboundRPC()`
func backpropAcrossReturn(f *funcContext, n *ast.ReturnStmt, blockIndex int32) {
	errExpr := n.Results[len(n.Results)-1]
	errObj := f.astContext.ObjectOf(errExpr)
	originalErrObj := errObj
	errValue := constant.ErrorUnknown

	if errObj == nil {
		panic(fmt.Sprintf("error type for return statement not found in function `%s`", f.caller.Name.Name))
	}

	// Helper function to update the error return metadata for the current return statement
	updateErrorMetadata := func() {
		if _, ok := f.allErrorReturnData[errObj]; !ok {
			f.allErrorReturnData[errObj] = make([]*ErrorReturnMetadata, 0)
		}
		f.allErrorReturnData[errObj] = append(f.allErrorReturnData[errObj], &ErrorReturnMetadata{
			ReturnStmt:       n,
			ErrorObj:         errObj,
			OriginalErrorObj: originalErrObj,
			Value:            errValue,
		})
	}

	// Defer to update the error return metadata for the current return statement
	defer updateErrorMetadata()

	switch e := errExpr.(type) {
	case *ast.Ident:
		identObj := f.astContext.ObjectOf(e)
		if identObj == nil {
			return
		}

		if f.astContext.IsNil(e) {
			errValue = constant.ErrorNil
			return
		}

		varObj, ok := identObj.(*types.Var)
		if !ok {
			return
		}

		// Check if nilability information of the error variable is available for the current block.
		// If yes, update the error return metadata object with the nilability information. This covers the cases:
		// - `if err != nil { return nil, err }`
		// - `if err == nil { return nil, err }`
		if t, ok := f.blockNilabilityInfo[blockIndex]; ok {
			if t.errVar == varObj {
				if t.nodesWithInferredValue[n] {
					errValue = t.value
				}
			} else {
				// This is the case of using another error variable in the return statement.
				// For example: `if err != nil { return nil, err2 }`
				// Here, `err2` is not the error variable used in the conditional branch, hence we set the errObj to the
				// conditional error variable and mark the error value as undetermined. Note that originalErrObj is set to
				// the error variable used in the return statement. Thus, for the above example:
				//  errObj = err,
				//  originalErrObj = err2,
				//  errValue = undetermined
				errObj = t.errVar
				errValue = constant.ErrorUndetermined
			}
		}

		// Check if the error variable is a global error variable.
		if errorValue, ok := f.astContext.GlobalErrorVarFailnessMap[varObj]; ok {
			errValue = errorValue
		}

	case *ast.CallExpr:
		errorArg := f.astContext.ErrorWrapperFunc(e)

		// Case 2: Check if the call expression is guarded by a conditional branch. Example:
		// ```
		// 	resp, err := f()
		// 	if err != nil {
		// 		return nil, errors.New("error")
		// 	}
		// ```
		// Here we can infer that `err` influences the error return of the function via the conditional branch.
		if v, ok := f.blockNilabilityInfo[blockIndex]; ok && v.errVar != nil && v.nodesWithInferredValue[n] {
			errObj = v.errVar
			if f.astContext.IsModeledFuncForErrNonnil(e) {
				errValue = v.value
			} else {
				// This is the case of an unrecognized function. For example, a user-defined, non-modeled function.
				// If the enclosing conditional block is `err == nil`, then we can mark the errValue of this return
				// statement as `nil`, since it is not influenced by `err`. Else, mark it as undetermined since the return
				// is influenced by `err != nil`.
				// ```
				// resp, err := f()
				// if err == nil {
				//		return someOtherFunc()  --> errValue = nil
				// }
				// ```
				// ```
				// if err != nil {
				//		return someOtherFunc()  --> errValue = undetermined
				// }
				// ```
				if v.value == constant.ErrorNil {
					errValue = constant.ErrorNil
				} else {
					errValue = constant.ErrorUndeterminedForCallExpr
				}
			}
			return
		}

		// Case 3: Check if the call expression is an error wrapper function. Example: `return nil, errors.Wrapf(err, "message")`.
		// If yes, we can infer that the error return of the function is influenced by the error argument of the wrapper.
		// This is a heuristic based on the anecdotal observation that error wrapper functions typically return a non-nil
		// error if the input error is non-nil.
		if errorArg != nil {
			rawErrorObj := f.astContext.ObjectOf(errorArg)
			if rawErrorObj != nil {
				errObj = rawErrorObj
				errValue = constant.ErrorNonnil
			}
			return
		}

		// Case 4: Check if the callee is from the list of outbound sink sites (e.g., `return OutboundRPC()`). If yes, update
		// the error return metadata object and extract the error return metadata for the call site.
		if e == f.callee {
			errValue = constant.ErrorNonnil
			updateErrorMetadata()
			extractErrRetForCallsite(f, errObj)
		}

	case *ast.UnaryExpr, *ast.CompositeLit:
		// Handle the case of a unary expression or composite literal.
		// Example: `return nil, &InternalErrorWrapper{sourceErr: err}` or `return nil, InternalErrorWrapper{sourceErr: err}`
		if f.astContext.IsError(errExpr) {
			// Check if the error return is predicated on a `err != nil` conditional branch
			if v, ok := f.blockNilabilityInfo[blockIndex]; ok && v.errVar != nil && v.nodesWithInferredValue[n] {
				errObj = v.errVar
				errValue = v.value
				return
			}

			// Handle the case of an encapsulating error struct constructed from an error object.
			// Example:
			// ```
			// func foo() error {
			// 	err := RPC()
			// 	return nil, &InternalErrorWrapper{sourceErr: err}
			// }
			// ```
			// This is a fail-close case, since the error return struct encapsulates the RPC's error object.
			// To support this case, we scan the struct's fields to find the error object.

			// Note that we presume the encapsulating struct has only one field of type error since in the current analyzer
			// architecture we expect a 1:1 mapping between the error return and the error object. This is a fair assumption
			// since a struct encapsulating multiple error objects should be rare. We can revisit this assumption in the future
			// if we encounter such cases in the wild.

			getCompositeLit := func(expr ast.Expr) *ast.CompositeLit {
				switch c := expr.(type) {
				case *ast.UnaryExpr:
					if cl, ok := c.X.(*ast.CompositeLit); ok {
						return cl
					}
				case *ast.CompositeLit:
					return c
				}
				return nil
			}

			if compositeLit := getCompositeLit(errExpr); compositeLit != nil {
				var expr ast.Expr
				for _, field := range compositeLit.Elts {
					// Check if the field is a key-value expression. Example: `return nil, &InternalErrorWrapper{sourceErr: err}`
					if kv, ok := field.(*ast.KeyValueExpr); ok && f.astContext.IsError(kv.Key) {
						expr, _ = f.astContext.AnalyzeErrorValue(kv.Value)
						break
					} else if f.astContext.IsError(field) {
						// Check if the field is an identifier. Example: `return nil, &InternalErrorWrapper{err}`
						expr, _ = f.astContext.AnalyzeErrorValue(field)
						break
					}
				}
				if expr != nil {
					errObj = f.astContext.ObjectOf(expr)
					errValue = constant.ErrorNonnil
				}
			}
		}
	}
}

// getBlockNilabilityInfo analyzes the conditionals in the BuildAndTransformCFG related to error returns, determining the nilability of
// the error variables in the conditional branches, and returns a map of block indices to error variables and their nilability.
func (f *funcAnalyzer) getBlockNilabilityInfo(graph *cfg.CFG, postOrder []int32) map[int32]*blockConditionTuple {
	analyzeNodes := func(block *cfg.Block, errObj *types.Var) (map[ast.Node]bool, bool) {
		// Analyze nodes in the basic block to determine which nodes are directly influenced by the value of the error variable
		// inferred from the conditional branch. Consider the example below.
		// ```
		// resp, err := nonOutboundRetPtrError()
		//	if err != nil {
		//		return nil, err
		//	}
		//  log(err) // Here, `err = nil` inferred from the conditional branch influences the value of `err` at this node.
		//	resp, err = OutboundRPC()
		//	return resp, err // Here, `err = nil` inferred from the conditional branch does NOT influence the value of `err` at this node since it is reassigned.
		// ```
		// Therefore, `nodesWithInferredValue` for the above example will contain the node `log(err)` and not the nodes
		// after the reassignment of `err`.
		nodesWithInferredValue := make(map[ast.Node]bool)
		isAssigned := false
		for _, node := range block.Nodes {
			switch n := node.(type) {
			case *ast.AssignStmt:
				for _, lhs := range n.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok {
						if obj := f.astContext.ObjectOf(ident); obj != nil {
							if varObj, ok := obj.(*types.Var); ok && varObj == errObj {
								isAssigned = true
								break
							}
						}
					}
				}
			case *ast.ReturnStmt:
				// If it is a direct return of the callee, then the error return is considered to be implicitly
				// reassigned and the value is no longer influenced by the conditional branch.
				// Example:
				// ```
				// resp, err := f()
				// if err != nil {
				//		return nil, err
				// }
				// return OutboundRPC() // Here, error return is considered to be implicitly reassigned and not influenced by the conditional branch.
				// ```
				if len(n.Results) == 1 {
					if call, ok := n.Results[0].(*ast.CallExpr); ok {
						if call == f.callee {
							isAssigned = true
						}
					}
				}
			}

			if !isAssigned {
				nodesWithInferredValue[node] = true
			}
		}
		return nodesWithInferredValue, isAssigned
	}

	blockNilabilityInfo := make(map[int32]*blockConditionTuple)
	processBlockNilabilityInfo := func(block *cfg.Block, varObj *types.Var, value constant.ErrorValue) {
		if blockNilabilityInfo[block.Index] == nil {
			blockNilabilityInfo[block.Index] = &blockConditionTuple{errVar: varObj}
		}
		blockNilabilityInfo[block.Index].value = value
		blockNilabilityInfo[block.Index].nodesWithInferredValue, blockNilabilityInfo[block.Index].isAssigned = analyzeNodes(block, varObj)
	}

	// Worklist algorithm to propagate the nilability information inferred from the conditional branches to successor blocks.
	var worklist []*cfg.Block
	for i := 0; i < len(postOrder); i++ {
		worklist = append(worklist, graph.Blocks[postOrder[i]])
	}

	for len(worklist) > 0 {
		block := worklist[0]
		worklist = worklist[1:]

		if cond := getConditional(block); cond != nil {
			if f.astContext.IsModeledForSuppression(cond) {
				continue
			}

			errExpr, value := f.astContext.AnalyzeErrorValue(cond)
			if value == constant.ErrorNil {
				var varObj *types.Var
				if ident, ok := errExpr.(*ast.Ident); ok {
					if obj := f.astContext.ObjectOf(ident); obj != nil {
						varObj = obj.(*types.Var)
					}
				}
				if varObj == nil {
					continue
				}

				// Since we canonicalize the conditionals in the CFG in the preprocessing step, we can assume that the
				// true branch represents the `err == nil` case and the false branch represents the `err != nil` case.
				processBlockNilabilityInfo(block.Succs[0], varObj, constant.ErrorNil)
				processBlockNilabilityInfo(block.Succs[1], varObj, constant.ErrorNonnil)

				// Add the successor blocks to the worklist for further processing
				worklist = append(worklist, block.Succs...)
				continue
			}
		}

		// If the block has already been processed, propagate the nilability information to the successor blocks.
		// If the block contains re-assignment of the error variable, then we do not propagate the nilability information
		// inferred from the conditional expression since the value has likely changed.
		if blockInfo, ok := blockNilabilityInfo[block.Index]; ok && !blockInfo.isAssigned {
			for _, succ := range block.Succs {
				// If the successor block has already been processed, check if there is conflicting nilability information.
				if succInfo, ok := blockNilabilityInfo[succ.Index]; ok {
					if succInfo.value != blockInfo.value {
						// update the nilability information of the successor block to "undetermined" to
						// indicate overconstrained nilability information.
						processBlockNilabilityInfo(succ, blockInfo.errVar, constant.ErrorUndetermined)
						worklist = append(worklist, succ)
					}
				} else {
					processBlockNilabilityInfo(succ, blockInfo.errVar, blockInfo.value)
					worklist = append(worklist, succ)
				}
			}
		}
	}

	// Post-process the map to remove blocks with "undetermined" nilability information
	for _, block := range graph.Blocks {
		if v, ok := blockNilabilityInfo[block.Index]; ok {
			if v.value == constant.ErrorUndetermined {
				delete(blockNilabilityInfo, block.Index)
			}
		}
	}

	return blockNilabilityInfo
}

// getConditional returns the expression on which the block branches, if it is a conditional branch.
func getConditional(block *cfg.Block) ast.Expr {
	if block == nil || len(block.Nodes) == 0 || len(block.Succs) != 2 {
		return nil
	}
	if expr, ok := block.Nodes[len(block.Nodes)-1].(ast.Expr); ok {
		return expr
	}
	return nil
}

// computePostOrder computes the postorder of depth first search tree (DFST) of the live blocks of the BuildAndTransformCFG.
// The backpropagation algorithm converges faster if the BuildAndTransformCFG blocks are traversed in postorder, compared to the random
// order (Check [Kam, Ullman 76']).
func computePostOrder(blocks []*cfg.Block) []int32 {
	numBlocks := len(blocks)
	visited := make([]bool, numBlocks)
	postOrder := make([]int32, 0, numBlocks)
	var visit func(curBlock int32)
	visit = func(curBlock int32) {
		visited[curBlock] = true
		for _, suc := range blocks[curBlock].Succs {
			sucIdx := suc.Index
			if !visited[sucIdx] {
				visit(sucIdx)
			}
		}
		postOrder = append(postOrder, curBlock)
	}

	// Start traversal from entry block (index 0): https://pkg.go.dev/golang.org/x/tools/go/cfg#CFG
	visit(0)

	return postOrder
}
