package errorprop

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnonymousFunc1() (*S, error) {
	// Anonymous function: FailClose, Caller: FailClose
	callRPC := func() (*S, error) {
		resp, err := OutboundRPC()
		if err != nil {
			return nil, err
		}
		return resp, err
	}
	resp2, err2 := callRPC()
	if err2 != nil {
		return nil, err2
	}
	return resp2, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnonymousFunc2() (*S, error) {
	// Anonymous function: FailClose, Caller: FailClose
	callRPC := func() (*S, error) {
		resp, err := OutboundRPC()
		if err != nil {
			return nil, err
		}
		return resp, err
	}
	return callRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnonymousFunc3() (*S, error) {
	// Anonymous function: FailClose, Caller: FailClose
	callRPC := func() (*S, error) {
		return OutboundRPC()
	}
	return callRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnonymousFunc4() (*S, error) {
	// Anonymous function: FailClose, Caller: FailClose
	callRPC := func() (*S, error) {
		return OutboundRPC()
	}
	resp2, err2 := callRPC()
	if err2 != nil {
		return nil, err2
	}
	return resp2, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnonymousFunc5() (*S, error) {
	// Anonymous function: FailClose, Caller: FailOpen
	callRPC := func() (*S, error) {
		resp, err := OutboundRPC()
		if err != nil {
			return nil, err
		}
		return resp, err
	}
	resp2, err2 := callRPC()
	if err2 != nil {
		err2 = nil
	}
	return resp2, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnonymousFunc6() (*S, error) {
	// Anonymous function: FailClose, Caller: FailOpen
	callRPC := func() (*S, error) {
		resp, err := OutboundRPC()
		if err != nil {
			return nil, err
		}
		return resp, err
	}
	resp2, err2 := callRPC()
	if err2 != nil {
		return nil, nil
	}
	return resp2, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnonymousFunc7() (*S, error) {
	// Anonymous function: FailClose, Caller: FailOpen
	callRPC := func() (*S, error) {
		return OutboundRPC()
	}
	resp2, err2 := callRPC()
	if err2 != nil {
		return nil, nil
	}
	return resp2, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnonymousFunc8() (*S, error) {
	// Anonymous function: FailClose, Caller: FailOpen
	callRPC := func() (*S, error) {
		return OutboundRPC()
	}
	resp2, err2 := callRPC()
	if err2 != nil {
		err2 = nil
	}
	return resp2, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnonymousFunc9() (*S, error) {
	// Anonymous function: FailOpen => Caller: FailOpen
	callRPC := func() (*S, error) {
		resp, err := OutboundRPC()
		if err != nil {
			return nil, nil
		}
		return resp, err
	}
	resp2, err2 := callRPC()
	if err2 != nil {
		return nil, err2
	}
	return resp2, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnonymousFunc10() (*S, error) {
	// Anonymous function: FailOpen => Caller: FailOpen
	callRPC := func() (*S, error) {
		resp, err := OutboundRPC()
		if err != nil {
			err = nil
		}
		return resp, err
	}
	return callRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnonymousFunc11() (*S, error) {
	// Anonymous function: FailOpen => Caller: FailOpen
	callRPC := func() (*S, error) {
		resp, _ := OutboundRPC()
		return resp, nil
	}
	return callRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnonymousFunc12() (*S, error) {
	// Anonymous function: FailOpen => Caller: FailOpen
	callRPC := func() (*S, error) {
		resp, _ := OutboundRPC()
		return resp, nil
	}
	resp2, err2 := callRPC()
	if err2 != nil {
		return nil, err2
	}
	return resp2, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testChainedAnonymousFunc1() (*S, error) {
	call2 := func() (*S, error) {
		return OutboundRPC()
	}
	call1 := func() (*S, error) {
		return call2()
	}
	return call1()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testChainedAnonymousFunc2() (*S, error) {
	call2 := func() (*S, error) {
		resp, err := OutboundRPC()
		if err != nil {
			return nil, nil
		}
		return resp, nil
	}
	call1 := func() (*S, error) {
		return call2()
	}
	return call1()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testChainedAnonymousFunc3() (*S, error) {
	call2 := func() (*S, error) {
		return OutboundRPC()
	}
	call1 := func() (*S, error) {
		return call2()
	}
	unrelatedFunc := func() (*S, error) {
		return &S{}, nil
	}

	if cond {
		return call1()
	}
	return unrelatedFunc()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNestedAnonymousFunc1() (*S, error) {
	callRPC := func() (*S, error) {
		innerCall := func() (*S, error) {
			return OutboundRPC()
		}
		return innerCall()
	}

	return callRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testNestedAnonymousFunc2() (*S, error) {
	callRPC := func() (*S, error) {
		innerCall := func() (*S, error) {
			return OutboundRPC()
		}
		resp, err := innerCall()
		if err != nil {
			return nil, nil
		}
		return resp, nil
	}

	return callRPC()
}
