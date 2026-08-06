package cfgfuncs

func testSimpleAssignment() {
	var t1 *int
	a := func() {
		print(*t1)
	}
	a()

	var t2 *int
	b := func() {
		print(*t1)
		print(*t2)
	}

	i := 1
	t1 = &i
	t2 = nil

	b()
}

func testAssignOneToOneAssingments() {
	var t1 *int
	a, b := func(t *int) {
		print(*t)
	}, func(t2 *int) {
		print(*t2)
	}
	a(t1)
	b(t1)

}

func testNestedAnonymousFuncAssignment() {
	var t1 *int
	var t2 *int
	a := func() {
		print(*t1)
		var t2 *int
		print(*t2)
		b := func() {
			print(*t1)
			print(*t2)
			if t2 != nil {
				print(*t2)
			}
		}
		b()
	}
	i := 1
	t2 = &i
	print(*t2)
	a()
}

func testImplicitAndExplicitArguments() {
	var t1 *int
	var t2 *int
	f := func(t *int) {
		print(*t)
		g := func(p *int) {
			print(*t)
			print(*p)
			print(*t2)
		}
		i := 1
		t = &i
		g(t1)
	}

	f(t2)
}

func testShadowedClosureVariable() {
	var a *int
	i := 1
	b := &i

	func() {
		print(*a)
		func(a *int) {
			print(*a)
		}(b)
	}()
}

func testWithNamedReturn(i *int) (r *int) {
	print(*i)
	r = i
	k := func() {
		print(*r)
	}
	k()
	return
}

func testPassingArgsExplicityToAnonFuncs() {
	// Now we test uses of variables in the closure in anonymous functions. This is a simple test
	// such that all used variables are explicitly passed as params in to the anonymous functions.

	// Passing nilable variables as parameters.
	var t1 *int
	func(t *int) {
		print(*t)
	}(t1)

	func(t *int) {
		if t != nil {
			print(*t)
		}
	}(t1)

	// Pass a nonnil struct as parameter
	t2 := A2{}
	func(t *A2) *int {
		return t.a
	}(&t2)

	// Pass nilable struct as parameter.
	var t3 *A2
	func(t *A2) *int {
		return t.a
	}(t3)

	// test nested anonymous function

	var t4 *int
	func(t *int) {
		var t2 *int
		func(n1 *int, n2 *int) {
			print(*n1)
			print(*n2)
		}(t, t2)
	}(t4)

	c := 1
	t5 := &c
	func(t *int) {
		print(*t)
		var t2 *int
		func(n1 *int, n2 *int) {
			print(*n1)
			print(*n2)
			if n2 != nil {
				print(*n2)
			}
		}(t, t2)
	}(t5)
}

type B2 struct{}

func (b *B2) testOverWriteReceiver() {
	b = nil
	func() {
		print(*b)
	}()
}

func (b *B2) testPassReceiver() {
	func(nb *B2) {
		print(*nb)
	}(b)
}

func (b *B2) testGuardReceiver() {
	if b == nil {
		return
	}
	func(nb *B2) {
		print(*nb)
	}(b)
}

func nilReceiverTestDriver() {
	var b *B2
	b.testPassReceiver()
	b.testGuardReceiver()
}

func (a *A2) testNonNilReceiver() {
	func(na *A2) {
		print(*na)
	}(&A2{})
}

func testNilFlowFromClosure() {

	var t *int
	func() {
		print(*t)
	}()

	i := 1
	t = &i
	func() {
		print(*t)
	}()

	func() {
		print(*t)
		print(*t)
	}()

	t = nil

	func() {
		print(*t)
		print(*t)
	}()

	t = &i

	func() {
		print(*t)
		t = nil
		print(*t)
	}()

	print(*t)

	// test nested anonymous functions

	var t1 *int
	func() {
		var t2 *int
		func() {
			print(*t1)
			print(*t2)
		}()
	}()

	c := 1
	t3 := &c // t5 is nonnil now
	func() {
		print(*t3)
		var t4 *int
		func() {
			print(*t3)
			print(*t4)
			if t4 != nil {
				print(*t4)
			}
		}()
	}()

	var t5 *int
	func() {
		print(*t1)
		print(*t3)
		print(*t5)
	}()

}

type A2 struct {
	a *int
	b *A2
	c *A2
	d *int
}

func retNilable() *int {
	return nil
}

func simple() {
	aNonnilPtr := &A2{}
	print(*(aNonnilPtr.a))

	func() {
		var t *int
		print(*t)
		t2 := 1
		ptr := &t2
		print(*ptr)
		t3 := retNilable()
		print(*t3)
		var aPtr *A2
		print(*aPtr)
		aNonnilPtr := &A2{}
		print(*(aNonnilPtr.a))
		print(aNonnilPtr.c.a)
	}()

	a := func() {
		var t *int
		print(*t)
	}
	print(a)

	var f func() = func() {
		var t *int
		print(*t)
	}

	f()
}

// test nested anonymous functions
func nestedFunc() {
	func() {
		func() {
			var t *int
			print(*t)
		}()
		a := func() {
			var t *int
			print(*t)
		}
		print(a)
	}()

	func() {
		func() {
			func() {
				var t *int
				print(*t)
			}()
		}()
	}()
}

var a = func() {
	var t *int
	print(*t)
}
