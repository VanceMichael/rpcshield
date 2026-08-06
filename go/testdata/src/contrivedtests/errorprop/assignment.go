package errorprop

import (
	"errors"
)

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment1() (*S, error) {
	resp, err := OutboundRPC()
	if cond {
		err = errors.New("error")
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment2() (*S, error) {
	resp, err := OutboundRPC()
	if cond {
		err = nil
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment3() (*S, error) {
	resp, err := OutboundRPC()
	if cond {
		err = nil
	} else {
		err = errors.New("error")
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorAssignment4() (*S, error) {
	resp, err := OutboundRPC()
	err = nil
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment5() (*S, error) {
	resp, err := OutboundRPC()
	err = errors.New("error")
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment6() (*S, error) {
	resp, err := OutboundRPC()
	err = wrapError(err)
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorAssignment7() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		err = nil
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment8() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		err = nil
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment9(anotherCond bool) (*S, error) {
	resp, err := OutboundRPC()
	if cond && anotherCond {
		err = nil
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment10(anotherCond bool) (*S, error) {
	resp, err := OutboundRPC()
	if cond || anotherCond {
		err = nil
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment11() (*S, error) {
	resp, err := OutboundRPC()
	err2 := err
	return resp, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorAssignment12() (*S, error) {
	resp, err := OutboundRPC()
	err2 := err
	if err2 != nil {
		err2 = nil
	}
	return resp, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment13() (*S, error) {
	resp, err := OutboundRPC()
	err2 := err
	err3 := err2
	err4 := err3
	return resp, err4
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorAssignment14() (*S, error) {
	resp, err := OutboundRPC()
	err = retAnotherError()
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment15() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		err = errors.New("error")
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorAssignment16() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		err = retAnotherError()
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorAssignment17() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		err = retAnotherError()
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorAssignment18() (*S, error) {
	resp, err := OutboundRPC()
	err = errors.New("error")
	err = nil
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorAssignment19() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		err = errors.New("error")
	}
	err = nil
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAssignment20() (*S, error) {
	resp, err := nonOutboundRetPtrError()
	if err != nil {
		return nil, err
	}
	resp, err = OutboundRPC()
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAssignment21() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		resp, err = nonOutboundRetPtrError()
		return resp, err
	}
	return nil, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAssignment22() (*S, error) {
	resp, err := OutboundRPC()
	resp, err = nonOutboundRetPtrError()
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAssignment23() (*S, error) {
	resp, err := nonOutboundRetPtrError()
	if err != nil {
		return nil, err
	}
	resp, err = OutboundRPC()
	resp, err = nonOutboundRetPtrError()
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAssignment24() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err
	}
	resp, err = nonOutboundRetPtrError()
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNamedReturnAssignment1() (resp *S, err error) {
	resp, err = OutboundRPC()
	if cond {
		err = errors.New("error")
	}
	return
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNamedReturnAssignment2() (resp *S, err error) {
	resp, err = OutboundRPC()
	if err != nil {
		err = errors.New("error")
	}
	return
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testNamedReturnAssignment3() (resp *S, err error) {
	resp, err = OutboundRPC()
	if err != nil {
		err = nil
	}
	return
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNamedReturnAssignment4() (resp *S, err error) {
	resp, err = OutboundRPC()
	if err != nil {
		err = wrapError(err)
	} else {
		err = nil
	}
	return
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNamedReturnAssignment5() (resp *S, err error) {
	resp, err = OutboundRPC()
	if err != nil {
		err = wrapError(err)
	} else {
		err = errors.New("error")
	}
	return
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNamedReturnReassignments1() (s *S, err error) {
	err = retErr()
	if err != nil {
		return nil, err
	}

	result, err := OutboundRPC()
	if err != nil {
		return nil, err
	}
	_ = result

	return s, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testNamedReturnReassignments2() (s *S, err error) {
	var err2 error = nil

	result, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	_ = result

	return s, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testSharedErrorReassignments1() (*S, error) {
	var s *S
	var err error

	err = retErr()
	if err != nil {
		return nil, err
	}

	result, err := OutboundRPC()
	if err != nil {
		return nil, err
	}
	_ = result

	return s, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testSharedErrorReassignments2() (*S, error) {
	var s *S
	var err error
	var err2 error = nil

	result, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	_ = result

	return s, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherErrReturnWithAssignmentFromBranchVar1() (*S, error) {
	var err error
	resp := &S{}

	vresp, verr := OutboundRPC()
	if verr != nil || vresp == nil {
		err = verr
		return resp, err
	}
	resp = vresp

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherErrReturnWithAssignmentFromBranchVar2() (*S, error) {
	var err error
	resp := &S{}

	vresp, verr := OutboundRPC()
	if verr != nil || vresp == nil {
		err = verr
		err = nil
		return resp, err
	}
	resp = vresp

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherErrReturnWithAssignmentFromBranchVar3() (*S, error) {
	var err error
	resp := &S{}

	vresp, verr := OutboundRPC()
	if verr != nil || vresp == nil {
		if cond {
			err = verr
		} else {
			err = nil
		}
		return resp, err
	}
	resp = vresp

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherErrReturnWithAssignmentFromBranchVar4() (*S, error) {
	var err, err2, err3 error
	resp := &S{}

	_, verr := OutboundRPC()
	if verr != nil {
		err3 = verr
		err2 = err3
		err = err2
		return resp, err
	}

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorDefaultValuePassedViaAssignment1() (*S, error) {
	var err2 error
	resp, err := OutboundRPC()
	if err != nil {
		err = err2
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorDefaultValuePassedViaAssignment2() (*S, error) {
	var err2 error
	resp, err := OutboundRPC()
	if err != nil {
		err3 := err2
		err4 := err3
		err = err4
		return nil, err
	}
	return resp, nil
}
