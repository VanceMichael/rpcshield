package graph

import (
	"go/types"

	"github.com/uber-research/rpcshield/go/common/utils"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var _contextContext = pkgNamePair{
	pkg:  "context",
	name: "Context",
}

var _syncLocker = pkgNamePair{
	pkg:  "sync",
	name: "Locker",
}

var _builtinError = pkgNamePair{
	pkg:  "builtin",
	name: "error",
}

type pkgNamePair struct {
	pkg  string
	name string
}

// populateNeededTypes populates the neededTypes map with the types from the given program.
func (g *builderImpl) populateNeededTypes(prog *ssa.Program) {
	for _, pkg := range prog.AllPackages() {
		if pkg == nil || pkg.Pkg == nil || pkg.Pkg.Path() == "" {
			continue
		}
		curPath := pkg.Pkg.Path()
		for k, v := range g.neededTypes {
			if v != nil {
				continue
			}
			if curPath != k.pkg {
				continue
			}
			obj := pkg.Pkg.Scope().Lookup(k.name)
			if obj == nil || obj.Type() == nil {
				g.logger.Fatalf("no type!")
			}

			iface, ok := obj.Type().Underlying().(*types.Interface)
			if !ok {
				g.logger.Fatalf("not an interface!")
			}
			g.neededTypes[k] = iface
		}
	}
	// builtin.error is a special case, as it is not in any package.
	// manually add it. It has an Error() method that returns a string.
	g.neededTypes[_builtinError] = types.NewInterfaceType([]*types.Func{
		types.NewFunc(0, nil, "Error", types.NewSignature(nil, nil, types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.String])), false)),
	}, nil)
}

// isDeferCall checks if the ssa.CallInstruction is a deferred call
func (g *builderImpl) isDeferCall(instr ssa.CallInstruction) bool {
	// Check if the instruction is of type *ssa.Defer
	if _, ok := instr.(*ssa.Defer); ok {
		return true
	}
	return false
}

// isGoCall checks if the ssa.CallInstruction is a Go call
func (g *builderImpl) isGoCall(instr ssa.CallInstruction) bool {
	// Check if the instruction is of type *ssa.Defer
	if _, ok := instr.(*ssa.Go); ok {
		return true
	}
	return false
}

// isErrorMethodCall checks if the ssa.CallInstruction is a method invocation
// where the method is named "Error", takes no parameters (other than the receiver),
// and returns a string (i.e., invocation site of builtin.error.Error)
func (g *builderImpl) isErrorMethodCall(instr ssa.CallInstruction) bool {
	call := instr.Common()

	// Get the method being invoked
	method := call.Method
	if call.IsInvoke() {
		// Check that the method name is "Error"
		if method.Name() != "Error" {
			return false
		}
		if call.Signature() == nil {
			g.logger.Debugf("No value or signature")
			return false
		}
		// if there is a receiver, check if it implements the builtin.error interface
		if call.Value == nil {
			return false
		}
		return g.typeImplements(call.Value.Type(), g.neededTypes[_builtinError])
	}
	// Non invoke mode. Check the signature of the call to make sure its name is Error and it returns a string
	if call.Value == nil {
		return false
	}
	// Value will be a *ssa.Function
	// Convert it to a *ssa.Function
	fn, ok := call.Value.(*ssa.Function)
	if !ok {
		return false
	}
	// Check if the function name is Error
	if fn.Name() != "Error" {
		return false
	}
	// Check if the function returns a string
	s := fn.Signature
	return s != nil && s.Params().Len() == 0 && s.Results().Len() == 1 && s.Results().At(0).Type() == types.Typ[types.String]
}

// deref returns the underlying type if it's a pointer, else the type itself.
func deref(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// callSendsContext checks if the call site sends a context.Context as an argument.
func (g *builderImpl) callSendsContext(c *ssa.CallCommon) bool {
	sig := c.Signature()
	if sig == nil {
		return false
	}
	return g.hasTypeInArguments(sig, g.neededTypes[_contextContext])
}

func (g *builderImpl) invokesHelper(call *ssa.CallCommon, neededType *types.Interface) bool {
	// Check if the call is an invoke (method call)
	if !call.IsInvoke() {
		return false
	}

	// Get the method being invoked
	method := call.Method
	if method == nil {
		return false
	}

	found := false
	// Check if the method.Name() matches one of the method names in sync.Locker
	for i := 0; i < neededType.NumMethods(); i++ {
		if neededType.Method(i).Name() == method.Name() {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	// Check the receiver implements the _syncLocker
	return g.typeImplements(call.Value.Type(), neededType)
}

func (g *builderImpl) invokesContext(call ssa.CallInstruction) bool {
	contextContext := g.neededTypes[_contextContext]
	return g.invokesHelper(call.Common(), contextContext)
}

// invokesMutex checks if the call site invokes the Lock/Unlock method on a sync.Mutex.
func (g *builderImpl) invokesLocker(call ssa.CallInstruction) bool {
	locker := g.neededTypes[_syncLocker]
	return g.invokesHelper(call.Common(), locker)
}

// callReturnsError checks if the call site returns a builtin.error.
func (g *builderImpl) callReturnsError(c *ssa.CallCommon) bool {
	return g.checkReturnContainsType(c.Signature(), g.neededTypes[_builtinError])
}

// sendsCtxtAndReturnErr checks if the call site is likely to be a downstream RPC call.
func (g *builderImpl) sendsCtxtAndReturnErr(site ssa.CallInstruction, callee *ssa.Function) bool {
	c := site.Common()

	if g.config.DisableRelaxedContext {
		return g.callSendsContext(c) && g.callReturnsError(c)
	}

	// Context may be sent via call arguments, callee receiver fields, or captured free vars.
	ctxtOK := g.callOrCalleeCarriesType(c, callee, g.neededTypes[_contextContext])
	// Error may be returned directly, present in callee receiver fields, or captured free vars.
	errOK := g.callOrCalleeReturnsOrCapturesType(c, callee, g.neededTypes[_builtinError])
	return ctxtOK && errOK
}

// callOrCalleeCarriesType returns true if either the call site carries the expected type
// in its arguments/receiver signature or the callee function carries it via its receiver
// or captured free variables.
func (g *builderImpl) callOrCalleeCarriesType(c *ssa.CallCommon, callee *ssa.Function, expectedType *types.Interface) bool {
	if c != nil && g.hasTypeInArguments(c.Signature(), expectedType) {
		return true
	}
	return g.funcStateContainsType(callee, expectedType)
}

// callOrCalleeReturnsOrCapturesType returns true if the call signature returns the
// expected type (directly or within a struct per checkReturnContainsType) or the
// callee carries the expected type via receiver fields or captured free variables.
func (g *builderImpl) callOrCalleeReturnsOrCapturesType(c *ssa.CallCommon, callee *ssa.Function, expectedType *types.Interface) bool {
	if c != nil && g.checkReturnContainsType(c.Signature(), expectedType) {
		return true
	}
	return g.funcStateContainsType(callee, expectedType)
}

// funcStateContainsType checks callee receiver fields and free vars for the expected type.
func (g *builderImpl) funcStateContainsType(f *ssa.Function, expectedType *types.Interface) bool {
	if f == nil {
		return false
	}
	if g.funcReceiverContainsType(f, expectedType) {
		return true
	}
	if g.funcFreeVarsContainType(f, expectedType) {
		return true
	}
	return false
}

// funcReceiverContainsType returns true if the callee receiver implements/contains expectedType.
func (g *builderImpl) funcReceiverContainsType(f *ssa.Function, expectedType *types.Interface) bool {
	if f == nil || f.Signature == nil || f.Signature.Recv() == nil {
		return false
	}
	recvType := f.Signature.Recv().Type()
	// If receiver is (pointer to) struct, check its fields.
	// Note: Direct implementation by receiver type does NOT qualify.
	// That is, g.typeImplements(recvType, expectedType) is skipped.
	if s, ok := g.asAStructType(recvType); ok {
		if g.structContainsIFaceField(s, expectedType) {
			return true
		}
	}
	return false
}

// funcFreeVarsContainType returns true if any captured variable implements/contains expectedType.
func (g *builderImpl) funcFreeVarsContainType(f *ssa.Function, expectedType *types.Interface) bool {
	if f == nil {
		return false
	}
	for _, fv := range f.FreeVars {
		// Since free variables are captured by reference, we need to check if the type implements the expectedType.
		// We need to dereference the type to check if it implements the expectedType.
		t := deref(fv.Type())
		if g.typeImplements(t, expectedType) {
			return true
		}
		if s, ok := g.asAStructType(t); ok && g.structContainsIFaceField(s, expectedType) {
			return true
		}
	}
	return false
}

// asAStructType checks if the given type is a struct or a pointer to a struct and returns the struct type.
func (g *builderImpl) asAStructType(t types.Type) (*types.Struct, bool) {
	// Check if the type is a pointer to a struct
	if ptr, ok := t.(*types.Pointer); ok {
		if s, ok := ptr.Elem().Underlying().(*types.Struct); ok {
			return s, true
		}
	}
	// Check if the type is a struct
	if s, ok := t.Underlying().(*types.Struct); ok {
		return s, true
	}
	return nil, false
}

// hasTypeInArguments checks if the given signature has a type that implements the expectedType either as a direct
// parameter or as a field in a struct parameter.
// If the expectedType is found in a struct parameter, it returns true.
// If the expectedType is found in a non-struct parameter, it returns true only if there are more than one parameters.
// If the expectedType is not found, it returns false.
// If the receiver is not nil, it checks if the receiver struct contains the expectedType field.
func (g *builderImpl) hasTypeInArguments(sig *types.Signature, expectedType *types.Interface) bool {
	for i := 0; i < sig.Params().Len(); i++ {
		paramType := sig.Params().At(i).Type().Underlying()
		// Check if paramType implements expectedType.
		if g.typeImplements(paramType, expectedType) {
			g.logger.Debugf("hasTypeInArguments passed typeImplements, parmas = %v\n", sig.Params().Len())
			return sig.Params().Len() > 1
		}

		structType, ok := g.asAStructType(paramType)
		if !ok {
			continue
		}
		if g.structContainsIFaceField(structType, expectedType) {
			return true
		}
	}

	if receiver := sig.Recv(); receiver != nil && receiver.Type() != nil {
		derefedReceiver, ok := g.asAStructType(receiver.Type())
		if !ok {
			return false
		}
		return g.structContainsIFaceField(derefedReceiver, expectedType)
	}
	return false
}

// structContainsIFaceFiled checks if the structType contains a field that implements the expectedType.
func (g *builderImpl) structContainsIFaceField(structType *types.Struct, expectedType *types.Interface) bool {
	for i := 0; i < structType.NumFields(); i++ {
		field := structType.Field(i)
		fieldType := field.Type()

		if g.typeImplements(fieldType, expectedType) {
			return true
		}
	}
	return false
}

// typeImplements checks if the given type implements the expectedType.
func (g *builderImpl) typeImplements(typ types.Type, ifaceType *types.Interface) bool {
	baseType := deref(typ)
	return types.Implements(typ, ifaceType) || types.Implements(baseType, ifaceType) || types.Implements(types.NewPointer(baseType), ifaceType)
}

// checkReturnContainsType checks if the given signature returns a type that implements the expectedType.
// If the last return type implements the expectedType, it returns true.
// If the last return type is a struct that contains a field that implements the expectedType, it returns true.
// If the return type is a struct that contains the expectedType, it returns true.
func (g *builderImpl) checkReturnContainsType(sig *types.Signature, expectedType *types.Interface) bool {
	numResults := sig.Results().Len()
	if numResults == 0 {
		// Allow zero return types for struct receivers.
		// Get the receiver from the signature.
		if sig.Recv() == nil {
			return false
		}
		receiver := sig.Recv().Type()
		structType, ok := g.asAStructType(receiver)
		if !ok {
			// If the receiver is not a struct, return false.
			return false
		}
		return g.structContainsIFaceField(structType, expectedType)
	}

	lastResultType := sig.Results().At(numResults - 1).Type()

	// Check if the last return type implements the expectedType.
	if g.typeImplements(lastResultType, expectedType) {
		return true
	}

	// If not, check if it returns a single struct containing the expectedType.
	if numResults == 1 {
		structType, ok := g.asAStructType(lastResultType)
		if !ok {
			return false
		}
		return g.structContainsIFaceField(structType, expectedType)
	}

	if !g.config.DisableRelaxedContext {
		// Even if there are multiple return values, consider the receiver fields carrying the type
		// as a proxy for the callee producing the type through state.
		if sig.Recv() != nil {
			if s, ok := g.asAStructType(sig.Recv().Type()); ok {
				if g.structContainsIFaceField(s, expectedType) {
					return true
				}
			}
		}
	}

	return false
}

// isImpossibleInvoke checks if the call is an invoke and the target function name does not match the call method name.
func (g *builderImpl) isImpossibleInvoke(call ssa.CallInstruction, target *ssa.Function) bool {
	if call == nil || target == nil {
		return false
	}
	c := call.Common()
	if c.IsInvoke() == false {
		return false
	}

	// Cannot invoke cross-package unexported methods
	callerPkg := call.Parent().Pkg
	targetPkg := target.Pkg
	if callerPkg != targetPkg {
		if !c.Method.Exported() {
			return true
		}
	}
	return c.Method.Name() != target.Name()
}

func (g *builderImpl) getFuncsBlockedByFile(ssaProg *ssa.Program) (map[*ssa.Function]bool, error) {
	result := map[*ssa.Function]bool{}
	for f := range ssautil.AllFunctions(ssaProg) {
		if utils.IsGlueFile(ssaProg.Fset, f.Pos(), g.config.CodebasePkgPrefix) {
			result[f] = true
			curPath := ssaProg.Fset.File(f.Pos()).Name()
			g.logger.Debugf("getFuncsBlockedByFile blocking: %s at path %s", f.String(), curPath)
		}
	}
	return result, nil
}
