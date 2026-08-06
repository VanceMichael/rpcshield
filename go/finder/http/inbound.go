package http

import (
	"fmt"

	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/grail"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// InboundFinder finds HTTP inbound points (endpoints) in Go services.
// It analyzes SSA programs to discover HTTP handler registrations across different HTTP mux implementations.
type InboundFinder struct {
	logger       *zap.SugaredLogger
	muxProviders []MuxProvider
}

// NewInboundFinder creates a new InboundFinder instance.
// muxProviders is a slice of HTTP mux providers that define how to match mux creation and handler registration.
func NewInboundFinder(logger *zap.SugaredLogger, muxProviders []MuxProvider) *InboundFinder {
	return &InboundFinder{
		logger:       logger,
		muxProviders: muxProviders,
	}
}

// Find analyzes an SSA program to discover HTTP inbound points for the given service.
// Returns a slice of InboundPoint structs representing discovered HTTP endpoints.
func (hf *InboundFinder) Find(service grail.Service, ssaProg *ssa.Program) ([]fty.InboundPoint, error) {
	if ssaProg == nil {
		return nil, fmt.Errorf("ssa program is nil")
	}

	var inbounds []fty.InboundPoint
	fs := ssautil.AllFunctions(ssaProg)

	for f := range fs {
		if f.Pkg == nil {
			continue
		}

		for _, b := range f.Blocks {
			for _, instr := range b.Instrs {
				for _, muxProvider := range hf.muxProviders {
					// Match mux creation patterns like:
					// router := mux.NewRouter()
					// mux := http.NewServeMux()
					// gin := gin.New()
					if muxProvider.MatchMuxCreation(instr) {
						referrers := instr.(*ssa.Call).Referrers()
						if referrers == nil || len(*referrers) == 0 {
							continue
						}

						visited := make(map[*ssa.Function]bool)
						visitedVals := make(map[ssa.Value]bool)
						ibs, err := hf.findInboundsFromReferrers(*referrers, muxProvider, visited, visitedVals)
						if err != nil {
							hf.logger.Warnf("Error finding inbounds from mux referrers: %v", err)
							continue
						}
						inbounds = append(inbounds, ibs...)
					}
				}
			}
		}
	}
	// add HTTP type to all inbound points from the finder
	for i := range inbounds {
		inbounds[i].Type = fty.TypeHTTP
	}

	return inbounds, nil
}

// findInboundsFromReferrers recursively processes referrers to find inbound points.
// It handles direct handler registrations, delegated function calls, and router
// values derived from the original router (e.g. an embedded RouterGroup accessed
// via a field, or a sub-group returned by a grouping call).
//
// visited guards against infinite recursion through delegate functions; visitedVals
// guards against re-walking the same derived router value.
func (hf *InboundFinder) findInboundsFromReferrers(referrers []ssa.Instruction, muxProvider MuxProvider, visited map[*ssa.Function]bool, visitedVals map[ssa.Value]bool) ([]fty.InboundPoint, error) {
	var inbounds []fty.InboundPoint

	for _, referrer := range referrers {
		// Match handler registration patterns like:
		// router.HandleFunc("/path", handler)
		// mux.HandleFunc("/path", handler)
		// gin.GET("/health", healthCheck)
		ibs, ok := muxProvider.MatchHandlerRegistration(referrer)
		if ok {
			inbounds = append(inbounds, ibs...)
			continue
		}

		// Follow router-typed derived values. Some frameworks register routes on
		// a value derived from the original router rather than on the router
		// itself. For example, gin verb methods live on *gin.RouterGroup, so a
		// call like engine.GET(...) is lowered to (&engine.RouterGroup).GET(...):
		// the direct referrer of the engine is a FieldAddr, and the registration
		// hangs off that. Sub-groups (grp := engine.Group("/v1")) behave the same
		// way. Whenever a referrer is itself a router-typed value, recurse into
		// its referrers.
		if v, isVal := referrer.(ssa.Value); isVal && !visitedVals[v] && muxProvider.MatchMuxRouterType(v) {
			visitedVals[v] = true
			if vRefs := v.Referrers(); vRefs != nil {
				derivedInbounds, err := hf.findInboundsFromReferrers(*vRefs, muxProvider, visited, visitedVals)
				if err != nil {
					hf.logger.Warnf("Error finding inbounds from derived router value %v: %v", v, err)
				} else {
					inbounds = append(inbounds, derivedInbounds...)
				}
			}
		}

		// Handle delegation case
		//
		// Example:
		//  func addRoutes(m *mux.Router) {
		//      m.HandleFunc("/sub/route", ...)
		//  }
		//
		//  func main() {
		//      m := mux.NewRouter()
		//      m.HandleFunc("/route", ...)
		//      addRoutes(m) // Delegate to addRoutes
		//  }
		call, ok := referrer.(*ssa.Call)
		if !ok {
			continue
		}

		fn := call.Call.StaticCallee()
		if fn == nil {
			continue
		}

		// Prevent infinite recursion. Unlikely to encounter this case, but we
		// must do this to prevent unnecessary timeouts.
		//
		// Example:
		//  func addRoutes(m *mux.Router) {
		//      m.HandleFunc("/sub/route", ...)
		//      addRoutes(m); // Recursion!
		//  }
		if visited[fn] {
			continue
		}
		visited[fn] = true

		for i, arg := range call.Call.Args {
			if muxProvider.MatchMuxRouterType(arg) {
				paramReferrers := fn.Params[i].Referrers()
				if paramReferrers == nil {
					continue
				}

				// Recursively process the delegated function
				delegatedInbounds, err := hf.findInboundsFromReferrers(*paramReferrers, muxProvider, visited, visitedVals)
				if err != nil {
					hf.logger.Warnf("Error finding inbounds from delegated function %s: %v", fn.Name(), err)
					continue
				}
				inbounds = append(inbounds, delegatedInbounds...)
			}
		}

		// Clean up visited map for this branch
		delete(visited, fn)
	}

	return inbounds, nil
}
