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

// NewCodebase creates a new Codebase instance with the given entry package and all packages.
func NewCodebase(logger *zap.SugaredLogger, entry *packages.Package, allPkgs []*packages.Package, keepPkgs map[string]bool) (*Codebase, error) {
	pkgMap := make(map[string]*packages.Package)
	for _, pkg := range allPkgs {
		if _, ok := pkgMap[pkg.PkgPath]; ok {
			return nil, fmt.Errorf("duplicate package found: %q", pkg.PkgPath)
		}
		pkgMap[pkg.PkgPath] = pkg
	}

	codebaseOnly := codebasePkgs(logger, entry, allPkgs, keepPkgs)

	return &Codebase{
		Entry:        entry,
		All:          allPkgs,
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
