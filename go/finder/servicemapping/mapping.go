package servicemapping

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/grail"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
)

// FullyQualifiedType represents a fully qualified type in a generated package.
type FullyQualifiedType struct {
	// PkgPath is the package path.
	PkgPath string
	// Name is the type name.
	Name string
}

// MarshalText marshals the FullyQualifiedType into a byte slice such that it can be used in map
// keys when marshaling to JSON.
func (f FullyQualifiedType) MarshalText() ([]byte, error) {
	return []byte(f.PkgPath + "." + f.Name), nil
}

// UnmarshalText unmarshals the FullyQualifiedType from a byte slice such that it can be used in map
// keys when unmarshalling from JSON.
func (f *FullyQualifiedType) UnmarshalText(text []byte) error {
	index := strings.LastIndex(string(text), ".")
	if index == -1 {
		return fmt.Errorf("invalid fully qualified type: %q", string(text))
	}
	f.PkgPath, f.Name = string(text[:index]), string(text[index+1:])
	return nil
}

// ServiceMapper infers the service that owns an outbound RPC client type, and ranks candidate
// services once package information is available. Mapper (below) is the default implementation,
// used by both the OSS CLI and Uber's monorepo wiring unless a caller supplies their own
// ServiceMapper.
type ServiceMapper interface {
	// InferService infers the service that owns clientType, or nil if inference fails.
	InferService(clientType FullyQualifiedType) *grail.Service
	// RankCachedServices ranks, in-place, any cached multi-candidate mappings using pkgs.
	RankCachedServices(pkgs []*packages.Package)
}

// Mapper is responsible for mapping client package paths to services.
type Mapper struct {
	serviceMap          map[string]grail.Service
	logger              *zap.SugaredLogger
	clientTypeToService ServiceMapping
	codebasePkgPrefix   string
}

// New creates a new Mapper. It optionally takes a mapping from client package paths to
// services with IDL information that is taken from other sources (e.g., FxScanner).
//
// codebasePkgPrefix is the same codebase-package-path prefix passed to finder.New (see
// config.Config.CodebasePkgPrefix); it is used by nameMatch to distinguish the generated
// "thriftrw|gogoproto/<codebasePkgPrefix>/..." path segment from the rest of the client package
// path. It's variadic so existing call sites (which pre-date this parameter) don't all need
// updating; only the first value, if any, is used.
func New(allServices []grail.Service, clientTypeToService ServiceMapping, logger *zap.SugaredLogger, codebasePkgPrefix ...string) *Mapper {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if clientTypeToService == nil {
		clientTypeToService = make(ServiceMapping)
	}

	serviceMap := make(map[string]grail.Service, len(allServices))
	for _, service := range allServices {
		if existing, ok := serviceMap[service.Name]; ok {
			panic(fmt.Sprintf("duplicate grail service name found: %v vs %v", service, existing))
		}
		serviceMap[service.Name] = service
	}

	var prefix string
	if len(codebasePkgPrefix) > 0 {
		prefix = codebasePkgPrefix[0]
	}

	return &Mapper{
		serviceMap:          serviceMap,
		clientTypeToService: clientTypeToService,
		logger:              logger,
		codebasePkgPrefix:   prefix,
	}
}

// RankCachedServices ranks the cached services in the client type to service mapping. We
// prefer (1) relevant ones, and then (2) pick the one with the highest tier (to reduce false positives).
// The ranking is done in-place.
func (m *Mapper) RankCachedServices(pkgs []*packages.Package) {
	pkgMap := make(map[string]*packages.Package, len(pkgs))
	for _, pkg := range pkgs {
		pkgMap[pkg.PkgPath] = pkg
	}

	for _, services := range m.clientTypeToService {
		if len(services) == 1 {
			continue
		}

		slices.SortFunc(services, func(a, b *ServiceWithIDLInfo) int {
			isARelevant, isBRelevant := a.IsRelevant(pkgMap), b.IsRelevant(pkgMap)
			switch {
			case isARelevant && !isBRelevant:
				return -1
			case !isARelevant && isBRelevant:
				return 1
			default:
				// If both are relevant (or irrelevant), compare tiers.
				return cmp.Compare(a.Service.Tier, b.Service.Tier)
			}
		})
	}
}

// InferService infers the service name from the client package path, nil is returned if
// inference fails for any reason.
func (m *Mapper) InferService(clientType FullyQualifiedType) *grail.Service {
	// Strategy 1: predefined service names.
	switch {
	case clientType.PkgPath == constant.CadenceClientPkg && clientType.Name == "Client":
		svc := m.serviceMap[constant.CadenceServiceName]
		return &svc
	default:
	}

	// Strategy 2: fetch the existing mapping from outside sources (e.g., Fx constructor scanning).
	if s, ok := m.clientTypeToService[clientType]; ok {
		if len(s) == 1 {
			return &s[0].Service
		}
		m.logger.Warnw("Multiple services found for client type, picking the top-ranked one", "client_type", clientType, "picked", s[0].Service, "num_services", len(s))
		return &s[0].Service
	}

	// Strategy 3 (fallback): name matching.
	m.logger.Infow("Falling back to name matching for OB service mapping", "pkg_path", clientType)
	return m.nameMatch(clientType.PkgPath)
}
