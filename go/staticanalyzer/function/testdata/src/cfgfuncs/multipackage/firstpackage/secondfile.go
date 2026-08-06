package firstpackage

type B struct {
	myA *A
}

func (b *B) GetMyA() *A {
	return b.myA
}

func (b *B) CheckReflect() bool {
	return b == b.GetMyA().GetMyB()
}
