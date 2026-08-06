package ssaanal

import (
	"errors"
	"go/token"
	"go/types"

	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
)

// AnalyzeCall performs SSA analysis on the given function and call instruction.
// It returns constant.FailOpen, constant.FailClose, or constant.FailUnknown based on the analysis.
func AnalyzeCall(f *ssa.Function, call ssa.CallInstruction) (constant.Failness, error) {
	// Validate inputs.
	if err := validateInput(f, call); err != nil {
		return constant.FailUnknown, err
	}

	errorRetIndex, err := getErrorReturnIndex(f)
	if err != nil {
		// This should never happen. But if we are ever called for a function that does not return an error, we should fail-open.
		return constant.FailOpen, nil
	}

	// Extract the error ssa.Value from the call instruction.
	theError, _, err := getErrorValue(call)
	if err != nil {
		// If the error is not extracted, determine if it's constant.FailOpen.
		if err == errErrorNotExtracted {
			return constant.FailOpen, nil
		}
		return constant.FailUnknown, err
	}

	// Analyze references to the error value.
	result, err := analyzeErrorReferences(f, errorRetIndex, theError)
	if err != nil {
		return constant.FailUnknown, err
	}

	return result, nil
}

// validateInput checks if the inputs to analyzeCall are valid.
func validateInput(f *ssa.Function, call ssa.CallInstruction) error {
	if f == nil || call == nil || call.Parent() != f || call.(*ssa.Call) == nil {
		return errors.New("invalid input for analyzeCall")
	}
	return nil
}

// getErrorValue extracts the error value from the call instruction.
// It returns the error value, the index of the error value in the results, and an error if any.
func getErrorValue(call ssa.CallInstruction) (ssa.Value, int, error) {
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	sig := call.Common().Signature()
	results := sig.Results()
	n := results.Len()

	switch {
	case n == 1:
		// Single return value.
		if types.Implements(results.At(0).Type(), errorType) {
			return call.(ssa.Value), 0, nil
		}
		return nil, -1, errNotErrorType

	case n > 1:
		// Multiple return values.
		errorIndex, err := findErrorIndex(results, errorType)
		if err != nil {
			if err == errMultipleErrorReturns {
				// Return the extract with the sentinel error so callers can decide.
				val, idx, extractErr := findErrorExtract(call, errorIndex)
				if extractErr != nil {
					return nil, -1, extractErr
				}
				return val, idx, errMultipleErrorReturns
			}
			return nil, -1, err
		}
		return findErrorExtract(call, errorIndex)

	default:
		return nil, -1, errors.New("unexpected number of return values")
	}
}

// findErrorIndex returns the index of the error return value in the results.
// If there are multiple error return values, it returns the index of the last one
// along with errMultipleErrorReturns.
func findErrorIndex(results *types.Tuple, errorType *types.Interface) (int, error) {
	errorCount := 0
	errorIndex := -1
	for i := 0; i < results.Len(); i++ {
		if types.Implements(results.At(i).Type(), errorType) {
			errorCount++
			errorIndex = i
		}
	}
	if errorCount == 0 {
		return -1, errNoErrorReturn
	}
	if errorCount > 1 {
		return errorIndex, errMultipleErrorReturns
	}
	return errorIndex, nil
}

var errErrorNotExtracted = errors.New("error value not extracted")
var errNoErrorReturn = errors.New("no error return value found")
var errMultipleErrorReturns = errors.New("unexpected number of error return values")
var errNotErrorType = errors.New("unexpected return value type: does not implement error")

// findErrorExtract finds the ssa.Extract instruction for the error at the given index.
func findErrorExtract(call ssa.CallInstruction, errorIndex int) (ssa.Value, int, error) {
	callIns, ok := call.(*ssa.Call)
	if !ok {
		return nil, errorIndex, errors.New("call instruction is not a Call")
	}
	referrers := callIns.Referrers()
	if referrers == nil {
		return nil, errorIndex, errErrorNotExtracted
	}

	for _, ref := range *referrers {
		if extract, ok := ref.(*ssa.Extract); ok && extract.Tuple == callIns && extract.Index == errorIndex {
			return extract, errorIndex, nil
		}
	}
	return nil, errorIndex, errErrorNotExtracted
}

// getErrorReturnIndex returns the index of the error return value in the function.
// If there are multiple error return values, it returns the index of the last one.
// If there are no error return values, it returns -1.
func getErrorReturnIndex(f *ssa.Function) (int, error) {
	errorType := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	// Get the function signature.
	rets := f.Signature.Results()
	// Get the index of the last return type that can be assigned to error.
	for i := rets.Len() - 1; i >= 0; i-- {
		if types.Implements(rets.At(i).Type(), errorType) {
			return i, nil
		}
	}
	return -1, errNoErrorReturn
}

func nonDebugReferrers(v ssa.Value) []ssa.Instruction {
	var referrers []ssa.Instruction
	for _, ref := range *v.Referrers() {
		if _, ok := ref.(*ssa.DebugRef); !ok {
			referrers = append(referrers, ref)
		}
	}
	return referrers
}

// analyzeErrorReferences analyzes how the error value is used and returns constant.FailOpen or constant.FailClose.
// It returns constant.FailUnknown if the analysis is inconclusive.
func analyzeErrorReferences(f *ssa.Function, errorRetIndex int, theError ssa.Value) (constant.Failness, error) {
	// Find all non-debug referrers to the error value.
	referrers := nonDebugReferrers(theError)

	if referrers == nil {
		return constant.FailOpen, nil // Error is not checked.
	}
	var binOp *ssa.BinOp
	returns := []*ssa.Return{}

	// Collect return statements and identify if there's exactly one BinOp, if any.
	for _, ref := range referrers {
		switch instr := ref.(type) {
		case *ssa.Return:
			returns = append(returns, instr)
		case *ssa.BinOp:
			if binOp != nil {
				// More than one BinOp; falls under "UNKNOWN".
				return constant.FailUnknown, errors.New("more than one BinOp instruction")
			}
			// Check if the BinOp is comparing the error with nil. We cannot handle other cases.
			if !isComparisonWithNil(instr, theError) {
				// The BinOp is not comparing theError with nil.
				return constant.FailUnknown, errors.New("BinOp is not a comparison with nil")
			}
			binOp = instr
		default:
			// TODO: Handle other referrer types for wrappers or logging.
			// Found a referrer that is neither Return nor BinOp.
			return constant.FailUnknown, errors.New("unexpected referrer type")
		}
	}
	// Case 1: this is NOT fail-close for us. It is a property of this code and not dictated by the behavior of the callee.
	// PATTERN:
	// func foo() (Response, error) {
	//   resp, err := RPC()
	//   return resp, errors.New("xyz")
	// }
	// Hence, if there is no BinOp and no return referencing the error, it is fail-open by design.
	if binOp == nil && len(returns) == 0 {
		return constant.FailOpen, nil
	}

	// We either have a BinOp or at least one return referencing the error or both.

	// Case 2: Error is returned as is without checking with nil.
	// PATTERN:
	// func foo() (Response, error) {
	//   resp, err := RPC()
	//   return resp, err
	// }
	// Hence, if there is no BinOp, but there is at least one return referencing the error, it is fail-close.
	if binOp == nil {
		return constant.FailClose, nil // Error is returned as is without checking.
	}

	// We have one BinOp and the rest are Returns.
	// Ensure the BinOp is used in exactly one if statement; more complex control flow is constant.FailUnknown.
	ifStmt, ok := singleIfUser(binOp)
	if !ok {
		return constant.FailUnknown, errors.New("binary operation result is not used in a single If statement")
	}

	// Determine the blocks for error equality checks.
	errEqNilBlk, errEqNonNilBlk := determineBlocks(ifStmt, binOp.Op)

	// Analyze dominance relationships.
	// If at least one return statement returning the err value is not dominated by the err == nil block, it is constant.FailClose.
	for _, ret := range returns {
		if !errEqNilBlk.Dominates(ret.Block()) {
			return constant.FailClose, nil // At least one return not dominated by errEqNilBlk.
		}
	}
	// We have one final but complicated case to handle.
	// We have no return statement that is not dominated by the err == nil block. But we may have a definitive non-nil return under the err != nil predicate.
	// We need to check if there is a definitive non-nil error return indicating FailClose.
	return isDefinitiveNonNilErrorReturn(errEqNonNilBlk, errorRetIndex)
}

// determineBlocks identifies the blocks corresponding to err == nil and err != nil.
func determineBlocks(ifStmt *ssa.If, op token.Token) (errEqNilBlk, errNeqNilBlk *ssa.BasicBlock) {
	thenBlock := ifStmt.Block().Succs[0]
	elseBlock := ifStmt.Block().Succs[1]

	if op == token.NEQ {
		return elseBlock, thenBlock
	}
	return thenBlock, elseBlock
}

// isComparisonWithNil checks if the binary operation compares the error with nil.
func isComparisonWithNil(binOp *ssa.BinOp, theError ssa.Value) bool {
	if binOp.Op != token.EQL && binOp.Op != token.NEQ {
		return false
	}
	x, y := binOp.X, binOp.Y
	return (x == theError && isNil(y)) || (y == theError && isNil(x))
}

// isNil checks if the SSA value is a nil constant.
func isNil(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	return ok && c.IsNil()
}

// singleIfUser ensures the binary operation result is used in exactly one If statement.
func singleIfUser(binOp *ssa.BinOp) (*ssa.If, bool) {
	refs := nonDebugReferrers(binOp)
	if refs == nil || len(refs) != 1 {
		return nil, false
	}
	ifStmt, ok := (refs)[0].(*ssa.If)
	return ifStmt, ok
}

// isDefinitiveNonNilErrorReturn checks if the given block has a definitive non-nil error return indicating FailClose.
/* In here we are interested in identifying this pattern (PATTERN 1) where the statement S1 does not use err itself, but under the err != nil
   predicate it returns a guaranteed non-nil error.
PATTERN 1:
func foo() (Response, error) {
  resp, err := RPC()
  if err != nil {
S1:     return nil, errors.New("xyz")
  }
  return rep, nil
}

More patterns are recognizable:
PATTERN 2: here RPC1 is treated fail-close because S2 returns non-nil error always under the err != nil predicate.
func foo() (Response, error) {
  resp, err := RPC1()
  if err != nil {
        r2, err2 := RPC2() // or any call
        if err2 {
S1:			log(something)
		}
S2:     return nil, errors.New("xyz")
  }
  return rep, nil
}

PATTERN 3: here RPC1 is Not fail-close because S2 is not guaranteed to always execute when err != nil predicate is taken.
This will be treated fail-open as per the current design.
func foo() (Response, error) {
  resp, err := RPC1()
  if err != nil {
        r2, err2 := RPC2() // or any call
        if err2 {
S1:			return r2, err2
		}
S2:     return nil, errors.New("xyz") // not always guranteed to execute in the predicate.
  }
  return rep, nil
}
*/
func isDefinitiveNonNilErrorReturn(entry *ssa.BasicBlock, errorRetIndex int) (constant.Failness, error) {
	// First get all the blocks dominated by the entry block.
	blocks := getSingleEntryMultiExitBlocks(entry)
	// There shall be at least one block.
	// Just a defensive check. It will never be true under our constraints.
	if len(blocks) == 0 {
		return constant.FailUnknown, errors.New("no blocks in the region")
	}
	// Get the exit blocks.
	// Here we may end up with zero blocks because we may have no exit blocks at all.
	// That is fine. It indicates fail-open.
	exitBlocks, err := getExitBlocks(blocks)
	if err != nil || len(exitBlocks) == 0 {
		return constant.FailOpen, nil
	}

	// Get the unpredicated blocks.
	unpredicatedBlocks := getUnpredicatedBlocks(exitBlocks, entry)
	// Here we may end up with zero blocks because we may have no unpredicated blocks at all.
	// That is fine. It indicates not a definitive non-nil error return. Hence, fail-open.
	if len(unpredicatedBlocks) == 0 {
		return constant.FailOpen, nil
	}

	// Retain only the blocks that end in a return statement.
	unpredicatedReturnBlocks := filterReturnBlocks(unpredicatedBlocks)
	// Here we may end up with zero blocks because we may have no return blocks at all.
	// That is fine. It indicates not a definitive non-nil error return. Hence, fail-open.
	if len(unpredicatedReturnBlocks) == 0 {
		return constant.FailOpen, nil
	}
	firstExitBlock, err := getFirstExitBlock(unpredicatedReturnBlocks)
	if err != nil {
		return constant.FailOpen, err
	}
	// Get the return value from the first exit block.
	// We have identified an unpredicated returning block.
	// The return value from the block may be a constant `nil` or a non-constant value.
	// Get that value and infer the fail-close/fail-open.
	returnValue, err := getTheReturnValueFromBlock(firstExitBlock, errorRetIndex)
	// The error shall never happen under our constraints. This is a defensive check.
	if err != nil {
		return constant.FailUnknown, err
	}
	// Check if the return value is non-nil.

	// Here we take this assumption:
	// if the return value is a constant `nil`, it is a fail-open.
	// if the return value is a non-constant value and defined via a known non-nil error creator, it is a fail-close.
	// if the return value is a non-constant value and not defined via a known non-nil error creator, it is a fail-unknown.
	// All the following cases fall in this last category.
	// 1. `return _aNilValue`.
	// 2. `return myMap[_keyforNilValue]`
	// 3. `return myStruct{}`.
	// 4. `copyErr := nil; return copyErr.
	// Note: it is possible make a simplification here by assuming all non-constant values are non-nil and hence fail-close.
	// We'll consider this in the future.

	if isNil(returnValue) {
		return constant.FailOpen, nil
	}
	if errorDefinedByKnownNonNilErrorCreator(returnValue) {
		return constant.FailClose, nil
	}
	return constant.FailUnknown, nil
}

// filterReturnBlocks filters the blocks that end in a return statement.
func filterReturnBlocks(blocks []*ssa.BasicBlock) []*ssa.BasicBlock {
	var returnBlocks []*ssa.BasicBlock
	for _, block := range blocks {
		if _, ok := isReturnBlock(block); ok {
			returnBlocks = append(returnBlocks, block)
		}
	}
	return returnBlocks
}

// getSingleEntryMultiExitBlocks returns the blocks dominated by the entry block.
// Use Dominees to recursively get all blocks dominated by the entry block.
func getSingleEntryMultiExitBlocks(entry *ssa.BasicBlock) []*ssa.BasicBlock {
	// Initialize the list of blocks with the entry block.
	// The end result will have the entry block and all the blocks dominated by the entry block.
	blocks := []*ssa.BasicBlock{entry}
	// Get the blocks dominated by the entry block.
	dominees := entry.Dominees()
	// Recursively get all blocks dominated by the entry block.
	for _, dominee := range dominees {
		blocks = append(blocks, getSingleEntryMultiExitBlocks(dominee)...)
	}
	return blocks
}

// getExitBlocks returns the blocks that exit out of the region, where region is a set of blocks.
// Any block that has no successor in the region is considered an exit block.
func getExitBlocks(region []*ssa.BasicBlock) ([]*ssa.BasicBlock, error) {
	if len(region) == 0 {
		return nil, errors.New("no blocks in the region")
	}
	// Initialize the list of exit blocks.
	exitBlocks := []*ssa.BasicBlock{}
	// Make a map of blocks in the region for quick lookup.
	blockMap := make(map[*ssa.BasicBlock]struct{})
	for _, block := range region {
		blockMap[block] = struct{}{}
	}
	// Find the exit blocks.
	for _, block := range region {

		// returning blocks.
		if _, ok := isReturnBlock(block); ok {
			exitBlocks = append(exitBlocks, block)
			continue
		}

		// Block has a successor outside the region.
		for _, succ := range block.Succs {
			if _, ok := blockMap[succ]; !ok {
				exitBlocks = append(exitBlocks, block)
				break
			}
		}
	}
	return exitBlocks, nil
}

// getFirstExitBlock returns the first exit block among the exit blocks.
// It is the one that dominates all other exit blocks.
func getFirstExitBlock(exitBlocks []*ssa.BasicBlock) (*ssa.BasicBlock, error) {
	if len(exitBlocks) == 0 {
		return nil, errors.New("no exit blocks")
	}
	// Initialize the first exit block.
	firstExitBlock := exitBlocks[0]
	// Find the first exit block.
	for _, exitBlock := range exitBlocks {
		if firstExitBlock.Dominates(exitBlock) {
			firstExitBlock = exitBlock
		}
	}
	return firstExitBlock, nil
}

// getUnpredicatedBlocks returns the blocks that execute without any control flow.
// This is computed finding getting the set of dominating blocks of each exit block and then taking the intersection.
func getUnpredicatedBlocks(exitBlocks []*ssa.BasicBlock, entry *ssa.BasicBlock) []*ssa.BasicBlock {
	// Initialize the unpreducated blocks.
	unpredicatedBlocks := make(map[*ssa.BasicBlock]map[*ssa.BasicBlock]struct{})
	// Find the unpreducated blocks.
	for _, exitBlock := range exitBlocks {
		// Get the set of dominating blocks of the exit block until the entry block.
		dominatingBlocks := getDominatingBlocks(exitBlock, entry)
		// Add the dominating blocks to the unpreducated blocks.
		unpredicatedBlocks[exitBlock] = dominatingBlocks
	}
	// Find the intersection of the dominating blocks of all exit blocks.
	intersection := getIntersection(unpredicatedBlocks)
	return intersection
}

// getDominatingBlocks returns the set of blocks that dominate the given block until the entry block.
func getDominatingBlocks(block, entry *ssa.BasicBlock) map[*ssa.BasicBlock]struct{} {
	// Initialize the dominating blocks.
	dominatingBlocks := map[*ssa.BasicBlock]struct{}{
		block: {},
	}
	// Find the dominating blocks.
	for block != entry {
		dominatingBlocks[block] = struct{}{}
		block = block.Idom()
	}
	return dominatingBlocks
}

// getIntersection returns the intersection of the dominating blocks of all exit blocks.
// Just an N^2 operation.
func getIntersection(unpredicatedBlocks map[*ssa.BasicBlock]map[*ssa.BasicBlock]struct{}) []*ssa.BasicBlock {
	// Initialize with the first set of dominating blocks.
	intersection := map[*ssa.BasicBlock]struct{}{}
	for _, blocks := range unpredicatedBlocks {
		for block := range blocks {
			intersection[block] = struct{}{}
		}
		break
	}
	// Find the intersection.
	for _, blocks := range unpredicatedBlocks {
		for block := range intersection {
			if _, ok := blocks[block]; !ok {
				delete(intersection, block)
			}
		}
	}
	// Convert the map to a slice.
	var intersectionBlocks []*ssa.BasicBlock
	for block := range intersection {
		intersectionBlocks = append(intersectionBlocks, block)
	}
	return intersectionBlocks
}

// isReturnBlock checks if the given block ends in a return statement.
func isReturnBlock(block *ssa.BasicBlock) (*ssa.Return, bool) {
	for _, instr := range block.Instrs {
		if v, ok := instr.(*ssa.Return); ok {
			return v, true
		}
	}
	return nil, false
}

// getTheReturnValueFromBlock returns the return value from the given block.
// It returns error if the block does not have a return value.
// If it has a constant `nil` return value, it returns value, nil.
// If it has a non-constant return value, it returns the value.
func getTheReturnValueFromBlock(block *ssa.BasicBlock, errorRetIndex int) (ssa.Value, error) {
	// Get the nondebug return instructions from the block.
	var returns *ssa.Return
	returns, ok := isReturnBlock(block)
	if !ok {
		return nil, errors.New("block does not have a return value")
	}
	// Get the return value from the return instruction.
	if returns.Results == nil {
		return nil, errors.New("block does not have a return value")
	}
	// Return the value at the "error" index.
	return returns.Results[errorRetIndex], nil
}

// errorDefinedByKnownNonNilErrorCreator checks if the error is defined by a known non-nil error creator such as
// errors.New, fmt.Errorf, etc. function calls.
func errorDefinedByKnownNonNilErrorCreator(val ssa.Value) bool {
	def, ok := val.(ssa.Instruction)
	if !ok {
		return false // Not an instruction.
	}
	call, ok := def.(*ssa.Call)
	if !ok {
		return false // Not a call instruction.
	}
	fn := call.Call.StaticCallee()
	if fn == nil {
		return false // Not a static callee.
	}
	// Check if the function is a known non-nil error creator.
	return isKnownNonNilErrorCreator(fn)
}

// isKnownNonNilErrorCreator checks if the function is a known non-nil error creator.
func isKnownNonNilErrorCreator(fn *ssa.Function) bool {
	// List of known non-nil error creators.
	knownCreators := []string{
		"errors.New",
		"fmt.Errorf",
	}
	// Check if the function is a known non-nil error creator.
	for _, creator := range knownCreators {
		if fn.String() == creator {
			return true
		}
	}
	return false
}
