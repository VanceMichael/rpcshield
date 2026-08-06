package packageC

import (
	"cfgfuncs/multipackage/packageA"
	"cfgfuncs/multipackage/packageB"
)

func M() packageA.I {
	var v packageA.I = &packageB.A{"abc"}
	return v
}
