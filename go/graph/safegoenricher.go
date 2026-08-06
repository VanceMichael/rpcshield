package graph

import (
	"errors"
	"go/types"
	"regexp"

	"github.com/uber-research/rpcshield/go/common/utils"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// DefaultSafeGoPkgPath is the default value of Config.SafeGoPkgPath, used whenever that field is
// left empty. It points at this repo's own features/safego test fixture.
const DefaultSafeGoPkgPath = "github.com/uber-research/rpcshield/go/features/safego"

const (
	safeGoType int = iota
	notSafeGoType
)

// UnboundIndex for local free var
const UnboundIndex int = -1

// SafeGoEdgeMetaInfo is a map of call instructions instr to a map of newly added edges to interested bound var used in lamda function
type SafeGoEdgeMetaInfo map[ssa.CallInstruction]map[*callgraph.Edge][]ssa.Value

// freeVarCalleeIndexMap is a map of freevar to inner Callee instruction with binding index
type freeVarCalleeIndexMap map[ssa.Value]map[ssa.CallInstruction]struct{ indx int }

// freeVarCalleeMap is a map of freevar to inner Callee instruction
type freeVarCalleeMap map[ssa.Value]map[ssa.CallInstruction]struct{}

// safeGoSiteVarCalleeMap is a map of safego call instruction to a map of freevar to inner Callee instruction
type safeGoSiteVarCalleeMap map[ssa.CallInstruction]freeVarCalleeMap

// safeGoRegexes returns the two regexes used to recognize the safego package and its Go(...)
// call sites, derived from b.config.SafeGoPkgPath (or DefaultSafeGoPkgPath when empty), compiling
// and caching them on first use so repeated safego-detection calls within this builder's lifetime
// don't recompile the pattern.
func (b *builderImpl) safeGoRegexes() (pkgRegex *regexp.Regexp, callerPattern *regexp.Regexp) {
	b.safeGoRegexOnce.Do(func() {
		pkgPath := b.config.SafeGoPkgPath
		if pkgPath == "" {
			pkgPath = DefaultSafeGoPkgPath
		}
		quoted := regexp.QuoteMeta(pkgPath)
		b.safeGoPkgRegex = regexp.MustCompile("^" + quoted + "$")
		b.safeGoCallerPattern = regexp.MustCompile("^" + quoted + `\.Go\(.*$`)
	})
	return b.safeGoPkgRegex, b.safeGoCallerPattern
}

func (b *builderImpl) enrichWithSafeGoCall(cg *callgraph.Graph) (SafeGoEdgeMetaInfo, error) {
	// The function reconstruct the call graph and link the caller to safego anonymous function
	// It returns meta information in {anonymous call instr: {new edge : var (err channel, shared err var)}}
	safeGoImporters, err := b.getSafeGoImporters()
	if err != nil {
		return nil, err
	}
	// Iterate over all functions and process the SafeGo calls.
	wholeProgSafeGoInfo := make(SafeGoEdgeMetaInfo)
	for fn := range cg.Nodes {
		// This will skip nil packages such as synthetic functions. But we are not interested in analyzing their internals.
		if _, ok := safeGoImporters[fn.Pkg]; !ok {
			b.logger.Debugw("[safegoEnrich] skipping SafeGo function", "function", fn.String())
			continue
		}

		// Ignore the safego package itself.
		pkgRegex, _ := b.safeGoRegexes()
		if pkgRegex.MatchString(fn.Pkg.Pkg.Path()) {
			continue
		}

		safeGoCallSiteVars := b.processSafeGoInFunc(fn, cg)
		b.logger.Debugw("[safegoEnrich] enrichWithSafeGO: processing function", "function", fn.String())
		b.logger.Debugw("[safegoEnrich] size of safeGoCallSiteVars", "size", len(safeGoCallSiteVars))

		//Append to global map
		for callSite, evmap := range safeGoCallSiteVars {
			wholeProgSafeGoInfo[callSite] = evmap
		}
	}
	cgChecker(cg, true, true)
	return wholeProgSafeGoInfo, nil
}

// getCFFImporters returns the packages that directly import CFF.
func (b *builderImpl) getSafeGoImporters() (map[*ssa.Package]struct{}, error) {
	pkgRegex, _ := b.safeGoRegexes()
	pkgs, err := utils.GetPackagesDirectlyImportingPkgsByPathRegex(b.ssaProg, pkgRegex)
	if err != nil {
		b.logger.Debug("failed to get packages directly importing SageGO", "error", err)
		return nil, err
	}
	for pkg := range pkgs {
		// Ignore if the package path itself matches the regex
		if pkgRegex.MatchString(pkg.Pkg.Path()) {
			continue
		}
		b.logger.Debugw("package directly importing SageGO", "package", pkg.Pkg.Path())
	}
	return pkgs, nil
}

// getCFFCallType returns the type of the CFF call.
func (b *builderImpl) getSafeGoCallType(call ssa.CallInstruction) (int, error) {
	if call.Common().StaticCallee() == nil || call.Common().Value == nil {
		return notSafeGoType, errors.New("call instruction does not have a static callee")
	}

	if call.Value() == nil {
		return notSafeGoType, errors.New("call instruction does not have a value, either go statement or defer statement")
	}

	_, callerPattern := b.safeGoRegexes()
	safegoCaller := callerPattern.MatchString(call.Value().String())
	if safegoCaller {
		return safeGoType, nil
	}
	return notSafeGoType, nil
}

func (b *builderImpl) getSafeGoCallsites(fn *ssa.Function) map[int]map[ssa.CallInstruction]struct{} {
	// Get all callsites that are of CFF calling patterns.
	safeGoCallInstrs := map[int]map[ssa.CallInstruction]struct{}{
		safeGoType: {},
	}
	for _, blk := range fn.Blocks {
		for _, instr := range blk.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			typ, err := b.getSafeGoCallType(call)
			if err != nil || typ == notSafeGoType {
				b.logger.Debugw("failed to get SafeGo call type", "error", err, "call", call.String())
				continue
			}
			// only one type of call inst
			if typ == safeGoType {
				if _, exists := safeGoCallInstrs[safeGoType]; !exists {
					safeGoCallInstrs[safeGoType] = make(map[ssa.CallInstruction]struct{})
				}
				safeGoCallInstrs[safeGoType][call] = struct{}{}
			}
		}
	}
	return safeGoCallInstrs
}

func checkErrorChannel(t types.Type) bool {
	// Check for `chan error`
	if isErrorChannel(t) {
		return true
	}
	// Check for `*chan error`
	ptrType, ok := t.(*types.Pointer)
	if ok {
		return isErrorChannel(ptrType.Elem())
	}
	return false
}

func checkErrorVar(t types.Type) bool {
	// Check for `error`
	if isErrorLikeType(t) {
		return true
	}
	// Check for `*error`
	ptrType, ok := t.(*types.Pointer)
	if ok {
		return isErrorLikeType(ptrType.Elem())
	}
	return false
}

func isErrorChannel(t types.Type) bool {
	// Check if the type is a channel
	chanType, ok := t.(*types.Chan)
	if !ok {
		return false
	}
	// Check if the channel's element type satisfies the `error` interface
	return isErrorLikeType(chanType.Elem())
}

func isErrorLikeType(t types.Type) bool {
	errorInterface := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	return types.Implements(t, errorInterface)
}

func (b *builderImpl) filterBoundVar(funcBindings []ssa.Value) map[int]bool {
	qualifiedBindingsMask := make(map[int]bool)
	for i, binding := range funcBindings {
		if b.typeImplements(binding.Type().Underlying(), b.neededTypes[_contextContext]) {
			b.logger.Debugw(" Capture Binding[%d]: %s type: %s)\n", i, binding.Name(), binding.Type())
			qualifiedBindingsMask[i] = true
		} else if checkErrorVar(binding.Type()) {
			b.logger.Debugw(" Capture Binding[%d]: %s type: %s)\n", i, binding.Name(), binding.Type())
			qualifiedBindingsMask[i] = true
		} else if checkErrorChannel(binding.Type()) {
			b.logger.Debugw(" Capture Binding[%d]: %s type: %s)\n", i, binding.Name(), binding.Type())
			qualifiedBindingsMask[i] = true
		} else {
			b.logger.Debugw(" unrelated type", "func arg", binding.Name(), "type", binding.Type())
		}
	}
	return qualifiedBindingsMask
}

func (b *builderImpl) processSafeGoInFunc(fn *ssa.Function, cg *callgraph.Graph) SafeGoEdgeMetaInfo {
	callSiteToCallee := map[ssa.CallInstruction]map[*ssa.Function]struct{}{}
	safeGoInstrs := b.getSafeGoCallsites(fn)
	safeGoCallSiteVarMap := make(safeGoSiteVarCalleeMap)
	for callsite := range safeGoInstrs[safeGoType] {
		// meta information to show which freevar is used in the safego
		// varCallMap : [var] => Callee(potential RPC) in the lamda
		// varCallMap maps a ssa value (var) to the set of call instructions inside the lambda that can flow to this variable.
		varCallMap, err := b.processSafeGoCall(callsite.Common())
		if err != nil {
			b.logger.Error("[safegoEnrich] Failed to process SafeGo callsite", "error", err, "callsite", callsite.String())
			continue
		}
		// handle both makeClosure and static function, not support dynamic function
		if len(varCallMap) != 0 {
			// callsite is the safego callsite, which must have its Arg[0] as an ssa.Function
			// rfn is the wrapped function (lamda) of the safego callsite
			rfn := getResolvedFunction(callsite.Common().Args[0])
			if rfn == nil {
				b.logger.Debugw("[safegoEnrich] Unable to resolve SafeGo callsite (not a closure or static function)", "callsite", callsite.String())
			}
			callSiteToCallee[callsite] = make(map[*ssa.Function]struct{})
			callSiteToCallee[callsite][rfn] = struct{}{}
			safeGoCallSiteVarMap[callsite] = varCallMap
		}
	}

	// modify call graph and skip safe-go and its anonymous function. Direct link inner RPC call.
	if len(callSiteToCallee) != 0 {
		// Delete the outgoing edges from the outer to the safeGo.
		// For the safe go to the current lamda, we do not delete edge now, since safego --> go$ --> .... --> lamda
		b.deleteCGEdges(callSiteToCallee, cg.Nodes[fn], cg)
		// Add new edges from the callsites to the current callees.
		siteToEdge, _ := b.addCGEdges(callSiteToCallee, cg.Nodes[fn], cg, true /* registerAsAsync */)
		metaInfo := b.createEdgeMetaInfo(siteToEdge, safeGoCallSiteVarMap)
		return metaInfo
	}
	return nil
}

func (b *builderImpl) createEdgeMetaInfo(siteToEdge callsiteMap, safeGoCallSiteVarMap safeGoSiteVarCalleeMap) SafeGoEdgeMetaInfo {
	// Example: F ----[site][edge][FreeVar]---> SafeGoLamda
	// return a map of call instructions to a map of newly added edges to interested vars used in lamda function
	/**
	func AFunction(ctx context.Context) (*Response, error) {
		errChan := make(chan error, 2)
		safego.Go(func() {
			defer wg.Done()
			_, err := generalFCall(ctx, genCallInput)
			if err != nil {
				errChan <- err
			}
		})
		return resp, nil
	}

	// Call Graph Edge
	AFunction ----- e -----> AFunction$1 (lamda)
	SafeGoEdgeMetaInfo: {AFunction$1: {e: [errChan]}}
	 **/
	metaInfo := make(SafeGoEdgeMetaInfo)
	for site, edges := range siteToEdge {
		metaInfo[site] = make(map[*callgraph.Edge][]ssa.Value)
		for _, edge := range edges {
			if vmap, ok := safeGoCallSiteVarMap[site]; ok {
				// get keys of vmap which are freevar
				for v := range vmap {
					metaInfo[site][edge] = append(metaInfo[site][edge], v)
				}
			}
		}
	}
	return metaInfo
}

func (b *builderImpl) getNecessaryFreeVars(qualifiedBindingsMask map[int]bool, closureFn *ssa.Function) map[ssa.Value]int {
	// FreeVar and binding is 1:1 mapping, filter by order
	qualifiedFreeVars := make(map[ssa.Value]int)
	for i, freeVar := range closureFn.FreeVars {
		if qualifiedBindingsMask[i] {
			// Match the free variable to the corresponding binding
			qualifiedFreeVars[freeVar] = i
		}
	}
	return qualifiedFreeVars
}

type wrapFuncType int

const (
	isMakeClosure    wrapFuncType = iota // 0
	isStaticFunction                     // 1
	unKnown                              // 2
)

func checkWrapFuncType(val ssa.Value) wrapFuncType {
	switch val.(type) {
	case *ssa.Function:
		return isStaticFunction
	case *ssa.MakeClosure:
		return isMakeClosure
	}
	return unKnown
}

func (b *builderImpl) getCallSitesReachingChanSend(fn *ssa.Function) (freeVarCalleeIndexMap, error) {
	var sendInst []ssa.Instruction
	// collect all send instructions in the static function
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if _, ok := instr.(*ssa.Send); ok {
				sendInst = append(sendInst, instr)
			}
		}
	}
	varCallMap := make(freeVarCalleeIndexMap)
	for _, instr := range sendInst {
		depsInstructions := b.findTransitiveReachingDefs(instr)
		// RPC callee where send instruction is affected by
		ssafuncCall := make(map[ssa.CallInstruction]struct{})
		for dinstr := range depsInstructions {
			// filter out not call instructions
			call, ok := dinstr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			ssafuncCall[call] = struct{}{}
		}

		// TODO: simply use the left hand side of the send instruction. Need to find the real channel later
		localVar := instr.(*ssa.Send).Chan
		varCallMap[localVar] = make(map[ssa.CallInstruction]struct{ indx int })
		for ssafn := range ssafuncCall {
			varCallMap[localVar][ssafn] = struct{ indx int }{indx: UnboundIndex}
		}
	}
	return varCallMap, nil
}

func (b *builderImpl) processSafeGoCall(fn *ssa.CallCommon) (freeVarCalleeMap, error) {
	// return the wrapped satisfied function in SafeGo call
	if len(fn.Args) == 0 {
		return nil, errors.New("callCommon does not have any arguments")
	}
	wrapfType := checkWrapFuncType(fn.Args[0])
	var varCallMap freeVarCalleeMap
	// handle both makeClosure and static function, not support dynamic function
	if wrapfType == isStaticFunction {
		argfn := fn.Args[0].(*ssa.Function)
		freeVarCallIdxMap, err := b.getCallSitesReachingChanSend(argfn)
		varCallMap = b.mapLocalVarToBoundVar(freeVarCallIdxMap, make([]ssa.Value, 0))
		if err != nil {
			return varCallMap, err
		}
		return varCallMap, nil
	} else if wrapfType == isMakeClosure {
		mc, _ := fn.Args[0].(*ssa.MakeClosure)
		b.logger.Debugw("Closure function: %s, %d \n", mc.Fn.String(), len(mc.Bindings))
		//First pruning the Safego Callsite if the binding does not have [context, error variable, error channel]
		qualifiedBindingsMask := b.filterBoundVar(mc.Bindings)
		// If no qualified bindings, skip the SafeGO call
		if len(qualifiedBindingsMask) == 0 {
			return varCallMap, nil
		}
		closureFn, ok := mc.Fn.(*ssa.Function)
		if !ok {
			return nil, errors.New("closure function is not a ssa.Function")
		}
		qualifiedFreeVars := b.getNecessaryFreeVars(qualifiedBindingsMask, closureFn)
		// freeVarCallIdxMap: [freeVar] => [CalleeInstruction] => binding index
		freeVarCallIdxMap, err := b.findCalleesInsideClosure(qualifiedFreeVars)
		if err != nil {
			return varCallMap, err
		}
		// freevar is out of scope and restore binding var
		// used for callerfunction dataflow analysis
		//varCallMap: [freeVar] => Callee(potential RPC) in the lamda
		varCallMap = b.mapLocalVarToBoundVar(freeVarCallIdxMap, mc.Bindings)
		return varCallMap, nil
	}

	return varCallMap, errors.New("callCommon does not have a closure or * Function as the first argument")
}

func (b *builderImpl) mapLocalVarToBoundVar(freeVarCallIdxMap freeVarCalleeIndexMap, mcBindings []ssa.Value) freeVarCalleeMap {
	varCallMap := make(map[ssa.Value]map[ssa.CallInstruction]struct{})
	for fv, callInstrIndexMap := range freeVarCallIdxMap {
		// fv is local free var and will be remapped to the binding var
		for callInstr, index := range callInstrIndexMap {
			switch {
			case index.indx == UnboundIndex:
				// fv to callee could be 1:n mapping
				if _, vok := varCallMap[fv]; !vok {
					varCallMap[fv] = make(map[ssa.CallInstruction]struct{})
				}
				varCallMap[fv][callInstr] = struct{}{}

			case index.indx >= len(mcBindings):
				// Skip out-of-bounds indexes
				continue

			case mcBindings[index.indx] == nil:
				// Skip nil values in slice
				continue

			default:
				val := mcBindings[index.indx]
				if _, vok := varCallMap[val]; !vok {
					varCallMap[val] = make(map[ssa.CallInstruction]struct{})
				}
				varCallMap[val][callInstr] = struct{}{}
			}
		}
	}
	return varCallMap
}

func (b *builderImpl) findCalleesInsideClosure(freeVars map[ssa.Value]int) (freeVarCalleeIndexMap, error) {
	varCallIndexMap := make(freeVarCalleeIndexMap)
	for freeVar, index := range freeVars {
		// the ssafuncCall is call instruction(potential RPC) affect freeVar
		ssafuncCall, err := b.processFreeVarRelatedFunc(freeVar)
		if err != nil {
			b.logger.Debugw("processFreeVarRelatedFunc failed to find functions", "error", err)
			continue
		}
		varCallIndexMap[freeVar] = make(map[ssa.CallInstruction]struct{ indx int })
		for _, ssafn := range ssafuncCall {
			varCallIndexMap[freeVar][ssafn] = struct{ indx int }{indx: index}
		}
	}
	return varCallIndexMap, nil
}

// This function is used to find the transitive references of a free variable.
/*
 1. Send to a channel often first extract a local chan and then send instruction happens. Hence, we trace down to a send instruction from the free variable.
    t6 = * errChan
    some instruction....
    *ssa.Send: send t6 <- t4
 2. Store to the shared var is of interest.
	some instruction....
	*sharedErr = t4
both t4 will be tracked backward in func findTransitiveReachingDefs()
*/
func (b *builderImpl) findTransitiveRefs(freeVar ssa.Value) map[ssa.Instruction]struct{} {
	visited := make(map[ssa.Instruction]bool)    // To track visited instructions
	queue := []ssa.Value{freeVar}                // Queue for BFS-style traversal
	result := make(map[ssa.Instruction]struct{}) // Store instructions where tracking stops

	// Process the queue
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:] // Dequeue

		// Get all referrers (instructions using `current`)
		if current.Referrers() == nil {
			continue
		}

		for _, instr := range *current.Referrers() {
			if instr == nil || visited[instr] {
				continue
			}

			visited[instr] = true // Mark as visited
			switch inst := instr.(type) {
			case *ssa.Send:
				// If `current` is on the left-hand side of send, we stop
				if inst.Chan == current {
					result[instr] = struct{}{}
					continue
				}
			case *ssa.Store:
				// If `current` is the value being stored (LHS), we stop
				if inst.Addr == current {
					result[instr] = struct{}{}
					continue
				}
				queue = append(queue, inst.Val)
			case *ssa.Call:
				// when reach as a function arg, stop
			case *ssa.UnOp, *ssa.Convert, *ssa.Extract:
				// These operations create new values, track their results
				if val, ok := instr.(ssa.Value); ok {
					queue = append(queue, val)
				}
			}
		}
	}
	return result
}

// This function is used to find the transitive reaching definitions of an instruction.
// Collected all involved instructions and will be filtered out to find the function call instructions later.
// Example:
/*
	t0 = * ctx
	t1 = * genCallInput
	t2 = call generalFCall([*ctx *genCallInput])
	*ssa.Extract: extract t2 #0
	*ssa.Extract: extract t2 #1
	*sharedErr = t4
	Start from t4, all instruction affect t4 will be collected.
*/
func (b *builderImpl) findTransitiveReachingDefs(instr ssa.Instruction) map[ssa.Instruction]struct{} {
	dependencies := make(map[ssa.Instruction]struct{}) // To store unique instructions
	stack := []ssa.Instruction{instr}

	for len(stack) > 0 {
		// Pop the last element from the stack
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Skip if already visited
		if _, visited := dependencies[current]; visited {
			continue
		}

		// Mark the instruction as visited
		dependencies[current] = struct{}{}

		// Add all operands (users of this instruction) to the stack
		for _, operand := range current.Operands(nil) {
			if operand != nil {
				if opInstr, ok := (*operand).(ssa.Instruction); ok {
					stack = append(stack, opInstr)
				}
			}
		}
	}
	return dependencies
}

func (b *builderImpl) processFreeVarRelatedFunc(freeVar ssa.Value) ([]ssa.CallInstruction, error) {
	result := b.findTransitiveRefs(freeVar)
	// located send and store instructions involving the FreeVar (top -> down)
	depsInstructions := make(map[ssa.Instruction]struct{})
	callInstrList := make([]ssa.CallInstruction, 0)

	for instr := range result {
		// Direct usage of FreeVar in Store or Send instructions
		switch i := instr.(type) {
		case *ssa.Send:
			b.logger.Debugw("Send instruction using FreeVar: Chan=%v, Val=%v\n", i.Chan, i.X)
			depsInstructions = b.findTransitiveReachingDefs(instr) // find the source of the sender (bottom -> up)

		case *ssa.Store:
			b.logger.Debugw("Shared Var Store instruction using FreeVar: LHS=%v, Val=%v\n", i.Addr, i.Val)
			depsInstructions = b.findTransitiveReachingDefs(instr) // find the source of RHS (bottom -> up)
		}

		for dinstr := range depsInstructions {
			// filter out not call instructions
			call, ok := dinstr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			callInstrList = append(callInstrList, call)
		}
	}
	return callInstrList, nil
}
