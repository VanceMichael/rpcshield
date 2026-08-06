package graph

import (
	"sort"
	"unsafe"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

// explorationConfig holds configurable thresholds for CCT path explosion mitigation.
type explorationConfig struct {
	// PermutationThreshold is the number of times the same function-set hash must
	// be observed reaching a sink before permutation detection triggers.
	// Once detected, canonical ordering eliminates redundant permutations.
	PermutationThreshold int64

	// MinCoreSize is the minimum number of functions in the permutation core
	// required before registering a commutative group. Smaller cores are
	// unlikely to cause meaningful explosion and may be false positives.
	MinCoreSize int

	// MaxFunctionVisits limits how many times we explore any single function's
	// subtree. When a function is visited more than this threshold, it's marked
	// as a "convergence point" and further visits skip re-exploring its subtree.
	// This detects DAG fan-out where many paths converge at the same function.
	MaxFunctionVisits int64
}

// defaultExplorationConfig returns the default exploration configuration.
func defaultExplorationConfig() explorationConfig {
	return explorationConfig{
		PermutationThreshold: 100,
		MinCoreSize:          3,
		MaxFunctionVisits:    1000, // After N visits, skip re-exploring subtree
	}
}

// explorationState tracks per-root-source exploration state for path explosion mitigation.
type explorationState struct {
	// sinkTrackers tracks per-sink path set hashes for permutation detection.
	sinkTrackers map[*ssa.Function]*sinkTracker

	// commutativeGroups holds detected permutation groups for canonical ordering.
	commutativeGroups []*commutativeGroup

	// config holds the configurable thresholds.
	config explorationConfig

	// functionVisits tracks how many times each function has been visited.
	// High visit counts indicate convergence points in the DAG.
	functionVisits map[*ssa.Function]int64

	// convergencePoints are functions that have been visited too many times.
	// Their subtrees won't be re-explored.
	convergencePoints map[*ssa.Function]bool

	// convergenceSkips counts how many times we skipped due to convergence.
	convergenceSkips int64
}

func newExplorationState() *explorationState {
	return newExplorationStateWithConfig(defaultExplorationConfig())
}

func newExplorationStateWithConfig(cfg explorationConfig) *explorationState {
	return &explorationState{
		sinkTrackers:      make(map[*ssa.Function]*sinkTracker),
		functionVisits:    make(map[*ssa.Function]int64),
		convergencePoints: make(map[*ssa.Function]bool),
		config:            cfg,
	}
}

// --- Permutation explosion detection and pruning ---

// commutativeGroup is a set of functions detected to be interchangeable on paths
// to a specific sink. The canonical ordering eliminates redundant permutations:
// group members may only be visited in ascending rank order on any path.
type commutativeGroup struct {
	members map[*ssa.Function]int // function → canonical rank
}

// setInfo tracks observations of a specific function-set hash reaching a sink.
type setInfo struct {
	count        int64
	firstSeq     []*ssa.Function            // ordered function sequence from the first observed path
	coreSet      map[*ssa.Function]struct{} // accumulated functions at differing positions across observations
	lastCoreSize int                        // coreSet size at last registration, to detect growth
}

// sinkTracker counts how many times each unique function-set reaches a given sink,
// enabling demand-driven detection of permutation explosions.
type sinkTracker struct {
	seenSets   map[uint64]*setInfo
	totalPaths int64 // Total paths to this sink across all function-sets

	// Diagnostic counters
	uniqueSets int64 // Number of unique function-set hashes seen
}

// computePathSetHash computes an order-independent hash of the function set on the path.
// Uses Zobrist-style XOR of mixed pointer values. The hash is commutative so
// different orderings of the same function set produce the same hash.
func computePathSetHash(path map[*ssa.Function]struct{}) uint64 {
	var h uint64
	for fn := range path {
		v := uint64(uintptr(unsafe.Pointer(fn)))
		// Bit mixing: multiply by a large odd constant, then fold
		v *= 0x517CC1B727220A95
		v ^= v >> 32
		h ^= v
	}
	return h
}

// extractPathSequence returns the ordered sequence of callee functions reached
// via indirect or invoke (non-static) callsites on the current call stack.
// Only non-static callsites can have multiple possible callees, so only those
// contribute to permutation — static calls always resolve identically.
func extractPathSequence(curStack []*callgraph.Edge) []*ssa.Function {
	seq := make([]*ssa.Function, 0, len(curStack))
	for _, edge := range curStack {
		if edge == nil || edge.Callee == nil || edge.Callee.Func == nil {
			continue
		}
		// Only include callees reached via non-static (indirect/invoke) callsites.
		if edge.Site != nil && edge.Site.Common().StaticCallee() == nil {
			seq = append(seq, edge.Callee.Func)
		}
	}
	return seq
}

// accumulateCore compares two ordered function sequences and adds functions
// at differing positions into the coreSet. Called incrementally for each new
// observation against the firstSeq, so the coreSet grows to capture all
// functions that ever appear at a different position.
func accumulateCore(first, current []*ssa.Function, coreSet map[*ssa.Function]struct{}) {
	n := len(first)
	if len(current) < n {
		n = len(current)
	}
	for i := 0; i < n; i++ {
		if first[i] != current[i] {
			coreSet[first[i]] = struct{}{}
			coreSet[current[i]] = struct{}{}
		}
	}
}

// trackSinkPath records a path reaching sinkFn and triggers permutation detection
// when the same function-set hash is observed PermutationThreshold times.
// Each observation is compared against the first to incrementally grow the set of
// functions at differing positions (the permutation core). This avoids the fragility
// of comparing only two snapshots, which can miss core members that happened to
// coincide in those two particular orderings.
func (es *explorationState) trackSinkPath(
	sinkFn *ssa.Function,
	path map[*ssa.Function]struct{},
	curStack []*callgraph.Edge,
) {
	if es.config.PermutationThreshold <= 0 {
		return
	}

	tracker := es.sinkTrackers[sinkFn]
	if tracker == nil {
		tracker = &sinkTracker{seenSets: make(map[uint64]*setInfo)}
		es.sinkTrackers[sinkFn] = tracker
	}

	// Increment total paths counter for this sink
	tracker.totalPaths++

	setHash := computePathSetHash(path)
	info := tracker.seenSets[setHash]
	if info == nil {
		// First time seeing this function set — record the sequence.
		tracker.uniqueSets++
		info = &setInfo{
			firstSeq: extractPathSequence(curStack),
			coreSet:  make(map[*ssa.Function]struct{}),
		}
		tracker.seenSets[setHash] = info
	}
	info.count++

	// From the 2nd observation onward, accumulate differing positions into coreSet.
	if info.count > 1 {
		currentSeq := extractPathSequence(curStack)
		accumulateCore(info.firstSeq, currentSeq, info.coreSet)
	}

	// Register when the threshold is reached and the core has grown since
	// last registration. The lastCoreSize guard prevents redundant re-registration.
	if info.count >= es.config.PermutationThreshold &&
		len(info.coreSet) >= es.config.MinCoreSize &&
		len(info.coreSet) > info.lastCoreSize {
		core := make([]*ssa.Function, 0, len(info.coreSet))
		for fn := range info.coreSet {
			core = append(core, fn)
		}
		info.lastCoreSize = len(info.coreSet)
		es.registerCommutativeGroup(core)
	}
}

// ShouldSkipConvergence checks if a function should be skipped because it's a
// convergence point (visited too many times). Returns true if we should skip.
// This must be called BEFORE exploring a function's subtree.
func (es *explorationState) ShouldSkipConvergence(fn *ssa.Function) bool {
	if es.config.MaxFunctionVisits <= 0 {
		return false
	}

	// Already marked as convergence point - skip
	if es.convergencePoints[fn] {
		es.convergenceSkips++
		return true
	}

	// Track visit and check threshold
	es.functionVisits[fn]++
	if es.functionVisits[fn] > es.config.MaxFunctionVisits {
		es.convergencePoints[fn] = true
		// Allow the triggering visit to complete so we explore the subtree at least
		// MaxFunctionVisits+1 times before cutting off. Future visits will skip.
	}

	return false
}

// GetConvergencePoints returns the functions identified as convergence points.
func (es *explorationState) GetConvergencePoints() []*ssa.Function {
	result := make([]*ssa.Function, 0, len(es.convergencePoints))
	for fn := range es.convergencePoints {
		result = append(result, fn)
	}
	return result
}

// ConvergenceSkips returns the number of times we skipped due to convergence.
func (es *explorationState) ConvergenceSkips() int64 {
	return es.convergenceSkips
}

// registerCommutativeGroup creates a commutative group from the detected core
// functions and assigns canonical ranks sorted by function name.
func (es *explorationState) registerCommutativeGroup(core []*ssa.Function) {
	// Sort by function string for deterministic canonical ordering.
	sort.Slice(core, func(i, j int) bool {
		return core[i].String() < core[j].String()
	})
	group := &commutativeGroup{members: make(map[*ssa.Function]int, len(core))}
	for rank, fn := range core {
		group.members[fn] = rank
	}
	es.commutativeGroups = append(es.commutativeGroups, group)
}

// shouldSkipPermutation returns true if visiting callee would violate the
// canonical ordering of any detected commutative group. The rule is:
// if a group member with a higher rank is already on the current path,
// a lower-ranked member must not be visited (it would create a permutation
// of an already-explored ordering).
func (es *explorationState) shouldSkipPermutation(
	path map[*ssa.Function]struct{},
	callee *ssa.Function,
) bool {
	for _, group := range es.commutativeGroups {
		if group.shouldSkip(path, callee) {
			return true
		}
	}
	return false
}

// shouldSkip returns true if visiting callee violates this group's canonical ordering.
func (cg *commutativeGroup) shouldSkip(
	path map[*ssa.Function]struct{},
	callee *ssa.Function,
) bool {
	calleeRank, inGroup := cg.members[callee]
	if !inGroup {
		return false
	}
	// If any group member already on the path has a higher rank, skip callee.
	for fn := range path {
		if rank, ok := cg.members[fn]; ok && rank > calleeRank {
			return true
		}
	}
	return false
}

// CommutativeGroupCount returns the number of commutative groups detected.
func (es *explorationState) CommutativeGroupCount() int {
	return len(es.commutativeGroups)
}

// PermutationThresholdHitPerSink returns a map from sink function name to whether any
// function-set observation count reached the permutation threshold for that sink.
// If sinkNames is provided, it maps *ssa.Function to a custom name (e.g., canonical name);
// otherwise, Function.String() is used.
func (es *explorationState) PermutationThresholdHitPerSink(sinkNames map[*ssa.Function]string) map[string]bool {
	result := make(map[string]bool)
	for sinkFn, tracker := range es.sinkTrackers {
		hit := false
		for _, info := range tracker.seenSets {
			if info.count >= es.config.PermutationThreshold && len(info.coreSet) >= es.config.MinCoreSize {
				hit = true
				break
			}
		}
		name := sinkFn.String()
		if sinkNames != nil {
			if cn, ok := sinkNames[sinkFn]; ok {
				name = cn
			}
		}
		result[name] = hit
	}
	return result
}

// Metrics returns a SourceExplorationMetrics summarizing this exploration state.
func (es *explorationState) Metrics(sinkNames map[*ssa.Function]string) SourceExplorationMetrics {
	return SourceExplorationMetrics{
		ConvergencePoints:     len(es.convergencePoints),
		ConvergenceSkips:      es.convergenceSkips,
		CommutativeGroups:     es.CommutativeGroupCount(),
		PermutationHitPerSink: es.PermutationThresholdHitPerSink(sinkNames),
	}
}

// DiagnosticSummary represents diagnostic statistics about exploration.
type DiagnosticSummary struct {
	SinkName     string
	TotalPaths   int64
	UniqueSets   int64
	MaxPerSet    int64
	AvgPerSet    float64
	CommutGroups int
	CoreSizes    []int // Size of core sets for sets that hit threshold
}

// GetDiagnostics returns diagnostic summaries for all tracked sinks.
func (es *explorationState) GetDiagnostics() []DiagnosticSummary {
	var summaries []DiagnosticSummary
	for sinkFn, tracker := range es.sinkTrackers {
		summary := DiagnosticSummary{
			SinkName:     sinkFn.String(),
			TotalPaths:   tracker.totalPaths,
			UniqueSets:   tracker.uniqueSets,
			CommutGroups: len(es.commutativeGroups),
		}

		var maxCount int64
		var totalCount int64
		for _, info := range tracker.seenSets {
			totalCount += info.count
			if info.count > maxCount {
				maxCount = info.count
			}
			if info.count >= es.config.PermutationThreshold && len(info.coreSet) >= es.config.MinCoreSize {
				summary.CoreSizes = append(summary.CoreSizes, len(info.coreSet))
			}
		}
		summary.MaxPerSet = maxCount
		if tracker.uniqueSets > 0 {
			summary.AvgPerSet = float64(totalCount) / float64(tracker.uniqueSets)
		}
		summaries = append(summaries, summary)
	}
	return summaries
}
