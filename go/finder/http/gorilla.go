package http

import (
	"fmt"
	"strings"

	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/grail"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
)

// GorillaMuxProvider implements the Provider interface for the Gorilla mux library.
type GorillaMuxProvider struct {
	baseProvider

	handlerParser  *HandlerParser
	endpointParser *EndpointParser
}

// NewGorillaMuxProvider creates a new GorillaMuxProvider instance.
func NewGorillaMuxProvider(logger *zap.SugaredLogger, service grail.Service, codebase *fty.Codebase) *GorillaMuxProvider {
	handlerParser := NewHandlerParser(logger, codebase)
	endpointParser := NewEndpointParser(UnknownEndpoint)
	return &GorillaMuxProvider{
		baseProvider: baseProvider{
			logger:   logger,
			service:  service,
			codebase: codebase,
		},
		handlerParser:  handlerParser,
		endpointParser: endpointParser,
	}
}

var _ MuxProvider = (*GorillaMuxProvider)(nil)

// MatchMuxRouterType checks if the value is a Gorilla router.
func (p *GorillaMuxProvider) MatchMuxRouterType(val ssa.Value) bool {
	ty := val.Type()
	return ty.String() == "*github.com/gorilla/mux.Router"
}

// MatchMuxCreation checks if the instruction is a Gorilla mux.NewRouter() call.
func (p *GorillaMuxProvider) MatchMuxCreation(instr ssa.Instruction) bool {
	call, ok := instr.(*ssa.Call)
	if !ok {
		return false
	}

	if call.Call.Method != nil {
		return false
	}

	fn, ok := call.Call.Value.(*ssa.Function)
	if !ok {
		return false
	}

	if fn.Pkg == nil {
		return false
	}

	return fn.Pkg.Pkg.Path() == "github.com/gorilla/mux" && fn.Name() == "NewRouter"
}

// MatchHandlerRegistration checks if the instruction is a Handle or HandleFunc method call and extracts the inbound point.
func (p *GorillaMuxProvider) MatchHandlerRegistration(instr ssa.Instruction) (ib []fty.InboundPoint, ok bool) {
	call, ok := instr.(*ssa.Call)
	if !ok {
		return nil, false
	}

	fn, ok := call.Call.Value.(*ssa.Function)
	if !ok {
		return nil, false
	}

	if fn.Signature.Recv() == nil {
		return nil, false
	}

	methodName := fn.Name()

	if methodName == "HandleFunc" {
		return p.extractInboundFromHandleFunc(call)
	}

	return nil, false
}

func (p *GorillaMuxProvider) extractInboundFromHandleFunc(call *ssa.Call) ([]fty.InboundPoint, bool) {
	// 3 because the first argument is the router.
	if len(call.Call.Args) != 3 {
		return nil, false
	}

	endpointArg := call.Call.Args[1]
	handlerArg := call.Call.Args[2]

	endpoint := p.endpointParser.ParseEndpointWithDefault(endpointArg)
	handlers, err := p.handlerParser.ParseHandleFuncs(handlerArg)

	if err != nil {
		p.logger.Warnf("Could not extract handler function for %s : %T. Reason: %v", handlerArg.String(), handlerArg, err)

		var buf strings.Builder
		if x, ok := handlerArg.(*ssa.Function); ok {
			x.WriteTo(&buf)
			fmt.Println(buf.String())
		}

		return nil, false
	}

	ibs := make([]fty.InboundPoint, len(handlers))
	for i, handler := range handlers {
		ibs[i] = fty.InboundPoint{
			Service:       p.service,
			Func:          handler.obj,
			Pkg:           handler.pkg,
			CanonicalName: canonicalName(endpoint, handler),
		}
	}

	return ibs, true
}
