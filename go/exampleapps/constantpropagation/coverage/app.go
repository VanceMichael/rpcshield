package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: umbrella test to cover various edge cases not covered by other tests.

//go:noinline
func sink() {
}

//go:noinline
func infeasible() {
}

//go:noinline
func feasible() {
}

//go:noinline
func source() {
	// Exercises the case where the Phi becomes a bottom.
	unknownPhiMerge()
	// Exercises the case where the Phi does not merge to a bottom.
	nonBottomPhiMerge()

	// Exercises the LHS binary operator is a bottom.
	binOpLHSBottom()

	// Exercises the RHS binary operator is a bottom.
	binOpRHSBottom()

	// Nonbottom binary operator.
	binOpNonBottom()

	// Exercises the case where unary operator is a bottom.
	unaryOpBottom()

	// Nonbottom unary operator.
	unaryOpNonBottom()

	// Channel reads are bottom.
	channelReadBottom()

	// Changing the type retains the value.
	changeTypeNonBottom()
	// Changing the type to a bottom.
	convertTypeNonBottom()

	// TypeAssertNonBottom
	var typedNil *int
	typeAssertNonBottom(42, typedNil, nil)

	// TypeAssertBottom
	unkInterface := unknownReturnInterface()
	typeAssertBottom(unkInterface)

	// TypeAssertValueNonBottom
	var typedNil2 *int
	typeAssertValueNonBottom(42, typedNil2, nil)

	// TypeAssertValueBottom
	unkInterface2 := unknownReturnInterface()
	typeAssertValueBottom(unkInterface2)

	// Divide by zero.
	divideByZero()
	// panic handler
	panicHandler()
}

//go:noinline
func unknownReturn() int {
	return 42
}

//go:noinline
func unknownReturnInterface() interface{} {
	return 42
}

// Test objective: phi instuctions correctly model incoming arguments to infer constantness for ConstantVal and FunctionVal.
//
//go:noinline
func nonBottomPhiMerge() {
	v := unknownReturn()
	var x int
	if v > 0 {
		x = 1
	} else {
		x = 1
	}
	// Here the x = phi(1, 1) will be 1.
	if x != 1 {
		infeasible()
		callSink(x) // This is an infeasible path because x will not be bottom.
	}

	// Another phi merge but for a function type.
	var f func() int
	f1 := func() int { return 1 }
	f2 := f1
	if v > 0 {
		f = f1
	} else {
		f = f2
	}
	// Here f = phi(f1, f2) will be f1 because f2 = f1.
	if f == nil {
		infeasible()
		callSink(1) // This is an infeasible path because f is not nil.
	}
}

// Test objective: phi instuctions correctly bottom for different ConstantVal values.
//
//go:noinline
func unknownPhiMerge() {
	v := unknownReturn()
	var x int
	if v > 0 {
		x = 1
	} else {
		x = 2
	}
	// Here the x = phi(1, 2) = bottom.
	if x != -42 {
		feasible()
		callSink(x) // This is feasible path because x will be bottom.
	}
}

//go:noinline
func callSink(v int) {
	sink()
}

// Test objective: binary operator is a bottom for ConstantVal when LHS is a bottom.
//
//go:noinline
func binOpLHSBottom() {
	x := unknownReturn()
	y := 1
	z := x + y // Here the LHS is a bottom.
	// z = bottom + 1 = bottom.
	if z != -42 {
		feasible()
		callSink(z) // This is feasible path because z will be bottom.
	}
}

// Test objective: binary operator is a bottom for ConstantVal when RHS is a bottom.
//
//go:noinline
func binOpRHSBottom() {
	x := 1
	y := unknownReturn()
	z := x + y // Here the RHS is a bottom.
	// z = 1 + bottom = bottom.
	if z != -42 {
		feasible()
		callSink(z) // This is feasible path because z will be bottom.
	}
}

// Test objective: binary operator is a non-bottom for ConstantVal when both LHS and RHS are ConstantVal.
//
//go:noinline
func binOpNonBottom() {
	x := 1
	y := 2
	z := x + y // Here the result is 3
	// More binary operators.
	u := 1
	v := z << u // 3 << 1 = 6
	w := v / 2  // QUO operator makes w = 6/2 = 3
	a := w % 2  // REM operator makes a = 3%2 = 1
	if a != 1 {
		// since a = 1, this is an infeasible path.
		infeasible()
		callSink(a) // This is an infeasible path.
	}
}

// Test objective: unary operator is a non-bottom for ConstantVal when the operand is a ConstantVal.

//go:noinline
func unaryOpNonBottom() {
	x := 1
	y := 2
	z := x + y // Here the result is 3
	a := -z    // Here the result is -3
	if a != -3 {
		// since a = -3, this is an infeasible path.
		infeasible()
		callSink(a) // This is an infeasible path.
	}
}

// Test objective: unary operator is a bottom for when the operand is a bottom.
//
//go:noinline
func unaryOpBottom() {
	x := unknownReturn()
	y := -x
	// y = - bottom = bottom.
	if y != -42 {
		// y = bottom, this is a feasible path.
		feasible()
		callSink(y) // This is a feasible path because y will be bottom.
	}
}

// Test objective: channel read is a bottom.
//
//go:noinline
func channelReadBottom() {
	ch := make(chan int, 1)
	ch <- 42
	v := <-ch // Here the channel read will be bottom.
	if v == 42 {
		// v = bottom, this is a feasible path.
		feasible()
		callSink(v) // This is a feasible path because the channel read will be bottom.
	}
}

type myInt int

// Test objective: changing the type of a ConstantVal preserves the value.

//go:noinline
func changeTypeNonBottom() {
	x := myInt(42) // This creates a ssa.ChangeType instruction. The underlying data is 42.
	// changing 42 to myInt preserves the value.
	y := int(x)  // This creates a ssa.ChangeType instruction. The underlying data is 42.
	z := int(42) // This creates a ssa.ChangeType instruction.
	// Since y = 42 and z = 42, the condition y != z is false, making this an infeasible path.
	if y != z {
		infeasible()
		callSink(y) // This is an infeasible path because y == z.
	}
}

// Test objective: converting the type of a ConstantVal preserves the value.
//
//go:noinline
func convertTypeNonBottom() {
	x := "a string"
	y := []byte(x) // This is ssa.Convert instruction and we retain the underlying data
	z := string(y) // This is ssa.Convert instruction and we retain the underlying data
	// Here z = "a string" because the underlying data is the same.
	// since z != "a string", this is an infeasible condition.
	if z != "a string" {
		infeasible()
		callSink(1) // This is an infeasible path because z == "a string".
	}
}

// Test objective: type assertion extracts the correct value and type from their interfaces.
//
//go:noinline
func typeAssertNonBottom(anInteger interface{}, anIntegerPointer interface{}, aNil interface{}) {
	x, ok := anInteger.(int) // This is ssa.TypeAssert instruction and we extract the integer out of the interface.
	// Since the interface is an int, ok is true.
	if !ok {
		infeasible()
		callSink(x) // This is an infeasible path because i is an int and ok is true.
	}
	// Since x = 42, this is an infeasible condition.
	if x != 42 {
		infeasible()
		callSink(x) // This is an infeasible path because x is 42.
	}
	// Since anIntegerPointer is a pointer type assertion, ok is false.
	y, ok := anIntegerPointer.(int)
	// Ok is false since anIntegerPointer is a pointer type.
	if ok {
		infeasible()
		callSink(y) // This is an infeasible path because j is intPtr
	}
	// Since type assertion failed, y = default value of int, which is 0.
	// Since y = 0 == 42, this is an infeasible condition.
	if y == 42 {
		infeasible()
		callSink(y) // This is an infeasible path because y will be nil Ptr
	}

	// aNil is a nil value and nil values cannot be converted to any type.
	// The type assertion fails and z will be nil.
	z, ok := aNil.(int)
	// ok is false hence, the condition is infeasible.
	if ok {
		infeasible()
		callSink(z) // This is an infeasible path because k is untyped nil
	}
	// Since type assertion failed, z = default value of int, which is 0.
	// Since z = 0 == 42, this is an infeasible condition.
	if z == 42 {
		infeasible()
		callSink(z) // This is an infeasible path because z will be 0 since k is untyped nil.
	}
}

// Test objective: type assertion is a bottom for when the operand's underlying value cannot be resolved.
//
//go:noinline
func typeAssertBottom(bottomValue interface{}) {
	// The value and type are both unknown.
	// Type assertion will return bottom for y and ok.
	y, ok := bottomValue.(int)
	if !ok {
		// We cannot resolve the type of bottomValue, hence this is a feasible path.
		feasible()
		callSink(y) // This is a feasible path because i any interface can be any type.
	} else {
		// We can resolve the type of bottomValue, hence this is a feasible path.
		callSink(y) // This is a feasible path because i any interface can be any type.
	}
	// Since y = bottom, the outcome is unknown.
	if y == 42 {
		// Since y = bottom, this is a feasible path.
		feasible()
		callSink(y) // This is a feasible path because y can take any value.
	} else {
		// We can resolve the type of bottomValue, hence this is a feasible path.
		callSink(y) // This is a feasible path because y can take any value.
	}
}

// Test objective: type assertion extracts the correct value and type from their interfaces.
//
//go:noinline
func typeAssertValueNonBottom(anInteger interface{}, ptrToInt interface{}, aNil interface{}) {
	// Since anInteger is an integer, x will be 42.
	x := anInteger.(int)
	if x != 42 {
		// Since x = 42, this is an infeasible condition.
		infeasible()
		callSink(x) // This is an infeasible path because x is an int that is 42
	}
	// Since ptrToInt is a pointer to an integer, but since it is a nil pointer, y will be nil.
	y := ptrToInt.(*int)
	if y != nil {
		// Since y = nil, this is an infeasible condition.
		infeasible()
		callSink(*y) // This is an infeasible path because y is nil
	}
	// Since aNil is a nil value, the type assertion will fail and y will be nil.
	y = aNil.(*int)
	if y != nil {
		// Since y = nil, this is an infeasible condition.
		infeasible()
		callSink(*y) // This is an infeasible path because y is nil from an untyped nil
	}
}

// Test objective: type assertion is a bottom non-comma syntax.
//
//go:noinline
func typeAssertValueBottom(anUnknownIface interface{}) {
	// Since anUnknownIface is an interface, x will be bottom.
	x := anUnknownIface.(int)
	if x != 42 {
		// Since x = bottom, this is a feasible path.
		feasible()
		callSink(x) // This is a feasible path because i is an interface and can be any type.
	} else {
		// Since x = bottom, this is a feasible path.
		feasible()
		callSink(x) // This is a feasible path because i is an interface and can be any type.
	}
}

// Test objective: divide by zero is a bottom.
//
//go:noinline
func divideByZero() {
	x := 1
	y := 0
	z := x / y // z will be bottom.
	// Divide by zero will return bottom.
	// Since z = bottom, this is a feasible condition.
	if z == 42 {
		// Since z = bottom, this is a feasible condition.
		feasible()
		callSink(z) // This is a feasible path because z will be bottom.
	}
	a := 1.0
	b := 0.0
	c := a / b // c will be bottom.
	// Since c = bottom, this is a feasible condition.
	if c == 42.0 {
		feasible()
		callSink(1) // This is a feasible path because c will be bottom.
	}

	// complex
	m := complex(1, 0)
	n := complex(0, 0)
	o := m / n // o will be bottom.
	// Since o = bottom, this is a feasible condition.
	if o == 42 {
		feasible()
		callSink(1) // This is a feasible path because o will be bottom.
	}
}

// Test objective: panic handler is a feasible path.
//
//go:noinline
func panicHandler() {
	// Recover block is always reachable.
	if recover() != nil {
		// Recover block is always reachable.
		feasible()
		callSink(1) // recover block is always reachable.
	}
	panic("panic")
}

func main() {
	source()
}
