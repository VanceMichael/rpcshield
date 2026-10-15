package cacheanalyzer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/uber-research/rpcshield/go/constant"
	"go.uber.org/zap"
)

const (
	// currentSchemaVersion is the on-disk record/file schema version. It must be bumped whenever
	// the cache file layout or record semantics change incompatibly.
	currentSchemaVersion = "v1"
	// currentToolVersion invalidates every persisted verdict when analyzer behavior changes in a
	// way the VerdictConfig fingerprint cannot capture (e.g. SCCP algorithm updates).
	currentToolVersion = 1
	// timestampLayout is the RFC3339 layout used in record timestamps.
	timestampLayout = time.RFC3339Nano
	// defaultFlushThreshold flushes pending records to disk once this many verdicts accumulated.
	defaultFlushThreshold = 512
	// lockTimeout bounds the time spent waiting on the cross-process cache lock.
	lockTimeout = 10 * time.Second
	// lockRetry is the interval between non-blocking lock attempts.
	lockRetry = 50 * time.Millisecond
)

// storedRecord is one versioned, reusable verdict for a (function, call site) pair.
type storedRecord struct {
	// Key is the sha256 hex of keyMaterial (identity + code/config/tool/schema versions).
	Key string `json:"key"`
	// SchemaVersion is the schema this record was written under.
	SchemaVersion string `json:"schema_version"`
	// ToolVersion is the analyzer-tool record version this record was written under.
	ToolVersion int `json:"tool_version"`
	// StableID is the human-stable function+site identity, used for diagnostics and for pruning
	// records superseded by a newer code/config version of the same function+site.
	StableID string `json:"stable_id"`
	// Status is the reusable failness verdict.
	Status string `json:"status"`
	// OriginalSource is the analyzer that actually produced the verdict (not the Cache facade).
	OriginalSource string `json:"original_source"`
	// CodeFingerprint hashes the analyzed-code version and the caller-function source bytes.
	CodeFingerprint string `json:"code_fingerprint"`
	// ConfigFingerprint hashes the verdict-affecting analyzer configuration.
	ConfigFingerprint string `json:"config_fingerprint"`
	// CreatedAt records when the verdict was first persisted (RFC3339Nano, UTC).
	CreatedAt string `json:"created_at"`
}

// cacheFile is the on-disk cache document. Writes always go through temp-file + fsync + rename so
// a reader never observes a partially written document.
type cacheFile struct {
	SchemaVersion string                  `json:"schema_version"`
	ToolVersion   int                     `json:"tool_version"`
	Records       map[string]storedRecord `json:"records"`
}

// persistentStore is the multi-process-safe on-disk verdict cache. Its zero-discipline rule:
// any storage failure (corrupt file, unsupported schema, unwritable directory, lock timeout)
// degrades this process to the normal analyzer chain with observable diagnostics; it can never
// change an analysis verdict.
type persistentStore struct {
	path     string
	dir      string
	lockPath string
	logger   *zap.SugaredLogger
	now      func() time.Time

	flushThreshold int

	ctr *counters

	mu             sync.RWMutex
	records        map[string]storedRecord
	pending        map[string]storedRecord
	writesDisabled bool
}

func newPersistentStore(path string, logger *zap.SugaredLogger, ctr *counters, now func() time.Time, flushThreshold int) *persistentStore {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	if flushThreshold <= 0 {
		flushThreshold = defaultFlushThreshold
	}
	s := &persistentStore{
		path:           path,
		dir:            filepath.Dir(path),
		lockPath:       path + ".lock",
		logger:         logger,
		now:            now,
		flushThreshold: flushThreshold,
		ctr:            ctr,
		records:        make(map[string]storedRecord),
		pending:        make(map[string]storedRecord),
	}
	s.load()
	return s
}

// load reads and validates the cache document. See the module-level design comments for the
// corruption/version policy.
func (s *persistentStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return // empty cache, normal first run
		}
		// Unreadable (permissions, I/O error): do not attempt to overwrite, just disable the
		// persistent layer for this run.
		s.disable(fmt.Sprintf("persistent verdict cache unreadable at %q", s.path), err)
		return
	}

	var cf cacheFile
	if jerr := json.Unmarshal(data, &cf); jerr != nil {
		s.salvageCorrupt(jerr)
		return
	}

	switch {
	case cf.SchemaVersion == "":
		s.salvageCorrupt(fmt.Errorf("missing schema version in cache file %q", s.path))
		return
	case cf.SchemaVersion != currentSchemaVersion:
		// A schema we do not understand. Never overwrite it; disable both reads and writes so a
		// newer tool's records are preserved and remain authoritative.
		s.disable(fmt.Sprintf("unsupported persistent cache schema version %q at %q (this tool uses %q)", cf.SchemaVersion, s.path, currentSchemaVersion), nil)
		return
	case cf.ToolVersion > currentToolVersion:
		// Written by a newer tool: same preservation policy as an unknown schema.
		s.disable(fmt.Sprintf("persistent cache at %q written by newer tool version %d (this tool uses %d)", s.path, cf.ToolVersion, currentToolVersion), nil)
		return
	case cf.ToolVersion < currentToolVersion:
		// Older records can never match current-version keys (tool version is key material), so
		// they are expired rather than corrupt. The layer stays enabled and repopulates.
		s.ctr.invalidRecords.Add(int64(len(cf.Records)))
		s.logger.Warnw("discarding persistent cache records written by an older tool version", "path", s.path, "file_tool_version", cf.ToolVersion, "current_tool_version", currentToolVersion, "discarded", len(cf.Records))
		return
	}

	loaded := make(map[string]storedRecord, len(cf.Records))
	rejected := 0
	for key, rec := range cf.Records {
		if !validStoredRecord(key, rec) {
			rejected++
			continue
		}
		loaded[key] = rec
	}
	s.mu.Lock()
	s.records = loaded
	s.mu.Unlock()
	if rejected > 0 {
		s.ctr.invalidRecords.Add(int64(rejected))
		s.logger.Warnw("ignored invalid records while loading persistent verdict cache", "path", s.path, "rejected", rejected, "loaded", len(loaded))
	}
	s.logger.Infow("loaded persistent verdict cache", "path", s.path, "records", len(loaded))
}

// validStoredRecord validates one record before it can be served from disk or merged.
func validStoredRecord(key string, rec storedRecord) bool {
	if rec.Key == "" || rec.Key != key {
		return false
	}
	if rec.SchemaVersion != currentSchemaVersion || rec.ToolVersion != currentToolVersion {
		return false
	}
	if rec.StableID == "" || rec.CodeFingerprint == "" || rec.ConfigFingerprint == "" {
		return false
	}
	return IsReusable(failnessOf(rec.Status), analyzerKindOf(rec.OriginalSource))
}

// salvageCorrupt moves an unreadable cache document aside (preserving its bytes for inspection)
// and starts with an empty store, so the cache can be re-established immediately. The salvage is
// coordinated through the cross-process lock and re-checks the file under it.
func (s *persistentStore) salvageCorrupt(cause error) {
	release, lockErr := s.acquireLock()
	if lockErr != nil {
		// Cannot coordinate: do not risk clobbering another process's recovery; stay disabled.
		s.disable("persistent verdict cache is corrupt and the cache lock could not be acquired", errors.Join(cause, lockErr))
		return
	}
	defer release()

	// Another process may already have salvaged or replaced the file; re-check before renaming.
	if data, err := os.ReadFile(s.path); err == nil {
		var cf cacheFile
		if json.Unmarshal(data, &cf) == nil && cf.SchemaVersion == currentSchemaVersion && cf.ToolVersion == currentToolVersion {
			loaded := make(map[string]storedRecord, len(cf.Records))
			for key, rec := range cf.Records {
				if validStoredRecord(key, rec) {
					loaded[key] = rec
				}
			}
			s.mu.Lock()
			s.records = loaded
			s.mu.Unlock()
			s.ctr.loadFailures.Add(1)
			s.logger.Warnw("persistent verdict cache was recovered by another process; using it", "path", s.path, "records", len(loaded), "original_error", cause)
			return
		}
		backup := fmt.Sprintf("%s.corrupt-%d", s.path, s.now().UnixNano())
		if err := os.Rename(s.path, backup); err != nil {
			s.disable("persistent verdict cache is corrupt and could not be moved aside", errors.Join(cause, err))
			return
		}
		s.ctr.loadFailures.Add(1)
		s.logger.Warnw("persistent verdict cache unreadable; moved corrupt file aside and starting with an empty cache", "path", s.path, "backup", backup, "error", cause)
		return
	} else if errors.Is(err, fs.ErrNotExist) {
		// Another process already moved it aside; simply start fresh.
		s.ctr.loadFailures.Add(1)
		s.logger.Warnw("persistent verdict cache already recovered by another process; starting with an empty cache", "path", s.path, "error", cause)
		return
	} else {
		s.disable("persistent verdict cache is corrupt and became unreadable", errors.Join(cause, err))
		return
	}
}

// disable marks the layer unusable for this process and records an observable load failure.
func (s *persistentStore) disable(msg string, cause error) {
	s.ctr.loadFailures.Add(1)
	s.mu.Lock()
	s.writesDisabled = true
	s.mu.Unlock()
	if cause != nil {
		s.logger.Warnw(msg+"; falling back to in-process analyzer chain", "error", cause)
	} else {
		s.logger.Warnw(msg + "; falling back to in-process analyzer chain")
	}
}

// lookup returns a validated record by full key hash.
func (s *persistentStore) lookup(key string) (storedRecord, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.writesDisabled {
		return storedRecord{}, false, nil
	}
	rec, ok := s.records[key]
	return rec, ok, nil
}

// add queues a verdict for persistence. If the layer is write-degraded the record is counted as a
// write degradation; it remains available in process memory regardless.
func (s *persistentStore) add(rec storedRecord) {
	s.mu.Lock()
	if s.writesDisabled {
		s.mu.Unlock()
		s.ctr.writeDegraded.Add(1)
		s.logger.Debugw("persistent cache write-degraded; verdict kept in process memory only", "key", rec.Key)
		return
	}
	s.pending[rec.Key] = rec
	n := len(s.pending)
	threshold := s.flushThreshold
	s.mu.Unlock()

	if n >= threshold {
		if err := s.flush(); err != nil {
			s.logger.Debugw("intermediate persistent cache flush failed; will retry at run end", "error", err)
		}
	}
}

// flush merges this process's pending verdicts with the latest on-disk document under an exclusive
// cross-process lock and atomically replaces the cache file. Valid on-disk records are never
// dropped; records for the same function+site with a different (older) code/config fingerprint are
// superseded. Errors never affect verdicts.
func (s *persistentStore) flush() error {
	s.mu.RLock()
	if s.writesDisabled {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	if len(s.pending) == 0 {
		s.mu.Unlock()
		return nil
	}
	pending := make(map[string]storedRecord, len(s.pending))
	for k, v := range s.pending {
		pending[k] = v
	}
	s.mu.Unlock()

	release, err := s.acquireLock()
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			// Definitive (unwritable directory): stop trying for this process so later add()
			// calls degrade immediately and every dropped record is counted.
			s.giveUpWrites(pending, err)
			return err
		}
		// Transient (lock busy, I/O hiccup): keep pending for the run-end retry without counting
		// a degradation yet.
		return fmt.Errorf("acquire cache lock: %w", err)
	}
	defer release()

	merged, err := s.readLatestForMerge()
	if err != nil {
		// Fatal merge-read (future schema appeared, corruption un-salvageable, permission loss):
		// stop trying to write this run.
		s.giveUpWrites(pending, err)
		return err
	}

	// Index on-disk records by stable function+site identity so we can supersede older versions
	// of the exact same site while leaving every unrelated verdict untouched.
	pendingStableIDs := make(map[string]bool, len(pending))
	for _, rec := range pending {
		pendingStableIDs[rec.StableID] = true
	}
	supersededKeys := make(map[string]bool)
	for key, rec := range merged {
		if pendingStableIDs[rec.StableID] {
			if _, keep := pending[key]; !keep {
				delete(merged, key)
				supersededKeys[key] = true
			}
		}
	}
	for key, rec := range pending {
		merged[key] = rec
	}

	if err := s.writeAtomically(merged); err != nil {
		s.giveUpWrites(pending, err)
		return err
	}

	s.mu.Lock()
	for key := range supersededKeys {
		delete(s.records, key)
	}
	for key, rec := range pending {
		s.records[key] = rec
		delete(s.pending, key)
	}
	s.mu.Unlock()

	s.logger.Debugw("flushed persistent verdict cache", "path", s.path, "new_records", len(pending), "superseded", len(supersededKeys), "total_records", len(merged))
	return nil
}

// readLatestForMerge re-reads and validates the on-disk document while holding the lock. It
// returns a writable map even when the file does not exist yet.
func (s *persistentStore) readLatestForMerge() (map[string]storedRecord, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return make(map[string]storedRecord), nil
		}
		return nil, fmt.Errorf("read cache file: %w", err)
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		// A concurrent writer using temp+rename cannot produce this; treat external corruption by
		// moving it aside, preserving the bytes.
		backup := fmt.Sprintf("%s.corrupt-%d", s.path, s.now().UnixNano())
		if renameErr := os.Rename(s.path, backup); renameErr != nil {
			return nil, fmt.Errorf("cache corrupt and backup failed: %w", errors.Join(err, renameErr))
		}
		s.ctr.loadFailures.Add(1)
		s.logger.Warnw("persistent verdict cache found corrupt during merge; moved aside", "path", s.path, "backup", backup, "error", err)
		return make(map[string]storedRecord), nil
	}
	if cf.SchemaVersion != currentSchemaVersion || cf.ToolVersion != currentToolVersion {
		// A newer/unknown document showed up (e.g. a newer tool ran concurrently). Never clobber.
		return nil, fmt.Errorf("cache file version unsupported during merge: schema=%q tool=%d", cf.SchemaVersion, cf.ToolVersion)
	}
	merged := make(map[string]storedRecord, len(cf.Records))
	rejected := 0
	for key, rec := range cf.Records {
		if !validStoredRecord(key, rec) {
			rejected++
			continue
		}
		merged[key] = rec
	}
	if rejected > 0 {
		s.ctr.invalidRecords.Add(int64(rejected))
		s.logger.Warnw("ignored invalid records while merging persistent verdict cache", "path", s.path, "rejected", rejected)
	}
	return merged, nil
}

// writeAtomically writes the document to a temp file in the destination directory, fsyncs it, and
// renames it over the cache file. The directory is fsynced best-effort so the rename survives a
// crash.
func (s *persistentStore) writeAtomically(records map[string]storedRecord) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	doc := cacheFile{
		SchemaVersion: currentSchemaVersion,
		ToolVersion:   currentToolVersion,
		Records:       records,
	}
	tmp, err := os.CreateTemp(s.dir, "."+filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp cache file: %w", err)
	}
	tmpName := tmp.Name()
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("encode cache file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("fsync temp cache file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp cache file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("replace cache file: %w", err)
	}
	syncDirBestEffort(s.dir)
	return nil
}

// giveUpWrites marks the layer write-degraded and accounts for the dropped pending records.
// Reads of already-loaded records remain allowed; subsequent add() calls are counted degraded.
func (s *persistentStore) giveUpWrites(dropped map[string]storedRecord, cause error) {
	s.ctr.writeDegraded.Add(int64(len(dropped)))
	s.mu.Lock()
	for key := range dropped {
		delete(s.pending, key)
	}
	// Keep the layer readable but stop attempting writes for this process.
	s.writesDisabled = true
	s.mu.Unlock()
	s.logger.Warnw("persistent verdict cache writes disabled for this process; verdicts unaffected", "path", s.path, "dropped_records", len(dropped), "error", cause)
}

// acquireLock takes an exclusive, process-advisory lock on the sibling lock file. It is
// non-blocking with bounded retries so a dead/locked peer degrades to the analyzer chain instead
// of hanging the analysis.
func (s *persistentStore) acquireLock() (func(), error) {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	f, err := os.OpenFile(s.lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open cache lock file: %w", err)
	}
	deadline := s.now().Add(lockTimeout)
	for {
		if err := tryLockFile(f); err == nil {
			return func() {
				_ = unlockFile(f)
				_ = f.Close()
			}, nil
		} else if !s.now().Before(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("timed out waiting for cache lock %q: %w", s.lockPath, err)
		}
		time.Sleep(lockRetry)
	}
}

// failnessOf/analyzerKindOf are thin string conversions used by record validation.
func failnessOf(s string) constant.Failness         { return constant.Failness(s) }
func analyzerKindOf(s string) constant.AnalyzerKind { return constant.AnalyzerKind(s) }
