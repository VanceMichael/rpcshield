package types

import (
	"encoding/json"
	"go/token"
	"go/types"

	"github.com/uber-research/rpcshield/go/grail"
	"golang.org/x/tools/go/packages"
)

type InboundType = EndpointType

// InboundPoint marks the package and the function that implements the declared service endpoint.
type InboundPoint struct {
	// Pkg is the declaring package.
	Pkg *packages.Package
	// Func is the function object that implements the declared service endpoint.
	Func *types.Func
	// Service is the service that the endpoint belongs to.
	Service grail.Service
	// Type is the type of the inbound call(e.g., "grpc", "thrift", "http").
	Type InboundType
	// CanonicalName is the canonical name of the inbound endpoint (by concatenating the canonical
	// service name and the endpoint name in lower case), e.g.,
	// "uber.infra.progsys.cffdemo.cffdemo__create".
	CanonicalName string
}

type inboundPointRepr struct {
	PkgPath       string         `json:"pkg_path"`
	Func          string         `json:"func"`
	Position      token.Position `json:"position"`
	Service       grail.Service  `json:"service"`
	Type          string         `json:"type,omitempty"`
	CanonicalName string         `json:"canonical_name"`
}

func (i InboundPoint) toRepr() inboundPointRepr {
	return inboundPointRepr{
		PkgPath:       i.Pkg.PkgPath,
		Position:      i.Pkg.Fset.Position(i.Func.Pos()),
		Func:          i.Func.Name(),
		Service:       i.Service,
		Type:          string(i.Type),
		CanonicalName: i.CanonicalName,
	}
}

// MarshalJSON converts the InboundPoint to human-readable JSON for debugging purposes only. It
// cannot be converted back to InboundPoint.
func (i InboundPoint) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.toRepr())
}

// MergeInboundPoints merges two slices of InboundPoint with deduplication.
func MergeInboundPoints(a, b []InboundPoint) []InboundPoint {
	seen := make(map[inboundPointRepr]bool, len(a)+len(b))
	merged := make([]InboundPoint, 0, len(a)+len(b))
	for _, points := range [2][]InboundPoint{a, b} {
		for _, p := range points {
			repr := p.toRepr()
			if !seen[repr] {
				merged = append(merged, p)
			}
			seen[repr] = true
		}
	}

	return merged
}
