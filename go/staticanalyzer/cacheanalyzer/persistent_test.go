package cacheanalyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/loader"
	"github.com/uber-research/rpcshield/go/rta/rtatest"
	"go.uber.org/zap"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// TestMain arranges for a real Go SDK under bazel test (no-op under plain `go test`).
func TestMain(m *testing.M) {
	rtatest.RunWithGoSDK(m)
}

const _fixturePkg = "fixturepkg"

// fixtureSrc variants: the function body is kept minimal so positions are easy to reason about.
const fixtureSrcV1 = `package fixturepkg

func anRPCCall() error { return nil }

func analyzeCaller() error {
	if err := anRPCCall(); err != nil {
		return err
	}
	return nil
}
`

// Same body plus a no-op line before the call: the call site moves down a line, identity changes.
const fixtureSrcMovedSite = `package fixturepkg

func anRPCCall() error { return nil }

func analyzeCaller() error {
	_ = 0
	if err := anRPCCall(); err != nil {
		return err
	}
	return nil
}
`

// Changed function body while keeping the call at the exact same line/column: only statements
// after the call are added, so StableID is identical but the caller-source hash changes.
const fixtureSrcChangedBody = `package fixturepkg

func anRPCCall() error { return nil }

func analyzeCaller() error {
	if err := anRPCCall(); err != nil {
		return err
	}
	x := 0
	_ = x
	return nil
}
`

// ssaPair is a loaded function/call-instruction pair.
type ssaPair struct {
	prog *ssa.Program
	fn   *ssa.Function
	call ssa.CallInstruction
}

// writeFixture lays out a GOPATH-shaped tree with the given source and loads SSA from it.
func writeFixture(t *testing.T, src string) string {
	t.Helper()
	gopath := t.TempDir()
	pkgDir := filepath.Join(gopath, "src", _fixturePkg)
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "a.go"), []byte(src), 0o644))
	return gopath
}

func loadSSAPair(t *testing.T, gopath string) ssaPair {
	t.Helper()
	pkgs, err := loader.LoadAll(gopath, nil)
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)
	for _, p := range pkgs {
		require.Emptyf(t, p.Errors, "package %s errors: %v", p.PkgPath, p.Errors)
	}
	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics|ssa.GlobalDebug)
	prog.Build()

	var found ssaPair
	for _, sp := range ssaPkgs {
		if sp == nil || sp.Pkg == nil || sp.Pkg.Path() != _fixturePkg {
			continue
		}
		fn := sp.Func("analyzeCaller")
		require.NotNil(t, fn, "analyzeCaller not found")
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				if call, ok := instr.(*ssa.Call); ok && strings.Contains(call.Common().Value.String(), "anRPCCall") {
					found = ssaPair{prog: prog, fn: fn, call: call}
					return found
				}
			}
		}
	}
	t.Fatal("anRPCCall call instruction not found")
	return found
}

func testVerdictConfig() VerdictConfig {
	return VerdictConfig{
		EnabledAnalyzers: []constant.AnalyzerKind{constant.Predefined, constant.SCCP, constant.SSA, constant.AST},
	}
}

func newPersistentTestAnalyzer(t *testing.T, cachePath, codeVersion string, cfg VerdictConfig) CacheAnalyzer {
	if t != nil {
		t.Helper()
	}
	return NewCacheAnalyzer(
		WithLogger(zap.NewNop().Sugar()),
		WithPersistentCache(cachePath),
		WithCodeVersion(codeVersion),
		WithVerdictConfig(cfg),
	)
}

// --- In-process behavior unchanged when no persistent path is configured ---------------------

func TestNewCacheAnalyzer_NoPersistenceTouchesNoFiles(t *testing.T) {
	t.Parallel()
	ca := NewCacheAnalyzer()
	p := loadSSAPair(t, writeFixture(t, fixtureSrcV1))
	_, hit, err := ca.AnalyzeCall(p.fn, p.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit)
	m := ca.Metrics()
	require.Equal(t, int64(1), m.Misses)
	require.Zero(t, m.PersistentHits)
	require.NoError(t, ca.Flush())
}

// --- Cross-process reuse ---------------------------------------------------------------------

func TestPersistentCache_CrossInstanceReuse(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	cfg := testVerdictConfig()

	gopath1 := writeFixture(t, fixtureSrcV1)
	p1 := loadSSAPair(t, gopath1)
	ca1 := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfg)

	info, hit, err := ca1.AnalyzeCall(p1.fn, p1.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit)
	require.Equal(t, constant.FailUnknown, info.Status)

	ca1.InsertToCache(p1.fn, p1.call, constant.FailClose, constant.SCCP)
	// The in-memory layer serves the repeat immediately.
	info, hit, err = ca1.AnalyzeCall(p1.fn, p1.call)
	require.NoError(t, err)
	require.Equal(t, HitInProcess, hit)
	require.Equal(t, constant.FailClose, info.Status)
	require.Equal(t, constant.SCCP, info.Source)
	require.NoError(t, ca1.Flush())
	require.FileExists(t, cachePath)

	// A second "process": fresh SSA rebuild (new pointers), fresh analyzer, same cache file,
	// same code version and analyzer configuration.
	p2 := loadSSAPair(t, writeFixture(t, fixtureSrcV1))
	ca2 := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfg)
	info, hit, err = ca2.AnalyzeCall(p2.fn, p2.call)
	require.NoError(t, err)
	require.Equal(t, HitPersistent, hit)
	require.Equal(t, constant.FailClose, info.Status)
	require.Equal(t, constant.SCCP, info.Source)
	m := ca2.Metrics()
	require.Equal(t, int64(1), m.PersistentHits)
	require.Zero(t, m.LoadFailures)

	// After a persistent hit the in-memory layer takes over for repeats in this process.
	_, hit, _ = ca2.AnalyzeCall(p2.fn, p2.call)
	require.Equal(t, HitInProcess, hit)
}

// Real separate OS processes exercising the same cache file.
func TestPersistentCache_SubprocessReuse(t *testing.T) {
	if mode := os.Getenv("RPCSHIELD_CACHE_CHILD_MODE"); mode != "" {
		runCacheChild(t, mode)
		return
	}
	t.Parallel()

	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	gopath := writeFixture(t, fixtureSrcV1)

	writer := exec.Command(os.Args[0], "-test.run=^TestPersistentCache_SubprocessReuse$", "-test.v")
	writer.Env = append(os.Environ(),
		"RPCSHIELD_CACHE_CHILD_MODE=write",
		"RPCSHIELD_CACHE_PATH="+cachePath,
		"RPCSHIELD_CACHE_GOPATH="+gopath,
	)
	writeOut, err := writer.CombinedOutput()
	require.NoError(t, err, "writer child failed: %s", writeOut)
	require.Contains(t, string(writeOut), "CACHE_CHILD_WRITE_DONE")
	require.FileExists(t, cachePath)

	reader := exec.Command(os.Args[0], "-test.run=^TestPersistentCache_SubprocessReuse$", "-test.v")
	reader.Env = append(os.Environ(),
		"RPCSHIELD_CACHE_CHILD_MODE=read",
		"RPCSHIELD_CACHE_PATH="+cachePath,
		"RPCSHIELD_CACHE_GOPATH="+gopath,
	)
	readOut, err := reader.CombinedOutput()
	require.NoError(t, err, "reader child failed: %s", readOut)
	require.Contains(t, string(readOut), "CACHE_CHILD_HIT FailClose SCCP", "reader output: %s", readOut)
}

func runCacheChild(t *testing.T, mode string) {
	cachePath := os.Getenv("RPCSHIELD_CACHE_PATH")
	gopath := os.Getenv("RPCSHIELD_CACHE_GOPATH")
	p := loadSSAPair(t, gopath)
	ca := newPersistentTestAnalyzer(nil, cachePath, "sha-child", testVerdictConfig())
	switch mode {
	case "write":
		if _, _, err := ca.AnalyzeCall(p.fn, p.call); err == nil {
			fmt.Println("CACHE_CHILD_UNEXPECTED_HIT")
			os.Exit(1)
		}
		ca.InsertToCache(p.fn, p.call, constant.FailClose, constant.SCCP)
		if err := ca.Flush(); err != nil {
			fmt.Println("CACHE_CHILD_FLUSH_ERROR", err)
			os.Exit(1)
		}
		fmt.Println("CACHE_CHILD_WRITE_DONE")
	case "read":
		info, hit, err := ca.AnalyzeCall(p.fn, p.call)
		if err != nil || hit != HitPersistent {
			fmt.Printf("CACHE_CHILD_MISS hit=%s err=%v\n", hit, err)
			os.Exit(1)
		}
		fmt.Printf("CACHE_CHILD_HIT %s %s\n", info.Status, info.Source)
	default:
		os.Exit(2)
	}
}

// --- Invalidation: source / call location / config / code version ----------------------------

func TestPersistentCache_SourceAndLocationInvalidation(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	cfg := testVerdictConfig()

	p1 := loadSSAPair(t, writeFixture(t, fixtureSrcV1))
	ca1 := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfg)
	ca1.InsertToCache(p1.fn, p1.call, constant.FailClose, constant.SCCP)
	require.NoError(t, ca1.Flush())
	require.Len(t, readCacheDoc(t, cachePath).Records, 1)

	// Moved call site (new line) -> miss; it must not reuse the old line's verdict.
	pMoved := loadSSAPair(t, writeFixture(t, fixtureSrcMovedSite))
	caMoved := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfg)
	_, hit, err := caMoved.AnalyzeCall(pMoved.fn, pMoved.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit, "moving the call site must invalidate the cache")

	// Changed caller function body (call on the same line/column) -> miss via caller-source hash.
	// Committing the new-body verdict supersedes the old one for the same stable function+site,
	// so exactly one record survives instead of accumulating stale versions.
	p2 := loadSSAPair(t, writeFixture(t, fixtureSrcChangedBody))
	ca2 := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfg)
	_, hit, err = ca2.AnalyzeCall(p2.fn, p2.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit, "changing the caller source must invalidate the cache")
	ca2.InsertToCache(p2.fn, p2.call, constant.FailOpen, constant.SSA)
	require.NoError(t, ca2.Flush())

	doc := readCacheDoc(t, cachePath)
	require.Len(t, doc.Records, 1, "older code fingerprint of the identical function+site is superseded")
	var rec storedRecord
	for _, r := range doc.Records {
		rec = r
	}
	require.Equal(t, string(constant.FailOpen), rec.Status)
	require.Equal(t, string(constant.SSA), rec.OriginalSource)

	// The new-body verdict is then reusable by the next process.
	p3 := loadSSAPair(t, writeFixture(t, fixtureSrcChangedBody))
	ca3 := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfg)
	info, hit, err := ca3.AnalyzeCall(p3.fn, p3.call)
	require.NoError(t, err)
	require.Equal(t, HitPersistent, hit)
	require.Equal(t, constant.FailOpen, info.Status)
}

func TestPersistentCache_ConfigAndCodeVersionInvalidation(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	baseCfg := testVerdictConfig()
	p := loadSSAPair(t, writeFixture(t, fixtureSrcV1))

	ca1 := newPersistentTestAnalyzer(t, cachePath, "sha-a", baseCfg)
	ca1.InsertToCache(p.fn, p.call, constant.FailClose, constant.SCCP)
	require.NoError(t, ca1.Flush())

	// Identical config/code on a fresh analyzer -> persistent hit.
	caSame := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	_, hit, err := caSame.AnalyzeCall(p.fn, p.call)
	require.NoError(t, err)
	require.Equal(t, HitPersistent, hit)

	// Different code version (Git SHA) -> miss.
	caNewSHA := newPersistentTestAnalyzer(t, cachePath, "sha-b", baseCfg)
	_, hit, err = caNewSHA.AnalyzeCall(p.fn, p.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit, "code version change must invalidate")

	// Verdict-affecting option toggled -> miss.
	cfgToggled := testVerdictConfig()
	cfgToggled.NilOtherSourcesAfterSrc = true
	caOpt := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfgToggled)
	_, hit, err = caOpt.AnalyzeCall(p.fn, p.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit, "analyzer option change must invalidate")

	// Analyzer set changed (AST disabled) -> miss.
	cfgShorter := VerdictConfig{EnabledAnalyzers: []constant.AnalyzerKind{constant.Predefined, constant.SCCP, constant.SSA}}
	caShort := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfgShorter)
	_, hit, err = caShort.AnalyzeCall(p.fn, p.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit, "enabled analyzer set change must invalidate")

	// Analyzer order changed -> miss.
	cfgReordered := VerdictConfig{EnabledAnalyzers: []constant.AnalyzerKind{constant.AST, constant.Predefined, constant.SCCP, constant.SSA}}
	caReorder := newPersistentTestAnalyzer(t, cachePath, "sha-a", cfgReordered)
	_, hit, err = caReorder.AnalyzeCall(p.fn, p.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit, "analyzer order change must invalidate")
}

func TestVerdictConfig_Fingerprint(t *testing.T) {
	t.Parallel()
	a := VerdictConfig{
		EnabledAnalyzers:           []constant.AnalyzerKind{constant.SSA, constant.AST},
		KnownErrorConstructorFuncs: []string{"errors.New", "fmt.Errorf"},
	}
	b := VerdictConfig{
		EnabledAnalyzers:           []constant.AnalyzerKind{constant.SSA, constant.AST},
		KnownErrorConstructorFuncs: []string{"fmt.Errorf", "errors.New"}, // different slice order
	}
	require.Equal(t, a.fingerprint(), b.fingerprint(), "constructor funcs are order-independent")

	c := a
	c.EnabledAnalyzers = []constant.AnalyzerKind{constant.AST, constant.SSA}
	require.NotEqual(t, a.fingerprint(), c.fingerprint(), "analyzer order participates in the fingerprint")

	d := a
	d.NilOtherSourcesAfterSrc = true
	require.NotEqual(t, a.fingerprint(), d.fingerprint())
}

// --- Only complete, reusable conclusions are committed --------------------------------------

func TestPersistentCache_RejectsNonReusableConclusions(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	p := loadSSAPair(t, writeFixture(t, fixtureSrcV1))
	ca := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())

	rejected := []struct {
		status constant.Failness
		source constant.AnalyzerKind
	}{
		{constant.FailUnknownWithError, constant.SSA}, // chain error
		{constant.FailOpen, constant.Unknown},        // missing source
		{constant.FailClose, constant.CacheAnalyzerKind}, // cache facade is not a real producer
		{constant.Failness("Bogus"), constant.SSA},   // invalid status
	}
	for _, r := range rejected {
		ca.InsertToCache(p.fn, p.call, r.status, r.source)
		_, _, err := ca.AnalyzeCall(p.fn, p.call)
		require.Error(t, err, "rejected conclusion must not be served (status=%s source=%s)", r.status, r.source)
	}
	require.GreaterOrEqual(t, ca.Metrics().InvalidRecords, int64(len(rejected)))
	require.NoError(t, ca.Flush())

	// With nothing reusable committed, the flush creates no file (or an empty document).
	if _, statErr := os.Stat(cachePath); statErr == nil {
		doc := readCacheDoc(t, cachePath)
		require.Empty(t, doc.Records, "no rejected conclusion may be persisted")
	}

	// A valid conclusion is still served and persisted.
	ca.InsertToCache(p.fn, p.call, constant.FailUnknown, constant.SCCP) // plain Unknown from a real analyzer is reusable
	info, hit, err := ca.AnalyzeCall(p.fn, p.call)
	require.NoError(t, err)
	require.Equal(t, HitInProcess, hit)
	require.Equal(t, constant.FailUnknown, info.Status)
	require.NoError(t, ca.Flush())
	doc := readCacheDoc(t, cachePath)
	require.Len(t, doc.Records, 1)
}

// --- Corruption / unsupported schema ----------------------------------------------------------

func TestPersistentCache_CorruptFileSalvaged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "verdicts.json")
	require.NoError(t, os.WriteFile(cachePath, []byte("{not json"), 0o644))

	ca := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	m := ca.Metrics()
	require.Equal(t, int64(1), m.LoadFailures)

	p := loadSSAPair(t, writeFixture(t, fixtureSrcV1))
	_, hit, err := ca.AnalyzeCall(p.fn, p.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit, "corrupt file must not yield hits, analysis falls through to the chain")

	ca.InsertToCache(p.fn, p.call, constant.FailClose, constant.SCCP)
	require.NoError(t, ca.Flush())

	// Cache re-established from the same run; old bytes preserved as a backup.
	ca2 := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	_, hit, err = ca2.AnalyzeCall(p.fn, p.call)
	require.NoError(t, err)
	require.Equal(t, HitPersistent, hit)
	backups, err := filepath.Glob(filepath.Join(dir, "verdicts.json.corrupt-*"))
	require.NoError(t, err)
	require.NotEmpty(t, backups, "corrupt file must be preserved for inspection")
}

func TestPersistentCache_PartiallyInvalidRecords(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	p := loadSSAPair(t, writeFixture(t, fixtureSrcV1))

	// Seed a valid record through a real analyzer, then tamper one extra record into the file.
	ca := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	ca.InsertToCache(p.fn, p.call, constant.FailClose, constant.SCCP)
	require.NoError(t, ca.Flush())
	doc := readCacheDoc(t, cachePath)
	doc.Records["deadbeef"] = storedRecord{
		Key: "deadbeef", SchemaVersion: currentSchemaVersion, ToolVersion: currentToolVersion,
		StableID: "x", Status: string(constant.FailUnknownWithError), OriginalSource: string(constant.SSA),
		CodeFingerprint: "c", ConfigFingerprint: "f",
	}
	writeCacheDoc(t, cachePath, doc)

	ca2 := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	require.Equal(t, int64(1), ca2.Metrics().InvalidRecords)
	info, hit, err := ca2.AnalyzeCall(p.fn, p.call)
	require.NoError(t, err)
	require.Equal(t, HitPersistent, hit)
	require.Equal(t, constant.FailClose, info.Status)
}

func TestPersistentCache_UnsupportedSchemaPreserved(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	original, err := json.Marshal(cacheFile{SchemaVersion: "v999", ToolVersion: 99, Records: map[string]storedRecord{}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cachePath, original, 0o644))

	ca := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	require.Equal(t, int64(1), ca.Metrics().LoadFailures)

	p := loadSSAPair(t, writeFixture(t, fixtureSrcV1))
	ca.InsertToCache(p.fn, p.call, constant.FailClose, constant.SCCP)
	require.NoError(t, ca.Flush())
	require.Equal(t, int64(1), ca.Metrics().WriteDegraded, "older tool must not clobber a newer-schema cache")

	after, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	require.JSONEq(t, string(original), string(after), "newer-schema document must be left untouched")
}

func TestPersistentCache_NewerToolVersionPreserved(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	doc := cacheFile{SchemaVersion: currentSchemaVersion, ToolVersion: currentToolVersion + 1, Records: map[string]storedRecord{}}
	writeCacheDoc(t, cachePath, doc)

	ca := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	require.Equal(t, int64(1), ca.Metrics().LoadFailures)
	p := loadSSAPair(t, writeFixture(t, fixtureSrcV1))
	ca.InsertToCache(p.fn, p.call, constant.FailClose, constant.SCCP)
	require.NoError(t, ca.Flush())
	require.Equal(t, int64(1), ca.Metrics().WriteDegraded)
}

// --- Read-only / unwritable directory ---------------------------------------------------------

func TestPersistentCache_ReadOnlyDirDegradesAndRecovers(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission semantics do not apply when running as root")
	}
	if runtime.GOOS == "windows" {
		t.Skip("chmod permission model differs on windows")
	}
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	cachePath := filepath.Join(dir, "verdicts.json")

	ca := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	p := loadSSAPair(t, writeFixture(t, fixtureSrcV1))

	// Analysis path stays fully functional: lookups miss, no panic, no verdict impact.
	_, hit, err := ca.AnalyzeCall(p.fn, p.call)
	require.Error(t, err)
	require.Equal(t, HitMiss, hit)
	ca.InsertToCache(p.fn, p.call, constant.FailClose, constant.SCCP)
	flushErr := ca.Flush()
	require.Error(t, flushErr, "flush reports the degradation")
	require.GreaterOrEqual(t, ca.Metrics().WriteDegraded, int64(1))
	require.NoFileExists(t, cachePath)

	// Permissions recover -> later runs establish the cache as usual.
	require.NoError(t, os.Chmod(dir, 0o755))
	ca2 := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	ca2.InsertToCache(p.fn, p.call, constant.FailClose, constant.SCCP)
	require.NoError(t, ca2.Flush())
	require.FileExists(t, cachePath)
	ca3 := newPersistentTestAnalyzer(t, cachePath, "sha-a", testVerdictConfig())
	_, hit, err = ca3.AnalyzeCall(p.fn, p.call)
	require.NoError(t, err)
	require.Equal(t, HitPersistent, hit)
}

// --- Concurrent processes (simulated by independent stores) ----------------------------------

func TestPersistentCache_ConcurrentIndependentWriters(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")

	const writers = 8
	const perWriter = 25
	var wg sync.WaitGroup
	// A reader continuously re-opens the cache; every version it observes must parse (no torn
	// reads thanks to temp-file + rename).
	stop := make(chan struct{})
	readerErr := make(chan error, 1)
	go func() {
		for {
			select {
			case <-stop:
				readerErr <- nil
				return
			default:
				if data, err := os.ReadFile(cachePath); err == nil {
					var doc cacheFile
					if err := json.Unmarshal(data, &doc); err != nil {
						readerErr <- err
						return
					}
				}
			}
		}
	}()

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			ctr := &counters{}
			s := newPersistentStore(cachePath, zap.NewNop().Sugar(), ctr, func() time.Time { return time.Now().UTC() }, 1_000_000)
			for i := 0; i < perWriter; i++ {
				key := fmt.Sprintf("writer%d-record%d", w, i)
				s.add(syntheticRecord(key, fmt.Sprintf("site-%d-%d", w, i)))
			}
			if err := s.flush(); err != nil {
				t.Errorf("writer %d flush: %v", w, err)
			}
		}(w)
	}
	wg.Wait()
	close(stop)
	require.NoError(t, <-readerErr, "reader never observed a partially written document")

	// Union of all writers survives.
	final := newPersistentStore(cachePath, zap.NewNop().Sugar(), &counters{}, func() time.Time { return time.Now().UTC() }, 1_000_000)
	require.Equal(t, writers*perWriter, len(final.records))

	// Overlapping writes (same record from multiple writers) converge without corruption.
	var wg2 sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			ctr := &counters{}
			s := newPersistentStore(cachePath, zap.NewNop().Sugar(), ctr, func() time.Time { return time.Now().UTC() }, 1_000_000)
			s.add(syntheticRecord("shared-key", "shared-site"))
			require.NoError(t, s.flush())
		}()
	}
	wg2.Wait()
	final2 := newPersistentStore(cachePath, zap.NewNop().Sugar(), &counters{}, func() time.Time { return time.Now().UTC() }, 1_000_000)
	require.Len(t, final2.records, writers*perWriter+1)
}

func TestPersistentCache_IntermediateFlushThreshold(t *testing.T) {
	t.Parallel()
	cachePath := filepath.Join(t.TempDir(), "verdicts.json")
	ctr := &counters{}
	s := newPersistentStore(cachePath, zap.NewNop().Sugar(), ctr, func() time.Time { return time.Now().UTC() }, 2)
	s.add(syntheticRecord("k1", "site1"))
	s.add(syntheticRecord("k2", "site2")) // triggers threshold flush
	require.FileExists(t, cachePath)
	doc := readCacheDoc(t, cachePath)
	require.Len(t, doc.Records, 2)
	s.mu.Lock()
	require.Empty(t, s.pending)
	s.mu.Unlock()

	s.add(syntheticRecord("k3", "site3")) // below threshold, stays pending until explicit flush
	doc = readCacheDoc(t, cachePath)
	require.Len(t, doc.Records, 2)
	require.NoError(t, s.flush())
	doc = readCacheDoc(t, cachePath)
	require.Len(t, doc.Records, 3)
}

// --- Stable identity helpers ------------------------------------------------------------------

func TestNormalizeFile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		filename string
		pkgPath  string
		want     string
	}{
		{
			name:     "lifted gopath",
			filename: "/tmp/gopath-123/src/example.internal/svc/handler.go",
			pkgPath:  "example.internal/svc",
			want:     "example.internal/svc/handler.go",
		},
		{
			name:     "bazel sandbox",
			filename: "/private/var/tmp/_bazel/x/execroot/w/src/example.internal/svc/sub/h.go",
			pkgPath:  "example.internal/svc/sub",
			want:     "example.internal/svc/sub/h.go",
		},
		{
			name:     "module cache keeps version",
			filename: "/go/pkg/mod/example.com/foo@v1.2.3/x/a.go",
			pkgPath:  "example.com/foo@v1.2.3/x",
			want:     "example.com/foo@v1.2.3/x/a.go",
		},
		{
			name:     "pkg path not found falls back to base",
			filename: "/somewhere/else/a.go",
			pkgPath:  "example.internal/svc",
			want:     "a.go",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, normalizeFile(tc.filename, tc.pkgPath))
		})
	}
}

// --- helpers ----------------------------------------------------------------------------------

func syntheticRecord(key, stableID string) storedRecord {
	return storedRecord{
		Key:               key,
		SchemaVersion:     currentSchemaVersion,
		ToolVersion:       currentToolVersion,
		StableID:          stableID,
		Status:            string(constant.FailClose),
		OriginalSource:    string(constant.SCCP),
		CodeFingerprint:   "code-" + key,
		ConfigFingerprint: "cfg",
		CreatedAt:         time.Now().UTC().Format(timestampLayout),
	}
}

func readCacheDoc(t *testing.T, path string) cacheFile {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var doc cacheFile
	require.NoError(t, json.Unmarshal(data, &doc), "cache file must be valid JSON")
	return doc
}

func writeCacheDoc(t *testing.T, path string, doc cacheFile) {
	t.Helper()
	data, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
}
