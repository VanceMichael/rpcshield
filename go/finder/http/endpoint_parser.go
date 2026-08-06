package http

import (
	"fmt"
	"strconv"

	"golang.org/x/tools/go/ssa"
)

// EndpointParser provides functionality to parse HTTP endpoint strings from SSA values.
type EndpointParser struct {
	unknownDefault string
}

// NewEndpointParser creates a new EndpointParser with the specified default value for unknown endpoints.
func NewEndpointParser(unknownDefault string) *EndpointParser {
	return &EndpointParser{
		unknownDefault: unknownDefault,
	}
}

// ParseEndpoint extracts the endpoint string (if available) from the argument.
// Returns an error if the endpoint could not be extracted.
func (p *EndpointParser) ParseEndpoint(val ssa.Value) (string, error) {
	if e, ok := val.(*ssa.Const); ok {
		return strconv.Unquote(e.Value.ExactString())
	}
	return "", fmt.Errorf("Failed to parse endpoint from %v (of type %T), likely because it is dynamic", val, val)
}

// ParseEndpointOr extracts the endpoint string or returns the default value if extraction fails.
func (p *EndpointParser) ParseEndpointOr(val ssa.Value, defaultValue string) string {
	endpoint, err := p.ParseEndpoint(val)
	if err != nil {
		return defaultValue
	}
	return endpoint
}

// ParseEndpointWithDefault extracts the endpoint string or returns the parser's default value if extraction fails.
func (p *EndpointParser) ParseEndpointWithDefault(val ssa.Value) string {
	endpoint, err := p.ParseEndpoint(val)
	if err != nil {
		return p.unknownDefault
	}
	return endpoint
}
