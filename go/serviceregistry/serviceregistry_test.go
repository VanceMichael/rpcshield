package serviceregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/cmd/rpcshield/common"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	"github.com/uber-research/rpcshield/go/grail"
)

// fakeGrail is a GrailClient whose responses can be swapped between calls.
type fakeGrail struct {
	mu              sync.Mutex
	services        []grail.Service
	groups          []grail.NetworkGroup
	servicesErr     error
	groupsErr       error
	servicesCalls   int
	groupsCalls     int
	servicesByCall  [][]grail.Service
	groupsByCall    [][]grail.NetworkGroup
}

func (f *fakeGrail) AllServices(context.Context) ([]grail.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.servicesCalls++
	if f.servicesErr != nil {
		return nil, f.servicesErr
	}
	if len(f.servicesByCall) > 0 {
		i := f.servicesCalls - 1
		if i >= len(f.servicesByCall) {
			i = len(f.servicesByCall) - 1
		}
		return cloneServices(f.servicesByCall[i]), nil
	}
	return cloneServices(f.services), nil
}

func (f *fakeGrail) AllNetworkGroups(context.Context) ([]grail.NetworkGroup, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.groupsCalls++
	if f.groupsErr != nil {
		return nil, f.groupsErr
	}
	if len(f.groupsByCall) > 0 {
		i := f.groupsCalls - 1
		if i >= len(f.groupsByCall) {
			i = len(f.groupsByCall) - 1
		}
		return append([]grail.NetworkGroup(nil), f.groupsByCall[i]...), nil
	}
	return append([]grail.NetworkGroup(nil), f.groups...), nil
}

func (f *fakeGrail) calls() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.servicesCalls, f.groupsCalls
}

func cloneServices(in []grail.Service) []grail.Service {
	out := make([]grail.Service, len(in))
	copy(out, in)
	return out
}

func testServices(tag string) []grail.Service {
	return []grail.Service{
		{Name: "service-" + tag, Tier: 1},
		{Name: "service-shared", Tier: 3},
	}
}

func testGroups(tag string) []grail.NetworkGroup {
	return []grail.NetworkGroup{{Group: "group-" + tag, ServiceNames: []string{"service-shared"}}}
}

func testMapping(tag string) servicemapping.ServiceMapping {
	return servicemapping.ServiceMapping{
		{
			PkgPath: "gogoproto/example.internal/client/" + tag,
			Name:    "Client",
		}: {
			{
				Service:             grail.Service{Name: "service-" + tag, Tier: 1},
				ConstructorPkgPaths: []string{"example.internal/svc-" + tag},
			},
		},
	}
}

// prepareTestWorkspace creates the directory skeleton the Fx scanner expects.
// An empty src/ tree scans successfully (empty mapping); pass withSrc=false to
// simulate a fatal workspace walk failure.
func prepareTestWorkspace(t *testing.T, withSrc bool) string {
	t.Helper()
	workspace := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, "udeploy", "pinocchio"), 0o755))
	if withSrc {
		require.NoError(t, os.MkdirAll(filepath.Join(workspace, "src"), 0o755))
	}
	return workspace
}

func newTestProvider(t *testing.T, workspace string, g *fakeGrail) *DefaultServiceRegistryProvider {
	t.Helper()
	return &DefaultServiceRegistryProvider{
		Grail:                      g,
		Workspace:                  workspace,
		CachedGrailAllServicesJSON: common.CacheOptionAuto,
		CachedFxMappingsJSON:       common.CacheOptionAuto,
	}
}

func writeJSONFile(t *testing.T, path string, value any) []byte {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	b, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, b, 0o644))
	return b
}

func writeLegacyCaches(t *testing.T, workspace string, services []grail.Service, mapping servicemapping.ServiceMapping) {
	t.Helper()
	root := filepath.Join(workspace, "build", "rpcshield")
	writeJSONFile(t, filepath.Join(root, legacyServicesFileName), services)
	writeJSONFile(t, filepath.Join(root, legacyFxMappingsFileName), mapping)
}

func activeGenerationID(t *testing.T, store *generationStore) string {
	t.Helper()
	pointer, err := store.readActivePointer()
	require.NoError(t, err)
	return pointer.Generation
}

func requireGenerationSource(t *testing.T, registry ServiceRegistry, component string) {
	t.Helper()
	provenance, ok := registry.Meta.Components[component]
	require.True(t, ok, "missing provenance for component %q", component)
	require.Equal(t, sourceGeneration, provenance.Source, "component %q", component)
	require.NotEmpty(t, provenance.SHA256, "component %q", component)
	require.FileExists(t, provenance.Path)
}

// --- managed (auto/refresh) generation behavior ---

func TestProvideManaged_RefreshPublishesCompleteGeneration(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	g := &fakeGrail{services: testServices("v1"), groups: testGroups("v1")}
	provider := newTestProvider(t, workspace, g)

	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	require.Equal(t, testServices("v1"), registry.Services)
	require.Equal(t, testGroups("v1"), registry.NetworkGroups)
	require.False(t, registry.Meta.FellBack)
	require.Contains(t, registry.Meta.Generation, "gen-")
	requireGenerationSource(t, registry, ComponentServices)
	requireGenerationSource(t, registry, ComponentNetworkGroups)
	requireGenerationSource(t, registry, ComponentFxMappings)

	store := newGenerationStore(workspace, nil)
	id := activeGenerationID(t, store)
	require.Equal(t, id, registry.Meta.Generation)
	genDir := filepath.Join(store.generationsDir(), id)
	for _, name := range []string{genManifestName, servicesFileName, networkGroupsFileName, fxMappingsFileName} {
		require.FileExists(t, filepath.Join(genDir, name))
	}

	// A subsequent auto run must hit the generation on disk and not refetch.
	registry2, err := newTestProvider(t, workspace, g).Provide(context.Background())
	require.NoError(t, err)
	require.Equal(t, id, registry2.Meta.Generation)
	servicesCalls, groupsCalls := g.calls()
	require.Equal(t, 1, servicesCalls, "auto must not refetch services for a fresh generation")
	require.Equal(t, 1, groupsCalls, "auto must not refetch network groups for a fresh generation")
}

func TestProvideManaged_RefreshFailureAfterServicesKeepsPreviousGeneration(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	g := &fakeGrail{services: testServices("v1"), groups: testGroups("v1")}
	provider := newTestProvider(t, workspace, g)

	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	require.Equal(t, "service-v1", registry.Services[0].Name)
	firstID := registry.Meta.Generation
	store := newGenerationStore(workspace, nil)

	// New services would be written by the old code before the network-groups
	// fetch failed; the new layout must not expose them.
	g.services = testServices("v2")
	g.groupsErr = errors.New("grail network groups unavailable")
	provider.CachedGrailAllServicesJSON = common.CacheOptionRefresh
	provider.CachedFxMappingsJSON = common.CacheOptionRefresh

	registry, err = provider.Provide(context.Background())
	require.NoError(t, err, "a complete previous generation must be served")
	require.True(t, registry.Meta.FellBack, "fallback must be explicitly recorded")
	require.Contains(t, registry.Meta.FallbackReason, "network groups")
	require.Equal(t, firstID, registry.Meta.Generation)
	require.Equal(t, "service-v1", registry.Services[0].Name, "new services must not be paired with old data")
	require.Equal(t, firstID, activeGenerationID(t, store), "active pointer must be untouched")

	// The following auto run reads the intact generation normally.
	autoProvider := newTestProvider(t, workspace, g)
	autoProvider.CachedGrailAllServicesJSON = common.CacheOptionAuto
	autoProvider.CachedFxMappingsJSON = common.CacheOptionAuto
	registry, err = autoProvider.Provide(context.Background())
	require.NoError(t, err)
	require.False(t, registry.Meta.FellBack)
	require.Equal(t, firstID, registry.Meta.Generation)
	require.Equal(t, "service-v1", registry.Services[0].Name)
}

func TestProvideManaged_FirstRefreshFailureWithoutBaselineFailsNamingComponent(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	g := &fakeGrail{services: testServices("v1"), groupsErr: errors.New("grail down")}
	provider := newTestProvider(t, workspace, g)
	provider.CachedFxMappingsJSON = common.CacheOptionRefresh

	_, err := provider.Provide(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "network groups")
	require.Contains(t, err.Error(), "no recoverable complete generation")

	store := newGenerationStore(workspace, nil)
	_, statErr := os.Stat(store.activePointerPath())
	require.ErrorIs(t, statErr, os.ErrNotExist, "no pointer may be published on a failed first refresh")

	// Once the underlying problem is repaired, refresh succeeds.
	g.groupsErr = nil
	registry, err := newTestProvider(t, workspace, g).Provide(context.Background())
	require.NoError(t, err)
	require.False(t, registry.Meta.FellBack)
	require.Equal(t, "service-v1", registry.Services[0].Name)
}

func TestProvideManaged_FatalFxScanFailureKeepsPreviousGeneration(t *testing.T) {
	t.Parallel()
	// First refresh succeeds with an intact src/ tree.
	workspace := prepareTestWorkspace(t, true)
	g := &fakeGrail{services: testServices("v1"), groups: testGroups("v1")}
	provider := newTestProvider(t, workspace, g)
	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	firstID := registry.Meta.Generation

	// Removing src/ makes the workspace walk fail fatally (ErrFxScanFailed).
	require.NoError(t, os.RemoveAll(filepath.Join(workspace, "src")))
	provider.CachedFxMappingsJSON = common.CacheOptionRefresh
	registry, err = provider.Provide(context.Background())
	require.NoError(t, err)
	require.True(t, registry.Meta.FellBack)
	require.Contains(t, registry.Meta.FallbackReason, servicemapping.ErrFxScanFailed.Error())
	require.Equal(t, firstID, registry.Meta.Generation)
}

func TestProvideManaged_LegacyCachesAdoptedOnFailedRefresh(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	legacyServices := testServices("legacy")
	legacyMapping := testMapping("legacy")
	writeLegacyCaches(t, workspace, legacyServices, legacyMapping)
	servicesPath := filepath.Join(workspace, "build", "rpcshield", legacyServicesFileName)
	mappingPath := filepath.Join(workspace, "build", "rpcshield", legacyFxMappingsFileName)
	servicesInfoBefore := requireStat(t, servicesPath)
	mappingInfoBefore := requireStat(t, mappingPath)

	g := &fakeGrail{services: testServices("fresh"), groupsErr: errors.New("grail down")}
	provider := newTestProvider(t, workspace, g)
	provider.CachedFxMappingsJSON = common.CacheOptionRefresh

	// First run: legacy caches are adopted as the baseline, the fresh fetch
	// fails, and the run completes on the adopted legacy generation.
	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	require.True(t, registry.Meta.FellBack)
	require.Contains(t, registry.Meta.FallbackReason, "network groups")
	require.Equal(t, legacyServices, registry.Services)
	require.Contains(t, registry.Meta.Generation, "gen-")
	require.Equal(t, sourceLegacy, registry.Meta.Components[ComponentServices].Source)
	require.Equal(t, sourceLegacy, registry.Meta.Components[ComponentFxMappings].Source)
	require.Equal(t, sourceUnavailable, registry.Meta.Components[ComponentNetworkGroups].Source)

	// The legacy files themselves must remain untouched (read-only takeover).
	requireStatUnchanged(t, servicesPath, servicesInfoBefore)
	requireStatUnchanged(t, mappingPath, mappingInfoBefore)

	// After the problem is repaired, an explicit refresh publishes a fresh
	// complete generation.
	g.groupsErr = nil
	registry, err = provider.Provide(context.Background())
	require.NoError(t, err)
	require.False(t, registry.Meta.FellBack)
	require.Equal(t, "service-fresh", registry.Services[0].Name)
	requireGenerationSource(t, registry, ComponentNetworkGroups)
}

func TestProvideManaged_AutoServesFreshLegacyCachesWithoutFetch(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	writeLegacyCaches(t, workspace, testServices("legacy"), testMapping("legacy"))
	g := &fakeGrail{services: testServices("fresh"), groups: testGroups("fresh")}

	registry, err := newTestProvider(t, workspace, g).Provide(context.Background())
	require.NoError(t, err)
	require.False(t, registry.Meta.FellBack)
	require.Contains(t, registry.Meta.Generation, "legacy-")
	require.Equal(t, "service-legacy", registry.Services[0].Name)
	require.Equal(t, sourceLegacy, registry.Meta.Components[ComponentServices].Source)
	servicesCalls, groupsCalls := g.calls()
	require.Zero(t, servicesCalls)
	require.Zero(t, groupsCalls)

	// The read-through must not mutate the workspace on its own.
	_, err = os.Stat(filepath.Join(workspace, "build", "rpcshield", registryDirName))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestProvideManaged_StaleLegacyCachesTriggerAdoptionAndRefresh(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	writeLegacyCaches(t, workspace, testServices("legacy"), testMapping("legacy"))
	oldTime := time.Now().Add(-maxGenerationAge - time.Hour)
	root := filepath.Join(workspace, "build", "rpcshield")
	require.NoError(t, os.Chtimes(filepath.Join(root, legacyServicesFileName), oldTime, oldTime))
	require.NoError(t, os.Chtimes(filepath.Join(root, legacyFxMappingsFileName), oldTime, oldTime))

	g := &fakeGrail{services: testServices("fresh"), groups: testGroups("fresh")}
	registry, err := newTestProvider(t, workspace, g).Provide(context.Background())
	require.NoError(t, err)
	require.Equal(t, "service-fresh", registry.Services[0].Name)
	require.Contains(t, registry.Meta.Generation, "gen-")
	requireGenerationSource(t, registry, ComponentServices)
}

func TestProvideManaged_CorruptActiveGenerationFallsBackToPrevious(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	g := &fakeGrail{services: testServices("v1"), groups: testGroups("v1")}
	provider := newTestProvider(t, workspace, g)
	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	firstID := registry.Meta.Generation
	store := newGenerationStore(workspace, nil)

	// Publish a second generation, then corrupt its services component.
	g.services = testServices("v2")
	g.groups = testGroups("v2")
	provider.CachedGrailAllServicesJSON = common.CacheOptionRefresh
	provider.CachedFxMappingsJSON = common.CacheOptionRefresh
	registry, err = provider.Provide(context.Background())
	require.NoError(t, err)
	secondID := registry.Meta.Generation
	require.NotEqual(t, firstID, secondID)

	corruptPath := filepath.Join(store.generationsDir(), secondID, servicesFileName)
	require.NoError(t, os.WriteFile(corruptPath, []byte("{not json"), 0o644))

	// Auto must detect the corruption, record the fallback, and serve the
	// previous complete generation.
	registry, err = newTestProvider(t, workspace, g).Provide(context.Background())
	require.NoError(t, err)
	require.True(t, registry.Meta.FellBack)
	require.Contains(t, registry.Meta.FallbackReason, ComponentServices)
	require.Equal(t, firstID, registry.Meta.Generation)
	require.Equal(t, "service-v1", registry.Services[0].Name)
}

func TestProvideManaged_MissingComponentFallsBackThenFailsWhenNothingLeft(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	g := &fakeGrail{services: testServices("v1"), groups: testGroups("v1")}
	provider := newTestProvider(t, workspace, g)
	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	firstID := registry.Meta.Generation
	store := newGenerationStore(workspace, nil)

	// Publish a second generation and delete its mapping file.
	g.services = testServices("v2")
	g.groups = testGroups("v2")
	provider.CachedGrailAllServicesJSON = common.CacheOptionRefresh
	provider.CachedFxMappingsJSON = common.CacheOptionRefresh
	registry, err = provider.Provide(context.Background())
	require.NoError(t, err)
	secondID := registry.Meta.Generation
	require.NoError(t, os.Remove(filepath.Join(store.generationsDir(), secondID, fxMappingsFileName)))

	registry, err = newTestProvider(t, workspace, g).Provide(context.Background())
	require.NoError(t, err)
	require.True(t, registry.Meta.FellBack)
	require.Equal(t, firstID, registry.Meta.Generation)
	require.Contains(t, registry.Meta.FallbackReason, ComponentFxMappings)

	// Remove the only fallback generation and make a refresh impossible: the
	// error must name the missing component and state that nothing is left.
	require.NoError(t, os.RemoveAll(filepath.Join(store.generationsDir(), firstID)))
	g.groupsErr = errors.New("grail down")
	_, err = newTestProvider(t, workspace, g).Provide(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), ComponentFxMappings)
	require.Contains(t, err.Error(), "no recoverable complete generation")
}

func TestProvideManaged_ManifestChecksumMismatchSelfHealsOnRefresh(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	g := &fakeGrail{services: testServices("v1"), groups: testGroups("v1")}
	provider := newTestProvider(t, workspace, g)
	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	store := newGenerationStore(workspace, nil)
	id := registry.Meta.Generation

	// Tamper with the manifest without updating the active pointer digest.
	manifestPath := filepath.Join(store.generationsDir(), id, genManifestName)
	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	var manifest generationManifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	manifest.ScanWarnings = 999
	tampered, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(manifestPath, tampered, 0o644))

	// Auto cannot trust the active candidate; with Grail reachable it refreshes
	// and publishes a new, internally consistent generation.
	registry, err = newTestProvider(t, workspace, g).Provide(context.Background())
	require.NoError(t, err)
	require.False(t, registry.Meta.FellBack)
	require.NotEqual(t, id, registry.Meta.Generation)
	require.Equal(t, "service-v1", registry.Services[0].Name)
}

func TestProvideManaged_ConcurrentRefreshesAndReadsNeverExposePartialGeneration(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	g := &fakeGrail{}
	for i := range 8 {
		g.servicesByCall = append(g.servicesByCall, testServices(fmt.Sprintf("v%d", i)))
		g.groupsByCall = append(g.groupsByCall, testGroups(fmt.Sprintf("v%d", i)))
	}

	const writers, readers = 6, 8
	var wg sync.WaitGroup
	errs := make(chan error, writers+readers)
	regCh := make(chan ServiceRegistry, writers+readers)
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := newTestProvider(t, workspace, g)
			p.CachedGrailAllServicesJSON = common.CacheOptionRefresh
			p.CachedFxMappingsJSON = common.CacheOptionRefresh
			registry, err := p.Provide(context.Background())
			if err != nil {
				errs <- err
				return
			}
			regCh <- registry
		}()
	}
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond)
			registry, err := newTestProvider(t, workspace, g).Provide(context.Background())
			if err != nil {
				errs <- err
				return
			}
			regCh <- registry
		}()
	}
	wg.Wait()
	close(errs)
	close(regCh)
	for err := range errs {
		require.NoError(t, err)
	}

	store := newGenerationStore(workspace, nil)
	pointer, err := store.readActivePointer()
	require.NoError(t, err)
	active, err := store.loadGeneration(pointer.Generation, pointer.ManifestSHA256)
	require.NoError(t, err, "active generation must be complete after concurrent refreshes")
	require.NotEmpty(t, active.bundle.Services)

	for registry := range regCh {
		require.NotEmpty(t, registry.Services)
		// Components returned to a reader must be mutually paired: every
		// service referenced by the mapping has to be in the services slice of
		// the same result (never paired with another writer's services).
		serviceNames := make(map[string]struct{}, len(registry.Services))
		for _, service := range registry.Services {
			serviceNames[service.Name] = struct{}{}
		}
		for _, infos := range registry.ClientTypeMapping {
			for _, info := range infos {
				_, ok := serviceNames[info.Service.Name]
				require.True(t, ok, "foreign pairing for %q in generation %q",
					info.Service.Name, registry.Meta.Generation)
			}
		}
	}
}

func TestStore_ConcurrentPublishesKeepBundleComponentsPaired(t *testing.T) {
	t.Parallel()
	store := newGenerationStore(t.TempDir(), nil)
	require.NoError(t, os.MkdirAll(store.generationsDir(), 0o755))

	bundle := func(tag string) registryBundle {
		return registryBundle{
			Services:      testServices(tag),
			NetworkGroups: testGroups(tag),
			Mapping:       testMapping(tag),
			Sources: map[string]string{
				ComponentServices:      componentSourceGrail,
				ComponentNetworkGroups: componentSourceGrail,
				ComponentFxMappings:    componentSourceScan,
			},
		}
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for _, tag := range []string{"A", "B"} {
		wg.Add(1)
		go func(tag string) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := store.stageAndPublish(bundle(tag)); err != nil {
					t.Errorf("publish %s: %v", tag, err)
					return
				}
			}
		}(tag)
	}

	// Readers must always observe a bundle whose mapping references only
	// services from the same bundle -- never services from one writer paired
	// with the other writer's mapping. loadBest cross-validates exactly that.
	// Before the first publish completes there is briefly nothing to read.
	assertPaired := func(gen *loadedGeneration) {
		serviceNames := map[string]bool{}
		for _, s := range gen.bundle.Services {
			serviceNames[s.Name] = true
		}
		for _, infos := range gen.bundle.Mapping {
			for _, info := range infos {
				require.True(t, serviceNames[info.Service.Name],
					"mapping paired with foreign generation's services: %q not in generation %q",
					info.Service.Name, gen.id)
			}
		}
	}
	// Poll while the writers keep publishing (writes are much slower than
	// in-memory reads, so a fixed iteration count could finish before the
	// first generation hits disk).
	defer func() {
		close(stop)
		wg.Wait()
	}()
	observed := 0
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		gen, _, _, err := store.loadBest()
		if err != nil {
			// Before the first publish completes there is briefly nothing to
			// read; anything else is a real failure.
			if strings.Contains(err.Error(), "no service registry generation found") {
				time.Sleep(time.Millisecond)
				continue
			}
			require.NoError(t, err)
		}
		require.NotNil(t, gen)
		assertPaired(gen)
		observed++
	}
	require.Greater(t, observed, 50, "expected many consistent reads during the publish window")
}

// --- explicit read-only file paths ---

func TestProvideExternal_BothExplicitPathsAreReadOnly(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	externalDir := t.TempDir()
	servicesPath := filepath.Join(externalDir, "my_services.json")
	mappingPath := filepath.Join(externalDir, "my_fx_mappings.json")
	servicesBytes := writeJSONFile(t, servicesPath, testServices("ext"))
	mappingBytes := writeJSONFile(t, mappingPath, testMapping("ext"))
	servicesInfo := requireStat(t, servicesPath)
	mappingInfo := requireStat(t, mappingPath)

	g := &fakeGrail{services: testServices("ignored"), groups: testGroups("ignored")}
	provider := &DefaultServiceRegistryProvider{
		Grail:                      g,
		Workspace:                  workspace,
		CachedGrailAllServicesJSON: common.CacheOption(servicesPath),
		CachedFxMappingsJSON:       common.CacheOption(mappingPath),
	}
	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	require.Equal(t, testServices("ext"), registry.Services)
	require.Contains(t, registry.Meta.Generation, "external-")
	require.Equal(t, sourceExplicit, registry.Meta.Components[ComponentServices].Source)
	require.Equal(t, servicesPath, registry.Meta.Components[ComponentServices].Path)
	require.Equal(t, sha256Hex(servicesBytes), registry.Meta.Components[ComponentServices].SHA256)
	require.Equal(t, sourceExplicit, registry.Meta.Components[ComponentFxMappings].Source)
	require.Equal(t, sha256Hex(mappingBytes), registry.Meta.Components[ComponentFxMappings].SHA256)
	require.Nil(t, registry.NetworkGroups)
	require.Equal(t, sourceUnavailable, registry.Meta.Components[ComponentNetworkGroups].Source)

	// Nothing may be written into the workspace and the explicit files are
	// untouched.
	_, err = os.Stat(filepath.Join(workspace, "build"))
	require.ErrorIs(t, err, os.ErrNotExist)
	requireStatUnchanged(t, servicesPath, servicesInfo)
	requireStatUnchanged(t, mappingPath, mappingInfo)
	servicesCalls, groupsCalls := g.calls()
	require.Zero(t, servicesCalls)
	require.Zero(t, groupsCalls)

	// A corrupt explicit file is a hard error that names the file and
	// component; there is no generation store to fall back to.
	require.NoError(t, os.WriteFile(servicesPath, []byte("{"), 0o644))
	_, err = provider.Provide(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "explicit services cache")
	require.Contains(t, err.Error(), servicesPath)
}

func TestProvideExternal_MappingReferencingMissingServiceIsAccepted(t *testing.T) {
	t.Parallel()
	// The Fx mapping embeds the owning service itself, so an explicit mapping
	// produced against a larger/different services list remains usable.
	workspace := prepareTestWorkspace(t, true)
	externalDir := t.TempDir()
	servicesPath := filepath.Join(externalDir, "services.json")
	mappingPath := filepath.Join(externalDir, "mapping.json")
	writeJSONFile(t, servicesPath, []grail.Service{{Name: "only-this", Tier: 1}})
	writeJSONFile(t, mappingPath, testMapping("other"))

	provider := &DefaultServiceRegistryProvider{
		Grail:                      &fakeGrail{},
		Workspace:                  workspace,
		CachedGrailAllServicesJSON: common.CacheOption(servicesPath),
		CachedFxMappingsJSON:       common.CacheOption(mappingPath),
	}
	_, err := provider.Provide(context.Background())
	require.NoError(t, err)
}

func TestProvideExternal_MixedExplicitServicesAndManagedMapping(t *testing.T) {
	t.Parallel()
	workspace := prepareTestWorkspace(t, true)
	externalDir := t.TempDir()
	servicesPath := filepath.Join(externalDir, "services.json")
	writeJSONFile(t, servicesPath, testServices("ext"))
	servicesInfo := requireStat(t, servicesPath)

	g := &fakeGrail{groups: testGroups("g1")}
	provider := &DefaultServiceRegistryProvider{
		Grail:                      g,
		Workspace:                  workspace,
		CachedGrailAllServicesJSON: common.CacheOption(servicesPath),
		CachedFxMappingsJSON:       common.CacheOptionRefresh,
	}
	registry, err := provider.Provide(context.Background())
	require.NoError(t, err)
	require.Equal(t, testGroups("g1"), registry.NetworkGroups)
	require.Equal(t, sourceExplicit, registry.Meta.Components[ComponentServices].Source)
	require.Equal(t, sourceFetched, registry.Meta.Components[ComponentFxMappings].Source)
	require.Equal(t, sourceGrail, registry.Meta.Components[ComponentNetworkGroups].Source)

	// The explicit services file is still read-only; the managed mapping uses
	// the legacy single-file cache.
	legacyMappingPath := filepath.Join(workspace, "build", "rpcshield", legacyFxMappingsFileName)
	require.FileExists(t, legacyMappingPath)
	requireStatUnchanged(t, servicesPath, servicesInfo)

	// Second run: mapping served from the legacy cache, no new network fetch.
	provider.CachedFxMappingsJSON = common.CacheOptionAuto
	registry, err = provider.Provide(context.Background())
	require.NoError(t, err)
	require.Equal(t, sourceCache, registry.Meta.Components[ComponentFxMappings].Source)
	require.Equal(t, sourceUnavailable, registry.Meta.Components[ComponentNetworkGroups].Source)
	_, groupsCalls := g.calls()
	require.Equal(t, 1, groupsCalls)
}

// --- small fs-stat helpers ---

type fileInfoSummary struct {
	size    int64
	modTime time.Time
	mode    os.FileMode
}

func requireStat(t *testing.T, path string) fileInfoSummary {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return fileInfoSummary{size: info.Size(), modTime: info.ModTime(), mode: info.Mode()}
}

func requireStatUnchanged(t *testing.T, path string, before fileInfoSummary) {
	t.Helper()
	after := requireStat(t, path)
	require.Equal(t, before, after, "file %q must not be modified", path)
}
