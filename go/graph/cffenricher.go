package graph

import (
	"errors"
	"go/types"
	"regexp"
	"strings"

	"github.com/uber-research/rpcshield/go/common/utils"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// TODO: cover additional constructs in the CFF (Codegen Fan-out Fan-in) framework.

/*
		Need to capture the following CFF pattern:
	    err := _cffFlowcffdemo_43_9  (ctx,
	        _cffWithEmittercffdemo_3224879298  (cffemit.Logs(log)),
	        _cffInstrumentFlowcffdemo_398550328  ("lucky_flow"),
	        _cffParamscffdemo_46_3  (random, name),
	        _cffResultscffdemo_47_3 (&cffdemo),
	        _cffTaskcffdemo_634495231[func(ctx context.Context, name string) stringWrapper] (func(ctx context.Context, name string) stringWrapper {
	            return stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
	        }),
	        _cffTaskcffdemo_1059985446[func(ctx context.Context, lucky int32) (bool, error)] (func(ctx context.Context, lucky int32) (bool, error) {
	            return c.lottery.Play(ctx, lucky)
	        }),
	        _cffTaskcffdemo_3592525902[func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*entity.CffDemo, error)](func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*entity.CffDemo, error) {
	            return mapper.LuckyResultToCffDemo(prefixName.name, lucky, won)
	        }),
	        _cffTaskcffdemo_39807803802866684455[func(ctx context.Context, cffdemo *entity.CffDemo) error]  (func(ctx context.Context, cffdemo *entity.CffDemo) error {
	            return c.repo.Set(ctx, cffdemo)
	        },
	            _cffInvokecffdemo_3365180733  (true),
	        ),
	    )
		and transform the call to look like

		err := _cff.Flow() ->  edges to
			func(ctx context.Context, name string) stringWrapper]
			func(ctx context.Context, lucky int32) (bool, error)
			func(ctx context.Context, lucky int32, prefixName stringWrapper, won bool) (*entity.CffDemo, error)
			func(ctx context.Context, cffdemo *entity.CffDemo) error
*/

/*
err := _cffParallelapp1_50_9  (

	ctx,
	_cffWithEmitterapp1_3224879298  (cffemit.Logs(logger)),
	_cffInstrumentParallelapp1_398550328  ("lucky"),
	_cffContinueOnErrorapp1_3365180733  (false),
	_cffConcurrencyapp1_2515107422  (2),
	_cffTaskapp1_1745492981[func(ctx context.Context) error]  (func(ctx context.Context) error {
		sw := stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
		fmt.Println(sw)
		return nil
	}),
	_cffTaskapp1_1745492981[func(ctx context.Context) error]  (func(ctx context.Context) error {
		ojHasOrderedBefore, ojErr := HasOrderedBefore(ctx, int8Arg)
		if ojHasOrderedBefore {
			return nil
		}
		return ojErr
	}),
	_cffTaskapp1_1745492981[func(ctx context.Context) error] (func(ctx context.Context) error {
		resp = &Response{Success: false}
		return errors.New("error")
	}),
	)

	and transform the call to look like

		err := _cff.Parallel() ->  edges to
			func(ctx context.Context) error
			func(ctx context.Context) error
			func(ctx context.Context) error
*/
// _cffPkgRegex matches both the internal-fork CFF import path and the public OSS
// "go.uber.org/cff" import path (including its "/internal/scheduler" subpackage, which is where
// the *Scheduler.Enqueue method lives that _cffEnqueuePattern-style detection below looks for).
var _cffPkgRegex = regexp.MustCompile(`^(code\.uber\.internal/go/cff|go\.uber\.org/cff)(/.+)?$`)
var _cffFlowPattern = regexp.MustCompile(`_cffFlow.+_\d+`)
var _cffParallelPattern = regexp.MustCompile(`_cffParallel.+_\d+`)
var _cffSlicePattern = regexp.MustCompile(`_cffSlice.+_\d+`)
var _cffMapPattern = regexp.MustCompile(`_cffMap.+_\d+`)

var _cffTaskPattern = regexp.MustCompile(`_cffTask.+_\d+`)
var _remainingCFFPattern = regexp.MustCompile(`_cff[A-Z].+_\d+`)

// CFF FallbackWith pattern is used to capture the fallback function in the CFF call.
// A cff call with a fallback function will have a pattern like this:
//
//	_cffTaskapp_10599854463522357227[func(ctx context.Context, lucky int32) (bool, error)] /*line :51:11*/ (func(ctx context.Context, lucky int32) (bool, error) {
//		return false, nil
//	}, _cffFallbackWithapp_53_6[bool] /*line :53:22*/ (true)),
var _cffFallbackWithPattern = regexp.MustCompile(`_cffFallbackWith.+_\d+`)

// _cffSliceEndPattern and _cffMapEndPattern match the synthesized wrapper the current internal
// codegen tool emits for a cff.SliceEnd/cff.MapEnd option, e.g.
// "_cffSliceEndapp_1745492981(endClosure)"/"_cffMapEndapp_1745492981(endClosure)" -- one of the
// trailing arguments to the cff.Slice/cff.Map wrapper call (see processCFFTaskCall). Note these
// are substrings of _cffSlicePattern/_cffMapPattern too (e.g. "_cffSliceEnd..." matches
// "_cffSlice.+_\d+"); that's harmless for getCFFCallType's classification of the End wrapper's
// own callsite (it's never reached by getCFFTaskCallsites' operand walk regardless of which type
// it's classified as -- it's nested inside the Slice/Map wrapper's own arguments, not a direct
// argument of the enclosing Parallel call), but processCFFTaskCall checks these patterns
// explicitly (via findCFFEndFuncArg) to recover the End closure the walk can't reach on its own.
var _cffSliceEndPattern = regexp.MustCompile(`_cffSliceEnd.+_\d+`)
var _cffMapEndPattern = regexp.MustCompile(`_cffMapEnd.+_\d+`)

// _cffEnqueueMethodName is the method used by the current (inline-codegen) shape of
// go.uber.org/cff to register a task closure for concurrent execution:
//
//	sched := cff.NewScheduler(...)
//	task0.job = sched.Enqueue(ctx, cff.Job{
//		Run: task0.run, // a closure wrapping the user's cff.Task function
//	})
//
// Unlike the older, internal-fork-only codegen shape above (which emits a uniquely-named
// generated function per Flow/Task/Parallel/etc call that we can regex-match by name), this shape
// has no distinguished per-call generated symbol at all -- every flow's scheduling boilerplate
// looks identical modulo the closures it captures. So instead of name matching, we detect the
// *Scheduler.Enqueue call structurally (by method name + receiver package) and recover the task
// closure by walking the "cff.Job{Run: ...}" argument with the same generic SSA operand-tracing
// (findCFFTaskFuncs/ssaValueVisitor) used for the old shape's task arguments -- see
// processCFFEnqueueCall.
const _cffEnqueueMethodName = "Enqueue"

const (
	cffTaskType int = iota
	cffFlowType
	cffParallelType
	cffSliceType
	cffMapType
	cffFallbackWithType
	cffRemainingType
	cffEnqueueType
	notCffType
)

const (
	notSyntheticFunc int = iota
	boundSyntheticFunc
	thunkSyntheticFunc
	wrapperSyntheticFunc
	unknownSyntheticFunc
)

const (
	_boundMethod = "$bound"
	_thunkMethod = "$thunk"
)

// enrichWithCFFCall enriches the callgraph with CFF calls.
func (b *builderImpl) enrichWithCFFCall(cg *callgraph.Graph) error {
	// Get all packages that directly import CFF.
	cffImporters, err := b.getCFFImporters()
	if err != nil {
		return err
	}

	// Iterate over all functions and process the CFF calls.
	for fn := range cg.Nodes {
		// This will skip nil packages such as synthetic functions. But we are not interested in analyzing their internals.
		if _, ok := cffImporters[fn.Pkg]; !ok {
			b.logger.Debugw("skipping CFF function", "function", fn.String())
			continue
		}

		// Ignore the CFF package itself.
		if _cffPkgRegex.MatchString(fn.Pkg.Pkg.Path()) {
			b.logger.Debugw("skipping CFF function", "function", fn.String())
			continue
		}

		pkgNameStr := fn.Pkg.Pkg.Name()
		// Every call to a CFF function will have a pattern of _cff.*<pkgName>.*<number>.
		// We need to only process the functions that are of this pattern.
		pattern := regexp.MustCompile(`_cff.*` + pkgNameStr + `_.*\d+`)
		if pattern.MatchString(fn.Name()) {
			b.logger.Debugw("skipping CFF function", "function", fn.String())
			continue
		}
		if n, ok := cg.Nodes[fn]; !ok || n == nil {
			b.logger.Debugw("enrichWithCFFCall: function node not found in callgraph", "function", fn.String())
			continue
		}
		b.logger.Debugw("enrichWithCFFCall: processing function", "function", fn.String())
		b.processCFFInFunc(fn, cg)
	}
	// Ensure cg stability.
	cgChecker(cg, true, true)
	return nil
}

// getCFFImporters returns the packages that directly import CFF.
func (b *builderImpl) getCFFImporters() (map[*ssa.Package]struct{}, error) {
	pkgs, err := utils.GetPackagesDirectlyImportingPkgsByPathRegex(b.ssaProg, _cffPkgRegex)
	if err != nil {
		b.logger.Debug("failed to get packages directly importing CFF", "error", err)
		return nil, err
	}
	for pkg := range pkgs {
		// Ignore if the package path itself matches the regex
		if _cffPkgRegex.MatchString(pkg.Pkg.Path()) {
			continue
		}
		b.logger.Debug("package directly importing CFF", "package", pkg.Pkg.Path())
	}
	return pkgs, nil
}

// processCFFInFunc processes the CFF calls in the function.
func (b *builderImpl) processCFFInFunc(fn *ssa.Function, cg *callgraph.Graph) {
	// Get all callsites that are of CFF calling patterns.
	cffInstrs := b.getCFFCallsites(fn)
	callSiteToCallee := map[ssa.CallInstruction]map[*ssa.Function]struct{}{}
	b.logger.Debugw("processing CFF callsites, found callsites", "function", fn.String(), "task", len(cffInstrs[cffTaskType]), "flow", len(cffInstrs[cffFlowType]), "parallel", len(cffInstrs[cffParallelType]), "remaining", len(cffInstrs[cffRemainingType]))

	// Process the cffFlowType callsites.
	for callsite := range cffInstrs[cffFlowType] {
		funcs, err := b.processCFFFlowCall(callsite.Common(), fn, cffInstrs[cffTaskType], cg)
		if err != nil {
			// log error
			b.logger.Error("failed to process CFF flow callsite", "error", err, "callsite", callsite.String())
			continue
		}
		callSiteToCallee[callsite] = funcs
	}
	// Process the cffParallelType callsites.
	for callsite := range cffInstrs[cffParallelType] {
		funcsTaskType, err := b.processCFFFlowCall(callsite.Common(), fn, cffInstrs[cffTaskType], cg)
		if err != nil {
			b.logger.Error("failed to process CFF parallel callsite", "error", err, "callsite", callsite.String())
			continue
		}
		// cff.Slice is only an option for cff.Parallel.
		funcsSliceType, err := b.processCFFFlowCall(callsite.Common(), fn, cffInstrs[cffSliceType], cg)
		if err != nil {
			b.logger.Error("failed to process CFF parallel callsite", "error", err, "callsite", callsite.String())
			continue
		}
		// cff.Map is only an option for cff.Parallel.
		funcsMapType, err := b.processCFFFlowCall(callsite.Common(), fn, cffInstrs[cffMapType], cg)
		if err != nil {
			b.logger.Error("failed to process CFF parallel callsite", "error", err, "callsite", callsite.String())
			continue
		}
		// Concatenate the maps under cff.Parallel.
		funcs := map[*ssa.Function]struct{}{}
		for f := range funcsTaskType {
			funcs[f] = struct{}{}
		}
		for f := range funcsSliceType {
			funcs[f] = struct{}{}
		}
		for f := range funcsMapType {
			funcs[f] = struct{}{}
		}
		callSiteToCallee[callsite] = funcs
	}

	// Process the cffEnqueueType callsites (the inline-codegen shape's task registration, see
	// _cffEnqueueMethodName). Each Enqueue callsite stands in for both a task and its own flow: it
	// has no separate generated flow-boundary call to attach to, so we wire its edge directly.
	for callsite := range cffInstrs[cffEnqueueType] {
		funcs, err := b.processCFFEnqueueCall(callsite.Common(), fn)
		if err != nil {
			b.logger.Error("failed to process CFF enqueue callsite", "error", err, "callsite", callsite.String())
			continue
		}
		callSiteToCallee[callsite] = funcs
	}

	// Delete the outgoing edges from the callsites to the current callees.
	b.deleteCGEdges(callSiteToCallee, cg.Nodes[fn], cg)
	// Add new edges from the callsites to the current callees.
	_, newEdges := b.addCGEdges(callSiteToCallee, cg.Nodes[fn], cg, true /* registerAsAsync */)
	b.addEdgesToNoPruneSet(newEdges)

	// Collect all callsites for cffTaskType, cffSliceType, cffMapType, cffFallbackWithType, and
	// cffRemainingType.
	toDelete := map[ssa.CallInstruction]map[*ssa.Function]struct{}{}
	for k, v := range cffInstrs {
		switch k {
		case cffTaskType, cffSliceType, cffMapType, cffFallbackWithType, cffRemainingType:
			for callsite := range v {
				toDelete[callsite] = nil // not needed
			}
		}
	}
	// Delete the callsites that are already processed.
	b.deleteCGEdges(toDelete, cg.Nodes[fn], cg)
}

// getCFFCallsites returns the callsites that are of CFF calling patterns.
func (b *builderImpl) getCFFCallsites(fn *ssa.Function) map[int]map[ssa.CallInstruction]struct{} {
	// Get all callsites that are of CFF calling patterns.
	cffInstrs := map[int]map[ssa.CallInstruction]struct{}{
		cffTaskType:         {},
		cffFlowType:         {},
		cffParallelType:     {},
		cffSliceType:        {},
		cffMapType:          {},
		cffFallbackWithType: {},
		cffRemainingType:    {},
		cffEnqueueType:      {},
	}

	for _, blk := range fn.Blocks {
		for _, instr := range blk.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			b.logger.Debug("processing CFF callsite", "call", call.Common().Value.Name())
			typ, err := getCFFCallType(call)
			if err != nil || typ == notCffType {
				b.logger.Debugw("failed to get CFF call type", "error", err, "call", call.String())
				continue
			}
			switch typ {
			case cffTaskType:
				cffInstrs[cffTaskType][call] = struct{}{}
			case cffFlowType:
				cffInstrs[cffFlowType][call] = struct{}{}
			case cffParallelType:
				cffInstrs[cffParallelType][call] = struct{}{}
			case cffFallbackWithType:
				cffInstrs[cffFallbackWithType][call] = struct{}{}
			case cffSliceType:
				cffInstrs[cffSliceType][call] = struct{}{}
			case cffMapType:
				cffInstrs[cffMapType][call] = struct{}{}
			case cffRemainingType:
				cffInstrs[cffRemainingType][call] = struct{}{}
			case cffEnqueueType:
				cffInstrs[cffEnqueueType][call] = struct{}{}
			}
		}
	}
	return cffInstrs
}

// getCFFCallType returns the type of the CFF call.
func getCFFCallType(call ssa.CallInstruction) (int, error) {
	if call.Common().StaticCallee() == nil || call.Common().Value == nil {
		return notCffType, errors.New("call instruction does not have a static callee")
	}
	isTask := _cffTaskPattern.MatchString(call.Common().Value.Name())
	if isTask {
		return cffTaskType, nil
	}
	isFlow := _cffFlowPattern.MatchString(call.Common().Value.Name())
	if isFlow {
		return cffFlowType, nil
	}
	isParallel := _cffParallelPattern.MatchString(call.Common().Value.Name())
	if isParallel {
		return cffParallelType, nil
	}
	isSlice := _cffSlicePattern.MatchString(call.Common().Value.Name())
	if isSlice {
		return cffSliceType, nil
	}
	isMap := _cffMapPattern.MatchString(call.Common().Value.Name())
	if isMap {
		return cffMapType, nil
	}
	isFallback := _cffFallbackWithPattern.MatchString(call.Common().Value.Name())
	if isFallback {
		return cffFallbackWithType, nil
	}
	isRemaining := _remainingCFFPattern.MatchString(call.Common().Value.Name())
	if isRemaining {
		return cffRemainingType, nil
	}
	if isCFFEnqueueCall(call) {
		return cffEnqueueType, nil
	}
	return notCffType, nil
}

// isCFFEnqueueCall returns true if the call is a *scheduler.Scheduler.Enqueue call -- the
// structural (name-independent) marker of a CFF task registration under the current inline-codegen
// shape of go.uber.org/cff (see _cffEnqueueMethodName).
func isCFFEnqueueCall(call ssa.CallInstruction) bool {
	callee := call.Common().StaticCallee()
	if callee == nil || callee.Name() != _cffEnqueueMethodName || callee.Pkg == nil {
		return false
	}
	return _cffPkgRegex.MatchString(callee.Pkg.Pkg.Path())
}

// deleteCGEdges deletes the outgoing edges from the callsites.
func (b *builderImpl) deleteCGEdges(callsites map[ssa.CallInstruction]map[*ssa.Function]struct{}, node *callgraph.Node, cg *callgraph.Graph) {
	for i, edge := range node.Out {
		site := edge.Site
		if _, ok := callsites[site]; !ok {
			continue
		}
		// Remove the incoming edge from the callee to the callsite.
		if err := b.removeIncomingEdge(edge.Callee, edge); err != nil {
			// log error
			continue
		}
		// Mark the edge for deletion.
		node.Out[i] = nil
	}
	// Remove the nil outgoing edges.
	curSet := node.Out[:0]
	for _, e := range node.Out {
		if e != nil {
			curSet = append(curSet, e)
		}
	}
	node.Out = curSet
}

// addCGEdges adds new edges from the callsites to its given callees.
// If registerAsAsync is true, the new edges are registered as async via createAndRegisterAsyncEdge.
func (b *builderImpl) addCGEdges(callsites map[ssa.CallInstruction]map[*ssa.Function]struct{}, node *callgraph.Node, cg *callgraph.Graph, registerAsAsync bool) (callsiteMap, map[*callgraph.Edge]struct{}) {
	siteToEdge := callsiteMap{}
	newEdges := map[*callgraph.Edge]struct{}{}
	for site, funcs := range callsites {
		for f := range funcs {
			calleeNode, ok := cg.Nodes[f]
			if !ok {
				// log error
				continue
			}
			newEdge := &callgraph.Edge{Caller: node, Site: site, Callee: calleeNode}
			newEdges[newEdge] = struct{}{}
			node.Out = append(node.Out, newEdge)
			calleeNode.In = append(calleeNode.In, newEdge)
			siteToEdge[site] = append(siteToEdge[site], newEdge)
		}
	}
	if registerAsAsync {
		b.createAndRegisterAsyncEdge(newEdges)
	}
	return siteToEdge, newEdges
}

// removeIncomingEdge removes the incoming edge from the node.
// This is O(n) in time but not called often.
func (b *builderImpl) removeIncomingEdge(node *callgraph.Node, edge *callgraph.Edge) error {
	if node == nil || edge == nil {
		return errors.New("node or edge is nil")
	}
	curSet := node.In[:0]
	found := false
	for _, e := range node.In {
		if e != edge {
			curSet = append(curSet, e)
		} else {
			found = true
		}
	}
	if !found {
		return errors.New("edge not found in the node")
	}
	node.In = curSet
	return nil
}

// processCFFFlowCall processes the CFF flow callsite.
func (b *builderImpl) processCFFFlowCall(fn *ssa.CallCommon,
	function *ssa.Function,
	cffInstrs map[ssa.CallInstruction]struct{},
	cg *callgraph.Graph) (map[*ssa.Function]struct{}, error) {
	// We need to walk every parameter of the call instruction and find the actual value being passed.
	// If the value is returned from a callInstruction of cffTaskType,
	// we need to find the actual function that is being passed by getting the information processCFFTaskCall.
	isCFFTaskCallsite := func(ins ssa.CallInstruction) bool {
		if ins == nil {
			return false
		}
		if _, ok := cffInstrs[ins]; ok {
			return true
		}
		return false
	}

	for i, arg := range fn.Args {
		b.logger.Debugw("processing CFF flow call argument", "pos", i, "arg", arg.String())
	}

	sites, err := getCFFTaskCallsites(fn.Args, function, isCFFTaskCallsite)
	if err != nil {
		return nil, err
	}
	// Get all functions that are being passed as the first argument to the CFF flow callsite.
	funcsMap := map[*ssa.Function]struct{}{}
	for site := range sites {
		b.logger.Debugw("processing CFF flow callsite", "callsite", site.String())
		funcs, err := b.processCFFTaskCall(site.Common(), function, cg)
		if err != nil {
			return nil, err
		}

		for f := range funcs {
			funcsMap[f] = struct{}{}
		}
	}
	return funcsMap, nil
}

func (b *builderImpl) checkCFFTaskFallback(v ssa.Value, function *ssa.Function, cg *callgraph.Graph) (*ssa.Function, bool) {
	// This is a simple backward flow through assignments and phi nodes until we reach a fallback function.
	// If we reach a fallback function, we return true.
	// If we reach a non-fallback function, we continue the flow.
	visited := make(map[ssa.Value]bool)
	stack := []ssa.Value{v}
	for len(stack) > 0 {
		// Pop the last element
		n := len(stack) - 1
		current := stack[n]
		stack = stack[:n]
		if current == nil || visited[current] {
			continue
		}
		visited[current] = true
		switch val := current.(type) {
		case *ssa.Function:
			// If it is a fallback function, return true.
			if _cffFallbackWithPattern.MatchString(val.Name()) {
				return val, true
			}
		case ssa.CallInstruction:
			// Add the function to the stack if it is a static callee
			if val.Common().StaticCallee() != nil {
				stack = append(stack, val.Common().StaticCallee())
			}
		// Although we can handle these cases do not arise because CFF codegen does not generate them.
		// If we ever try to use assignment of conditionals we get unexpected code generation directive "FallbackWith":
		// only cff.Flow, cff.Parallel, or cff.TaskGroup may be called at the top-level.
		// Hence, comment them out and if in the future we need to handle them, we can uncomment them.
		// case *ssa.BinOp:
		// 	// Add th RHS of the assignment operator to the stack.
		// 	if val.Op == token.ASSIGN {
		// 		stack = append(stack, val.Y)
		// 	}
		// case *ssa.Phi:
		// 	// Add all operands of the phi node to the stack.
		// 	for _, operand := range val.Edges {
		// 		stack = append(stack, operand)
		// 	}
		default:
			// noop
		}
	}

	return nil, false
}

// getWrapperType returns the type of the synthetic function.
// https://github.com/golang/tools/blob/master/go/ssa/wrappers.go
func (b *builderImpl) getWrapperType(fn *ssa.Function) int {
	// Synthetic functions do not have a package. Special handler them.
	if fn.Synthetic == "" {
		return notSyntheticFunc
	}
	// Bound methods end with "$bound"
	// Thunks end with "$thunk"
	// Wrappers don't have a name suffix but their Synthetic string contains "wrapper for"
	// check $bound in fn.Name
	if strings.HasSuffix(fn.Name(), _boundMethod) {
		return boundSyntheticFunc
	}
	if strings.HasSuffix(fn.Name(), _thunkMethod) {
		return thunkSyntheticFunc
	}
	if strings.Contains(fn.Synthetic, "wrapper for") {
		return wrapperSyntheticFunc
	}
	return unknownSyntheticFunc
}

// processCFFTaskCall processes the CFF task callsite walks the first argument and finds the actual function that is being passed.
func (b *builderImpl) processCFFTaskCall(fn *ssa.CallCommon, function *ssa.Function, cg *callgraph.Graph) (map[*ssa.Function]struct{}, error) {
	/*
			signature: func Task(fn interface{}, opts ...TaskOption) Option
		        _cffTaskcffdemo_634495231[func(ctx context.Context, name string) stringWrapper] (func(ctx context.Context, name string) stringWrapper {
					return stringWrapper{name: fmt.Sprintf("Demo:%s", name)}
				}),
	*/
	// process the callsite and capture the first argument.
	// Since the argument will be converted to an interface, we need to find the actual type by looking for the interface conversion that led to this interface.
	// Then, look for the actual definition of the function that is being passed as the first argument.
	// It could be a static function or a method, it could be a makeclosure instruction, or a function passed as a parameter.
	// Get the first parameter of CallCommon.
	if len(fn.Args) == 0 {
		return nil, errors.New("CallCommon does not have any arguments")
	}

	// Short circuit if there is a FallbackWith function.
	if len(fn.Args) > 1 {
		for _, arg := range fn.Args[1:] {
			fbFunc, ok := b.checkCFFTaskFallback(arg, function, cg)
			if !ok {
				continue
			}
			b.logger.Debugw("processCFFTaskCall: found fallback function", "function", function.String(), "callsite", fn.String(), "fallback", fbFunc.String())
			// No error, just empty callees.
			emptyFuncs := map[*ssa.Function]struct{}{}
			return emptyFuncs, nil
		}
	}

	contextCallingFilter := b.cffContextCallingFilter

	// Find the actual function that is being passed as the first argument.
	seeds := []ssa.Value{fn.Args[0]}

	// cff.Slice/cff.Map optionally take a trailing SliceEnd/MapEnd option. Under the current
	// internal codegen's hybrid shape, that closure is wrapped in its own synthesized
	// _cffSliceEnd.../_cffMapEnd... call among the remaining arguments to the same
	// _cffSlice.../_cffMap... wrapper call this function is processing -- e.g. the third argument
	// to "_cffSliceXXX(mainClosure, items, _cffSliceEndXXX(endClosure))". getCFFTaskCallsites'
	// operand walk never reaches this call (it stops once it finds the outer wrapper call that
	// matches the caller's requested type), so recover it explicitly and seed it into the same
	// findCFFTaskFuncs walk as the main closure.
	if len(fn.Args) > 1 {
		for _, arg := range fn.Args[1:] {
			if endArg, ok := findCFFEndFuncArg(arg); ok {
				seeds = append(seeds, endArg)
			}
		}
	}

	funcs, err := findCFFTaskFuncs(seeds, function, contextCallingFilter)
	b.logger.Debugw("processCFFTaskCall: found functions", "functions", funcs)
	if err != nil {
		b.logger.Error("processCFFTaskCall failed to find functions", "error", err)
		return nil, err
	}
	return funcs, nil
}

// findCFFEndFuncArg checks whether v is itself a call to a synthesized _cffSliceEnd.../
// _cffMapEnd... wrapper (see _cffSliceEndPattern/_cffMapEndPattern), and if so returns that
// wrapper call's own first argument -- the real user-provided SliceEnd/MapEnd closure.
func findCFFEndFuncArg(v ssa.Value) (ssa.Value, bool) {
	call, ok := v.(ssa.CallInstruction)
	if !ok {
		return nil, false
	}
	callee := call.Common().StaticCallee()
	if callee == nil {
		return nil, false
	}
	if !_cffSliceEndPattern.MatchString(callee.Name()) && !_cffMapEndPattern.MatchString(callee.Name()) {
		return nil, false
	}
	if len(call.Common().Args) == 0 {
		return nil, false
	}
	return call.Common().Args[0], true
}

// cffContextCallingFilter returns true if f looks like a user-provided CFF task/flow closure,
// as opposed to other functions incidentally reached by the same SSA operand walk (see
// findCFFTaskFuncs) -- CFF's own package internals, or (under the current internal codegen's
// hybrid shape, see cffappctxlessslice) a synthesized top-level helper like
// _cffSlice<pkg>_<line>_<col> that the user's real closure is passed through as an argument.
// Used to pick the actual task function out of the SSA values reachable from a CFF task/Enqueue
// call's arguments (see processCFFTaskCall and processCFFEnqueueCall).
//
// A context.Context-first parameter is common, and for cff.Task/cff.Tasks mandatory, but is NOT
// required for cff.Slice/cff.Map task functions: cff.Slice's doc comment documents
// "func(idx int, item someType) { ... }" (no context.Context at all) as a valid, supported
// signature -- ads-campaign-manager's real ValidateAdGroups uses exactly this shape. Requiring
// context.Context unconditionally silently dropped such closures (and everything reachable
// through them) from the call graph entirely, rather than erroring loudly.
func (b *builderImpl) cffContextCallingFilter(f *ssa.Function) bool {
	// Special case, if it is a synthetic function, like a bound or a thunk, skip the package check.
	isWrapper := false
	switch b.getWrapperType(f) {
	case boundSyntheticFunc, thunkSyntheticFunc, wrapperSyntheticFunc:
		isWrapper = true
	case unknownSyntheticFunc: // NOOP for now
	case notSyntheticFunc:
	default: // NOOP for now
	}

	// If it is a function call to cff package, skip it.
	if (!isWrapper) && (f == nil || f.Pkg == nil || _cffPkgRegex.MatchString(f.Pkg.Pkg.Path()) || f.Signature == nil) {
		b.logger.Debugw("cffContextCallingFilter: skipping function", "function", f.String())
		return false
	}
	// Exclude CFF's own synthesized top-level helper functions (e.g. _cffParallel..._<line>_<col>,
	// _cffSlice..._<line>_<col>). These live in the user's own package (so the cff-package check
	// above doesn't catch them) but are code-generation plumbing that the same operand walk can
	// reach (e.g. as the callee of a call whose argument is the real task closure) -- not a
	// task/flow closure themselves.
	if !isWrapper && _remainingCFFPattern.MatchString(f.Name()) {
		b.logger.Debugw("cffContextCallingFilter: skipping synthesized CFF helper function", "function", f.String())
		return false
	}
	if f.Signature.Params().Len() == 0 {
		// A zero-arg task closure (e.g. cff.Tasks(func() {...})) is still a legitimate user
		// closure.
		b.logger.Debugw("cffContextCallingFilter: found zero-arg function", "function", f.String())
		return true
	}
	b.logger.Debugw("cffContextCallingFilter: found function", "function", f.String())
	return true
}

// processCFFEnqueueCall processes a *scheduler.Scheduler.Enqueue callsite (see
// _cffEnqueueMethodName) and recovers the underlying task closure(s) passed via its
// "cff.Job{Run: ...}" argument. Unlike processCFFTaskCall, there's no single fixed argument index
// to seed the search from -- the Job struct is usually the last argument, but we scan all of them
// (findCFFTaskFuncs + cffContextCallingFilter safely ignore non-function/non-context-taking
// values) so this doesn't depend on exact argument ordering.
func (b *builderImpl) processCFFEnqueueCall(fn *ssa.CallCommon, function *ssa.Function) (map[*ssa.Function]struct{}, error) {
	if len(fn.Args) == 0 {
		return nil, errors.New("CallCommon does not have any arguments")
	}
	seeds := cffJobRunFieldSeeds(fn.Args)
	if len(seeds) == 0 {
		return nil, errors.New("could not locate cff.Job Run field among Enqueue call arguments")
	}
	funcs, err := findCFFTaskFuncs(seeds, function, b.cffContextCallingFilter)
	b.logger.Debugw("processCFFEnqueueCall: found functions", "functions", funcs)
	if err != nil {
		b.logger.Error("processCFFEnqueueCall failed to find functions", "error", err)
		return nil, err
	}
	// Mirror processCFFTaskCall's FallbackWith short-circuit (see checkCFFTaskFallback) for the
	// inline-codegen shape: a fallback-guarded task has no separately detectable fallback call or
	// closure (the fallback value substitution is inlined into the task's own run closure), so we
	// instead recognize it structurally by the emitter calls the codegen only emits on the
	// recovered-error/recovered-panic path (see hasCFFFallback).
	for f := range funcs {
		if hasCFFFallback(f) {
			b.logger.Debugw("processCFFEnqueueCall: found fallback-guarded task", "function", f.String())
			return map[*ssa.Function]struct{}{}, nil
		}
	}
	return funcs, nil
}

// _cffTaskErrorRecoveredMethod and _cffTaskPanicRecoveredMethod are the go.uber.org/cff/scheduler.TaskEmitter
// methods the inline codegen calls only on a fallback-guarded task's recovered-error/recovered-panic
// path (as opposed to TaskError/TaskPanic, called when there is no cff.FallbackWith(...) and the
// error/panic instead propagates) -- see hasCFFFallback.
const (
	_cffTaskErrorRecoveredMethod = "TaskErrorRecovered"
	_cffTaskPanicRecoveredMethod = "TaskPanicRecovered"
)

// hasCFFFallback reports whether fn's body (a cff task's run closure under the inline codegen
// shape) calls one of the TaskEmitter methods that only fire on a cff.FallbackWith(...) recovery
// path, meaning fn is fallback-guarded.
func hasCFFFallback(fn *ssa.Function) bool {
	for _, blk := range fn.Blocks {
		for _, instr := range blk.Instrs {
			call, ok := instr.(*ssa.Call)
			if !ok {
				continue
			}
			var name string
			if call.Call.Method != nil {
				name = call.Call.Method.Name()
			} else if callee := call.Call.StaticCallee(); callee != nil {
				name = callee.Name()
			}
			if name == _cffTaskErrorRecoveredMethod || name == _cffTaskPanicRecoveredMethod {
				return true
			}
		}
	}
	return false
}

// _cffJobRunFieldName is the field on go.uber.org/cff/scheduler.Job (aliased cff.Job) that carries
// the task closure. The struct's other field, Dependencies, holds pointers to *other*,
// already-enqueued tasks' *ScheduledJob values -- walking it would transitively pull in unrelated
// sibling tasks' closures, so cffJobRunFieldSeeds deliberately looks up only the Run field by name
// instead of handing the whole cff.Job value to the generic field-tracing helpers.
const _cffJobRunFieldName = "Run"

// cffJobRunFieldSeeds locates, among an Enqueue call's arguments, the *ssa.FieldAddr instances
// that address the cff.Job.Run field, to be used as seeds for findCFFTaskFuncs.
func cffJobRunFieldSeeds(args []ssa.Value) []ssa.Value {
	var seeds []ssa.Value
	for _, arg := range args {
		root := resolveFieldRoot(arg)
		alloc, ok := root.(*ssa.Alloc)
		if !ok {
			continue
		}
		ptr, ok := alloc.Type().Underlying().(*types.Pointer)
		if !ok {
			continue
		}
		st, ok := ptr.Elem().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		runField := -1
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i).Name() == _cffJobRunFieldName {
				runField = i
				break
			}
		}
		if runField == -1 {
			continue
		}
		refs := alloc.Referrers()
		if refs == nil {
			continue
		}
		for _, ref := range *refs {
			if fa, ok := ref.(*ssa.FieldAddr); ok && fa.Field == runField {
				seeds = append(seeds, fa)
			}
		}
	}
	return seeds
}

// addRelevantStoreTypeRefs adds the relevant store type references to the stack.
func addRelevantStoreTypeRefs(refs *[]ssa.Instruction, stack []ssa.Value, function *ssa.Function) []ssa.Value {
	if refs == nil {
		return stack
	}

	for _, r := range *refs {
		if r.Parent() != function {
			continue
		}

		switch instr := r.(type) {
		case *ssa.Store:
			stack = append(stack, instr.Val)

		case *ssa.IndexAddr:
			stack = append(stack, instr)

		case *ssa.FieldAddr:
			stack = append(stack, instr)
		}

	}
	return stack
}

// resolveFieldRoot unwraps a chain of loads (*ssa.UnOp) to find the base pointer value a
// FieldAddr's base was ultimately loaded from, so callers can find *other* FieldAddr instances
// derived from the same underlying object (see findFieldAddrsForField).
func resolveFieldRoot(v ssa.Value) ssa.Value {
	for {
		unop, ok := v.(*ssa.UnOp)
		if !ok {
			return v
		}
		v = unop.X
	}
}

// findFieldAddrsForField walks forward from root (a pointer-typed value) through any chain of
// loads (*ssa.UnOp) and collects every *ssa.FieldAddr targeting the given field index that is
// (transitively, through loads) derived from root.
func findFieldAddrsForField(root ssa.Value, field int, seen map[ssa.Value]bool) []*ssa.FieldAddr {
	if seen[root] {
		return nil
	}
	seen[root] = true
	refs := root.Referrers()
	if refs == nil {
		return nil
	}
	var out []*ssa.FieldAddr
	for _, ref := range *refs {
		switch r := ref.(type) {
		case *ssa.FieldAddr:
			if r.Field == field {
				out = append(out, r)
			}
		case *ssa.UnOp:
			out = append(out, findFieldAddrsForField(r, field, seen)...)
		}
	}
	return out
}

func ssaValueVisitor(v ssa.Value, stack []ssa.Value, function *ssa.Function) []ssa.Value {
	switch val := v.(type) {
	case *ssa.Alloc:
		// Need to include all stores to this alloc.
		stack = addRelevantStoreTypeRefs(val.Referrers(), stack, function)
	case *ssa.Global:
		// Add all referrers to the global
		stack = addRelevantStoreTypeRefs(val.Referrers(), stack, function)
	case *ssa.IndexAddr:
		// Need to get all stores to the address
		stack = addRelevantStoreTypeRefs(val.Referrers(), stack, function)
	case *ssa.FieldAddr:
		// Need to get all stores to the field address -- this is how a struct-literal field
		// (e.g. the "Run" closure in "cff.Job{Run: closure}", the current inline-codegen shape's
		// task-registration pattern) gets populated, as opposed to being passed as a direct call
		// argument (the older codegen shape's pattern, which needs no field-store tracing at all).
		stack = addRelevantStoreTypeRefs(val.Referrers(), stack, function)
		// The write may go through a *different* FieldAddr instance than the one being read here,
		// if the read and write each derive the base struct pointer through separate loads of a
		// shared alloc/free-variable -- e.g. a captured "taskN := new(struct{...})" mutated via
		// "taskN.run = ..." and later read back via a fresh load when building
		// "cff.Job{Run: taskN.run}". Walk back to the common root pointer and re-scan every
		// FieldAddr instance (transitively) derived from it for the same field index.
		root := resolveFieldRoot(val.X)
		for _, fa := range findFieldAddrsForField(root, val.Field, map[ssa.Value]bool{}) {
			stack = addRelevantStoreTypeRefs(fa.Referrers(), stack, function)
		}

	case *ssa.MakeClosure:
		// Only the closure's own function is a task/flow candidate. Its captured bindings are
		// just data -- one of them can itself be a closure captured from an outer scope (e.g. the
		// inline-codegen shape's per-task wrapper closure closes over the user's real task
		// closure as a bound parameter), and walking into it via the generic operand-walk below
		// would spuriously surface that captured closure as a second "found" function alongside
		// the wrapper itself.
		stack = append(stack, val.Fn)

	default:
		// Append all operands of the instruction to the stack.
		if instr, ok := val.(ssa.Instruction); ok {
			var operands []*ssa.Value
			operands = instr.Operands(nil)
			for _, operand := range operands {
				if operand != nil && *operand != nil {
					stack = append(stack, *operand)
				}
			}
		}
	}
	return stack
}

// findCFFTaskFuncs finds the functions that are being passed as the first argument to the CFF task callsite.
func findCFFTaskFuncs(v []ssa.Value,
	function *ssa.Function,
	funcFilter func(*ssa.Function) bool) (map[*ssa.Function]struct{}, error) {
	funcs := map[*ssa.Function]struct{}{}
	visited := make(map[ssa.Value]bool)
	stack := append([]ssa.Value{}, v...)

	for len(stack) > 0 {
		// Pop the last element
		n := len(stack) - 1
		current := stack[n]
		stack = stack[:n]

		if current == nil || visited[current] {
			continue
		}

		visited[current] = true
		switch val := current.(type) {
		case *ssa.Function:
			if funcFilter(val) {
				funcs[val] = struct{}{}
			}
		case ssa.CallInstruction:
			// Add all operands
			c := val.Common()
			// Add the value
			if c.Value != nil {
				stack = append(stack, c.Value)
			}
			stack = append(stack, c.Args...)
		default:
			stack = ssaValueVisitor(val, stack, function)
		}
	}
	return funcs, nil
}

// getCFFTaskCallsites returns the callsites that are of CFF task calling patterns.
func getCFFTaskCallsites(v []ssa.Value,
	function *ssa.Function,
	siteFilter func(ssa.CallInstruction) bool) (map[ssa.CallInstruction]struct{}, error) {
	taskIns := map[ssa.CallInstruction]struct{}{}
	visited := make(map[ssa.Value]bool)
	stack := append([]ssa.Value{}, v...)

	for len(stack) > 0 {
		// Pop the last element
		n := len(stack) - 1
		current := stack[n]
		stack = stack[:n]

		if current == nil || visited[current] {
			continue
		}
		visited[current] = true

		switch val := current.(type) {
		case ssa.CallInstruction:
			if siteFilter(val) {
				taskIns[val] = struct{}{}
			}
		default:
			stack = ssaValueVisitor(val, stack, function)
		}
	}
	return taskIns, nil
}
