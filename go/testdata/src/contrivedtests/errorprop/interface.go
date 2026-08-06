package errorprop

// outbound_callsite: Foo
// expect_func_validity: FuncValid
// expect_status: FailClose
func testInterfaceCall1() (*S, error) {
	anon1 := func(x I) (*S, error) {
		resp, err := x.Foo()
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	return anon1(&S{})
}

// outbound_callsite: Foo
// expect_func_validity: FuncValid
// expect_status: FailClose
func testInterfaceCall2() (*S, error) {
	anon1 := func(x I) (*S, error) {
		resp, err := x.Foo()
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	resp, err := anon1(&S{})
	return resp, err
}

// outbound_callsite: Foo
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testInterfaceCall3() (*S, error) {
	anon1 := func(x I) (*S, error) {
		resp, err := x.Foo()
		if err != nil {
			return nil, nil
		}
		return resp, nil
	}
	return anon1(&S{})
}

// outbound_callsite: Foo
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testInterfaceCall4() (*S, error) {
	anon1 := func(x I) (*S, error) {
		resp, err := x.Foo()
		if err != nil {
			return nil, nil
		}
		return resp, nil
	}
	resp, err := anon1(&S{})
	return resp, err
}
