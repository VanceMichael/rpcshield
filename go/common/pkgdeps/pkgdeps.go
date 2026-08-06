package pkgdeps

import "golang.org/x/tools/go/packages"

// Info stores a package and its dependent packages.
type Info struct {
	Pkg  *packages.Package
	Deps map[string]*packages.Package
}

// New creates a new instance of the Info struct.
func New(pkg *packages.Package, allPkgs map[string]*packages.Package) *Info {
	return &Info{
		Pkg:  pkg,
		Deps: dependentPackages(pkg, allPkgs),
	}
}

// dependentPackages returns a map of dependent packages of the given package `pkgdeps`.
func dependentPackages(pkg *packages.Package, allPkgs map[string]*packages.Package) map[string]*packages.Package {
	deps := make(map[string]*packages.Package)
	for depPkgPath := range pkg.Imports {
		if depPkg, ok := allPkgs[depPkgPath]; ok {
			deps[depPkgPath] = depPkg
		}
	}
	return deps
}
