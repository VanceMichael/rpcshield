package errorprop

import (
	"errors"

	"../anotherpkg"
)

var IsInEmergencyAreaFailOpen = true

func IsDependencyFailureError(err error) bool {
	logError(err)
	return true
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsDependencyFailureErrorSuppression1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if IsDependencyFailureError(err) {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testIsDependencyFailureErrorSuppression2() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if IsDependencyFailureError(err) {
			return nil, err
		}
		return &S{}, nil
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsDependencyFailureErrorSuppression3() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if !IsDependencyFailureError(err) {
			return nil, err
		}
		return &S{}, nil
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testIsDependencyFailureErrorSuppression4() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if !IsDependencyFailureError(err) {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsDependencyFailureErrorSuppression5() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if IsDependencyFailureError(err) {
			err = nil
		}
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testIsDependencyFailureErrorSuppression6() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond {
			return nil, err
		}
		if IsDependencyFailureError(err) {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsDependencyFailureErrorSuppression7() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if IsDependencyFailureError(err) {
			return &S{}, nil
		} else {
			err = errors.New("error")
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testIsDependencyFailureErrorSuppression8() (*S, error) {
	resp1, err1 := OutboundRPC()
	if err1 != nil {
		if IsDependencyFailureError(err1) {
			return &S{}, err1
		}
		return resp1, errors.New("error")
	}

	resp2, err2 := nonOutboundRetPtrError()
	if err2 != nil {
		if IsDependencyFailureError(err2) {
			return &S{}, nil
		}
		return nil, err2
	}
	return resp2, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsDependencyFailureErrorSuppression9() (*S, error) {
	resp1, err1 := nonOutboundRetPtrError()
	if err1 != nil {
		if IsDependencyFailureError(err1) {
			return &S{}, err1
		}
		return resp1, errors.New("error")
	}

	resp2, err2 := OutboundRPC()
	if err2 != nil {
		if IsDependencyFailureError(err2) {
			return &S{}, nil
		}
		return nil, err2
	}
	return resp2, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testIsDependencyFailureErrorSuppression10() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if IsDependencyFailureError(err) {
			logError(err)
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsDependencyFailureErrorSuppressionAnotherPkg() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if anotherpkg.IsDependencyFailureError(err) {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

type A struct {
	IsInEmergencyAreaFailOpen bool
	Type                      string
}

func (*A) IsDependencyFailureError(err error) bool {
	return true
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsDependencyFailureErrorSuppressionMethod() (*S, error) {
	resp, err := OutboundRPC()
	a := &A{}
	if err != nil {
		if a.IsDependencyFailureError(err) {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsInEmergencyAreaFailOpenSuppression1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if IsInEmergencyAreaFailOpen {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testIsInEmergencyAreaFailOpenSuppression2() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if IsInEmergencyAreaFailOpen {
			return nil, err
		}
		return &S{}, nil
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsInEmergencyAreaFailOpenSuppression3() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if !IsInEmergencyAreaFailOpen {
			return nil, err
		}
		return &S{}, nil
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testIsInEmergencyAreaFailOpenSuppression4() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if !IsInEmergencyAreaFailOpen {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsInEmergencyAreaFailOpenSuppression5() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if anotherpkg.IsInEmergencyAreaFailOpen {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testIsInEmergencyAreaFailOpenSuppression6() (*S, error) {
	resp, err := OutboundRPC()
	a := &A{IsInEmergencyAreaFailOpen: true}
	if err != nil {
		if a.IsInEmergencyAreaFailOpen {
			return &S{}, nil
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testEmptyTrueBranch1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if anotherpkg.IsInEmergencyAreaFailOpen {
			return &S{}, nil
		}
	}

	for _, a := range []*A{&A{Type: "abc"}, &A{Type: "xyz"}, nil} {
		if a != nil {
			if a.Type == "abc" {
				return &S{}, nil
			}
		} else {
			continue
		}
	}

	return resp, errors.New("error")
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testEmptyFalseBranch2() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if anotherpkg.IsInEmergencyAreaFailOpen {
			return &S{}, nil
		}
	}

	for _, a := range []*A{&A{Type: "abc"}, &A{Type: "xyz"}, nil} {
		if a == nil {
			continue
		} else {
			if a.Type == "abc" {
				return &S{}, nil
			}
		}
	}

	return resp, errors.New("error")
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testEmptyTrueAndFalseBranch1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if anotherpkg.IsInEmergencyAreaFailOpen {
			return &S{}, nil
		}
	}

	for _, a := range []*A{&A{Type: "abc"}, &A{Type: "xyz"}, nil} {
		if a != nil {
			// empty true branch
		} else {
			// empty false branch
		}
	}

	return resp, errors.New("error")
}
