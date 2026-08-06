package firstpackage

type A struct {
	myB *B
}

func (a *A) GetMyB() *B {
	return a.myB
}

func (a *A) CheckReflect() bool {
	return a == a.GetMyB().GetMyA()
}
