// Package serviceregistry provides ServiceRegistry, the combined view of the service
// list/tiers and RPC-client-type-to-service-ownership mapping that rpcshield.Driver needs to
// run whole-program analysis for a service, and ServiceRegistryProvider, the pluggable
// interface for supplying it.
package serviceregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/uber-research/rpcshield/go/cmd/rpcshield/common"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	"github.com/uber-research/rpcshield/go/grail"
	"go.uber.org/zap"
)

// Per-run provenance labels surfaced in RegistryMeta.Components (and metrics).
const (
	// sourceExplicit: component was read from a caller-provided file path
	// (read-only; never written to).
	sourceExplicit = "explicit"
	// sourceCache: managed component served from an on-disk cache without a
	// fetch this run (legacy single-file cache in mixed/explicit mode).
	sourceCache = "cache"
	// sourceFetched: managed component had to be (re)built this run.
	sourceFetched = "fetched"
	// sourceGrail: network groups fetched directly from Grail this run.
	sourceGrail = "grail"
)

// ServiceRegistry is the combined view of the service list/tiers and RPC-client-type
// ownership mapping rpcshield.Driver needs to run whole-program analysis for a service.
type ServiceRegistry struct {
	// Services is the full list of known services (name, tier, burst-strategy metadata),
	// passed straight through to rpcshield.Driver.AllServices.
	Services []grail.Service
	// NetworkGroups is the list of network groups the Fx mapping was built from.
	// It is only populated when the mapping was actually (re)built this run, or
	// when served from a complete registry generation that includes it; it may be
	// empty on a legacy-cache hit, matching the historical behavior where network
	// groups are only consumed while building ClientTypeMapping.
	NetworkGroups []grail.NetworkGroup
	// ClientTypeMapping maps an outbound RPC client type to the service(s) that own it,
	// passed to servicemapping.New to build a ServiceMapper.
	ClientTypeMapping servicemapping.ServiceMapping
	// Meta identifies the registry generation the other fields come from, where
	// each component was read from, and whether this run fell back to an older
	// generation after the active candidate turned out unusable or a refresh
	// failed. Callers (metrics, logs) should report it; custom providers may
	// leave it zero.
	Meta RegistryMeta
}

// ComponentProvenance says where one registry component came from during this run.
type ComponentProvenance struct {
	// Source is one of the source* constants: "generation" (active generation
	// directory), "legacy" (adopted/taken-over legacy cache), "explicit"
	// (caller-provided read-only file), "cache", "fetched", "grail", or
	// "unavailable" (component does not exist in the source, e.g. network groups
	// in the legacy layout).
	Source string
	// Path is the file the component was read from, empty when it was produced
	// in-process or is unavailable.
	Path string
	// SHA256 is the hex SHA-256 of the component bytes that were read, empty for
	// in-process/unavailable components.
	SHA256 string
}

// RegistryMeta identifies the registry generation used by a run.
type RegistryMeta struct {
	// Generation is the generation id ("gen-<timestamp>-<random>"), the
	// content-derived "legacy-*"/"external-*" id for legacy/external assembly, or
	// empty for custom ServiceRegistryProviders.
	Generation string
	// CreatedAt is when the generation was built (legacy caches report the newest
	// legacy file's mtime). Zero for external/custom assembly.
	CreatedAt time.Time
	// Components maps Component* constants to where that component came from.
	Components map[string]ComponentProvenance
	// FellBack is true when this run is not serving the would-be-current
	// candidate: either the active generation was unusable and a complete older
	// generation (or the legacy caches) was used, or a refresh failed and the
	// previous generation was kept.
	FellBack bool
	// FallbackReason describes the fallback when FellBack is set.
	FallbackReason string
}

// ServiceRegistryProvider supplies the ServiceRegistry rpcshield.Driver needs for a
// whole-program analysis run. DefaultServiceRegistryProvider is the default implementation,
// used by both the OSS CLI and Uber's monorepo wiring. Callers who don't have Uber's Grail
// service registry or example.internal Fx-constructor-scanning conventions can supply their
// own ServiceRegistryProvider instead; they are not required to use the on-disk generation
// layout (see generation.go) at all.
type ServiceRegistryProvider interface {
	Provide(ctx context.Context) (ServiceRegistry, error)
}

// GrailClient is the subset of *grail.Grail's API DefaultServiceRegistryProvider needs.
type GrailClient interface {
	AllServices(ctx context.Context) ([]grail.Service, error)
	AllNetworkGroups(ctx context.Context) ([]grail.NetworkGroup, error)
}

// DefaultServiceRegistryProvider reproduces rpcshield's original service-registry pipeline,
// made failure-atomic: the service list, the network groups and the Fx-scanner mapping are
// fetched/scanned into an inactive generation directory and validated as one bundle before
// they become visible; any failure keeps serving the previous complete generation. See
// generation.go for the on-disk protocol and its failure semantics.
type DefaultServiceRegistryProvider struct {
	// Grail is the Grail client used to fetch services/network groups.
	Grail GrailClient
	// Workspace is the workspace root the FxScanner AST prepass scans, and under
	// which cache files are stored:
	//   <Workspace>/build/rpcshield/registry/...                 (generation store)
	//   <Workspace>/build/rpcshield/grail_all_services.json      (legacy cache)
	//   <Workspace>/build/rpcshield/fx_mappings.json             (legacy cache)
	Workspace string
	// Logger is used for progress/diagnostic logging; nil uses a no-op logger.
	Logger *zap.SugaredLogger
	// CachedGrailAllServicesJSON controls how the Grail service list is cached/refreshed; see
	// common.CacheOption. "auto"/"refresh" participate in the atomic generation store; an
	// explicit file path is read-only (never written to).
	CachedGrailAllServicesJSON common.CacheOption
	// CachedFxMappingsJSON controls how the FxScanner-produced client-type mapping is
	// cached/refreshed; see common.CacheOption. "auto"/"refresh" participate in the atomic
	// generation store; an explicit file path is read-only (never written to).
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

	// Any caller-provided file path keeps its historical read-only semantics:
	// the file is decoded directly and never written to. Only pure auto/refresh
	// combinations go through the atomic generation store.
	if isExplicitCacheOption(p.CachedGrailAllServicesJSON) || isExplicitCacheOption(p.CachedFxMappingsJSON) {
		return p.provideExternal(ctx, logger)
	}

	store := newGenerationStore(p.Workspace, logger)
	forceRefresh := p.CachedGrailAllServicesJSON == common.CacheOptionRefresh ||
		p.CachedFxMappingsJSON == common.CacheOptionRefresh
	return store.provideManaged(forceRefresh, func() (registryBundle, error) {
		return p.buildFreshBundle(ctx, logger)
	})
}

// buildFreshBundle performs the fetch/scan half of a refresh: AllServices, then
// AllNetworkGroups, then the Fx scanner over the workspace. All three outputs
// describe the same registry snapshot and are staged/published atomically by
// the generation store.
func (p *DefaultServiceRegistryProvider) buildFreshBundle(ctx context.Context, logger *zap.SugaredLogger) (registryBundle, error) {
	services, err := p.Grail.AllServices(ctx)
	if err != nil {
		return registryBundle{}, fmt.Errorf("fetch all services from Grail: %w", err)
	}
	logger.Infow("Fetched all services from Grail", "count", len(services))

	networkGroups, err := p.Grail.AllNetworkGroups(ctx)
	if err != nil {
		return registryBundle{}, fmt.Errorf("fetch all network groups from Grail: %w", err)
	}
	logger.Infow("Fetched all network groups from Grail", "count", len(networkGroups))

	scanner, err := servicemapping.NewFxScanner(p.Workspace, services, networkGroups, p.CodebasePkgPrefix, logger)
	if err != nil {
		return registryBundle{}, fmt.Errorf("create Fx scanner: %w", err)
	}
	mapping, scanErr := scanner.Mappings()
	warnings := 0
	if scanErr != nil {
		// A workspace walk failure means the mapping cannot cover the codebase;
		// treat it as a fatal scan error so the refresh is aborted and the
		// previous complete generation stays active. Per-package load failures
		// and unresolved client constructors are non-fatal diagnostics the
		// scanner has always tolerated -- the partial mapping is still useful,
		// so the generation gets staged with the diagnostics recorded.
		if errors.Is(scanErr, servicemapping.ErrFxScanFailed) {
			return registryBundle{}, fmt.Errorf("fx workspace scan: %w", scanErr)
		}
		warnings = countJoinedErrors(scanErr)
		logger.Warnw("Fx scanner reported non-fatal diagnostics, staging the mapping anyway",
			"num_diagnostics", warnings, "error", scanErr)
	}

	return registryBundle{
		Services:      services,
		NetworkGroups: networkGroups,
		Mapping:       mapping,
		Sources: map[string]string{
			ComponentServices:      componentSourceGrail,
			ComponentNetworkGroups: componentSourceGrail,
			ComponentFxMappings:    componentSourceScan,
		},
		ScanWarnings: warnings,
	}, nil
}

// provideExternal assembles the registry when at least one cache option is an
// explicit read-only file path. Explicit files are never written; the other
// component (if managed) keeps using the legacy single-file cache through
// common.FetchFromCache. No atomic generation can be guaranteed across mixed
// sources -- the caller chooses that combination; provenance records exactly
// where every component came from.
func (p *DefaultServiceRegistryProvider) provideExternal(ctx context.Context, logger *zap.SugaredLogger) (ServiceRegistry, error) {
	root := filepath.Join(p.Workspace, "build", "rpcshield")
	components := make(map[string]ComponentProvenance, 3)

	var (
		services []grail.Service
		groups   []grail.NetworkGroup
		mapping  servicemapping.ServiceMapping
	)

	// Services component.
	if isExplicitCacheOption(p.CachedGrailAllServicesJSON) {
		path := string(p.CachedGrailAllServicesJSON)
		data, decoded, err := readExternalJSON[[]grail.Service](path)
		if err != nil {
			return ServiceRegistry{}, fmt.Errorf("explicit services cache %q: %w", path, err)
		}
		if err := validateServicesStandalone(decoded); err != nil {
			return ServiceRegistry{}, fmt.Errorf("explicit services cache %q: %w", path, err)
		}
		services = decoded
		components[ComponentServices] = ComponentProvenance{Source: sourceExplicit, Path: path, SHA256: sha256Hex(data)}
		logger.Infow("Using explicit services cache file (read-only)", "path", path)
	} else {
		cachePath := filepath.Join(root, legacyServicesFileName)
		fetched := false
		recreate := func() ([]grail.Service, error) {
			s, err := p.Grail.AllServices(ctx)
			if err != nil {
				return nil, fmt.Errorf("fetch all services from Grail: %w", err)
			}
			fetched = true
			return s, nil
		}
		v, err := common.FetchFromCache(p.CachedGrailAllServicesJSON, cachePath, recreate, logger)
		if err != nil {
			return ServiceRegistry{}, fmt.Errorf("fetch all services from Grail: %w", err)
		}
		services = v
		components[ComponentServices] = ComponentProvenance{
			Source: externalManagedSource(fetched), Path: cachePath, SHA256: hashMarshaled(v),
		}
	}

	// Fx mapping component.
	if isExplicitCacheOption(p.CachedFxMappingsJSON) {
		path := string(p.CachedFxMappingsJSON)
		data, decoded, err := readExternalJSON[servicemapping.ServiceMapping](path)
		if err != nil {
			return ServiceRegistry{}, fmt.Errorf("explicit fx mappings cache %q: %w", path, err)
		}
		mapping = decoded
		components[ComponentFxMappings] = ComponentProvenance{Source: sourceExplicit, Path: path, SHA256: sha256Hex(data)}
		logger.Infow("Using explicit fx mappings cache file (read-only)", "path", path)
	} else {
		cachePath := filepath.Join(root, legacyFxMappingsFileName)
		fetched := false
		recreate := func() (servicemapping.ServiceMapping, error) {
			networkGroups, err := p.Grail.AllNetworkGroups(ctx)
			if err != nil {
				return nil, fmt.Errorf("fetch all network groups from Grail: %w", err)
			}
			groups = networkGroups
			scanner, err := servicemapping.NewFxScanner(p.Workspace, services, networkGroups, p.CodebasePkgPrefix, logger)
			if err != nil {
				return nil, fmt.Errorf("create Fx scanner: %w", err)
			}
			m, scanErr := scanner.Mappings()
			if scanErr != nil && errors.Is(scanErr, servicemapping.ErrFxScanFailed) {
				return nil, fmt.Errorf("fx workspace scan: %w", scanErr)
			}
			if scanErr != nil {
				logger.Warnw("Fx scanner reported non-fatal diagnostics, keeping the mapping anyway",
					"num_diagnostics", countJoinedErrors(scanErr), "error", scanErr)
			}
			fetched = true
			return m, nil
		}
		v, err := common.FetchFromCache(p.CachedFxMappingsJSON, cachePath, recreate, logger)
		if err != nil {
			return ServiceRegistry{}, fmt.Errorf("fetch Fx mappings: %w", err)
		}
		mapping = v
		components[ComponentFxMappings] = ComponentProvenance{
			Source: externalManagedSource(fetched), Path: cachePath, SHA256: hashMarshaled(v),
		}
	}

	// Network groups are only available when the mapping was rebuilt this run.
	if groups != nil {
		components[ComponentNetworkGroups] = ComponentProvenance{Source: sourceGrail}
	} else {
		components[ComponentNetworkGroups] = ComponentProvenance{Source: sourceUnavailable}
	}

	generation := "external-" + (components[ComponentServices].SHA256 + components[ComponentFxMappings].SHA256)[:12]
	return ServiceRegistry{
		Services:          services,
		NetworkGroups:     groups,
		ClientTypeMapping: mapping,
		Meta: RegistryMeta{
			Generation: generation,
			Components: components,
		},
	}, nil
}

// isExplicitCacheOption reports whether opt is a caller-provided file path
// rather than the managed "auto"/"refresh" sentinels.
func isExplicitCacheOption(opt common.CacheOption) bool {
	return opt != common.CacheOptionAuto && opt != common.CacheOptionRefresh
}

func externalManagedSource(fetched bool) string {
	if fetched {
		return sourceFetched
	}
	return sourceCache
}

// readExternalJSON reads and decodes a read-only caller-provided JSON file,
// returning the raw bytes (for provenance/hashing) and the decoded value.
func readExternalJSON[T any](path string) ([]byte, T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, zero, fmt.Errorf("read file: %w", err)
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, zero, fmt.Errorf("decode file: %w", err)
	}
	return data, value, nil
}

func hashMarshaled(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return sha256Hex(b)
}

// validateServicesStandalone enforces the invariants the rest of the analysis
// relies on (and that would otherwise panic servicemapping.New): a non-empty,
// uniquely named service list.
func validateServicesStandalone(services []grail.Service) error {
	if len(services) == 0 {
		return errors.New("services component is empty")
	}
	seen := make(map[string]struct{}, len(services))
	for _, service := range services {
		if service.Name == "" {
			return errors.New("services component contains a service with an empty name")
		}
		if _, dup := seen[service.Name]; dup {
			return fmt.Errorf("services component declares duplicate service name %q", service.Name)
		}
		seen[service.Name] = struct{}{}
	}
	return nil
}

// countJoinedErrors best-effort counts the non-nil errors joined via
// errors.Join; any other error shape counts as one.
func countJoinedErrors(err error) int {
	if err == nil {
		return 0
	}
	type joined interface{ Unwrap() []error }
	if j, ok := err.(joined); ok {
		n := 0
		for _, e := range j.Unwrap() {
			if e != nil {
				n++
			}
		}
		if n > 0 {
			return n
		}
	}
	return 1
}
