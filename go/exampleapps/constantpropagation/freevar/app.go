package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: freevars are bound to callees correctly.

//go:noinline
func sink() {
}

func callSink(v int) {
	sink()
}

func feasible() {}

//go:noinline
func infeasible() {}

//go:noinline
func source() {
	freevar1 := 10 // Although it is a constant, it is passed via memory (captured by reference)
	freevar2 := 20 // Although it is a constant, it is passed via memory (captured by reference)
	f := func(x float32) {
		println(x)
		switch freevar1 * freevar2 { // 200
		case 0:
			// This is actually infeasible, but since SCCP does not model memory state, it is feasible.
			// infeasible()
			callSink(1)
		case 1:
			// This is actually infeasible, but since SCCP does not model memory state, it is feasible.
			// infeasible()
			callSink(1)
		default: //200
			// only feasible path
			feasible()
			callSink(1)
		}

		if freevar1+freevar2 > 0 {
			// only feasible path
			feasible()
			callSink(1)
		} else {
			// This is actually infeasible, but since SCCP does not model memory state, it is feasible.
			// infeasible()
			callSink(1)
		}

		switch freevar1 + int(x) + freevar2 { // 10 + 42 + 20 = 72
		case 0:
			// This is actually infeasible, but since SCCP does not model memory state, it is feasible.
			// infeasible()
			callSink(1)
		case 1:
			// This is actually infeasible, but since SCCP does not model memory state, it is feasible.
			// infeasible()
			callSink(1)
		case 72:
			// only feasible path
			feasible()
			callSink(1)
		default:
			// This is actually infeasible, but since SCCP does not model memory state, it is feasible.
			// infeasible()
			callSink(1)
		}
	}
	callee(f)
}

//go:noinline
func callee(f func(float32)) {
	f(42.0)
}

//go:noinline
func anotherIntFunc(i float32) {
	infeasible()
	println("anotherIntFunc")
}

type aType struct {
	field int
}

//go:noinline
func (a aType) anotherIntFunc(i float32) {
	infeasible()
	println("anotherIntFunc")
}

//go:noinline
func fakeAddressTaken() (func(float32), func(float32)) {
	return anotherIntFunc, aType{field: 42}.anotherIntFunc
}

func main() {
	_, _ = fakeAddressTaken()
	source()
}
