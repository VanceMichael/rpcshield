package pkgdeps

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

func TestDependentPackages(t *testing.T) {
	// Create a list of all packages
	allPkgs := map[string]*packages.Package{
		"pkg0": {PkgPath: "pkg0"},
		"pkg1": {PkgPath: "pkg1"},
		"pkg2": {PkgPath: "pkg2"},
		"pkg3": {PkgPath: "pkg3"},
		"pkg4": {PkgPath: "pkg4"},
	}

	// Create a new package
	pkg := &packages.Package{
		PkgPath: "pkg0",
		Imports: map[string]*packages.Package{
			"pkg1": allPkgs["pkg1"],
			"pkg2": allPkgs["pkg2"],
		},
	}

	// Call the function
	pkgWrapper := New(pkg, allPkgs)
	// Check if the result is as expected
	require.Equal(t, 2, len(pkgWrapper.Deps))
	require.NotNilf(t, pkgWrapper.Deps["pkg1"], "Expected pkg1 to be a dependent package")
	require.Equal(t, "pkg1", pkgWrapper.Deps["pkg1"].PkgPath)
	require.NotNilf(t, pkgWrapper.Deps["pkg2"], "Expected pkg2 to be a dependent package")
	require.Equal(t, "pkg2", pkgWrapper.Deps["pkg2"].PkgPath)
}
