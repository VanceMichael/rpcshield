package servicemapping

import (
	"cmp"
	"errors"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/uber-research/rpcshield/go/grail"
	"github.com/uber-research/rpcshield/go/loader"
	"github.com/uber-research/rpcshield/go/staticanalyzer/asthelper"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
)

// ErrFxScanFailed wraps fatal FxScanner failures that invalidate the whole
// scan (e.g. the workspace walk itself fails), as opposed to tolerated
// per-package diagnostics (unresolved client constructors, individual package
// load errors). Callers that refresh an atomic registry generation must abort
// the refresh when errors.Is(err, ErrFxScanFailed) and keep the previous
// complete generation; non-fatal diagnostics still leave a usable mapping.
var ErrFxScanFailed = errors.New("fx workspace scan failed")

type errCannotResolve struct {
	clientType FullyQualifiedType
	code       string
}

func (e *errCannotResolve) Error() string {
	return fmt.Sprintf("%s: resolve service name for %q", e.clientType, e.code)
}

// ServiceMapping is a mapping from the fully qualified client type to a list of unique
// services with auxiliary IDL information to aid further analyses.
type ServiceMapping map[FullyQualifiedType][]*ServiceWithIDLInfo

// AddMappings adds a new mapping to the ServiceMapping. If the client type already exists, it will
// check the existing mapped services to avoid duplicating the same service. However, the constructor
// package paths will be appended to the existing ones.
func (s ServiceMapping) AddMappings(clientType FullyQualifiedType, services ...*ServiceWithIDLInfo) {
	for _, service := range services {
		for _, v := range s[clientType] {
			if v.Service.Name == service.Service.Name {
				v.ConstructorPkgPaths = append(v.ConstructorPkgPaths, service.ConstructorPkgPaths...)
				return
			}

		}
		s[clientType] = append(s[clientType], service)
	}
}

// ServiceWithIDLInfo is a struct wrapping the service with additional IDL information for
// other uses.
type ServiceWithIDLInfo struct {
	// Service is the uber service information.
	Service grail.Service `json:"service"`
	// ConstructorPkgPaths is the list of package paths where the Fx constructor is located.
	ConstructorPkgPaths []string `json:"constructor_pkg_paths"`
}

// IsRelevant returns true if the found service has a constructor package listed in the given packages.
func (s ServiceWithIDLInfo) IsRelevant(allPkgs map[string]*packages.Package) bool {
	for _, pkgPath := range s.ConstructorPkgPaths {
		if _, ok := allPkgs[pkgPath]; ok {
			return true
		}
	}
	return false
}

// FxScanner is a scanner that scans the monorepo for Fx constructors to find the generated package
// path to service mapping names.
type FxScanner struct {
	workspace             string
	allServices           map[string]grail.Service
	logger                *zap.SugaredLogger
	configResolver        *configAliasResolver
	networkGroupToService map[string]string
	codebasePkgPrefix     string
}

// NewFxScanner creates a new FxScannerResultProvider. codebasePkgPrefix restricts the workspace
// walk to that GOPATH-style src/ subtree (see config.Config.CodebasePkgPrefix); pass "" to walk
// the entire workspace src/ tree.
func NewFxScanner(workspace string, allServices []grail.Service, allNetworkGroups []grail.NetworkGroup, codebasePkgPrefix string, logger *zap.SugaredLogger) (*FxScanner, error) {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	allServicesMap := make(map[string]grail.Service)
	for _, s := range allServices {
		allServicesMap[s.Name] = s
	}

	networkGroupToService := make(map[string]string)
	for _, group := range allNetworkGroups {
		// Network group and service names should have 1:1 mapping, if this is not the case we
		// should log a warning for it and fall back to the first service name.
		if len(group.ServiceNames) > 1 {
			logger.Warnw("Multiple service names under a service group", "service_group", group.Group, "service_names", group.ServiceNames)
		}
		networkGroupToService[group.Group] = group.ServiceNames[0]
	}

	configResolver, err := newConfigAliasResolver(workspace, codebasePkgPrefix, logger)
	if err != nil {
		return nil, fmt.Errorf("new config alias resolver: %w", err)
	}

	return &FxScanner{
		workspace:             workspace,
		allServices:           allServicesMap,
		networkGroupToService: networkGroupToService,
		configResolver:        configResolver,
		codebasePkgPrefix:     codebasePkgPrefix,
		logger:                logger,
	}, nil
}

// Mappings scans the monorepo for Fx constructors to find the mapping from the generated package
// path to service.
func (f *FxScanner) Mappings() (ServiceMapping, error) {
	// Analyze each Go package in parallel for best performance. However, since loading _all_
	// packages concurrently in the Go monorepo is memory-intensive, we create a worker pool for
	// parallelization.
	type result struct {
		pkg                 *loader.SimplePackage
		clientTypeToService ServiceMapping
		errors              []error
	}
	jobsChan := make(chan string)
	results := make(chan *result)

	var wg sync.WaitGroup
	var sendErr error
	// Start the publisher: walk the workspace and pushing Go directories to the jobs channel.
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := collectGoDirs(f.workspace, f.codebasePkgPrefix, jobsChan); err != nil {
			sendErr = fmt.Errorf("%w: %w", ErrFxScanFailed, err)
		}
	}()

	// Start the worker pool.
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for folder := range jobsChan {
				// We do not load test files since the service calls are usually mocked, and the
				// Fx constructors in most cases simply point to invalid service (e.g., "test-service" etc.)
				pkg, err := loader.LoadSimplePackage(folder, f.workspace, func(entry fs.DirEntry) bool { return !strings.HasSuffix(entry.Name(), "_test.go") })
				if err != nil {
					results <- &result{errors: []error{fmt.Errorf("loading package %q: %w", folder, err)}}
					continue
				}
				clientTypeToService, errs := f.analyzePkg(pkg)
				results <- &result{
					pkg:                 pkg,
					clientTypeToService: clientTypeToService,
					errors:              errs,
				}
			}
		}()
	}
	// A separate goroutine that closes the result channel once all workers are done.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Consumer in main goroutine: collect results from the workers.
	clientTypeToServices := make(ServiceMapping)
	var errs []error
	for r := range results {
		for k, v := range r.clientTypeToService {
			clientTypeToServices.AddMappings(k, v...)
		}
		errs = append(errs, r.errors...)
	}

	// Now that we have collected all errors from package analyses, we can remove ones where
	// we have found their mapping in other packages - only the errors that we cannot find a mapping
	// are interesting for us (to fix).
	var filteredErrs []error
	for _, err := range errs {
		var e *errCannotResolve
		if errors.As(err, &e) {
			if _, ok := clientTypeToServices[e.clientType]; ok {
				continue
			}
		}
		filteredErrs = append(filteredErrs, err)
	}

	// Send error should be the first one to be returned.
	filteredErrs = append([]error{sendErr}, filteredErrs...)

	// Sort the multiple services alphabetically for deterministic results.
	for _, services := range clientTypeToServices {
		slices.SortFunc(services, func(a, b *ServiceWithIDLInfo) int { return cmp.Compare(a.Service.Name, b.Service.Name) })
	}
	return clientTypeToServices, errors.Join(filteredErrs...)
}

func (f *FxScanner) analyzePkg(pkg *loader.SimplePackage) (ServiceMapping, []error) {
	var errs []error
	clientTypeToServiceNames := make(map[FullyQualifiedType]map[string]bool)

	// Collect package-level "var" declarations with constant string values once for later use.
	varConstVals := varConstStringVals(pkg)
	for _, node := range pkg.Files {
		// Try to find all call expressions that look like:
		// pkg.NewFooBarYARPCClient("service-name")
		// pkg.NewFxFooBarYARPCClient("service-name")
		// pkg.Client("service-name")
		// where pkg is either thriftrw or gogoproto generated package.
		// This gives us hint on the mapping from the generated package path to the service name.
		ast.Inspect(node, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if len(callExpr.Args) != 1 {
				return true
			}

			// Check for SelectorExpr in the call expression
			selector, ok := callExpr.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}

			recvObj, ok := pkg.TypesInfo.ObjectOf(receiver).(*types.PkgName)
			if !ok {
				return true
			}
			recvPkgPath := recvObj.Imported().Path()

			var clientType FullyQualifiedType
			switch {
			case isThriftrwClient(recvPkgPath, selector.Sel.Name):
				// thriftrw generates server, client, fx code in separate packages. Here for
				// thrift the receiver package is the fx package, so we need to adjust it to be
				// the client package to be stored in the mapping. For example:
				// "example.internal/foo/bar/foobarfx" -> "example.internal/foo/bar/foobarclient.Interface"
				clientType.PkgPath = strings.TrimSuffix(recvPkgPath, "fx") + "client"
				clientType.Name = "Interface"

			case isGogoprotoNewYARPClient(recvPkgPath, selector.Sel.Name):
				// gogoproto puts server, client, and fx code within the same package. The client
				// type name is baked in the constructor name, e.g.,
				// "NewFooBarYARPCClient" -> "FooBarYARPCClient" or
				// "NewFxFooBarYARPCClient" -> "FooBarYARPCClient".
				clientType.PkgPath = recvPkgPath
				clientType.Name = strings.TrimPrefix(strings.TrimPrefix(selector.Sel.Name, "New"), "Fx")

			default:
				return true
			}

			// Now we resolve the service name from the argument of the call expression.
			arg0 := ast.Unparen(callExpr.Args[0])

			// There might be two cases for the argument of the client constructor:
			// (1) A static string value (string literal or package-level constant), which can be directly resolved.
			//     For example, `pb.NewFooBarYARPCClient("service-name")` or
			//	   `pb.NewFooBarYARPCClient(_testCatalog)` where `_testCatalog` is defined as a package-local constant.
			// (2) A yarpc.ClientConfig call with static string value.
			//     For example, `pb.NewFooBarYARPCClient(yarpc.ClientConfig("service-name"))`.
			if v := staticStringVal(unwrapYARPCConfig(arg0), pkg.TypesInfo, varConstVals); v != "" {
				dir, err := filepath.Rel(f.workspace, pkg.Dir)
				if err != nil {
					errs = append(errs, fmt.Errorf("get relative path: %w", err))
					return true
				}
				// Resolve the service name with the config alias resolver.
				v = f.configResolver.resolve(dir, v)
				// If the service name is actually a network group, resolve it to the service name.
				if service, ok := f.networkGroupToService[v]; ok {
					v = service
				}
				if _, ok := clientTypeToServiceNames[clientType]; !ok {
					clientTypeToServiceNames[clientType] = make(map[string]bool)
				}
				clientTypeToServiceNames[clientType][v] = true
				return true
			}

			// Print the AST to buffer for easier debugging. We ignore the error since failing to
			// print would just result in empty code string.
			expr, _ := asthelper.FormatExpr(callExpr.Args[0], pkg.Fset)
			errs = append(errs, &errCannotResolve{clientType: clientType, code: expr})

			return true
		})
	}

	// Resolve the service name service with IDL information.
	clientTypeToService := make(ServiceMapping, len(clientTypeToServiceNames))
	for clientType, serviceNames := range clientTypeToServiceNames {
		for serviceName := range serviceNames {
			s, ok := f.allServices[serviceName]
			if !ok {
				errs = append(errs, fmt.Errorf("service name %q not found in all services", serviceName))
				continue
			}
			clientTypeToService.AddMappings(clientType, &ServiceWithIDLInfo{Service: s, ConstructorPkgPaths: []string{pkg.PkgPath}})
		}
	}

	return clientTypeToService, errs
}

// collectGoDirs collects all directories under the workspace (excluding testdata folders) that
// contain at least one Go file, and send it to the channel. It will close the channel once done.
// codebasePkgPrefix restricts the walk to that GOPATH-style src/ subtree; empty walks the entire
// workspace src/ tree.
// TODO: exclude files that are excluded in BUILD.bazel files.
func collectGoDirs(workspace string, codebasePkgPrefix string, dirChan chan string) error {
	defer close(dirChan)
	dirs := make(map[string]bool)
	err := filepath.WalkDir(filepath.Join(workspace, "src", codebasePkgPrefix), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// "testdata" is not relevant to our analyses and may contain invalid Go code.
		// TODO: if we are still missing a lot of mappings by excluding integration_tests, we may
		//  consider adding proper support for it (where the config files are located differently,
		//  usually within the same folder).
		if d.Name() == "testdata" || d.Name() == "integration_tests" {
			return fs.SkipDir
		}

		if !d.IsDir() && strings.HasSuffix(d.Name(), ".go") {
			dir := filepath.Dir(path)
			if dirs[dir] {
				return nil
			}
			dirs[dir] = true
			dirChan <- dir
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk workspace %q: %w", workspace, err)
	}

	return nil
}

// unwrapYARPCConfig unwraps the yarpc.ClientConfig("service-name") and returns the node for
// "service-name".
func unwrapYARPCConfig(expr ast.Expr) ast.Expr {
	if call, ok := expr.(*ast.CallExpr); ok && len(call.Args) == 1 {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ClientConfig" {
			return ast.Unparen(call.Args[0])
		}
	}
	return expr
}

func isGogoprotoNewYARPClient(pkgPath string, funcName string) bool {
	return strings.HasPrefix(pkgPath, "gogoproto/") &&
		strings.HasPrefix(funcName, "New") && strings.HasSuffix(funcName, "YARPCClient")
}

func isThriftrwClient(pkgPath string, funcName string) bool {
	return strings.HasPrefix(pkgPath, "thriftrw/") && funcName == "Client"
}

func varConstStringVals(pkg *loader.SimplePackage) map[types.Object]string {
	vals := make(map[types.Object]string)
	// Collect all `var foo = "bar"` declarations in the package and store their constant values.
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				continue
			}
			for _, spec := range genDecl.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if len(valueSpec.Names) != len(valueSpec.Values) {
					continue
				}
				for i := range len(valueSpec.Names) {
					variable, value := valueSpec.Names[i], valueSpec.Values[i]
					obj := pkg.TypesInfo.ObjectOf(variable)
					if obj == nil {
						continue
					}
					if tv, ok := pkg.TypesInfo.Types[value]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
						vals[obj] = constant.StringVal(tv.Value)
					}
				}
			}
		}
	}
	return vals
}

func staticStringVal(expr ast.Expr, typesInfo *types.Info, varConstStringVals map[types.Object]string) string {
	// It is a `const` value.
	if tv, ok := typesInfo.Types[expr]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
		return constant.StringVal(tv.Value)
	}

	// It can also be mistakenly declared as a "var" but with a constant value.
	if ident, ok := expr.(*ast.Ident); ok {
		if obj := typesInfo.ObjectOf(ident); obj != nil {
			if v, ok := varConstStringVals[obj]; ok {
				return v
			}
		}
	}
	return ""
}
