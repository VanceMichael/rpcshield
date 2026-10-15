package cacheanalyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber-research/rpcshield/go/constant"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
)

// CacheAnalyzer is an interface for analyzing and caching the failness of a function at an instruction.
type CacheAnalyzer interface {
	// AnalyzeCall returns the failness of a function at an instruction if it is found in the cache.
	// The returned HitKind classifies where the verdict was served from (in-process memory or the
	// persistent store), or HitMiss when neither layer has a reusable conclusion. A miss is not
	// an analysis error: callers must fall through to the normal analyzer chain.
	AnalyzeCall(f *ssa.Function, call ssa.Instruction) (OriginalFailnessInfo, HitKind, error)
	// InsertToCache inserts the failness of a function at an instruction into the cache.
	// Conclusions that are not reusable across runs (errors, panics, FailUnknownWithError, missing
	// analyzer source, incomplete outputs) are rejected and counted as invalid records rather than
	// stored -- see IsReusable.
	InsertToCache(f *ssa.Function, call ssa.Instruction, failness constant.Failness, source constant.AnalyzerKind)
	// Metrics returns a point-in-time snapshot of cache counters (hits by layer, misses,
	// invalidated/rejected records, write degradations, and persistent-store load failures).
	Metrics() Metrics
	// Flush persists pending records to the configured persistent cache file. It is a no-op for
	// an in-process-only analyzer. Flush never affects a verdict: all failures are reported via
	// Metrics/Logs and as a non-fatal error return.
	Flush() error
	// One day when we need to delete from the cache, we can add this method.
}

// OriginalFailnessInfo is a struct that contains the failness statuc and the source of the failness.
type OriginalFailnessInfo struct {
	Status constant.Failness
	Source constant.AnalyzerKind
}

// HitKind classifies the result of an AnalyzeCall lookup.
type HitKind string

const (
	// HitMiss means neither the in-process cache nor the persistent cache had a reusable verdict.
	HitMiss HitKind = "miss"
	// HitInProcess means the verdict was served from this process's in-memory cache.
	HitInProcess HitKind = "in_process"
	// HitPersistent means the verdict was loaded from the on-disk persistent cache.
	HitPersistent HitKind = "persistent"
)

// Metrics holds observable counters for cache analyzer behavior.
type Metrics struct {
	// InProcessHits counts verdicts served from the in-memory cache.
	InProcessHits int64
	// PersistentHits counts verdicts served from the persistent on-disk cache.
	PersistentHits int64
	// Misses counts lookups served by neither cache layer.
	Misses int64
	// InvalidRecords counts records rejected as non-reusable: invalid/expired entries observed
	// while loading or merging the persistent cache file, lookups that found a structurally bad
	// record, and attempts to insert a conclusion that IsReusable rejects.
	InvalidRecords int64
	// WriteDegraded counts records that could not be persisted because the cache is write-degraded
	// (unwritable directory, lock timeout on the final flush, unsupported newer schema, ...).
	WriteDegraded int64
	// LoadFailures counts persistent-cache (re)initialization failures (unreadable/corrupt file,
	// unsupported schema/tool version). Each failing process initialization contributes at most a
	// handful, so this is an integer counter rather than a boolean.
	LoadFailures int64
}

// counters holds the live atomic counters shared by the in-memory and persistent layers.
type counters struct {
	inProcessHits  atomic.Int64
	persistentHits atomic.Int64
	misses         atomic.Int64
	invalidRecords atomic.Int64
	writeDegraded  atomic.Int64
	loadFailures   atomic.Int64
}

func (c *counters) snapshot() Metrics {
	return Metrics{
		InProcessHits:  c.inProcessHits.Load(),
		PersistentHits: c.persistentHits.Load(),
		Misses:         c.misses.Load(),
		InvalidRecords: c.invalidRecords.Load(),
		WriteDegraded:  c.writeDegraded.Load(),
		LoadFailures:   c.loadFailures.Load(),
	}
}

// IsReusable reports whether a verdict may be cached and served back in a later analysis run.
//
// A conclusion is reusable only when:
//   - its failness is one of the known statuses,
//   - it is not FailUnknownWithError (a chain error that must be retried, not a verdict),
//   - its OriginalSource identifies a real analyzer that completed (Predefined, SCCP, SSA, AST,
//     LLM) -- neither Unknown nor the Cache facade itself.
//
// Plain FailUnknown IS reusable when a real analyzer produced it (it is a deliberate
// "undetermined error value" verdict), but an unknown status with no analyzer source (an
// unfinished/empty chain output) is not.
func IsReusable(status constant.Failness, source constant.AnalyzerKind) bool {
	if err := constant.ValidateFailness(status); err != nil {
		return false
	}
	if status == constant.FailUnknownWithError {
		return false
	}
	switch source {
	case constant.Predefined, constant.SCCP, constant.SSA, constant.AST, constant.LLM:
		return true
	default:
		return false
	}
}

// VerdictConfig captures the analyzer-chain inputs that can change an edge verdict and therefore
// must be part of the persistent cache key. Anything that only affects presentation (e.g. short
// caller names, report formatting) or that happens outside runAnalyzers (e.g. annotations) must
// NOT be added here.
type VerdictConfig struct {
	// EnabledAnalyzers is the ordered list of verdict-producing analyzers run by the chain
	// (Predefined, SCCP, SSA, AST, LLM), in chain order. Enabling/disabling an analyzer or
	// changing its position in the chain changes the fingerprint even if the set is identical.
	EnabledAnalyzers []constant.AnalyzerKind
	// NilOtherSourcesAfterSrc mirrors DriverOptions.NilOtherSourcesAfterSrc, an SCCP input.
	NilOtherSourcesAfterSrc bool
	// KnownErrorConstructorFuncs mirrors config.Config.KnownErrorConstructorFuncs, an SCCP input.
	KnownErrorConstructorFuncs []string
}

// fingerprint returns a stable hex hash of the verdict-affecting configuration.
func (c VerdictConfig) fingerprint() string {
	funcs := make([]string, len(c.KnownErrorConstructorFuncs))
	copy(funcs, c.KnownErrorConstructorFuncs)
	sort.Strings(funcs)
	enabled := make([]string, len(c.EnabledAnalyzers))
	for i, a := range c.EnabledAnalyzers {
		enabled[i] = string(a)
	}
	wire := struct {
		EnabledAnalyzers           []string `json:"enabled_analyzers"`
		NilOtherSourcesAfterSrc    bool     `json:"nil_other_sources_after_src"`
		KnownErrorConstructorFuncs []string `json:"known_error_constructor_funcs"`
	}{
		EnabledAnalyzers:           enabled,
		NilOtherSourcesAfterSrc:    c.NilOtherSourcesAfterSrc,
		KnownErrorConstructorFuncs: funcs,
	}
	b, err := json.Marshal(wire)
	if err != nil {
		// All fields are trivially marshalable; a failure here would be a programming error.
		panic(fmt.Sprintf("cacheanalyzer: marshal verdict config: %v", err))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type funcInsPair struct {
	f *ssa.Function
	c ssa.Instruction
}

type cacheAnalyzer struct {
	memMu sync.RWMutex
	cache map[funcInsPair]OriginalFailnessInfo

	ctr    *counters
	logger *zap.SugaredLogger

	persistent *persistentStore

	// codeVersion is the analyzed-code version (e.g. Git SHA), folded into every persistent key.
	codeVersion string
	// configFingerprint is precomputed from VerdictConfig.
	configFingerprint string

	// srcCache caches file contents read for stable identities, by absolute filename.
	srcMu     sync.Mutex
	srcCache  map[string][]byte
	// callEndCache maps a callsite's start position to its end position, lazily per SSA function,
	// so the stable call text can be extracted without re-walking the AST on every edge.
	endMu        sync.Mutex
	callEndCache map[*ssa.Function]map[token.Pos]token.Pos
}

// Option configures a CacheAnalyzer.
type Option func(*cacheAnalyzer)

// WithLogger sets the logger used for persistent-cache diagnostics. Defaults to a no-op logger.
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(c *cacheAnalyzer) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithPersistentCache enables the on-disk persistent cache at path. The file and its lock sibling
// are created lazily; a missing file starts an empty cache, while an unreadable/corrupt file or an
// unwritable directory only degrade this process to the normal in-memory behavior (see Metrics).
func WithPersistentCache(path string) Option {
	return func(c *cacheAnalyzer) {
		if path == "" {
			return
		}
		c.persistent = newPersistentStore(path, c.logger, c.ctr, func() time.Time { return time.Now().UTC() }, defaultFlushThreshold)
	}
}

// WithCodeVersion sets the analyzed-code version (typically the Git SHA) folded into every
// persistent key. Analyzing a different version yields cache misses even for identical sources.
func WithCodeVersion(version string) Option {
	return func(c *cacheAnalyzer) { c.codeVersion = version }
}

// WithVerdictConfig sets the verdict-affecting analyzer configuration fingerprint.
func WithVerdictConfig(cfg VerdictConfig) Option {
	return func(c *cacheAnalyzer) { c.configFingerprint = cfg.fingerprint() }
}

// withFlushThreshold overrides the number of pending records that triggers an intermediate flush.
// Intended for tests.
func withFlushThreshold(n int) Option {
	return func(c *cacheAnalyzer) {
		if c.persistent != nil && n > 0 {
			c.persistent.flushThreshold = n
		}
	}
}

// NewCacheAnalyzer creates a new CacheAnalyzer. Without WithPersistentCache it keeps the existing
// purely in-process behavior (no filesystem access whatsoever).
func NewCacheAnalyzer(opts ...Option) CacheAnalyzer {
	c := &cacheAnalyzer{
		cache:        make(map[funcInsPair]OriginalFailnessInfo),
		ctr:          &counters{},
		logger:       zap.NewNop().Sugar(),
		srcCache:     make(map[string][]byte),
		callEndCache: make(map[*ssa.Function]map[token.Pos]token.Pos),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// AnalyzeCall returns the failness of a function at an instruction if it is found in the cache.
func (c *cacheAnalyzer) AnalyzeCall(f *ssa.Function, call ssa.Instruction) (OriginalFailnessInfo, HitKind, error) {
	key := funcInsPair{f: f, c: call}
	c.memMu.RLock()
	info, ok := c.cache[key]
	c.memMu.RUnlock()
	if ok {
		c.ctr.inProcessHits.Add(1)
		return info, HitInProcess, nil
	}

	if c.persistent != nil {
		rec, found, idErr := c.lookupPersistent(f, call)
		switch {
		case idErr != nil:
			// A stable identity cannot be built for this edge (external function, no source,
			// unreadable file). It can never be a persistent hit; degrade to miss but keep the
			// in-memory layer usable for repeat occurrences within this process.
			c.logger.Debugw("persistent cache lookup skipped: stable identity unavailable", "caller", safeFuncString(f), "error", idErr)
		case found:
			info := OriginalFailnessInfo{Status: constant.Failness(rec.Status), Source: constant.AnalyzerKind(rec.OriginalSource)}
			// Re-validate defensively; a bad record must never become a verdict.
			if IsReusable(info.Status, info.Source) {
				c.memMu.Lock()
				c.cache[key] = info
				c.memMu.Unlock()
				c.ctr.persistentHits.Add(1)
				return info, HitPersistent, nil
			}
			c.logger.Warnw("persistent cache record failed validation on lookup; rejecting", "key", rec.Key, "status", rec.Status, "original_source", rec.OriginalSource)
			c.ctr.invalidRecords.Add(1)
		}
	}

	c.ctr.misses.Add(1)
	return OriginalFailnessInfo{Status: constant.FailUnknown, Source: constant.Unknown}, HitMiss, fmt.Errorf("failness not found in cache")
}

// InsertToCache inserts the failness of a function at an instruction into the cache.
func (c *cacheAnalyzer) InsertToCache(f *ssa.Function, call ssa.Instruction, failness constant.Failness, source constant.AnalyzerKind) {
	if !IsReusable(failness, source) {
		c.ctr.invalidRecords.Add(1)
		c.logger.Debugw("rejecting non-reusable analyzer conclusion", "caller", safeFuncString(f), "status", failness, "original_source", source)
		return
	}

	key := funcInsPair{f: f, c: call}
	info := OriginalFailnessInfo{Status: failness, Source: source}
	c.memMu.Lock()
	c.cache[key] = info
	c.memMu.Unlock()

	if c.persistent == nil {
		return
	}
	rec, err := c.buildRecord(f, call, failness, source)
	if err != nil {
		c.ctr.invalidRecords.Add(1)
		c.logger.Debugw("persistent cache record not built; verdict kept in process memory only", "caller", safeFuncString(f), "error", err)
		return
	}
	c.persistent.add(rec)
}

// Metrics returns a snapshot of the cache counters.
func (c *cacheAnalyzer) Metrics() Metrics { return c.ctr.snapshot() }

// Flush flushes pending persistent records.
func (c *cacheAnalyzer) Flush() error {
	if c.persistent == nil {
		return nil
	}
	if err := c.persistent.flush(); err != nil {
		c.logger.Warnw("persistent verdict cache flush failed; analysis verdict unaffected", "path", c.persistent.path, "error", err)
		return err
	}
	return nil
}

// astCallEnds returns a map from ast.CallExpr start positions to end positions within f's syntax.
// It is best-effort: an empty map simply means call text is omitted from the identity.
func (c *cacheAnalyzer) astCallEnds(f *ssa.Function) map[token.Pos]token.Pos {
	c.endMu.Lock()
	defer c.endMu.Unlock()
	if m, ok := c.callEndCache[f]; ok {
		return m
	}
	m := make(map[token.Pos]token.Pos)
	if node := f.Syntax(); node != nil {
		ast.Inspect(node, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				m[call.Pos()] = call.End()
			}
			return true
		})
	}
	c.callEndCache[f] = m
	return m
}

func safeFuncString(f *ssa.Function) string {
	if f == nil {
		return "<nil>"
	}
	return f.String()
}
