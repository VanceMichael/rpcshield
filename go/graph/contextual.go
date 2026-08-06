package graph

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// Arbitrary constants for contextual refinement.
// These are heuristics and can be tuned.
const threshold = 0.1
const _thresholdInvokeProbability = 0.5
const _toomanycallees = 10
const _contextRefinementThreshold = 0.1
const _contextRefinementMaxIter = 5
const _recvrAndImplProximity = 3.0
const _ifaceDefAndImplProximity = 2.0
const _ifaceCallerAndImplProximity = 1.0
const _suffixMatchMultiplier = 1.0

type callsiteMap map[ssa.CallInstruction][]*callgraph.Edge
type edgeSet map[*callgraph.Edge]struct{}

// Checks if the function takes an argument of matchWith signature,
// or a struct whose field contains a function with the signature matchWith.
func (b *builderImpl) funcPassesLambdaWithSignature(function *ssa.Function, matchWith *types.Signature) bool {
	for _, param := range function.Params {
		paramType := param.Type()

		// Check if the parameter type is a function signature.
		if sig, ok := paramType.Underlying().(*types.Signature); ok {
			if b.signaturesIdentical(matchWith, sig) {
				b.logger.Debugf("Found parameter with matching signature: %s", sig.String())
				return true
			}
		}

		// If the parameter is a struct, check its fields.
		derefedParam := deref(paramType)
		if structType, ok := derefedParam.Underlying().(*types.Struct); ok {
			if b.structHasFieldWithSignature(structType, matchWith) {
				b.logger.Debugf("Found struct field with matching signature")
				return true
			}
		}
	}
	b.logger.Debugf("No matching parameter found for function %s", function.Name())
	return false
}

// Helper function to check if a struct has a field whose type is a function signature matching sigType.
func (b *builderImpl) structHasFieldWithSignature(structType *types.Struct, sigType *types.Signature) bool {
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		fieldType := deref(field.Type())

		if sig, ok := fieldType.Underlying().(*types.Signature); ok {
			if b.signaturesIdentical(sigType, sig) {
				b.logger.Debugf("Found field with matching signature: %s", sig.String())
				return true
			}
		}
	}
	return false
}

// signaturesIdentical checks if two function signatures are identical.
// TODO: may be we can use type hash to compare the signatures.
func (b *builderImpl) signaturesIdentical(sig1, sig2 *types.Signature) bool {
	// Use types.Identical to compare the signatures structurally
	v := types.Identical(sig1, sig2)
	return v

}

// If there are many callees from the same callsite, check if a single callee is a lambda embedded in the static callchain.
// return the edges to be deleted.
/*
   \. |. / stop here since it explodes.
      | A()
      | B()
   /. |  \ walk from here up
 A.1() X() Y()
 Starting from A.1() walk up to B() and A() and check if the callee is a lambda. Here, A.1() is a lambda nested inside A() and we can prune X() and Y() and only retain A.1().
*/
func (b *builderImpl) calleeIsAClosureInCallchain(callerNode *callgraph.Node, csMap callsiteMap, calleeThreshold int) edgeSet {
	toDelete := edgeSet{}
	// for trivially simple case, return immediately.
	if len(callerNode.Out) <= 1 {
		return edgeSet{}
	}

	// Get all callsites from callerNode.Out.
	localMap := callsiteMap{}
	for _, outEdge := range callerNode.Out {
		localMap[outEdge.Site] = csMap[outEdge.Site]
	}

	for callsite, edges := range localMap {
		if len(edges) <= 1 {
			b.logger.Debugf("calleeIsAClosureInCallchain: skipping callsite %s with %d callees few edges", callsite.String(), len(edges))
			continue
		}
		// If it is an invoke, skip because we don't invoke a lambda.
		if callsite.Common().IsInvoke() {
			continue
		}

		// Check if there are too many callees.
		if len(edges) < calleeThreshold {
			continue
		}

		parentFuncs := map[*ssa.Function]struct{}{}
		parentFuncs = b.getNonRecursiveSingleParentCallChainHavingTypeArg(callerNode, parentFuncs, callsite.Common().Signature())
		if len(parentFuncs) == 0 {
			continue
		}

		for k := range parentFuncs {
			b.logger.Debugf("Parent func: %s", k.Name())
		}

		mayBeDroppedEdges := edgeSet{}
		for _, edge := range edges {
			if edge.Callee.Func == nil {
				continue
			}
			calleeParentScope := edge.Callee.Func.Parent()
			if _, ok := parentFuncs[calleeParentScope]; !ok {
				mayBeDroppedEdges[edge] = struct{}{}
			}
		}
		// Since we are here because there are too many callees > calleeThreshold. The following cases arise.
		// 1. All are in the call chain. Well, do nothing.
		// 2. Very few (say 1) is in the call chain. Drop the rest.
		// 3. Most are in the call chain, say 9, some are not. Well, drop the one not in the call chain.
		// 4. Some are in the call chain, say 5, some are not. Well, drop them optimistically.

		// If all are in the call chain, do nothing.
		if len(mayBeDroppedEdges) == len(edges) {
			// can't drop all
			b.logger.Debugf("calleeIsAClosureInCallchain: all callees are in the call chain, skipping")
			continue
		}

		// Case 2, 3, 4:
		retainEdges := []*callgraph.Edge{}
		for _, edge := range edges {
			if _, ok := mayBeDroppedEdges[edge]; ok {
				b.logger.Debugf("calleeIsAClosureInCallchain: dropping edge from %s to %s", edge.Caller.Func.Name(), edge.Callee.Func.Name())
				toDelete[edge] = struct{}{}
			} else {
				b.logger.Debugf("calleeIsAClosureInCallchain: retaining edge from %s to %s", edge.Caller.Func.Name(), edge.Callee.Func.Name())
				retainEdges = append(retainEdges, edge)
			}
		}
		csMap[callsite] = retainEdges
	}
	return toDelete
}

func (b *builderImpl) getNonRecursiveSingleParentCallChainHavingTypeArg(callerNode *callgraph.Node, visited map[*ssa.Function]struct{}, t *types.Signature) map[*ssa.Function]struct{} {
	if callerNode == nil {
		b.logger.Debugf("Caller node is nil")
		return visited
	}
	// Ensure no recursion.
	if _, ok := visited[callerNode.Func]; ok {
		b.logger.Debugf("Recursion detected at %s", callerNode.Func.Name())
		return visited
	}

	visited[callerNode.Func] = struct{}{}

	if !b.funcPassesLambdaWithSignature(callerNode.Func, t) {
		// Done. This is not a lambda passer.
		return visited
	}
	b.logger.Debugf("Function %s gets lambda with signature", callerNode.Func.String())

	// have a single caller.
	if len(callerNode.In) == 1 {
		return b.getNonRecursiveSingleParentCallChainHavingTypeArg(callerNode.In[0].Caller, visited, t)
	}
	return visited
}

// Prune low probability invokes.
func (b *builderImpl) pruneLowProbabilityInvokes(cg *callgraph.Graph, csMap callsiteMap, minNeededinvokeProbability float64) int {
	toDelete := edgeSet{}

	for site, edges := range csMap {
		if !site.Common().IsInvoke() {
			continue
		}
		edgeProbs := b.getProbabilitiesAtCallsite(cg, site, edges)
		if len(edgeProbs) != len(edges) {
			b.logger.Debugf("Mismatch in probabilities and edges")
			panic("Mismatch in probabilities and edges")
		}
		// update csMap with the new edges
		newEdges := []*callgraph.Edge{}
		for i, edge := range edges {
			if !b.isDontPruneEdge(edge) && (edgeProbs[i] < minNeededinvokeProbability) {
				b.logger.Debugf("pruneLowProbabilityInvokes: dropping edge from %v to %v with prob %v", edge.Caller.Func, edge.Callee.Func, edgeProbs[i])
				toDelete[edge] = struct{}{}
			} else {
				newEdges = append(newEdges, edge)
			}
		}
		csMap[site] = newEdges
	}
	// Delete the edges.
	b.makeEdgesNilInGraph(cg, toDelete)
	b.cleanupNilsInGraph(cg)
	return len(toDelete)
}

func (b *builderImpl) unwrapHelper(fn *ssa.Function) (*ssa.Function, error) {
	unwrapped, err := b.unwrapper.Unwrap(fn)
	if err != nil {
		return nil, fmt.Errorf("Failed to unwrap callee %s. Reason: %s", fn, err)
	}

	if unwrapped.Pkg == nil || unwrapped.Pkg.Pkg == nil {
		return nil, fmt.Errorf("Unwrapped callee %s has nil package", unwrapped)
	}
	return unwrapped, nil
}

// Get the probabilities for each callee at a callsite.
func (b *builderImpl) getProbabilitiesAtCallsite(cg *callgraph.Graph, callInst ssa.CallInstruction, edges []*callgraph.Edge) []float64 {
	if !callInst.Common().IsInvoke() {
		panic("Not an invoke instruction")
	}

	b.logger.Debugf("Getting probabilities for callsite %s in func %s", callInst.String(), callInst.Parent().String())
	callCommon := callInst.Common()
	// Special case of a single callee.
	if len(edges) == 1 {
		return []float64{1.0}
	}

	edgeScores := make(map[*callgraph.Edge]float64)

	for _, edge := range edges {
		edgeScores[edge] = 0.0

		// Note. Wrappers (e.g., $thunk, $bound) and instantiated generics have nil package.
		//
		// Try to get the underlying function to perform analysis on.
		resolvedCalleeFn, err := b.unwrapHelper(edge.Callee.Func)
		if err != nil {
			b.logger.Debugf("Failed to unwrap function %s. Reason: %s", edge.Callee.Func, err)
			continue
		}

		// Heuristic 1: Proximity of Interface Definition and Implementation
		// 3 types of proximity:
		// 1. Interface definition vs. implementation package (1st highest weight)
		// 2. Receiver interface type vs. implementation package (2nd  highest weight)
		// 3. Caller package vs. implementation package (3rd highest weight)
		methodPkg, ok := getMethodPkg(callCommon)
		if !ok {
			b.logger.Debugf("Failed to get method package for %s", callCommon.String())
			continue
		}
		recvrPkg, ok := getReceiverPkgPath(callCommon)
		if !ok {
			b.logger.Debugf("Failed to get receiver package for %s", callCommon.String())
			continue
		}

		// Again, the parent can also be wrapped
		resolvedCallerFn := callInst.Parent()
		if resolvedCallerFn == nil {
			b.logger.Debugf("callInst.Parent() is nil")
			continue
		}
		resolvedCallerFn, err = b.unwrapHelper(resolvedCallerFn)
		if err != nil {
			b.logger.Debugf("Failed to unwrap function %s. Reason: %s", resolvedCalleeFn, err)
			continue
		}

		implPkgPath := resolvedCalleeFn.Pkg.Pkg.Path()
		callerPkgPath := resolvedCallerFn.Pkg.Pkg.Path()

		recvrVsImplPkg := computeCommonPrefixPercentage(recvrPkg, implPkgPath) * _recvrAndImplProximity
		defVsImplPkg := computeCommonPrefixPercentage(methodPkg, implPkgPath) * _ifaceDefAndImplProximity
		callerVsImplPkg := computeCommonPrefixPercentage(callerPkgPath, implPkgPath) * _ifaceCallerAndImplProximity

		edgeScores[edge] += max(recvrVsImplPkg, defVsImplPkg, callerVsImplPkg)

		// TODO: Heuristic 2: FX Constructor Pattern
		// A. This package imports go.uber.org/fx
		// B. This package has a constructor function that takes the interface as an argument
		// C. The impl package imports go.uber.org/fx
		// D. The impl package has a constructor function that returns the interface
		// if callerImportsFx && callerHasConstructor && importsFX(edge.Callee.Func.Pkg) &&
		// 	hasConstructorReturningInterface(edge.Callee.Func.Package(), ifaceType) {
		// 	edgeScores[edge] += 2.0
		// }
	}

	// Normalize Scores to probabilities.
	maxScore := 0.0
	for _, score := range edgeScores {
		if score < 0 {
			score = 0
		}
		maxScore = max(maxScore, score)
	}

	if maxScore == 0 {
		maxScore = 1 // To prevent division by zero
	}

	edgeProbabilities := make([]float64, len(edges))
	for i, e := range edges {
		edgeProbabilities[i] = edgeScores[e] / maxScore
	}
	return edgeProbabilities
}

// Get the interface type from the call instruction
func getMethodPkg(callInst *ssa.CallCommon) (string, bool) {
	if callInst.Method == nil {
		return "", false
	}

	if callInst.Method.Pkg() == nil {
		return "", false
	}
	return callInst.Method.Pkg().Path(), true
}

// Get receiver type pkg path
func getReceiverPkgPath(callInst *ssa.CallCommon) (string, bool) {
	if callInst.Value == nil {
		return "", false
	}
	typ := callInst.Value.Type()
	if namedType, ok := typ.(*types.Named); ok {
		if namedType.Obj() == nil || namedType.Obj().Pkg() == nil {
			return "", false
		}
		pkg := namedType.Obj().Pkg().Path()
		return pkg, true
	}
	return "", false
}

// Compute the common prefix percentage between two package paths.
func computeCommonPrefixPercentage(pkgPath1, pkgPath2 string) float64 {
	parts1 := strings.Split(pkgPath1, "/")
	parts2 := strings.Split(pkgPath2, "/")
	minLen := min(len(parts1), len(parts2))
	matchCount := 0
	for i := 0; i < minLen; i++ {
		if parts1[i] == parts2[i] {
			matchCount++
		} else {
			break
		}
	}

	// Max of the total number of segments.
	totalSegments := max(len(parts1), len(parts2))

	if totalSegments == 0 {
		return 0.0
	}

	percentageMatch := float64(matchCount) / float64(totalSegments)
	return percentageMatch
}

// Get the map of callsites to callees.
func (b *builderImpl) buildCallsiteMap(cg *callgraph.Graph) callsiteMap {
	csMap := callsiteMap{}
	for _, n := range cg.Nodes {
		for _, edge := range n.Out {
			if edge.Site == nil {
				continue
			}
			csMap[edge.Site] = append(csMap[edge.Site], edge)
		}
	}
	return csMap
}

// Do the contextual refinement once.
func (b *builderImpl) doContextualRefinementOnce(cg *callgraph.Graph, csMap callsiteMap, toomanyCallsiteThreshold int) int {
	edgesToDelete := edgeSet{}
	for _, n := range cg.Nodes {
		tmp := b.calleeIsAClosureInCallchain(n, csMap, toomanyCallsiteThreshold)
		for k, v := range tmp {
			edgesToDelete[k] = v
		}
	}
	// Delete the edges.
	b.makeEdgesNilInGraph(cg, edgesToDelete)
	b.cleanupNilsInGraph(cg)
	return len(edgesToDelete)
}

// Do the contextual refinement.
func (b *builderImpl) doContextualRefinement(cg *callgraph.Graph, csMap callsiteMap, toomanyCallsiteThreshold int) {
	// Visit each node and do the contextual refinement.
	// Iterative refinement. Do it until there are no more refinements or we hit a threshold.
	// This is a very simple heuristic. We can do better.
	lastDeleted := 0
	for i := 0; i < _contextRefinementMaxIter; i++ {
		deletedCnt := b.doContextualRefinementOnce(cg, csMap, toomanyCallsiteThreshold)
		if float64(deletedCnt) < _contextRefinementThreshold*float64(lastDeleted) {
			break
		}
		lastDeleted = deletedCnt
	}
}

// Mark deleted edges as nil in the graph for cleanup.
func (b *builderImpl) makeEdgesNilInGraph(cg *callgraph.Graph, edges edgeSet) {
	for _, n := range cg.Nodes {
		for i, edge := range n.Out {
			if _, ok := edges[edge]; ok {
				n.Out[i] = nil
			}
		}
		for i, edge := range n.In {
			if _, ok := edges[edge]; ok {
				n.In[i] = nil
			}
		}
	}
}

// Perform context refinement.
func (b *builderImpl) doContextualGraphRefinement(cg *callgraph.Graph) {
	csMap := b.buildCallsiteMap(cg)
	b.doContextualRefinement(cg, csMap, _toomanycallees)
	b.pruneLowProbabilityInvokes(cg, csMap, _thresholdInvokeProbability)
}
