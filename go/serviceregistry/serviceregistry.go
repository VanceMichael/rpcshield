// Package serviceregistry provides ServiceRegistry, the combined view of the service
// list/tiers and RPC-client-type-to-service-ownership mapping that rpcshield.Driver needs to
// run whole-program analysis for a service, and ServiceRegistryProvider, the pluggable
// interface for supplying it.
package serviceregistry

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/uber-research/rpcshield/go/cmd/rpcshield/common"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	"github.com/uber-research/rpcshield/go/grail"
	"go.uber.org/zap"
)

// ServiceRegistry is the combined view of the service list/tiers and RPC-client-type
// ownership mapping rpcshield.Driver needs to run whole-program analysis for a service.
type ServiceRegistry struct {
	// Services is the full list of known services (name, tier, burst-strategy metadata),
	// passed straight through to rpcshield.Driver.AllServices.
	Services []grail.Service
	// NetworkGroups is the list of network groups fetched alongside Services. With
	// DefaultServiceRegistryProvider, this is only populated when ClientTypeMapping is
	// actually rebuilt (see its doc comment) -- it may be empty on a cache hit, matching
	// today's behavior where network groups are only ever used to build ClientTypeMapping.
	NetworkGroups []grail.NetworkGroup
	// ClientTypeMapping maps an outbound RPC client type to the service(s) that own it,
	// passed to servicemapping.New to build a ServiceMapper.
	ClientTypeMapping servicemapping.ServiceMapping
}

// ServiceRegistryProvider supplies the ServiceRegistry rpcshield.Driver needs for a
// whole-program analysis run. DefaultServiceRegistryProvider is the default implementation,
// used by both the OSS CLI and Uber's monorepo wiring. Callers who don't have Uber's Grail
// service registry or example.internal Fx-constructor-scanning conventions can supply their
// own ServiceRegistryProvider instead.
type ServiceRegistryProvider interface {
	Provide(ctx context.Context) (ServiceRegistry, error)
}

// GrailClient is the subset of *grail.Grail's API DefaultServiceRegistryProvider needs.
type GrailClient interface {
	AllServices(ctx context.Context) ([]grail.Service, error)
	AllNetworkGroups(ctx context.Context) ([]grail.NetworkGroup, error)
}

// DefaultServiceRegistryProvider reproduces rpcshield's original service-registry pipeline:
// fetch the service list from Grail (disk-cached), then run the FxScanner AST prepass over the
// workspace (also disk-cached) to build the outbound-client-type-to-service mapping.
type DefaultServiceRegistryProvider struct {
	// Grail is the Grail client used to fetch services/network groups.
	Grail GrailClient
	// Workspace is the workspace root the FxScanner AST prepass scans, and under which cache
	// files are stored (<Workspace>/build/rpcshield/{grail_all_services,fx_mappings}.json).
	Workspace string
	// Logger is used for progress/diagnostic logging; nil uses a no-op logger.
	Logger *zap.SugaredLogger
	// CachedGrailAllServicesJSON controls how the Grail service list is cached/refreshed; see
	// common.CacheOption.
	CachedGrailAllServicesJSON common.CacheOption
	// CachedFxMappingsJSON controls how the FxScanner-produced client-type mapping is
	// cached/refreshed; see common.CacheOption.
	CachedFxMappingsJSON common.CacheOption
	// CodebasePkgPrefix is passed straight through to servicemapping.NewFxScanner -- see
	// config.Config.CodebasePkgPrefix's doc comment. Empty matches every package.
	CodebasePkgPrefix string
}

// Provide implements ServiceRegistryProvider.
func (p *DefaultServiceRegistryProvider) Provide(ctx context.Context) (ServiceRegistry, error) {
	logger := p.Logger
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	recreateAllServices := func() ([]grail.Service, error) {
		s, err := p.Grail.AllServices(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetch all services from Grail: %w", err)
		}
		logger.Infow("Fetched all services from Grail", "count", len(s))
		return s, nil
	}
	services, err := common.FetchFromCache(p.CachedGrailAllServicesJSON, filepath.Join(p.Workspace, "build", "rpcshield", "grail_all_services.json"), recreateAllServices, logger)
	if err != nil {
		return ServiceRegistry{}, fmt.Errorf("fetch all services from Grail: %w", err)
	}

	var networkGroups []grail.NetworkGroup
	recreateFxMappings := func() (servicemapping.ServiceMapping, error) {
		networkGroups, err = p.Grail.AllNetworkGroups(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetch all network groups from Grail: %w", err)
		}
		scanner, err := servicemapping.NewFxScanner(p.Workspace, services, networkGroups, p.CodebasePkgPrefix, logger)
		if err != nil {
			return nil, fmt.Errorf("create Fx scanner: %w", err)
		}
		// Ignore the error since we don't want to fail the whole program analysis. The
		// returned mapping is still valid and useful for the rest of the program analysis.
		m, _ := scanner.Mappings()
		return m, nil
	}
	clientTypeToService, err := common.FetchFromCache(p.CachedFxMappingsJSON, filepath.Join(p.Workspace, "build", "rpcshield", "fx_mappings.json"), recreateFxMappings, logger)
	if err != nil {
		return ServiceRegistry{}, fmt.Errorf("fetch Fx mappings: %w", err)
	}

	return ServiceRegistry{
		Services:          services,
		NetworkGroups:     networkGroups,
		ClientTypeMapping: clientTypeToService,
	}, nil
}
