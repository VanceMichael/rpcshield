package multipackage

import (
	"cfgfuncs/multipackage/firstpackage"
	"cfgfuncs/multipackage/secondpackage"
)

func main() {
	a := &firstpackage.A{}
	b := &firstpackage.B{}
	c := secondpackage.C{true}
	c.Branch(a, b).CheckReflect()
}

func safeBoxManipulations() {
	c := secondpackage.CBox{}
	c.Box(nil)
	c.Ptr = nil
}

func unsafeBoxManipulations() *secondpackage.C {
	c := secondpackage.CBox{}
	if true {
		return c.Unbox()
	} else {
		return c.Ptr
	}
}
