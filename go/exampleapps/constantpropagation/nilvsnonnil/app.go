package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: comparing ConstantVal lattice with non-nil ConstantVal works and removes unreachable blocks.

//go:noinline
func sink() {
}

//go:noinline
func feasible() {}

//go:noinline
func callSink(v int) {
	sink()
}

//go:noinline
func infeasible() {}

//go:noinline
func source() {
	var p any = 42
	var q any = nil
	caller(p, q)
}

func caller(p, q any) {
	switch {
	case p == nil:
		infeasible()
		callSink(1)
	case nil == p:
		infeasible()
		callSink(1)
	case q != nil:
		infeasible()
		callSink(1)
	case nil != q:
		infeasible()
		callSink(1)
	}
}

func main() {
	source()
}
