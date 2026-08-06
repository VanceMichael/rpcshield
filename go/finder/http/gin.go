package http

import (
	"go/types"

	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/grail"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
)

// ginRouteMethods are the *gin.RouterGroup methods that register a route where
// the first argument (after the receiver) is the relative path.
var ginRouteMethods = map[string]bool{
	"GET":     true,
	"POST":    true,
	"PUT":     true,
	"DELETE":  true,
	"PATCH":   true,
	"HEAD":    true,
	"OPTIONS": true,
	"Any":     true,
}

// GinProvider implements the MuxProvider interface for github.com/gin-gonic/gin.
//
// It recognises engines created via gin.New()/gin.Default() and routes registered
// via the RouterGroup verb methods (GET/POST/.../Any) as well as Handle. gin's
// handlers are func(*gin.Context), so it uses a gin-specific handler-signature
// matcher while reusing the shared HandlerParser unwrapping machinery.
//
// Known limitation: routes registered on a sub-group returned by Group("/prefix")
// are detected, but the group's prefix is NOT prepended to the leaf path (gin
// stores the accumulated prefix on the returned *RouterGroup value, which is not
// resolved here). The relative path is reported as-is.
type GinProvider struct {
	baseProvider

	handlerParser  *HandlerParser
	endpointParser *EndpointParser
}

// NewGinProvider creates a new GinProvider instance.
func NewGinProvider(logger *zap.SugaredLogger, service grail.Service, codebase *fty.Codebase) *GinProvider {
	return &GinProvider{
		baseProvider: baseProvider{
			logger:   logger,
			service:  service,
			codebase: codebase,
		},
		handlerParser:  NewHandlerParserWithMatcher(logger, codebase, isGinHandlerFuncSignatureRelaxed),
		endpointParser: NewEndpointParser(UnknownEndpoint),
	}
}

var _ MuxProvider = (*GinProvider)(nil)

// MatchMuxRouterType checks if the value is a *gin.Engine or *gin.RouterGroup.
// gin verb methods are defined on *RouterGroup; calling them on an *Engine goes
// through the embedded RouterGroup field, so both must be recognised as routers.
func (p *GinProvider) MatchMuxRouterType(val ssa.Value) bool {
	switch val.Type().String() {
	case "*github.com/gin-gonic/gin.Engine", "*github.com/gin-gonic/gin.RouterGroup":
		return true
	}
	return false
}

// MatchMuxCreation checks if the instruction is a gin.New() or gin.Default() call.
func (p *GinProvider) MatchMuxCreation(instr ssa.Instruction) bool {
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

	if fn.Pkg.Pkg.Path() != "github.com/gin-gonic/gin" {
		return false
	}

	return fn.Name() == "New" || fn.Name() == "Default"
}

// MatchHandlerRegistration checks if the instruction is a gin route registration
// (GET/POST/.../Any/Handle) and extracts the inbound point(s).
func (p *GinProvider) MatchHandlerRegistration(instr ssa.Instruction) (ib []fty.InboundPoint, ok bool) {
	call, ok := instr.(*ssa.Call)
	if !ok {
		return nil, false
	}

	fn, ok := call.Call.Value.(*ssa.Function)
	if !ok {
		return nil, false
	}

	recv := fn.Signature.Recv()
	if recv == nil {
		return nil, false
	}

	// Only match methods whose receiver is gin's RouterGroup/Engine so we don't
	// accidentally match a same-named method (GET/Handle/...) on another type.
	if s := recv.Type().String(); s != "*github.com/gin-gonic/gin.RouterGroup" && s != "*github.com/gin-gonic/gin.Engine" {
		return nil, false
	}

	name := fn.Name()
	switch {
	case ginRouteMethods[name]:
		// verb(relativePath string, handlers ...HandlerFunc)
		// SSA args: (receiver, relativePath, variadicHandlerSlice)
		return p.extractInbound(call, 1, 2)
	case name == "Handle":
		// Handle(httpMethod, relativePath string, handlers ...HandlerFunc)
		// SSA args: (receiver, httpMethod, relativePath, variadicHandlerSlice)
		return p.extractInbound(call, 2, 3)
	}

	return nil, false
}

// extractInbound pulls the endpoint from pathIdx and the real handler from the
// variadic handler slice at sliceIdx.
func (p *GinProvider) extractInbound(call *ssa.Call, pathIdx, sliceIdx int) ([]fty.InboundPoint, bool) {
	if len(call.Call.Args) <= sliceIdx {
		return nil, false
	}

	endpoint := p.endpointParser.ParseEndpointWithDefault(call.Call.Args[pathIdx])

	handlerArg, ok := lastVariadicHandler(call.Call.Args[sliceIdx])
	if !ok {
		return nil, false
	}

	handlers, err := p.handlerParser.ParseHandleFuncs(handlerArg)
	if err != nil {
		p.logger.Debugf("Could not extract gin handler for %s (%T): %v", handlerArg.String(), handlerArg, err)
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

// lastVariadicHandler extracts the final handler from a variadic ...HandlerFunc
// argument. In gin, any leading handlers are middleware and the terminal one is
// the endpoint handler. The SSA shape of `verb("/x", mw, h)` is a slice over a
// freshly allocated array with an IndexAddr+Store per element; we return the
// value stored at the highest constant index.
func lastVariadicHandler(v ssa.Value) (ssa.Value, bool) {
	slice, ok := v.(*ssa.Slice)
	if !ok {
		return nil, false
	}

	alloc, ok := slice.X.(*ssa.Alloc)
	if !ok {
		return nil, false
	}

	var best ssa.Value
	bestIdx := int64(-1)

	refs := alloc.Referrers()
	if refs == nil {
		return nil, false
	}

	for _, r := range *refs {
		idxAddr, ok := r.(*ssa.IndexAddr)
		if !ok {
			continue
		}

		idxConst, ok := idxAddr.Index.(*ssa.Const)
		if !ok {
			continue
		}
		idx := idxConst.Int64()

		iaRefs := idxAddr.Referrers()
		if iaRefs == nil {
			continue
		}
		for _, ir := range *iaRefs {
			store, ok := ir.(*ssa.Store)
			if !ok || store.Addr != idxAddr {
				continue
			}
			if idx > bestIdx {
				bestIdx = idx
				best = store.Val
			}
		}
	}

	if best == nil {
		return nil, false
	}
	return best, true
}

// isGinContextType checks if the type is *github.com/gin-gonic/gin.Context
func isGinContextType(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}

	named, ok := ptr.Elem().(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == "github.com/gin-gonic/gin" && obj.Name() == "Context"
}

// isGinHandlerFuncSignatureRelaxed checks if the signature is a valid gin handler
// signature using the following heuristic:
//
//	Signature must contain at least a *gin.Context parameter
//
// gin's HandlerFunc is func(*gin.Context); the relaxed check tolerates extra
// parameters for parity with the net/http handler heuristic.
func isGinHandlerFuncSignatureRelaxed(sig *types.Signature) bool {
	params := sig.Params()
	for i := 0; i < params.Len(); i++ {
		if isGinContextType(params.At(i).Type()) {
			return true
		}
	}
	return false
}
