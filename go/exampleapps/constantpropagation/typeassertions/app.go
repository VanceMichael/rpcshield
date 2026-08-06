package main

// In this test app, if a path from source to sink has an infeasible() call in the same basic block as the call site, the path should be infeasible.

// Test objective: type assertions on known concrete/interface values across funcs, maps, slices, chans; extract the correct value and type from their interfaces.

//go:noinline
func sink() {
}

//go:noinline
func infeasible() {}

//go:noinline
func feasible() {}

//go:noinline
func callSink(v int) {
	sink()
}

//go:noinline
func aFunction(f float32) string {
	callSink(1)
	return "a string"
}

type iface1 interface {
	A() string
}

type iface2 interface {
	B() string
}

type myType struct {
	field int
}

func (m myType) A() string {
	return "a string"
}

func (m myType) B() string {
	return "b string"
}

//go:noinline
func source() {
	var fn any = aFunction
	var mAp any = map[string]string{"a": "b"}
	var i any = 42
	var t any = &myType{field: 42}
	var sSlice any = make([]string, 0)
	var arr any = []string{"a", "b"}
	var cHan any = make(chan string)

	caller(fn, mAp, i, t, sSlice, cHan, arr)
}

func caller(fnIn, mApIn, iIn, tIn, sliceIn, chanIn, arrIn any) {

	// Comma OK idiom
	fn, ok := fnIn.(func(float32) string)
	if !ok {
		infeasible()
		callSink(1)
	}
	if fn == nil {
		infeasible()
		callSink(1)
	}

	mAp, ok := mApIn.(map[string]string)
	if !ok {
		infeasible()
		callSink(1)
	}
	if mAp == nil {
		infeasible()
		callSink(1)
	}
	i, ok := iIn.(int)
	if !ok {
		infeasible()
		callSink(1)
	}
	if i == 0 {
		infeasible()
		callSink(1)
	}
	t, ok := tIn.(*myType)
	if !ok {
		infeasible()
		callSink(1)
	}
	if t == nil {
		infeasible()
		callSink(1)
	}

	myIface2, ok := tIn.(iface2)
	if !ok {
		infeasible()
		callSink(1)
	}
	if myIface2 == nil {
		infeasible()
		callSink(1)
	}

	myIface1, ok := tIn.(iface1)
	if !ok {
		infeasible()
		callSink(1)
	}
	if myIface1 == nil {
		infeasible()
		sink()
	}

	sLice, ok := sliceIn.([]string)
	if !ok {
		infeasible()
		callSink(1)
	}
	if sLice == nil {
		infeasible()
		callSink(1)
	}
	cHan, ok := chanIn.(chan string)
	if !ok {
		infeasible()
		callSink(1)
	}
	if cHan == nil {
		infeasible()
		callSink(1)
	}

	arr, ok := arrIn.([]string)
	if !ok {
		infeasible()
		callSink(1)
	}
	if arr == nil {
		infeasible()
		callSink(1)
	}

	// No comma OK idiom
	fn = fnIn.(func(float32) string)
	if fn == nil {
		infeasible()
		callSink(1)
	}
	mAp = mApIn.(map[string]string)
	if mAp == nil {
		infeasible()
		callSink(1)
	}
	i = iIn.(int)
	if i == 0 {
		infeasible()
		callSink(1)
	}
	t = tIn.(*myType)
	if t == nil {
		infeasible()
		callSink(1)
	}
	myIface2 = tIn.(iface2)
	if myIface2 == nil {
		infeasible()
		callSink(1)
	}
	myIface1 = tIn.(iface1)
	if myIface1 == nil {
		infeasible()
		callSink(1)
	}
	sLice = sliceIn.([]string)
	if sLice == nil {
		infeasible()
		callSink(1)
	}
	cHan = chanIn.(chan string)
	if cHan == nil {
		infeasible()
		callSink(1)
	}
	arr = arrIn.([]string)
	if arr == nil {
		infeasible()
		callSink(1)
	}
}

// All type assertions succeed, so none of the infeasible() paths are taken
func main() {
	source()
}
