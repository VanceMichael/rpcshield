package types

import (
	"fmt"
	"strings"

	rpcshieldconst "github.com/uber-research/rpcshield/go/constant"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
)

// Codebase represents a collection of Go packages with mapping utilities for analysis.
type Codebase struct {
	Entry        *packages.Package
	All          []*packages.Package
	CodebaseOnly []*packages.Package
	PkgMap       map[string]*packages.Package
}

// canonicalTargetSuffixes are the Bazel target names that own a package's "real" import path.
// Some custom rules (e.g. wrapper/preview libraries that embed a sibling library)
// deliberately reuse their embedded library's importpath under a differently-named sibling target,
// which surfaces as a second *packages.Package with an identical PkgPath when BazelLoader queries a
// whole package subtree. Such variants are not part of the
// real production dependency graph, so they're dropped in favor of the canonical target below.
var canonicalTargetSuffixes = []string{":go_default_library", ":go_default_binary"}

func isCanonicalTarget(id string) bool {
	for _, suffix := range canonicalTargetSuffixes {
		if strings.HasSuffix(id, suffix) {
			return true
		}
	}
	return false
}

// isGoProtoVariant returns true if the Bazel target *name* (not PkgPath) ends in the gazelle-standard
// "_go_proto" suffix used for go_proto_library targets generated from a .proto file. Some early-vendored
// external deps ship both a checked-in, non-generated go_library and a go_proto_library under the same
// PkgPath (e.g. github.com/coreos/etcd/raft/raftpb's "raftpb" vs "raftpb_go_proto"); the package's
// go_default_library alias resolves to the non-"_go_proto" target, so it's preferred over its
// "_go_proto" sibling when the two collide.
func isGoProtoVariant(id string) bool {
	name := id
	if idx := strings.LastIndex(id, ":"); idx != -1 {
		name = id[idx+1:]
	}
	return strings.HasSuffix(name, "_go_proto")
}

// DedupPackages returns allPkgs with duplicate PkgPath entries resolved: when two packages share a
// PkgPath, the one whose Bazel target is canonical (see canonicalTargetSuffixes) is kept and the
// other is dropped with a warning. If neither (or both) are canonical, it's treated as a genuine
// error.
func DedupPackages(logger *zap.SugaredLogger, allPkgs []*packages.Package) ([]*packages.Package, map[string]*packages.Package, error) {
	pkgMap := make(map[string]*packages.Package)
	deduped := make([]*packages.Package, 0, len(allPkgs))
	for _, pkg := range allPkgs {
		existing, ok := pkgMap[pkg.PkgPath]
		if !ok {
			pkgMap[pkg.PkgPath] = pkg
			deduped = append(deduped, pkg)
			continue
		}

		canonical, existingCanonical := isCanonicalTarget(pkg.ID), isCanonicalTarget(existing.ID)
		goProto, existingGoProto := isGoProtoVariant(pkg.ID), isGoProtoVariant(existing.ID)
		switch {
		case canonical && !existingCanonical:
			logger.Warnw("Duplicate package found; keeping canonical target and dropping non-canonical variant",
				"package", pkg.PkgPath, "kept", pkg.ID, "dropped", existing.ID)
			pkgMap[pkg.PkgPath] = pkg
			for i, p := range deduped {
				if p == existing {
					deduped[i] = pkg
					break
				}
			}
		case !canonical && existingCanonical:
			logger.Warnw("Duplicate package found; keeping canonical target and dropping non-canonical variant",
				"package", pkg.PkgPath, "kept", existing.ID, "dropped", pkg.ID)
		case !canonical && !existingCanonical && !goProto && existingGoProto:
			logger.Warnw("Duplicate package found; keeping non-go_proto target and dropping go_proto variant",
				"package", pkg.PkgPath, "kept", pkg.ID, "dropped", existing.ID)
			pkgMap[pkg.PkgPath] = pkg
			for i, p := range deduped {
				if p == existing {
					deduped[i] = pkg
					break
				}
			}
		case !canonical && !existingCanonical && goProto && !existingGoProto:
			logger.Warnw("Duplicate package found; keeping non-go_proto target and dropping go_proto variant",
				"package", pkg.PkgPath, "kept", existing.ID, "dropped", pkg.ID)
		default:
			return nil, nil, fmt.Errorf("duplicate package found: %q (%s vs %s)", pkg.PkgPath, existing.ID, pkg.ID)
		}
	}
	return deduped, pkgMap, nil
}

// NewCodebase creates a new Codebase instance with the given entry package and all packages.
func NewCodebase(logger *zap.SugaredLogger, entry *packages.Package, allPkgs []*packages.Package, keepPkgs map[string]bool) (*Codebase, error) {
	deduped, pkgMap, err := DedupPackages(logger, allPkgs)
	if err != nil {
		return nil, err
	}

	codebaseOnly := codebasePkgs(logger, entry, deduped, keepPkgs)

	return &Codebase{
		Entry:        entry,
		All:          deduped,
		CodebaseOnly: codebaseOnly,
		PkgMap:       pkgMap,
	}, nil
}

// codebasePkgs uses the entry package as the seed package to find its relevant packages as the service
// codebase. This is needed to restrict the radius of analysis since there may be many common
// modules that have inbound/outbound points (e.g., perf monitoring endpoints) that are not
// interesting to the analysis of the particular service.
//
// Therefore, we employ a simple heuristic that filters out any packages that do not share a
// common prefix 2 levels up from the entry.
func codebasePkgs(logger *zap.SugaredLogger, entryPkg *packages.Package, allPkgs []*packages.Package, keepPkgs map[string]bool) []*packages.Package {
	parts := strings.Split(entryPkg.PkgPath, "/")
	for i := 0; i < 2; i++ {
		if len(parts) <= 3 {
			break
		}
		parts = parts[:len(parts)-1]
	}
	commonPrefix := strings.Join(parts, "/")

	var skipped int
	codebasePkgs := make([]*packages.Package, 0)
	for _, pkg := range allPkgs {
		pkgPath := strings.TrimPrefix(strings.TrimPrefix(pkg.PkgPath, rpcshieldconst.ThriftrwPkgPrefix+"/"), rpcshieldconst.GogoprotoPkgPrefix+"/")
		if !strings.HasPrefix(pkgPath, commonPrefix) && !keepPkgs[pkgPath] {
			logger.Debugw("Skipping package since it does not have the common prefix", "prefix", commonPrefix, "package", pkg.PkgPath)
			skipped++
			continue
		}
		codebasePkgs = append(codebasePkgs, pkg)
	}

	if skipped != 0 {
		logger.Infow("Skipped packages since they do not have common prefixes", "num_skipped", skipped)
	}

	return codebasePkgs
}
