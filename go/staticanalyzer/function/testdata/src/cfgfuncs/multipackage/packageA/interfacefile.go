package packageA

import (
	"cfgfuncs/multipackage/packageB"
)

type I interface {
	Foo(x *packageB.A) (*packageB.A, string)
}
