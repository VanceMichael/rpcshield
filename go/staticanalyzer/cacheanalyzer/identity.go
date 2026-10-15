package cacheanalyzer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
)

// stableIdentity is the cross-process stable identity of a (caller function, call site) pair.
// Every field is derived from source-level information (import paths, names, source positions),
// never from SSA pointers, so two processes rebuilding SSA from the same code compute the same
// identity.
type stableIdentity struct {
	// PkgPath is the caller's import path, e.g. github.com/acme/svc/handler.
	PkgPath string `json:"pkg_path"`
	// Function is the SSA-style fully qualified function name (f.String()), including receiver
	// type and funclit numbering, e.g. github.com/acme/svc/handler.(*H).Serve.func1.
	Function string `json:"function"`
	// CallSiteFile is the source file of the call site, normalized against the package import
	// path so that lifted-GOPATH / bazel-sandbox prefixes do not change between processes.
	CallSiteFile string `json:"callsite_file"`
	// CallSiteLine and CallSiteColumn locate the call site within the file. Column is required to
	// distinguish multiple calls on a single line.
	CallSiteLine   int `json:"callsite_line"`
	CallSiteColumn int `json:"callsite_column"`
	// CallText is the raw source text of the call expression (best effort). Combined with line,
	// column, signature and caller source hash it makes moved/retyped sites miss even if positions
	// happen to collide.
	CallText string `json:"call_text,omitempty"`
	// CallSignature is the SSA signature of the call itself (its result tuple, e.g. (error)),
	// capturing callee-signature changes that alter error propagation even when caller text is
	// byte-identical.
	CallSignature string `json:"call_signature,omitempty"`
	// StaticCallee is f.String() of the static callee (if the call is statically dispatched).
	// Dynamic/interface calls leave this empty.
	StaticCallee string `json:"static_callee,omitempty"`
}

// stableID renders the identity as one stable, human-parseable string. It is also used to prune
// records superseded by a new code/config fingerprint for the very same function+site.
func (i stableIdentity) stableID() string {
	return strings.Join([]string{
		i.PkgPath,
		i.Function,
		fmt.Sprintf("%s:%d:%d", i.CallSiteFile, i.CallSiteLine, i.CallSiteColumn),
		i.StaticCallee,
		i.CallSignature,
	}, "|")
}

// keyMaterial is the full set of inputs hashed into a persistent record key. A change in the
// identity, analyzed code, tool/schema version, or verdict-affecting configuration changes the
// hash and therefore produces a miss.
type keyMaterial struct {
	SchemaVersion     string         `json:"schema_version"`
	ToolVersion       int            `json:"tool_version"`
	Identity          stableIdentity `json:"identity"`
	CodeFingerprint   string         `json:"code_fingerprint"`
	ConfigFingerprint string         `json:"config_fingerprint"`
}

// buildIdentity derives the stable identity and the raw caller-function source bytes for an SSA
// function and its call instruction.
func (c *cacheAnalyzer) buildIdentity(f *ssa.Function, call ssa.Instruction) (stableIdentity, []byte, error) {
	if f == nil || call == nil {
		return stableIdentity{}, nil, fmt.Errorf("nil function or instruction")
	}
	if f.Prog == nil || f.Prog.Fset == nil {
		return stableIdentity{}, nil, fmt.Errorf("function %q has no SSA program/file set", safeFuncString(f))
	}
	pkg := f.Pkg
	if pkg == nil || pkg.Pkg == nil {
		return stableIdentity{}, nil, fmt.Errorf("function %q has no package identity", safeFuncString(f))
	}
	node := f.Syntax()
	if node == nil {
		// External/native/synthetic functions have no source and are not analyzable inputs.
		return stableIdentity{}, nil, fmt.Errorf("function %q has no source syntax", safeFuncString(f))
	}

	fset := f.Prog.Fset
	callerStart := fset.Position(node.Pos())
	callerEnd := fset.Position(node.End())
	if callerStart.Filename == "" {
		return stableIdentity{}, nil, fmt.Errorf("function %q has no source file position", safeFuncString(f))
	}
	src, err := c.readSource(callerStart.Filename)
	if err != nil {
		return stableIdentity{}, nil, fmt.Errorf("read caller source %q: %w", callerStart.Filename, err)
	}
	if callerStart.Offset < 0 || callerEnd.Offset > len(src) || callerEnd.Offset < callerStart.Offset {
		return stableIdentity{}, nil, fmt.Errorf("caller source offsets out of range for %q", safeFuncString(f))
	}
	callerSource := src[callerStart.Offset:callerEnd.Offset]

	sitePos := call.Pos()
	if !sitePos.IsValid() {
		return stableIdentity{}, nil, fmt.Errorf("call site in %q has no valid source position", safeFuncString(f))
	}
	site := fset.Position(sitePos)

	id := stableIdentity{
		PkgPath:        pkg.Pkg.Path(),
		Function:       f.String(),
		CallSiteFile:   normalizeFile(site.Filename, pkg.Pkg.Path()),
		CallSiteLine:   site.Line,
		CallSiteColumn: site.Column,
	}
	if ci, ok := call.(ssa.CallInstruction); ok && ci.Common() != nil {
		if sig := ci.Common().Signature(); sig != nil {
			id.CallSignature = sig.String()
		}
		if callee := ci.Common().StaticCallee(); callee != nil {
			id.StaticCallee = callee.String()
		}
	}

	// Best-effort raw text of the call expression. Failure here only loses the extra text guard;
	// the caller-function source hash already covers every byte of the site anyway.
	if end, ok := c.astCallEnds(f)[call.Pos()]; ok && site.Filename == callerStart.Filename {
		endPos := fset.Position(end)
		if endPos.Offset >= site.Offset && endPos.Offset <= len(src) {
			id.CallText = string(src[site.Offset:endPos.Offset])
		}
	}

	if id.CallSiteFile == "" || id.CallSiteLine == 0 {
		return stableIdentity{}, nil, fmt.Errorf("call site in %q has no resolvable source location", safeFuncString(f))
	}
	return id, callerSource, nil
}

// buildRecord constructs a fully versioned stored record for a reusable conclusion.
func (c *cacheAnalyzer) buildRecord(f *ssa.Function, call ssa.Instruction, status constant.Failness, source constant.AnalyzerKind) (storedRecord, error) {
	id, callerSource, err := c.buildIdentity(f, call)
	if err != nil {
		return storedRecord{}, err
	}
	callerHash := sha256Hex(callerSource)
	codeFP := codeFingerprint(c.codeVersion, callerHash)
	km := keyMaterial{
		SchemaVersion:     currentSchemaVersion,
		ToolVersion:       currentToolVersion,
		Identity:          id,
		CodeFingerprint:   codeFP,
		ConfigFingerprint: c.configFingerprint,
	}
	kb, err := json.Marshal(km)
	if err != nil {
		return storedRecord{}, fmt.Errorf("marshal cache key material: %w", err)
	}
	return storedRecord{
		Key:               sha256Hex(kb),
		SchemaVersion:     currentSchemaVersion,
		ToolVersion:       currentToolVersion,
		StableID:          id.stableID(),
		Status:            string(status),
		OriginalSource:    string(source),
		CodeFingerprint:   codeFP,
		ConfigFingerprint: c.configFingerprint,
		CreatedAt:         c.persistent.now().Format(timestampLayout),
	}, nil
}

// lookupPersistent finds a record by recomputing its full key. It returns (zero, false, nil) on a
// miss and an error only when no stable identity could be built.
func (c *cacheAnalyzer) lookupPersistent(f *ssa.Function, call ssa.Instruction) (storedRecord, bool, error) {
	id, callerSource, err := c.buildIdentity(f, call)
	if err != nil {
		return storedRecord{}, false, err
	}
	km := keyMaterial{
		SchemaVersion:     currentSchemaVersion,
		ToolVersion:       currentToolVersion,
		Identity:          id,
		CodeFingerprint:   codeFingerprint(c.codeVersion, sha256Hex(callerSource)),
		ConfigFingerprint: c.configFingerprint,
	}
	kb, err := json.Marshal(km)
	if err != nil {
		return storedRecord{}, false, fmt.Errorf("marshal cache key material: %w", err)
	}
	return c.persistent.lookup(sha256Hex(kb))
}

// readSource returns the contents of filename, caching per process.
func (c *cacheAnalyzer) readSource(filename string) ([]byte, error) {
	c.srcMu.Lock()
	if src, ok := c.srcCache[filename]; ok {
		c.srcMu.Unlock()
		return src, nil
	}
	c.srcMu.Unlock()

	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	c.srcMu.Lock()
	c.srcCache[filename] = src
	c.srcMu.Unlock()
	return src, nil
}

// normalizeFile turns an on-disk filename into a process-independent path by keeping the package
// import path suffix (e.g. .../src/github.com/acme/svc/handler.go -> github.com/acme/svc/handler.go).
// When the import path cannot be located, it falls back to the base filename so the cache degrades
// to a miss across differing paths instead of yielding a wrong hit.
func normalizeFile(filename, pkgPath string) string {
	filename = filepath.ToSlash(filename)
	if pkgPath != "" {
		idx := strings.LastIndex(filename, pkgPath)
		if idx >= 0 {
			before := idx == 0 || filename[idx-1] == '/'
			after := idx+len(pkgPath) == len(filename) || filename[idx+len(pkgPath)] == '/'
			if before && after {
				return filename[idx:]
			}
		}
	}
	return filepath.ToSlash(filepath.Base(filename))
}

// codeFingerprint combines the analyzed-code version (e.g. Git SHA) with the hash of the exact
// caller-function source bytes. Either component changing produces a different fingerprint.
func codeFingerprint(codeVersion, callerHash string) string {
	sum := sha256.Sum256([]byte("code_version=" + codeVersion + "\ncaller_source=" + callerHash))
	return sha256Hex(sum[:])
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
