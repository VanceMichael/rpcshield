package http

import (
	"fmt"
	"go/types"

	"github.com/uber-research/rpcshield/go/common/syntheticfunc"
	"github.com/uber-research/rpcshield/go/common/xslice"
	fty "github.com/uber-research/rpcshield/go/finder/types"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

// HandlerParser provides functionality to parse HTTP handler functions from SSA values.
// It handles synthetic functions, wrapped handlers, and interface method resolution.
type HandlerParser struct {
	logger    *zap.SugaredLogger
	codebase  *fty.Codebase
	unwrapper *syntheticfunc.Unwrapper
	// isHandlerSig reports whether a function signature is a valid handler for the
	// framework this parser serves. It lets a single parser be reused across
	// frameworks whose handlers have different signatures (e.g. net/http's
	// func(http.ResponseWriter, *http.Request) vs gin's func(*gin.Context)).
	isHandlerSig func(*types.Signature) bool
}

// NewHandlerParser creates a new HandlerParser with the provided codebase and logger.
// It accepts the standard net/http handler signature (http.ResponseWriter, *http.Request).
func NewHandlerParser(logger *zap.SugaredLogger, codebase *fty.Codebase) *HandlerParser {
	return NewHandlerParserWithMatcher(logger, codebase, isHTTPHandlerFuncSignatureRelaxed)
}

// NewHandlerParserWithMatcher creates a HandlerParser that accepts handlers whose
// signature satisfies isHandlerSig. Framework providers (gin, ...) supply their own
// matcher so the shared unwrapping/interface-resolution machinery can be reused.
func NewHandlerParserWithMatcher(logger *zap.SugaredLogger, codebase *fty.Codebase, isHandlerSig func(*types.Signature) bool) *HandlerParser {
	return &HandlerParser{
		logger:       logger,
		codebase:     codebase,
		unwrapper:    syntheticfunc.NewUnwrapper(logger),
		isHandlerSig: isHandlerSig,
	}
}

// ParseHandleFuncs extracts the handler function from the argument.
func (p *HandlerParser) ParseHandleFuncs(val ssa.Value) ([]handler, error) {
	switch x := val.(type) {
	case *ssa.Function:
		// Handler is an SSA function. We treat this as the leaf. Extract
		// handler funcs from it (there could be multiple if it is an interface
		// method, for instance)
		return p.handlerFuncsFrom(x)
	case *ssa.Call:
		// Handler is wrapped in middleware or helper functions. Use a heuristic to parse the real handler.
		//
		// Example:
		//   wrapLogger(wrapState(handlerFunc)) is unwrapped to handlerFunc
		//
		// The unwrapping is performed by recursively traversing function call arguments that
		// resemble handler functions (i.e., have signatures matching http.HandlerFunc).

		var candidates []([]handler)
		for _, arg := range x.Call.Args {
			funcs, err := p.ParseHandleFuncs(arg)
			if err != nil {
				p.logger.Debugf("Failed to parse handler func from arg %v because %v", arg, err)
				continue
			}
			candidates = append(candidates, funcs)
		}

		if len(candidates) == 1 {
			return candidates[0], nil
		}

		if len(candidates) == 0 {
			// This could be a function call that _makes_ handler funcs
			// Use a heuristic to extract the handler func that is created
			//
			// Example:
			//  func MakeHandler() func(w http.ResponseWriter, r *http.Request) {
			//      return func(w http.ResponseWriter, r *http.Request) {
			//          ...
			//      }
			//  }
			//
			// We only handle the simply case where the body only conatins a
			// return statement that creates the handler

			var sig *types.Signature
			switch s := x.Type().(type) {
			case *types.Signature:
				sig = s
			case *types.Named:
				var ok bool
				sig, ok = s.Underlying().(*types.Signature)
				if !ok {
					break
				}
			}

			if sig != nil {
				if p.isHandlerSig(sig) {
					if body := x.Call.StaticCallee(); body != nil {
						instrs := body.Blocks[0].Instrs
						if ret, ok := instrs[len(instrs)-1].(*ssa.Return); ok {
							handlers, err := p.ParseHandleFuncs(ret.Results[0])
							if err == nil {
								return handlers, nil
							}
						}
					}
				}
			}
		}

		return nil, fmt.Errorf("Could not find a suitable candidate. Available options were %v", candidates)
	case *ssa.MakeClosure:
		// Handler is a closure. Recurse into its body.
		return p.ParseHandleFuncs(x.Fn)
	case *ssa.MakeInterface:
		// Handler was boxed into an interface (e.g. net/http's Handle takes an
		// http.Handler, and http.HandlerFunc(fn) is boxed to it). Recurse into
		// the concrete value being boxed.
		return p.ParseHandleFuncs(x.X)
	case *ssa.ChangeType:
		// Handler is a SSA ChangeType instruction. The compiler automatically
		// inserts these type conversions in some instances.
		return p.ParseHandleFuncs(x.X)
	default:
		return nil, fmt.Errorf("Failed to parse HandleFunc from %v because its types %T is unhandled", val, val)
	}
}

func (p *HandlerParser) handlerFuncsFrom(f *ssa.Function) ([]handler, error) {
	var objs []handler

	rf, err := p.unwrapper.TryUnwrap(f)
	if rf == nil && err != nil {
		return nil, err
	}

	switch x := rf.(type) {
	case *ssa.Function:
		obj := unwrapFuncObj(f)
		if obj == nil {
			return nil, fmt.Errorf("True source function %v returned a nil *types.Func object", f)
		}
		if pkg, ok := p.codebase.PkgMap[x.Pkg.Pkg.Path()]; ok {
			objs = append(objs, handler{
				pkg: pkg,
				obj: obj,
			})
		} else {
			return nil, fmt.Errorf("Could not find package for HTTP handler %v", f)
		}
	case *ssa.Call:
		typ := x.Call.Value.Type()
		if types.IsInterface(typ) {
			iface := typ.Underlying().(*types.Interface)
			methodName := x.Call.Method.Name()
			objs = append(objs, p.findImplementationsOfInterface(iface, methodName)...)
		} else {
			return nil, fmt.Errorf("Expected call to be a dynamic dispatch over an interface but got %v", x)
		}
	}

	// Keep only those funcs that are likely handlers for this framework
	objs = xslice.FilterInPlace(objs, func(h handler) bool {
		sig, ok := h.obj.Type().(*types.Signature)
		return ok && p.isHandlerSig(sig)
	})

	return objs, nil
}

// findImplementationsOfInterface finds all concrete types that implement the given interface
// and returns handlers for the specified method.
func (p *HandlerParser) findImplementationsOfInterface(iface *types.Interface, methodName string) []handler {
	var objs []handler
	var visitedTypes typeutil.Map

	for _, pkg := range p.codebase.CodebaseOnly {
		for _, obj := range pkg.TypesInfo.Defs {
			if obj == nil {
				continue
			}

			t := obj.Type()
			if r := visitedTypes.At(t); r != nil && r.(bool) {
				continue
			}

			if types.Implements(t, iface) && !types.IsInterface(t) {
				obj, err := lookupMethod(t, methodName, pkg.Types)
				if err != nil {
					continue
				}

				objs = append(objs, handler{
					pkg: pkg,
					obj: obj,
				})
			}

			visitedTypes.Set(t, true)
		}
	}

	return objs
}
