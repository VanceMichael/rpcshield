package finder

import (
	"fmt"
	"go/ast"
	"go/types"
	"slices"
	"strings"

	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/staticanalyzer/asthelper"
	"golang.org/x/tools/go/packages"
)

// Outbound finds all the call expressions that make outbound RPC calls for a particular service (
// denoted by the entry binary package).
// It returns the list of outbound points, the number of outbound calls that could not be associated with a service, and an error if any.
func (f *Finder) Outbound(entryPkg *packages.Package, allPkgs []*packages.Package) ([]fty.OutboundPoint, map[fty.OutboundType]int, error) {
	pkgMap := make(map[string]*packages.Package)
	outboundMissingServiceCount := make(map[fty.OutboundType]int)
	for _, pkg := range allPkgs {
		if _, ok := pkgMap[pkg.PkgPath]; ok {
			return nil, outboundMissingServiceCount, fmt.Errorf("duplicate package found: %q", pkg.PkgPath)
		}
		pkgMap[pkg.PkgPath] = pkg
	}

	var outbounds []fty.OutboundPoint
	for _, pkg := range allPkgs {
		// Outbound calls are only made inside internal packages.
		if !strings.HasPrefix(pkg.PkgPath, f.codebasePkgPrefix) {
			continue
		}

		for _, file := range pkg.Syntax {
			var calls []*ast.CallExpr
			ast.Inspect(file, func(n ast.Node) bool {
				if callExpr, ok := n.(*ast.CallExpr); ok {
					calls = append(calls, callExpr)
				}
				return true
			})
			for _, callExpr := range calls {
				sel, ok := callExpr.Fun.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				funcObj := pkg.TypesInfo.ObjectOf(sel.Sel)
				if funcObj == nil {
					continue
				}
				if funcObj.Pkg() == nil {
					continue
				}
				sig, ok := funcObj.Type().Underlying().(*types.Signature)
				// Type casts are also call expressions, so the funcObj's type might not be a signature.
				if !ok {
					continue
				}
				recv := sig.Recv()
				if recv == nil {
					continue
				}

				// Unwrap pointer types to get to the underlying named type.
				recvBaseType := asthelper.UnwrapPtr(recv.Type())
				recvType, ok := recvBaseType.(*types.Named)
				if !ok {
					continue
				}

				switch {
				case isGRPCClient(recvType.Obj()) || isThriftClient(recvType.Obj()):
					outbound, err := f.gogoprotoThriftOutbound(funcObj, recvType, callExpr, pkg, pkgMap)
					if err != nil {
						return nil, outboundMissingServiceCount, fmt.Errorf("finding gogoprotoThrift outbound: %w", err)
					}
					if outbound == nil {
						if isGRPCClient(recvType.Obj()) {
							outboundMissingServiceCount[fty.TypeGRPC]++
						} else if isThriftClient(recvType.Obj()) {
							outboundMissingServiceCount[fty.TypeThrift]++
						}
						continue
					}
					outbounds = append(outbounds, *outbound)

				case isCadenceClient(recvType.Obj()):
					outbounds = append(outbounds, f.cadenceOutbound(funcObj, recvType, callExpr, pkg, pkgMap))

				case f.enableGenericGRPCThriftDetection && (isGenericGRPCClient(recvType.Obj()) || isGenericThriftClient(recvType.Obj())):
					outbound, err := f.genericGRPCThriftOutbound(funcObj, recvType, callExpr, pkg, pkgMap)
					if err != nil {
						return nil, outboundMissingServiceCount, fmt.Errorf("finding generic gRPC/Thrift outbound: %w", err)
					}
					if outbound == nil {
						if isGenericGRPCClient(recvType.Obj()) {
							outboundMissingServiceCount[fty.TypeGRPC]++
						} else {
							outboundMissingServiceCount[fty.TypeThrift]++
						}
						continue
					}
					outbounds = append(outbounds, *outbound)

				case f.isHTTPClient(recvType.Obj()) && isAllowListedFunctionName(funcObj.Name()):
					outbounds = append(outbounds, f.httpOutbound(funcObj, recvType, callExpr, pkg, pkgMap))

				default:
					continue
				}
			}
		}
	}

	return outbounds, outboundMissingServiceCount, nil
}

func (f *Finder) cadenceOutbound(funcObj types.Object, recvType *types.Named, callExpr *ast.CallExpr, pkg *packages.Package, pkgMap map[string]*packages.Package) fty.OutboundPoint {
	clientType := servicemapping.FullyQualifiedType{PkgPath: recvType.Obj().Pkg().Path(), Name: recvType.Obj().Name()}
	service := f.serviceMapper.InferService(clientType)
	if service == nil {
		panic(fmt.Sprintf("service inference failed for cadence client type: %v", clientType))
	}
	return fty.OutboundPoint{
		Pkg:           pkg,
		Call:          callExpr,
		Service:       *service,
		Type:          fty.TypeCadence,
		CanonicalName: canonicalEndpoint(constant.CadenceServiceName, funcObj.Name()),
	}
}

func (f *Finder) gogoprotoThriftOutbound(funcObj types.Object, recvType *types.Named, callExpr *ast.CallExpr, pkg *packages.Package, pkgMap map[string]*packages.Package) (*fty.OutboundPoint, error) {
	canonicalServiceName, err := f.canonicalServiceName(pkgMap[recvType.Obj().Pkg().Path()], pkgMap)
	if err != nil {
		return nil, fmt.Errorf("finding canonical service name: %w", err)
	}

	f.logger.Debugw("Found outbound call", "type", recvType.String(), "function", funcObj.Name())

	clientType := servicemapping.FullyQualifiedType{PkgPath: recvType.Obj().Pkg().Path(), Name: recvType.Obj().Name()}
	service := f.serviceMapper.InferService(clientType)
	if service == nil {
		f.logger.Warnw("Skipping outbound call since service inference failed", "pkg", pkg.PkgPath, "function", funcObj.Name())
		return nil, nil
	}
	outboundType := fty.TypeGRPC
	if isThriftClient(recvType.Obj()) {
		outboundType = fty.TypeThrift
	}

	return &fty.OutboundPoint{
		Pkg:           pkg,
		Call:          callExpr,
		Service:       *service,
		Type:          outboundType,
		CanonicalName: canonicalEndpoint(canonicalServiceName, funcObj.Name()),
	}, nil
}

// isGRPCClient returns true if the given type is a gRPC client type.
// Specifically, the client type is suffixed with "YARPCClient", where the type is provided in a
// generated "gogoproto/*" package.
func isGRPCClient(t *types.TypeName) bool {
	return strings.HasPrefix(t.Pkg().Path(), constant.GogoprotoPkgPrefix+"/") && strings.HasSuffix(t.Name(), "YARPCClient")
}

// isThriftClient returns true if the given type is a Thrift client type.
// Specifically, the client type is named "Interface", where the type is provided in a generated
// "thriftrw/*" package, and the package name is suffixed with "client" (e.g., "otherserviceclient").
func isThriftClient(t *types.TypeName) bool {
	return strings.HasPrefix(t.Pkg().Path(), constant.ThriftrwPkgPrefix+"/") && (
	// `thriftrw/foo/barclient.Interface` for YARPC Thrift clients.
	(strings.HasSuffix(t.Pkg().Path(), "client") && t.Name() == "Interface") ||
		// `thriftrw/foo/bar.TChanFooBar` for TChannel Thrift clients.
		(strings.HasPrefix(t.Name(), "TChan") && !strings.HasSuffix(t.Name(), "Server")))
}

// isGenericGRPCClient returns true if t is a protoc-gen-go-grpc-generated client struct: every
// generated "xxxClient" struct holds a field of type google.golang.org/grpc.ClientConnInterface
// (or, for older-generated/hand-written clients, a concrete *grpc.ClientConn). Unlike
// isGRPCClient, this doesn't assume any particular package-path or type-name convention, so it
// works on arbitrary OSS gRPC services in addition to internal YARPC-over-protobuf ones.
func isGenericGRPCClient(t *types.TypeName) bool {
	return structHasFieldOfType(t, "google.golang.org/grpc", "ClientConnInterface") ||
		structHasFieldOfType(t, "google.golang.org/grpc", "ClientConn")
}

// isGenericThriftClient returns true if t is an Apache Thrift client struct: Apache Thrift's Go
// codegen (and thrift.NewTStandardClient(...) callers) hold a field of type
// github.com/apache/thrift/lib/go/thrift.TClient. Unlike isThriftClient, this doesn't assume
// YARPC/TChannel package-path or type-name conventions.
func isGenericThriftClient(t *types.TypeName) bool {
	return structHasFieldOfType(t, "github.com/apache/thrift/lib/go/thrift", "TClient")
}

// structHasFieldOfType returns true if t's underlying struct type has a field whose type is the
// named type pkgPath.typeName. t is resolved to its backing struct first via
// resolveConcreteClientType, since real caller code almost always holds the exported client
// *interface* (protoc-gen-go-grpc/Apache Thrift both generate an unexported struct implementing
// an exported interface, and constructors like NewXxxClient return the interface), not the
// concrete struct directly.
func structHasFieldOfType(t *types.TypeName, pkgPath, typeName string) bool {
	concrete := resolveConcreteClientType(t)
	if concrete == nil {
		return false
	}
	st, ok := asthelper.UnwrapPtr(concrete.Type()).Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		named, ok := asthelper.UnwrapPtr(st.Field(i).Type()).(*types.Named)
		if !ok {
			continue
		}
		obj := named.Obj()
		if obj.Pkg() != nil && obj.Pkg().Path() == pkgPath && obj.Name() == typeName {
			return true
		}
	}
	return false
}

// resolveConcreteClientType returns the *types.TypeName backing t's struct representation. If t
// is already a struct (or pointer to one), it is returned unchanged. If t is an interface --
// the shape real caller code actually holds, since protoc-gen-go-grpc and Apache Thrift codegen
// both export an interface (e.g. "XxxClient") backed by an unexported struct in the same package,
// and constructors like NewXxxClient return the interface, not the struct -- this scans t's own
// package scope for the (unique, by codegen convention) concrete struct type implementing it.
// Returns nil if no such struct is found.
func resolveConcreteClientType(t *types.TypeName) *types.TypeName {
	underlying := asthelper.UnwrapPtr(t.Type()).Underlying()
	switch underlying := underlying.(type) {
	case *types.Struct:
		return t
	case *types.Interface:
		if t.Pkg() == nil {
			return nil
		}
		scope := t.Pkg().Scope()
		for _, name := range scope.Names() {
			obj, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || obj == t {
				continue
			}
			if _, ok := asthelper.UnwrapPtr(obj.Type()).Underlying().(*types.Struct); !ok {
				continue
			}
			if types.Implements(obj.Type(), underlying) || types.Implements(types.NewPointer(obj.Type()), underlying) {
				return obj
			}
		}
	}
	return nil
}

// genericGRPCThriftOutbound builds an OutboundPoint for a call through a generic (non-YARPC)
// gRPC or Thrift client, identified via isGenericGRPCClient/isGenericThriftClient. Unlike
// gogoprotoThriftOutbound, it doesn't walk a gogoproto/thriftrw service-mapping directory
// convention (none exists for arbitrary OSS packages) -- it builds CanonicalName directly from
// the client's package path and method name, mirroring httpOutbound's simpler pattern.
func (f *Finder) genericGRPCThriftOutbound(funcObj types.Object, recvType *types.Named, callExpr *ast.CallExpr, pkg *packages.Package, pkgMap map[string]*packages.Package) (*fty.OutboundPoint, error) {
	f.logger.Debugw("Found generic gRPC/Thrift outbound call", "type", recvType.String(), "function", funcObj.Name())

	clientType := servicemapping.FullyQualifiedType{PkgPath: recvType.Obj().Pkg().Path(), Name: recvType.Obj().Name()}
	service := f.serviceMapper.InferService(clientType)
	if service == nil {
		f.logger.Warnw("Skipping outbound call since service inference failed", "pkg", pkg.PkgPath, "function", funcObj.Name())
		return nil, nil
	}
	outboundType := fty.TypeGRPC
	if isGenericThriftClient(recvType.Obj()) {
		outboundType = fty.TypeThrift
	}

	return &fty.OutboundPoint{
		Pkg:           pkg,
		Call:          callExpr,
		Service:       *service,
		Type:          outboundType,
		CanonicalName: canonicalEndpoint(recvType.Obj().Pkg().Path(), funcObj.Name()),
	}, nil
}

// isCadenceClient returns true if the given type is a Cadence client type.
func isCadenceClient(t *types.TypeName) bool {
	return t.Pkg().Path() == constant.CadenceClientPkg && t.Name() == "Client"
}

// allow list the function names for HTTP outbound
var allowListedFunctionNames = []string{
	"Get",
	"Post",
	"PostForm",
	"Do",
	"Head",
}

func (f *Finder) isHTTPClient(t *types.TypeName) bool {
	if t.Pkg().Path() == "net/http" && t.Name() == "Client" {
		return true
	}
	pkgQualifiedName := t.Pkg().Path() + "." + t.Name()
	return slices.Contains(f.additionalHTTPClientTypes, pkgQualifiedName)
}

func isAllowListedFunctionName(funcName string) bool {
	for _, wn := range allowListedFunctionNames {
		if strings.Contains(strings.ToLower(funcName), strings.ToLower(wn)) {
			return true
		}
	}
	return false
}

func (f *Finder) httpOutbound(funcObj types.Object, recvType *types.Named, callExpr *ast.CallExpr, pkg *packages.Package, pkgMap map[string]*packages.Package) fty.OutboundPoint {
	return fty.OutboundPoint{
		Pkg:           pkg,
		Call:          callExpr,
		Type:          fty.TypeHTTP,
		CanonicalName: canonicalEndpoint(recvType.Obj().Pkg().Path(), funcObj.Name()),
	}
}
