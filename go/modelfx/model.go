package model

import (
	"fmt"
	"go/types"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer is struct to hold fx analysis.
type Analyzer struct {
	Logger *zap.SugaredLogger
}

const (
	// fxPackagePath is the import path of the fx package.
	_fxPackagePath   = "go.uber.org/fx"
	_fxPackagePrefix = "go.uber.org/fx."
	_invokeKW        = "Invoke"
	_provideKW       = "Provide"
	_inKW            = "In"
	_outKW           = "Out"
)

// NewAnalyzer creates a new Analyzer.
func NewAnalyzer(logger *zap.SugaredLogger) (*Analyzer, error) {
	return &Analyzer{
		Logger: logger,
	}, nil
}

/*
Sample input program:

package main

import (
	"context"
	"net/http"

	"go.uber.org/fx"
)

// Logger interface
type Logger interface {
	Log(message string)
}

// SimpleLogger implementation
type SimpleLogger struct {
	Prefix string
}

func (l *SimpleLogger) Log(message string) {

}

// Database interface
type Database interface {
	Connect() string
}

// SimpleDatabase implementation
type SimpleDatabase struct {
	Logger Logger
}

func (db *SimpleDatabase) Connect() string {
	return ""
}

// NewLogger provides an instance of SimpleLogger
func NewLogger() Logger {
	return &SimpleLogger{Prefix: "INFO"}
}

// NewDatabase provides an instance of SimpleDatabase
func NewDatabase(logger Logger) Database {
	return &SimpleDatabase{
		Logger: logger,
	}
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(logger Logger, db Database) *http.Server {
	mux := http.NewServeMux()
	logger.Log(db.Connect()) // Log the database connection
	return &http.Server{
		Addr:    "8080",
		Handler: mux,
	}
}

// StartHTTPServer starts the HTTP server.
func StartHTTPServer(lc fx.Lifecycle, server *http.Server, logger Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Log("Starting HTTP server")
			go server.ListenAndServe()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Log("Stopping HTTP server")
			return server.Shutdown(ctx)
		},
	})
}

func main() {
	app := fx.New(
		fx.Provide(
			NewLogger,
			NewDatabase,
			NewHTTPServer,
		),
		fx.Invoke(
			StartHTTPServer,
		),
	)

	app.Run()
}

SSA for main

Function: main
# Name: yy.main
# Package: yy
func main():
0:                                                                entry P:0 S:0
	t0 = new [3]interface{} (varargs)                       *[3]interface{}
	t1 = &t0[0:int]                                            *interface{}
	t2 = make interface{} <- func() Logger (NewLogger)          interface{}
	*t1 = t2
	t3 = &t0[1:int]                                            *interface{}
	t4 = make interface{} <- func(logger Logger) Database (NewDatabase) interface{}
	*t3 = t4
	t5 = &t0[2:int]                                            *interface{}
	t6 = make interface{} <- func(logger Logger, db Database) *net/http.Server (NewHTTPServer) interface{}
	*t5 = t6
	t7 = slice t0[:]                                          []interface{}
	t8 = go.uber.org/fx.Provide(t7...)                go.uber.org/fx.Option
	t9 = new [1]interface{} (varargs)                       *[1]interface{}
	t10 = &t9[0:int]                                           *interface{}
	t11 = make interface{} <- func(lc go.uber.org/fx.Lifecycle, server *net/http.Server, logger Logger) (StartHTTPServer) interface{}
	*t10 = t11
	t12 = slice t9[:]                                         []interface{}
	t13 = go.uber.org/fx.Invoke(t12...)               go.uber.org/fx.Option
	t14 = new [2]go.uber.org/fx.Option (varargs)  *[2]go.uber.org/fx.Option
	t15 = &t14[0:int]                                *go.uber.org/fx.Option
	*t15 = t8
	t16 = &t14[1:int]                                *go.uber.org/fx.Option
	*t16 = t13
	t17 = slice t14[:]                              []go.uber.org/fx.Option
	t18 = go.uber.org/fx.New(t17...)                    *go.uber.org/fx.App
	t19 = (*go.uber.org/fx.App).Run(t18)                                 ()


*/

// findFunctions backward slices the value v to find all functions it depends on.
// for an input of the form `fx.Provide(NewLogger, NewDatabase, NewHTTPServer)`, it will return
// []*ssa.Function{NewLogger, NewDatabase, NewHTTPServer} in the example shown above.
// It is expected that the caller invokes this for an fx.Provide/Invoke call passing the
// argument to the call. Also, the argument to the call is expected to be a slice of interfaces.
func findFunctions(v ssa.Value) ([]*ssa.Function, error) {
	var funcs []*ssa.Function
	visited := make(map[ssa.Value]bool)
	stack := []ssa.Value{v}

	// Basic check that v is a slice of interfaces.
	if t, ok := v.Type().Underlying().(*types.Slice); !ok {
		return funcs, fmt.Errorf("expected a slice of interfaces, got %v", v.Type())
	} else if _, ok := t.Elem().Underlying().(*types.Interface); !ok {
		return funcs, fmt.Errorf("expected a slice of interfaces, got %v", v.Type())
	}

	for len(stack) > 0 {
		// Pop the last element
		n := len(stack) - 1
		current := stack[n]
		stack = stack[:n]

		if visited[current] {
			continue
		}
		visited[current] = true

		switch val := current.(type) {
		case *ssa.Function:
			// TODO: if it is a function call to fx package, skip it for now
			if val.Pkg != nil && val.Pkg.Pkg.Path() == _fxPackagePath {
				// We need to handle fx.Annotate etc here.
				continue
			}

			funcs = append(funcs, val)

		case *ssa.MakeInterface:
			stack = append(stack, val.X)

		case *ssa.Slice:
			stack = append(stack, val.X)

		case *ssa.Alloc:
			if val.Referrers() != nil {
				for _, ref := range *val.Referrers() {
					switch instr := ref.(type) {
					case *ssa.Store:
						stack = append(stack, instr.Val)

					case *ssa.IndexAddr:
						stack = append(stack, instr)
					}
				}
			}

		case *ssa.Const:
			// Ignore constants

		case *ssa.Phi:
			stack = append(stack, val.Edges...)

		case *ssa.UnOp:
			stack = append(stack, val.X)

		case *ssa.Convert:
			stack = append(stack, val.X)

		case *ssa.Extract:
			stack = append(stack, val.Tuple)

		case *ssa.FieldAddr:
			stack = append(stack, val.X)

		case *ssa.IndexAddr:
			if val.Referrers() != nil {
				for _, ref := range *val.Referrers() {
					if store, ok := ref.(*ssa.Store); ok {
						stack = append(stack, store.Val)
					}
				}
			}

		default:
			return nil, fmt.Errorf("unhandled type: %T", val)
		}
	}

	return funcs, nil
}

// importsFx checks if the package imports the fx package.
func importsFx(pkg *ssa.Package) bool {
	if pkg == nil {
		return false
	}
	if pkg.Pkg == nil {
		return false
	}
	for _, importedPkg := range pkg.Pkg.Imports() {
		if importedPkg.Path() == _fxPackagePath {
			return true
		}
	}
	return false
}

// getFxInOutHashes returns the hash of fx.In and fx.Out types.
func getFxInOutHashes(ssaPkgs []*ssa.Package, typeHasher *typeutil.Hasher) (uint32, uint32, error) {
	var fxInHash, fxOutHash uint32

	// Precompute the types of fx.In and fx.Out.
	for _, pkg := range ssaPkgs {
		if pkg.Pkg.Path() == _fxPackagePath {
			for _, mem := range pkg.Members {
				obj := mem.Object()
				if obj, ok := obj.(*types.TypeName); ok {
					switch obj.Name() {
					case _inKW:
						fxInHash = typeHasher.Hash(obj.Type())
					case _outKW:
						fxOutHash = typeHasher.Hash(obj.Type())
					}
				}
			}
			return fxInHash, fxOutHash, nil
		}
	}
	return fxInHash, fxOutHash, fmt.Errorf("fx.In/fx.Out not found")
}

// ExtractParams extracts the structs embedding fx.In or fx.Out.
// For example, the following struct:
//
//	type HandlerParams struct {
//		fx.In
//		Logger *Logger
//	}
func (a *Analyzer) ExtractParams(ssaPkgs []*ssa.Package) (map[*types.Named]struct{}, map[*types.Named]struct{}) {
	// Map to hold structs embedding fx.In or fx.Out.
	fxIns := make(map[*types.Named]struct{})
	fxOuts := make(map[*types.Named]struct{})

	// Type utilities for comparing types.
	typeHasher := typeutil.MakeHasher()
	fxInHash, fxOutHash, err := getFxInOutHashes(ssaPkgs, &typeHasher)
	if err != nil {
		a.Logger.Infow("Error getting fx.In/fx.Out hashes", "error", zap.Error(err))
		return fxIns, fxOuts
	}

	// Find Structs with In and Out in each package.
	for _, ssaPkg := range ssaPkgs {
		// Ignore the fx package itself.
		if ssaPkg.Pkg.Path() == _fxPackagePath {
			continue
		}

		// Skip packages that don't import fx.
		if !importsFx(ssaPkg) {
			continue
		}

		// Analyze types to find structs embedding fx.In or fx.Out.
		scope := ssaPkg.Pkg.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}

			// We're interested in named types.
			typeName, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}

			named, ok := typeName.Type().(*types.Named)
			if !ok {
				continue
			}

			structType, ok := named.Underlying().(*types.Struct)
			if !ok {
				continue
			}

			// Iterate over struct fields.
			for i := 0; i < structType.NumFields(); i++ {
				field := structType.Field(i)
				if !field.Anonymous() {
					continue
				}

				// Get the type of the anonymous field.
				fieldType := field.Type()
				hash := typeHasher.Hash(fieldType)

				if hash == fxInHash {
					// Found a struct with fx.In anonymous field.
					fxIns[named] = struct{}{}
					break
				}
				if hash == fxOutHash {
					// Found a struct with fx.Out anonymous field.
					fxOuts[named] = struct{}{}
					break
				}
			}
		}
	}

	return fxIns, fxOuts
}

// ExtractFxProvideAndInvoke extracts the functions passed to fx.Provide and fx.Invoke.
func (a *Analyzer) ExtractFxProvideAndInvoke(allSSAPkgs []*ssa.Package) (map[*ssa.Function]struct{}, map[*ssa.Function]struct{}) {
	providers := make(map[*ssa.Function]struct{})
	invokes := make(map[*ssa.Function]struct{})

	// Iterate over all packages and functions.
	for _, ssaPkg := range allSSAPkgs {
		// Skip the fx package itself.
		if ssaPkg.Pkg != nil && ssaPkg.Pkg.Path() == _fxPackagePath {
			continue
		}
		for _, mem := range ssaPkg.Members {
			fn, ok := mem.(*ssa.Function)
			if !ok {
				continue
			}
			for _, block := range fn.Blocks {
				for _, instr := range block.Instrs {
					callInstr, ok := instr.(*ssa.Call)
					if !ok {
						continue
					}

					callee := callInstr.Call.StaticCallee()
					if callee == nil || callee.Pkg == nil {
						continue
					}

					// Skip calls to functions calling the fx package.
					if callee.Pkg.Pkg.Path() != _fxPackagePath {
						continue
					}

					var useMap map[*ssa.Function]struct{}
					switch callee.Name() {
					case _provideKW:
						useMap = providers
					case _invokeKW:
						useMap = invokes
					default:
						// TODO: Handle fx.Annotate etc.
						continue
					}

					// Make sure only one argument is passed to fx.Provide/fx.Invoke.
					if len(callInstr.Call.Args) != 1 {
						a.Logger.Infow("Expected one argument to fx.Provide/fx.Invoke", "function", fn.Name(), "args", callInstr.Call.Args)
						continue
					}
					arg := callInstr.Call.Args[0]
					funcs, err := findFunctions(arg)
					if err != nil {
						a.Logger.Infow("Error finding functions", "error", zap.Error(err))
					} else {
						for _, f := range funcs {
							useMap[f] = struct{}{}
						}
					}
				}
			}
		}
	}
	return providers, invokes
}

// CallSiteInfo holds data about the function that takes address of a function.
type CallSiteInfo struct {
	// Function name string.
	EnclosingFunction *ssa.Function
	// Call site position.
	CallSite ssa.Instruction
}

// isExported checks if a name is exported (starts with an uppercase letter).
func isExported(name string) bool {
	if name == "" {
		return false
	}
	return strings.ToUpper(string(name[0])) == string(name[0])
}

// AllAddrTaken is a heavy hammer approach to get all address-taken functions in a non-fx package that
// includes the fx package. It filters by the function wrapping the address-taken to have at least one fx-type operand.
// For now it just dumps the result.
// TODO: refine and return the result.
func (a *Analyzer) AllAddrTaken(ssaPkgs []*ssa.Package, prog *ssa.Program) map[*ssa.Function][]CallSiteInfo {
	// Map to store functions whose addresses are taken.
	addressTakenFuncs := make(map[*ssa.Function][]CallSiteInfo)

	// Iterate over all packages.
	for _, ssaPkg := range ssaPkgs {
		pkgPath := ssaPkg.Pkg.Path()

		// Skip the fx package.
		if pkgPath == _fxPackagePath {
			continue
		}

		// Check if the package imports fx.
		importsFx := false
		for _, imp := range ssaPkg.Pkg.Imports() {
			if imp.Path() == _fxPackagePath {
				importsFx = true
				break
			}
		}

		if !importsFx {
			continue
		}

		// Iterate over all functions in the package.
		for _, mem := range ssaPkg.Members {
			fn, ok := mem.(*ssa.Function)
			if !ok {
				continue
			}

			// Skip synthetic functions.
			if fn.Synthetic != "" {
				continue
			}

			foundFxTypeInFunc := false
			addTakenInFunc := make(map[*ssa.Function][]CallSiteInfo)
			// Walk all instructions in the function.
			for _, block := range fn.Blocks {
				for _, instr := range block.Instrs {
					// Get the operands of the instruction.
					operands := instr.Operands(nil)
					for _, operand := range operands {
						if operand == nil || *operand == nil {
							continue
						}
						// Check if the operand is an fx type.
						if !foundFxTypeInFunc {
							if t := (*operand).Type(); t != nil {
								if strings.HasPrefix(t.String(), _fxPackagePrefix) {
									foundFxTypeInFunc = true
								}
							}
						}

						if funcVal, ok := (*operand).(*ssa.Function); ok {
							// Check if it's not a method.
							if funcVal.Signature.Recv() != nil {
								continue
							}

							// Check if the function is defined in the same package.
							if funcVal.Pkg != ssaPkg {
								continue
							}

							// Check if the function is exported (publicly visible).
							if !isExported(funcVal.Name()) {
								continue
							}

							info := CallSiteInfo{
								EnclosingFunction: fn,
								CallSite:          instr,
							}
							addTakenInFunc[funcVal] = append(addTakenInFunc[funcVal], info)
						}
					}
				}
			} // end of walking instructions
			// If we found an fx type in the function, add the functions whose addresses are taken.
			if foundFxTypeInFunc {
				for k, v := range addTakenInFunc {
					addressTakenFuncs[k] = append(addressTakenFuncs[k], v...)
				}
			}
		} // end of walking members
	}
	return addressTakenFuncs
}
