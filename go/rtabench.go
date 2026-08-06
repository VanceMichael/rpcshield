package rpcshield

import (
	"slices"
	"time"

	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/graph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// runCGConstructionOnly runs CG construction-only evaluation for benchmarking.
// It builds the call graph without outbound detection or path analysis.
func (d *Driver) runCGConstructionOnly(
	pkgs []*packages.Package,
	ssaProg *ssa.Program,
	ssaPkgs []*ssa.Package,
	isGlue bool,
	entryPkg *packages.Package,
	inbounds []fty.InboundPoint,
	serviceName string,
	result *Result,
) error {
	d.Logger.Infow("CG construction-only mode enabled, skipping outbound detection and path analysis")

	// Continue even if len(inbounds) == 0 since CG construction adds main as root.
	// If inbounds are available, they'll be added as additional roots in getCG().

	// Create empty outbounds list (required by graph builder)
	emptyOutbounds := []fty.OutboundPoint{}

	start := time.Now()
	// Parse RTA bench flavors and num workers
	rtaBenchFlavors, err := graph.ParseRTAFlavors(d.opts.RTABenchFlavors)
	if err != nil {
		return err
	}
	rtaBenchNumWorkers, err := graph.ParseRTANumWorkers(d.opts.RTABenchNumWorkers)
	if err != nil {
		return err
	}
	d.Logger.Infow("RTA bench configuration", "flavors", rtaBenchFlavors, "num_workers", rtaBenchNumWorkers)

	graphConfig := graph.Config{
		DisableSCCPAnalysis:     d.opts.DisableSCCPAnalysis,
		DisableRelaxedContext:   slices.Contains(d.opts.Config.GraphDisableContextRelaxServices, serviceName),
		DisableParallelCallTree: d.opts.DisableParallelCallTree,
		RTABenchFlavors:         rtaBenchFlavors,
		RTABenchNumWorkers:      rtaBenchNumWorkers,
		CheckIsomorphism:        d.opts.CheckIsomorphism,
	}
	d.Logger.Infow("Graph config", "config", graphConfig)

	b := graph.NewBuilder(d.Logger, pkgs, ssaProg, ssaPkgs, d.SSAASTBidirMap, d.SigASTBidirMap, isGlue, entryPkg, inbounds, emptyOutbounds, true, graphConfig)
	d.Logger.Infow("Starting CG construction-only evaluation")

	cgResult, err := b.BuildCGOnly()
	d.Logger.Infow("Finished CG construction-only evaluation")

	if err != nil {
		d.Logger.Errorf("Error in CG construction: %v", err)
		return err
	}

	// Merge CG construction metrics into result
	for name, v := range cgResult.Metrics {
		result.GraphMetrics[name] += v
	}

	result.RunMetrics[_graphBuildingDurationSecs] += time.Since(start).Seconds()
	d.Logger.Infow("CG construction-only mode completed", "metrics_count", len(cgResult.Metrics))

	return nil
}
