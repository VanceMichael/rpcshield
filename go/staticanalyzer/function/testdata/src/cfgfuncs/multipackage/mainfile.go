package multipackage

import (
	"go/ast"

	"cfgfuncs/multipackage/packageA"
	"cfgfuncs/multipackage/packageB"
	"cfgfuncs/multipackage/packageC"
)

func testMultiPkgInterfaceCalls() packageA.I {
	param2(&packageB.A{})

	packageC.M()

	var v packageA.I
	v = &packageB.A{"xyz"}
	return v
}

func param2(i packageA.I) {
	// do something ...
}

func m11() {
	var node ast.Node = &ast.FuncDecl{}
	node.Pos()
}
