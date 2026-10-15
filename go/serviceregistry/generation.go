package serviceregistry

// This file implements the on-disk "registry generation" store used by
// DefaultServiceRegistryProvider for the managed (auto/refresh) cache modes.
//
// A generation is the indivisible bundle of the three components the service
// registry is built from:
//
//   - services:        the Grail AllServices result,
//   - network_groups:  the Grail AllNetworkGroups result,
//   - fx_mappings:     the FxScanner mapping produced from the two above.
//
// The services tier hierarchy and the client ownership mapping must always be
// interpreted together: pairing new services with an old mapping (the bug this
// store exists to prevent) makes the analyzer attribute outbound clients using
// stale ownership. A refresh therefore stages the whole bundle in an inactive
// directory, validates it, and only then flips a single active pointer via an
// atomic rename. Readers either see the complete previous generation or the
// complete new one. A failed fetch/scan/encode/sync/publish never changes the
// active pointer, so every subsequent run keeps serving the last complete
// generation and explicitly records the fallback.
//
// Layout (under <Workspace>/build/rpcshield/):
//
//	grail_all_services.json        # legacy single-file cache (read-only takeover source)
//	fx_mappings.json               # legacy single-file cache (read-only takeover source)
//	registry/
//	  refresh.lock                 # cross-process flock held while refreshing
//	  active.json                  # current generation pointer (atomically renamed)
//	  generations/<gen-id>/
//	      manifest.json            # per-component size/hash + provenance
//	      services.json
//	      network_groups.json
//	      fx_mappings.json

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	"github.com/uber-research/rpcshield/go/grail"
	"go.uber.org/zap"
)

const (
	// registryDirName is the directory (under <Workspace>/build/rpcshield/) that
	// holds the active pointer, the refresh lock and all generation directories.
	registryDirName = "registry"
	// generationsDirName holds one directory per staged generation.
	generationsDirName = "generations"
	// activePointerName is the atomically-published pointer to the active generation.
	activePointerName = "active.json"
	// refreshLockName serializes refreshes across processes.
	refreshLockName = "refresh.lock"
	genManifestName = "manifest.json"

	servicesFileName      = "services.json"
	networkGroupsFileName = "network_groups.json"
	fxMappingsFileName    = "fx_mappings.json"

	// Legacy single-file cache names (previous on-disk layout).
	legacyServicesFileName   = "grail_all_services.json"
	legacyFxMappingsFileName = "fx_mappings.json"

	// Provenance of a component recorded in the generation manifest: where the
	// component was obtained when the generation was built.
	componentSourceGrail  = "grail"
	componentSourceScan   = "fx-scan"
	componentSourceLegacy = "legacy"

	// Per-run component sources surfaced in RegistryMeta/metrics.
	sourceGeneration  = "generation"
	sourceLegacy      = "legacy"
	sourceUnavailable = "unavailable"

	// maxGenerationAge mirrors common._maxCacheAge: an auto run refreshes when
	// the active generation is older than a week.
	maxGenerationAge = 7 * 24 * time.Hour
	// keepGenerations is the number of generation directories retained (the
	// active one plus a couple of previous, verified generations for fallback).
	keepGenerations = 3
)

// Registry component names. They are used as RegistryMeta.Components keys and
// as metric name suffixes.
const (
	ComponentServices      = "services"
	ComponentNetworkGroups = "network_groups"
	ComponentFxMappings    = "fx_mappings"
)

// generationManifest is the per-generation manifest describing and authenticating
// its component files.
type generationManifest struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	// Components maps the component name (Component* constants) to its file record.
	Components map[string]componentManifest `json:"components"`
	// ScanWarnings is the number of non-fatal Fx-scanner diagnostics observed
	// while building the generation (the mapping is still staged).
	ScanWarnings int `json:"scan_warnings,omitempty"`
}

// componentManifest describes one component file within a generation directory.
type componentManifest struct {
	// File is the component's file name inside the generation directory.
	File string `json:"file"`
	// Present is false only for components that never existed in the source the
	// generation was adopted from (network groups were not cached by the legacy
	// layout). A missing/corrupt file of a Present component invalidates the
	// generation.
	Present bool   `json:"present"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	// Source records how the component was obtained (componentSource* constants).
	Source string `json:"source"`
}

// activePointer is the content of active.json.
type activePointer struct {
	Generation  string    `json:"generation"`
	PublishedAt time.Time `json:"published_at"`
	// ManifestSHA256 authenticates the manifest of the generation that was
	// current at publish time, so a swapped/tampered manifest is detected.
	ManifestSHA256 string `json:"manifest_sha256"`
}

// registryBundle is an in-memory set of the three registry components, ready to
// be staged as a generation.
type registryBundle struct {
	Services      []grail.Service
	NetworkGroups []grail.NetworkGroup
	Mapping       servicemapping.ServiceMapping
	// Sources maps component name (Component*) to its componentSource* label.
	Sources map[string]string
	// CreatedAt is stamped onto the generation; zero means time.Now().UTC().
	CreatedAt time.Time
	// ScanWarnings is recorded in the manifest for observability.
	ScanWarnings int
}

// loadedGeneration is a generation that has been read from disk and fully
// verified.
type loadedGeneration struct {
	id       string
	dir      string
	manifest generationManifest
	bundle   registryBundle
}

// generationStore manages the generation directory under the rpcshield cache root.
type generationStore struct {
	// root is <Workspace>/build/rpcshield (also where the legacy files live).
	root string
	// dir is root/registry.
	dir    string
	logger *zap.SugaredLogger
}

func newGenerationStore(workspace string, logger *zap.SugaredLogger) *generationStore {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	root := filepath.Join(workspace, "build", "rpcshield")
	return &generationStore{
		root:   root,
		dir:    filepath.Join(root, registryDirName),
		logger: logger,
	}
}

func (s *generationStore) generationsDir() string {
	return filepath.Join(s.dir, generationsDirName)
}

func (s *generationStore) activePointerPath() string {
	return filepath.Join(s.dir, activePointerName)
}

func (s *generationStore) lockPath() string {
	return filepath.Join(s.dir, refreshLockName)
}

// provideManaged serves the registry when BOTH cache options are managed
// (auto/refresh, i.e. no explicit read-only file paths).
//
// forceRefresh corresponds to either option being "refresh": a new generation
// is built unconditionally. Any failure leaves (and, on first refresh, adopts
// from legacy files) a complete previous generation, which is then served with
// fallback recorded.
func (s *generationStore) provideManaged(forceRefresh bool, build func() (registryBundle, error)) (ServiceRegistry, error) {
	if !forceRefresh {
		if gen, fellBack, reason, err := s.loadBest(); err == nil {
			age := time.Since(gen.manifest.CreatedAt)
			if age < maxGenerationAge {
				s.logger.Infow("Using service registry generation",
					"generation", gen.id, "age", age.String(), "max_age", maxGenerationAge.String(),
					"fell_back", fellBack, "fallback_reason", reason)
				return gen.toRegistry(fellBack, reason), nil
			}
			s.logger.Infow("Service registry generation is stale, refreshing",
				"generation", gen.id, "age", age.String(), "max_age", maxGenerationAge.String())
		} else {
			s.logger.Infow("No usable service registry generation found, refreshing", "error", err)
		}
	}

	newGen, refreshErr := s.refresh(build)
	if refreshErr == nil {
		s.logger.Infow("Published new service registry generation", "generation", newGen.id)
		return newGen.toRegistry(false, ""), nil
	}

	// Any fetch/scan/encode/sync/publish error: keep serving the last complete
	// generation (the active pointer is guaranteed untouched on failure).
	oldGen, loadFellBack, loadReason, loadErr := s.loadBest()
	if loadErr != nil {
		return ServiceRegistry{}, fmt.Errorf("service registry refresh failed: %w; no recoverable complete generation: %v", refreshErr, loadErr)
	}
	reason := fmt.Sprintf("service registry refresh failed (%v); continuing with complete generation %q", refreshErr, oldGen.id)
	if loadFellBack && loadReason != "" {
		reason += "; " + loadReason
	}
	s.logger.Errorw("Service registry refresh failed, continuing with the previous complete generation",
		"error", refreshErr, "generation", oldGen.id)
	return oldGen.toRegistry(true, reason), nil
}

// refresh builds, stages, validates and atomically publishes one new generation.
// It serializes against other refreshes of the same workspace via an exclusive
// file lock. On entry it adopts the legacy single-file caches as a baseline
// generation when the store is still empty, so a fetch/scan failure on the
// first refresh with the new layout leaves a complete generation active.
func (s *generationStore) refresh(build func() (registryBundle, error)) (*loadedGeneration, error) {
	if err := os.MkdirAll(s.generationsDir(), 0o755); err != nil {
		return nil, fmt.Errorf("create registry directory: %w", err)
	}
	lock, err := acquireRefreshLock(s.lockPath())
	if err != nil {
		return nil, fmt.Errorf("acquire registry refresh lock: %w", err)
	}
	defer func() { _ = lock.release() }()

	s.removeStagingDirs()

	// Before risking any network/scan work, make sure a complete baseline is
	// published in the store (adopted from the legacy layout when necessary).
	// Legacy files alone are intentionally NOT considered sufficient here:
	// adopting them into a generation before the fetch means a failed
	// fetch/scan on the first refresh still leaves a complete active
	// generation behind.
	if gen, _, _, err := s.loadStoredBest(); err != nil || gen == nil {
		if _, adoptErr := s.importLegacyGeneration(); adoptErr != nil {
			if !errors.Is(adoptErr, errLegacyCacheAbsent) {
				s.logger.Warnw("Failed to adopt legacy service registry caches as refresh baseline", "error", adoptErr)
			}
		} else {
			s.logger.Infow("Adopted legacy service registry caches as refresh baseline")
		}
	}

	bundle, err := build()
	if err != nil {
		return nil, err // fetch/scan errors: active pointer intentionally untouched
	}
	gen, err := s.stageAndPublish(bundle)
	if err != nil {
		return nil, err // encode/sync/publish errors: active pointer untouched
	}
	s.pruneGenerations()
	return gen, nil
}

// stageAndPublish encodes the whole bundle into a private staging directory,
// verifies it by reading it back, then makes it visible by renaming the staging
// directory into place and atomically flipping the active pointer. Nothing the
// function does on the failure path is observable to readers.
func (s *generationStore) stageAndPublish(bundle registryBundle) (*loadedGeneration, error) {
	if err := validateBundle(bundle.Services, bundle.NetworkGroups, bundle.Mapping); err != nil {
		return nil, fmt.Errorf("validate registry bundle: %w", err)
	}

	createdAt := bundle.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	id, err := newGenerationID(createdAt)
	if err != nil {
		return nil, fmt.Errorf("generate generation id: %w", err)
	}

	stagingDir, err := os.MkdirTemp(s.generationsDir(), ".stage-*")
	if err != nil {
		return nil, fmt.Errorf("create staging directory: %w", err)
	}
	// On any error after this point, drop the half-written staging directory.
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	encoded := make(map[string][]byte, 3)
	encode := func(component string, value any) error {
		b, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("encode %s component: %w", component, err)
		}
		encoded[component] = b
		return nil
	}
	if err := encode(ComponentServices, bundle.Services); err != nil {
		return nil, err
	}
	if err := encode(ComponentNetworkGroups, bundle.NetworkGroups); err != nil {
		return nil, err
	}
	if err := encode(ComponentFxMappings, bundle.Mapping); err != nil {
		return nil, err
	}

	sources := bundle.Sources
	if sources == nil {
		sources = map[string]string{}
	}
	manifest := generationManifest{
		ID:           id,
		CreatedAt:    createdAt,
		Components:   map[string]componentManifest{},
		ScanWarnings: bundle.ScanWarnings,
	}
	writeComponent := func(component, fileName string, present bool) error {
		cm := componentManifest{File: fileName, Present: present, Source: sources[component]}
		if present {
			data := encoded[component]
			if err := writeFileAtomic(filepath.Join(stagingDir, fileName), data); err != nil {
				return fmt.Errorf("write %s component: %w", component, err)
			}
			cm.Size = int64(len(data))
			cm.SHA256 = sha256Hex(data)
		}
		manifest.Components[component] = cm
		return nil
	}
	if err := writeComponent(ComponentServices, servicesFileName, true); err != nil {
		return nil, err
	}
	// A freshly fetched (even if empty) network groups list is a real component;
	// only the legacy-adoption source is allowed to omit it.
	networkGroupsPresent := sources[ComponentNetworkGroups] != componentSourceLegacy
	if err := writeComponent(ComponentNetworkGroups, networkGroupsFileName, networkGroupsPresent); err != nil {
		return nil, err
	}
	if err := writeComponent(ComponentFxMappings, fxMappingsFileName, true); err != nil {
		return nil, err
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode generation manifest: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(stagingDir, genManifestName), manifestBytes); err != nil {
		return nil, fmt.Errorf("write generation manifest: %w", err)
	}

	// Read everything back and verify it before the generation can become
	// active. This fails on encoding corruption that happened to hit disk.
	staged := &loadedGeneration{id: id, dir: stagingDir}
	if err := staged.loadFromDir(manifestBytes); err != nil {
		return nil, fmt.Errorf("verify staged generation: %w", err)
	}

	// Move the verified generation directory into its final name. Readers never
	// reference it until the pointer flip below completes.
	finalDir := filepath.Join(s.generationsDir(), id)
	if err := os.Rename(stagingDir, finalDir); err != nil {
		return nil, fmt.Errorf("publish generation directory: %w", err)
	}
	if err := syncDir(s.generationsDir()); err != nil {
		return nil, fmt.Errorf("sync generations directory: %w", err)
	}
	staged.dir = finalDir

	// Flip the active pointer with a single atomic rename.
	pointer := activePointer{
		Generation:     id,
		PublishedAt:    time.Now().UTC(),
		ManifestSHA256: sha256Hex(manifestBytes),
	}
	pointerBytes, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode active pointer: %w", err)
	}
	if err := writeFileAtomic(s.activePointerPath(), pointerBytes); err != nil {
		return nil, fmt.Errorf("publish active generation pointer: %w", err)
	}

	committed = true
	return staged, nil
}

// loadBest returns the best complete generation readable right now: the active
// generation; if that one is unusable (missing files, corruption, manifest
// mismatch), the most recent other complete generation; the generation a
// concurrent refresh just flipped the pointer to; then the legacy single-file
// caches. The returned fallback flag/reason are set whenever a broken or
// missing active candidate had to be bypassed. It never takes the refresh
// lock, so it is safe to call from readers and while the caller holds the lock.
func (s *generationStore) loadBest() (*loadedGeneration, bool, string, error) {
	return s.loadBestInternal(true)
}

// loadStoredBest is loadBest restricted to the generation store: it never
// serves the legacy single-file caches (used while refreshing, to decide
// whether legacy caches must be adopted into the store first).
func (s *generationStore) loadStoredBest() (*loadedGeneration, bool, string, error) {
	return s.loadBestInternal(false)
}

func (s *generationStore) loadBestInternal(allowLegacy bool) (*loadedGeneration, bool, string, error) {
	pointer, pointerErr := s.readActivePointer()
	activeProblem := ""
	if pointerErr != nil && !errors.Is(pointerErr, fs.ErrNotExist) {
		activeProblem = fmt.Sprintf("active generation pointer %q unreadable: %v", s.activePointerPath(), pointerErr)
	}

	if activeProblem == "" && pointer.Generation != "" {
		gen, err := s.loadGeneration(pointer.Generation, pointer.ManifestSHA256)
		if err == nil {
			return gen, false, "", nil
		}
		activeProblem = fmt.Sprintf("active generation %q unusable: %v", pointer.Generation, err)
	}

	// Search the on-disk generations (newest first) for a complete fallback.
	for _, id := range s.listGenerationIDs() {
		if id == pointer.Generation {
			continue // already tried above
		}
		gen, err := s.loadGeneration(id, "")
		if err != nil {
			s.logger.Warnw("Skipping unusable service registry generation while looking for a fallback",
				"generation", id, "error", err)
			continue
		}
		reason := ""
		if activeProblem != "" {
			reason = fmt.Sprintf("%s; using previous complete generation %q", activeProblem, id)
		}
		return gen, activeProblem != "", reason, nil
	}

	// A concurrent refresh may have flipped the pointer (and pruned the
	// generation we started validating) while we were scanning. Re-read the
	// pointer before giving up: the generation it now points at is never
	// pruned.
	if refreshed, err := s.readActivePointer(); err == nil && refreshed.Generation != "" &&
		refreshed.Generation != pointer.Generation {
		if gen, loadErr := s.loadGeneration(refreshed.Generation, refreshed.ManifestSHA256); loadErr == nil {
			if activeProblem == "" {
				// No pointer existed when we started; a concurrent refresh just
				// published the first generation, so this is not a fallback.
				return gen, false, "", nil
			}
			return gen, true, fmt.Sprintf("%s; concurrent refresh published generation %q", activeProblem, refreshed.Generation), nil
		}
	}

	if allowLegacy {
		if legacy, err := s.loadLegacyGeneration(); err == nil {
			reason := ""
			if activeProblem != "" {
				reason = activeProblem + fmt.Sprintf("; using legacy caches %q and %q",
					filepath.Join(s.root, legacyServicesFileName), filepath.Join(s.root, legacyFxMappingsFileName))
			}
			return legacy, activeProblem != "", reason, nil
		}
	}

	if activeProblem != "" {
		return nil, false, "", fmt.Errorf("%s; no complete previous generation and legacy caches %q/%q are not recoverable",
			activeProblem,
			filepath.Join(s.root, legacyServicesFileName),
			filepath.Join(s.root, legacyFxMappingsFileName))
	}
	return nil, false, "", fmt.Errorf("no service registry generation found under %q", s.dir)
}

// readActivePointer reads and decodes active.json. A missing file returns
// fs.ErrNotExist-wrapped error with an empty pointer.
func (s *generationStore) readActivePointer() (activePointer, error) {
	var pointer activePointer
	b, err := os.ReadFile(s.activePointerPath())
	if err != nil {
		return pointer, err
	}
	if err := json.Unmarshal(b, &pointer); err != nil {
		return pointer, fmt.Errorf("decode active pointer %q: %w", s.activePointerPath(), err)
	}
	if pointer.Generation == "" {
		return pointer, fmt.Errorf("active pointer %q has empty generation id", s.activePointerPath())
	}
	return pointer, nil
}

// loadGeneration reads, authenticates, decodes and cross-validates one
// generation directory. expectedManifestSHA (when non-empty, i.e. when loading
// the generation referenced by the active pointer) must match the manifest
// file bytes.
func (s *generationStore) loadGeneration(id string, expectedManifestSHA string) (*loadedGeneration, error) {
	if id == "" || filepath.Base(id) != id || filepath.IsAbs(id) {
		return nil, fmt.Errorf("invalid generation id %q", id)
	}
	dir := filepath.Join(s.generationsDir(), id)
	manifestPath := filepath.Join(dir, genManifestName)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("component %q: read manifest: %w", genManifestName, err)
	}
	if expectedManifestSHA != "" && sha256Hex(manifestBytes) != expectedManifestSHA {
		return nil, fmt.Errorf("manifest checksum mismatch (pointer expects %s, file has %s)",
			expectedManifestSHA, sha256Hex(manifestBytes))
	}
	gen := &loadedGeneration{id: id, dir: dir}
	if err := gen.loadFromDir(manifestBytes); err != nil {
		return nil, err
	}
	return gen, nil
}

// loadFromDir decodes the manifest bytes and every present component file,
// authenticating size/hash and cross-validating the bundle. It works for both
// final generation directories and the in-progress staging directory. The
// manifest file itself is authenticated by the caller (against the active
// pointer digest) before this is called.
func (g *loadedGeneration) loadFromDir(manifestBytes []byte) error {
	if err := json.Unmarshal(manifestBytes, &g.manifest); err != nil {
		return fmt.Errorf("component %q: decode manifest: %w", genManifestName, err)
	}
	if g.manifest.ID == "" {
		return errors.New("manifest has empty generation id")
	}
	if g.id != "" && g.manifest.ID != g.id {
		return fmt.Errorf("manifest generation id %q does not match directory %q", g.manifest.ID, g.id)
	}

	for _, component := range []string{ComponentServices, ComponentNetworkGroups, ComponentFxMappings} {
		cm, ok := g.manifest.Components[component]
		if !ok {
			return fmt.Errorf("component %q: missing from manifest", component)
		}
		if !cm.Present {
			// Only the legacy layout is allowed to lack a component (network groups).
			if cm.Source != componentSourceLegacy {
				return fmt.Errorf("component %q: marked absent without a legacy provenance", component)
			}
			continue
		}
		if cm.File == "" {
			return fmt.Errorf("component %q: manifest has empty file name", component)
		}
		path := filepath.Join(g.dir, cm.File)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("component %q: %w", component, err)
		}
		if int64(len(data)) != cm.Size {
			return fmt.Errorf("component %q: size mismatch (manifest declares %d bytes, file has %d)", component, cm.Size, len(data))
		}
		if actual := sha256Hex(data); actual != cm.SHA256 {
			return fmt.Errorf("component %q: checksum mismatch (manifest declares %s, file has %s)", component, cm.SHA256, actual)
		}
		switch component {
		case ComponentServices:
			services, err := decodeServices(data)
			if err != nil {
				return fmt.Errorf("component %q: %w", component, err)
			}
			g.bundle.Services = services
		case ComponentNetworkGroups:
			groups, err := decodeNetworkGroups(data)
			if err != nil {
				return fmt.Errorf("component %q: %w", component, err)
			}
			g.bundle.NetworkGroups = groups
		case ComponentFxMappings:
			mapping, err := decodeMapping(data)
			if err != nil {
				return fmt.Errorf("component %q: %w", component, err)
			}
			g.bundle.Mapping = mapping
		}
	}

	if err := validateBundlePresence(&g.manifest); err != nil {
		return err
	}
	if err := validateBundle(g.bundle.Services, g.bundle.NetworkGroups, g.bundle.Mapping); err != nil {
		return err
	}
	return nil
}

// validateBundlePresence enforces that all components are present except that a
// legacy-provenanced generation may omit network groups (the legacy layout never
// cached them; the mapping is self-contained).
func validateBundlePresence(manifest *generationManifest) error {
	for _, component := range []string{ComponentServices, ComponentFxMappings} {
		if cm, ok := manifest.Components[component]; !ok || !cm.Present {
			return fmt.Errorf("component %q: required component is absent", component)
		}
	}
	return nil
}

// validateBundle cross-validates decoded component content against the other
// components, so structurally broken generations never become/remain active.
func validateBundle(services []grail.Service, _ []grail.NetworkGroup, mapping servicemapping.ServiceMapping) error {
	if err := validateServicesStandalone(services); err != nil {
		return err
	}
	serviceNames := make(map[string]struct{}, len(services))
	for _, service := range services {
		serviceNames[service.Name] = struct{}{}
	}
	// Every service referenced by the mapping must come from the same services
	// component. The scanner resolves service objects against the services list,
	// so a violation here means the bundle was assembled from mismatched sources.
	for clientType, infos := range mapping {
		for _, info := range infos {
			if info == nil {
				return fmt.Errorf("fx mappings component contains a nil entry for client %s", clientType)
			}
			if _, ok := serviceNames[info.Service.Name]; !ok {
				return fmt.Errorf("fx mappings component pairs client %s with service %q that is absent from the services component", clientType, info.Service.Name)
			}
		}
	}
	return nil
}

// listGenerationIDs returns valid-looking generation directory names ordered
// newest first (by manifest creation time, then id). Unreadable entries are
// skipped -- loadGeneration re-validates the returned candidates.
func (s *generationStore) listGenerationIDs() []string {
	entries, err := os.ReadDir(s.generationsDir())
	if err != nil {
		return nil
	}
	type candidate struct {
		id        string
		createdAt time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !isGenerationDir(name) {
			continue
		}
		manifestBytes, err := os.ReadFile(filepath.Join(s.generationsDir(), name, genManifestName))
		if err != nil {
			continue
		}
		var manifest generationManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil || manifest.CreatedAt.IsZero() {
			continue
		}
		candidates = append(candidates, candidate{id: name, createdAt: manifest.CreatedAt})
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		if d := b.createdAt.Compare(a.createdAt); d != 0 {
			return d
		}
		if a.id < b.id {
			return 1
		}
		if a.id > b.id {
			return -1
		}
		return 0
	})
	ids := make([]string, 0, len(candidates))
	for _, c := range candidates {
		ids = append(ids, c.id)
	}
	return ids
}

func isGenerationDir(name string) bool {
	// Final directories look like gen-<timestamp>-<random>; staging directories
	// (.stage-*) must never be picked up as generations.
	return len(name) > 4 && name[:4] == "gen-"
}

// pruneGenerations removes old generation directories, always keeping the
// active generation plus the newest few previous ones.
func (s *generationStore) pruneGenerations() {
	pointer, err := s.readActivePointer()
	if err != nil {
		return
	}
	ids := s.listGenerationIDs()
	keep := map[string]bool{pointer.Generation: true}
	kept := 0
	for _, id := range ids {
		if keep[id] {
			continue
		}
		if kept < keepGenerations-1 {
			keep[id] = true
			kept++
		}
	}
	for _, id := range ids {
		if keep[id] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.generationsDir(), id)); err != nil {
			s.logger.Warnw("Failed to prune old service registry generation", "generation", id, "error", err)
		} else {
			s.logger.Infow("Pruned old service registry generation", "generation", id)
		}
	}
}

// removeStagingDirs removes staging directories left behind by crashed
// refreshes. Callers must hold the refresh lock.
func (s *generationStore) removeStagingDirs() {
	entries, err := os.ReadDir(s.generationsDir())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 7 && entry.Name()[:7] == ".stage-" {
			if err := os.RemoveAll(filepath.Join(s.generationsDir(), entry.Name())); err != nil {
				s.logger.Warnw("Failed to remove stale registry staging directory", "dir", entry.Name(), "error", err)
			}
		}
	}
}

// errLegacyCacheAbsent indicates the legacy single-file caches do not exist.
var errLegacyCacheAbsent = errors.New("legacy service registry caches absent")

// readLegacyFiles reads and validates the legacy single-file caches.
func (s *generationStore) readLegacyFiles() (*loadedGeneration, error) {
	servicesPath := filepath.Join(s.root, legacyServicesFileName)
	mappingPath := filepath.Join(s.root, legacyFxMappingsFileName)
	servicesBytes, err := os.ReadFile(servicesPath)
	if err != nil {
		return nil, fmt.Errorf("%w: services: %v", errLegacyCacheAbsent, err)
	}
	mappingBytes, err := os.ReadFile(mappingPath)
	if err != nil {
		return nil, fmt.Errorf("%w: fx mappings: %v", errLegacyCacheAbsent, err)
	}
	services, err := decodeServices(servicesBytes)
	if err != nil {
		return nil, fmt.Errorf("decode legacy services cache %q: %w", servicesPath, err)
	}
	mapping, err := decodeMapping(mappingBytes)
	if err != nil {
		return nil, fmt.Errorf("decode legacy fx mappings cache %q: %w", mappingPath, err)
	}
	if err := validateBundle(services, nil, mapping); err != nil {
		return nil, fmt.Errorf("validate legacy caches: %w", err)
	}
	// Use the newest legacy file mtime as the generation timestamp so the usual
	// max-age staleness policy keeps applying after adoption.
	createdAt := latestModTime(servicesPath, mappingPath)
	manifest := generationManifest{
		ID:        "legacy-" + sha256Hex(append(append([]byte{}, servicesBytes...), mappingBytes...))[:12],
		CreatedAt: createdAt,
		Components: map[string]componentManifest{
			ComponentServices:      {File: legacyServicesFileName, Present: true, Size: int64(len(servicesBytes)), SHA256: sha256Hex(servicesBytes), Source: componentSourceLegacy},
			ComponentNetworkGroups: {File: networkGroupsFileName, Present: false, Source: componentSourceLegacy},
			ComponentFxMappings:    {File: legacyFxMappingsFileName, Present: true, Size: int64(len(mappingBytes)), SHA256: sha256Hex(mappingBytes), Source: componentSourceLegacy},
		},
	}
	return &loadedGeneration{
		id:       manifest.ID,
		dir:      s.root,
		manifest: manifest,
		bundle: registryBundle{
			Services:      services,
			Mapping:       mapping,
			NetworkGroups: nil,
		},
	}, nil
}

// loadLegacyGeneration reads the legacy caches without modifying disk (used by
// the read path as a last-resort fallback).
func (s *generationStore) loadLegacyGeneration() (*loadedGeneration, error) {
	return s.readLegacyFiles()
}

// importLegacyGeneration stages and publishes the legacy caches as a proper
// generation (called while holding the refresh lock, only when the store is
// empty). The legacy files themselves are never modified or deleted.
func (s *generationStore) importLegacyGeneration() (*loadedGeneration, error) {
	legacy, err := s.readLegacyFiles()
	if err != nil {
		return nil, err
	}
	bundle := registryBundle{
		Services:      legacy.bundle.Services,
		NetworkGroups: nil,
		Mapping:       legacy.bundle.Mapping,
		CreatedAt:     legacy.manifest.CreatedAt,
		Sources: map[string]string{
			ComponentServices:      componentSourceLegacy,
			ComponentNetworkGroups: componentSourceLegacy,
			ComponentFxMappings:    componentSourceLegacy,
		},
	}
	return s.stageAndPublish(bundle)
}

// toRegistry converts a verified generation into the ServiceRegistry returned
// to the caller, with full provenance metadata.
func (g *loadedGeneration) toRegistry(fellBack bool, fallbackReason string) ServiceRegistry {
	components := make(map[string]ComponentProvenance, 3)
	for _, component := range []string{ComponentServices, ComponentNetworkGroups, ComponentFxMappings} {
		cm := g.manifest.Components[component]
		provenance := ComponentProvenance{Source: sourceUnavailable}
		if cm.Present {
			provenance.Source = sourceGeneration
			if cm.Source == componentSourceLegacy {
				provenance.Source = sourceLegacy
			}
			provenance.Path = filepath.Join(g.dir, cm.File)
			provenance.SHA256 = cm.SHA256
		}
		// Absent components (legacy layout never cached network groups) stay
		// "unavailable".
		components[component] = provenance
	}
	return ServiceRegistry{
		Services:          g.bundle.Services,
		NetworkGroups:     g.bundle.NetworkGroups,
		ClientTypeMapping: g.bundle.Mapping,
		Meta: RegistryMeta{
			Generation:     g.id,
			CreatedAt:      g.manifest.CreatedAt,
			Components:     components,
			FellBack:       fellBack,
			FallbackReason: fallbackReason,
		},
	}
}

// --- encoding helpers ---

func decodeServices(b []byte) ([]grail.Service, error) {
	var services []grail.Service
	if err := json.Unmarshal(b, &services); err != nil {
		return nil, fmt.Errorf("decode services: %w", err)
	}
	return services, nil
}

func decodeNetworkGroups(b []byte) ([]grail.NetworkGroup, error) {
	var groups []grail.NetworkGroup
	if err := json.Unmarshal(b, &groups); err != nil {
		return nil, fmt.Errorf("decode network groups: %w", err)
	}
	return groups, nil
}

func decodeMapping(b []byte) (servicemapping.ServiceMapping, error) {
	mapping := make(servicemapping.ServiceMapping)
	if err := json.Unmarshal(b, &mapping); err != nil {
		return nil, fmt.Errorf("decode fx mappings: %w", err)
	}
	return mapping, nil
}

// --- low-level filesystem helpers ---

// writeFileAtomic writes data to path via a same-directory temp file, fsyncs it,
// and renames it into place, then fsyncs the directory. The rename is atomic on
// POSIX filesystems, which is what makes the active-pointer flip safe for
// concurrent readers and writers.
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file into place: %w", err)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	cleanup = false
	return nil
}

func syncDir(dir string) (err error) {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}

func latestModTime(paths ...string) time.Time {
	var latest time.Time
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	if latest.IsZero() {
		latest = time.Now().UTC()
	}
	return latest.UTC()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func newGenerationID(createdAt time.Time) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "gen-" + createdAt.UTC().Format("20060102T150405.000000000") + "-" + hex.EncodeToString(b[:]), nil
}
