package packageB

type A struct {
	S string
}

func (a *A) Foo(x *A) (*A, string) {
	var b *A
	return b, x.S
}
