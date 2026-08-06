package types

import (
	"encoding/json"
	"go/ast"
	"go/token"

	"github.com/uber-research/rpcshield/go/grail"
	"golang.org/x/tools/go/packages"
)

type OutboundType = EndpointType

// OutboundPoint marks the package and the call expression that makes an outbound RPC call.
type OutboundPoint struct {
	// Pkg is the package containing the call expression.
	Pkg *packages.Package `json:"-"`
	// Call is the call expression.
	Call *ast.CallExpr `json:"-"`
	// Service marks the service that the call expression is making an outbound call to.
	Service grail.Service `json:"service"`
	// CanonicalName is the canonical name of the outbound endpoint (by concatenating the canonical
	// service name and the endpoint name in lower case), e.g., "uber.infra.progsys.cffdemo.cffdemo_create".
	CanonicalName string `json:"canonical_name"`
	// Type is the type of the outbound call(e.g., "grpc", "thrift", "http").
	Type OutboundType `json:"type,omitempty"`
}

type outboundPointRepr struct {
	PkgPath       string         `json:"pkg_path"`
	Position      token.Position `json:"position"`
	Service       grail.Service  `json:"service"`
	CanonicalName string         `json:"canonical_name"`
	Type          string         `json:"type,omitempty"`
}

func (o OutboundPoint) toRepr() outboundPointRepr {
	return outboundPointRepr{
		PkgPath:       o.Pkg.PkgPath,
		Position:      o.Pkg.Fset.Position(o.Call.Pos()),
		Service:       o.Service,
		CanonicalName: o.CanonicalName,
		Type:          string(o.Type),
	}
}

// MarshalJSON converts the OutboundPoint to human-readable JSON for debugging purposes only. It
// cannot be converted back to OutboundPoint.
func (o OutboundPoint) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.toRepr())
}

// MergeOutboundPoints merges two slices of OutboundPoint with deduplication.
func MergeOutboundPoints(a, b []OutboundPoint) []OutboundPoint {
	seen := make(map[outboundPointRepr]bool, len(a)+len(b))
	merged := make([]OutboundPoint, 0, len(a)+len(b))
	for _, points := range [2][]OutboundPoint{a, b} {
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
