package secondpackage

import "cfgfuncs/multipackage/firstpackage"

type C struct {
	B bool
}

type CBox struct {
	Ptr *C
}

func (c CBox) Unbox() (ptr *C) {
	return c.Ptr
}

func (c CBox) Box(ptr *C) {
	c.Ptr = ptr
}

func (c C) Branch(a *firstpackage.A, b *firstpackage.B) *firstpackage.B {
	if c.B {
		return a.GetMyB()
	} else {
		return b
	}
}
