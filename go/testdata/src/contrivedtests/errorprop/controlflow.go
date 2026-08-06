package errorprop

import (
	"errors"
	"fmt"

	"../anotherpkg"
)

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalVarReturn() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalFuncReturn() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, errors.New("error")
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncInvalidAllRetNilAndOutboundFound
// expect_status: FailOpen
func testAlwaysFailOpen() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, nil
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAlwaysFailOpen3() (*S, error) {
	var err2 error
	_, err := OutboundRPC()
	logError(err)
	return nil, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAlwaysFailOpen4() (*S, error) {
	err2 := errors.New("some other error")
	_, err := OutboundRPC()
	logError(err)
	return nil, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testUnconditionalFuncReturn() (*S, error) {
	resp, err := OutboundRPC()
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn1() (*S, error) {
	resp, err := OutboundRPC()
	return resp, fmt.Errorf("error %w", err)
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn2() (*S, error) {
	resp, err := OutboundRPC()
	return resp, wrapError(err)
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn3(err error) (*S, error) {
	resp, err := OutboundRPC()
	return resp, errors.Join(err, wrapError(err))
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn4(err error) (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, wrapErrorManyArgsOneRet("ctx", "msg", err, 1)
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn5(err error) (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		_, err, _ = wrapErrorOneArgsManyRet(err)
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn6(err error) (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		_, err, _ = wrapErrorManyArgsManyRet("ctx", "msg", err, 1)
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn7(err error) (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, wrapError(wrapError(wrapError(fmt.Errorf("error %w", err))))
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn8(err error) (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, wrapErrorManyArgsOneRet("ctx", "msg", wrapError(err), 1)
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn9(err error) (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, wrapError(wrapError(wrapError(errors.New("new error"))))
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testWrapperErrorReturn10(err error) (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, wrapError(wrapError(wrapError(err)))
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncInvalidAllRetNilAndOutboundFound
// expect_status: FailOpen
func testSuppressedErrorReturn() (*S, error) {
	resp, _ := OutboundRPC()
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncInvalidOtherReason
// expect_status: FailOpen
func testSuppressedNoReturn() {
	_, _ = OutboundRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testFuncCallReturn() (*S, error) {
	return OutboundRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testLogError1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		logError(err)
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testLogError2() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		logError(err)
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testLogError3() (*S, error) {
	resp, err := OutboundRPC()
	logError(err)
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testLogError4() (*S, error) {
	resp, err := OutboundRPC()
	logError(err)
	return resp, errors.New("some other error")
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testLogError5() (*S, error) {
	resp, err := OutboundRPC()
	logError(err)
	return resp, retAnotherError()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testLogError6() (*S, error) {
	resp, err := OutboundRPC()
	logError(err)
	return resp, &myErr{}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testLogError7() (*S, error) {
	resp, err := OutboundRPC()
	logError(err)
	return resp, myErr{}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailUnknownIntended
func testRetAnotherFunc() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nonOutboundRetPtrError()
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testRetAnotherFunc2() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		return nonOutboundRetPtrError()
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testRetAnotherFunc3() (*S, error) {
	_, err := OutboundRPC()
	logError(err)
	return nonOutboundRetPtrError()
}

// outbound_callsite: OutsideRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherPkgRPC() (any, error) {
	resp, err := anotherpkg.OutsideRPC()
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testComplexControlFlow1(a []*string) (*S, error) {
	if a == nil || len(a) == 0 {
		return nil, errors.New("error")
	}

	if cond && len(a) != 2 {
		return nil, fmt.Errorf("error")
	}

	resp, err := OutboundRPC()

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testComplexControlFlow2(a []*string) (*S, error) {
	if a == nil || len(a) == 0 {
		return nil, errors.New("error")
	}

	if cond && len(a) != 2 {
		return nil, fmt.Errorf("error")
	}

	resp, _ := OutboundRPC()
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testComplexControlFlow3() (*S, error) {
	if !cond {
		return nil, nil
	}
	resp, err := OutboundRPC()
	if err != nil || cond {
		return &S{}, err
	}
	print(resp)

	resp2, err2 := nonOutboundRetPtrError()
	if err2 != nil {
		return &S{}, err2
	}

	return resp2, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testComplexControlFlow4() (*S, error) {
	if !cond {
		return nil, nil
	}
	resp, err := OutboundRPC()
	if err != nil || cond {
		return &S{}, nil
	}
	print(resp)

	resp2, err2 := nonOutboundRetPtrError()
	if err2 != nil {
		return &S{}, err2
	}

	return resp2, nil
}

// outbound_callsite: OutboundRPC
// outbound_callsite_pos: 1
// expect_func_validity: FuncValid
// expect_status: FailClose
func testMultipleCalls1() (*S, error) {
	var resp *S
	var err error

	// First call: FailClose
	resp, err = OutboundRPC()
	if err != nil {
		return resp, err
	}

	// Second call: FailOpen
	resp, err = OutboundRPC()
	if err != nil {
		return nil, nil
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// outbound_callsite_pos: 2
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testMultipleCalls2() (*S, error) {
	var resp *S
	var err error

	// First call: FailClose
	resp, err = OutboundRPC()
	if err != nil {
		return resp, err
	}

	// Second call: FailOpen
	resp, err = OutboundRPC()
	if err != nil {
		return nil, nil
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorEmbeddingInterface1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, NewInternalErrorWrapper(err)
	}
	return resp, nil
}

// outbound_callsite: OutboundRPCRetEmbeddedErr
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorEmbeddingInterface2() (*S, error) {
	resp, err := OutboundRPCRetEmbeddedErr()
	if err != nil {
		return nil, errors.New("error")
	}
	return resp, nil
}

// outbound_callsite: OutboundRPCRetEmbeddedErr
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorEmbeddingInterface3() (*S, ErrorWrapper) {
	resp, err := OutboundRPCRetEmbeddedErr()
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPCRetEmbeddedErr
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorEmbeddingInterface4() (*S, ErrorWrapper) {
	resp, err := OutboundRPCRetEmbeddedErr()
	if err != nil {
		return nil, nil
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testBlockNilabilityDirectReturn1() (*S, error) {
	resp, err := nonOutboundRetPtrError()
	if err != nil {
		return resp, err
	}
	return OutboundRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testBlockNilabilityDirectReturn2() (*S, error) {
	resp, err := nonOutboundRetPtrError()
	if err != nil {
		return resp, err
	}
	logError(err)
	return OutboundRPC()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testBlockNilabilityDirectReturn3() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return resp, err
	}
	logError(err)
	return nonOutboundRetPtrError()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testBlockNilabilityDirectReturn4() (*S, error) {
	_, err := OutboundRPC()
	if err != nil {
		logError(err)
	}
	return nonOutboundRetPtrError()
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testComplexControlFlowForMerging() (S, error) {
	_, err := OutboundRPC()
	if err != nil {
		if cond {
			stickyErr := err
			if stickyErr != nil || cond {
				subError := wrapError(stickyErr)
				err = wrapError(subError)
			}
		}
		if cond {
			return S{}, fmt.Errorf("error %w", wrapError(err))
		}
		return S{}, wrapError(err)
	}
	return S{}, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherRetVar1() (*S, error) {
	err2 := errors.New("some other error")
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherRetVar2() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		err2 := errors.New("some other error")
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherRetVar4() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		err2 := wrapError(err)
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherRetVar5() (*S, error) {
	resp, err := OutboundRPC()
	err2 := wrapError(err)
	if err != nil {
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherRetVar6() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		err2 := retAnotherError()
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherRetVar7() (*S, error) {
	err2 := retAnotherError()
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherRetVar8() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		_, err2 := nonOutboundRetPtrError()
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherRetVar9() (*S, error) {
	_, err2 := nonOutboundRetPtrError()
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherRetVar10() (*S, error) {
	s, err2, i := "abc", retAnotherError(), 2
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	_, _ = s, i
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherRetVar11() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		s, i, err2 := "abc", 2, retAnotherError()
		_, _ = s, i
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherRetVar12() (*S, error) {
	var err2 error = nil
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testAnotherRetVar13() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		var err2 error = nil
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherRetVar14() (*S, error) {
	var err2 error = errors.New("some other error")
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testAnotherRetVar15() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		var err2 error = errors.New("some other error")
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNamedReturn1() (resp *S, err error) {
	resp, err = OutboundRPC()
	return
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNamedReturn2() (resp *S, err error) {
	resp, err = OutboundRPC()
	if err != nil {
		return
	}
	return
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testNamedReturn3() (resp *S, err error) {
	resp, err = OutboundRPC()
	if err == nil {
		return
	}
	return &S{}, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNamedReturn4() (resp *S, err error) {
	if !cond {
		return nil, nil
	}
	resp, err = OutboundRPC()
	if err != nil || cond {
		return
	}
	print(resp)

	resp2, err2 := nonOutboundRetPtrError()
	if err2 != nil {
		return &S{}, err2
	}

	return resp2, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructPointer1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, &InternalErrorWrapper{
			sourceErr: err,
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructPointer2() (*S, error) {
	err2 := errors.New("some other error")

	resp, err := OutboundRPC()
	if err != nil {
		return nil, &InternalErrorWrapper{
			sourceErr: err2,
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructPointer3() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, &InternalErrorWrapper{
			sourceErr: errors.New("some other error"),
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructPointer4() (*S, error) {
	_, err := OutboundRPC()
	return nil, &InternalErrorWrapper{
		sourceErr: err,
	}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructPointer5() (*S, error) {
	err2 := errors.New("some other error")

	_, err := OutboundRPC()
	logError(err)
	return nil, &InternalErrorWrapper{
		sourceErr: err2,
	}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructPointer6() (*S, error) {
	_, err := OutboundRPC()
	logError(err)
	return nil, &InternalErrorWrapper{
		sourceErr: errors.New("some other error"),
	}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructPointer7() (*S, error) {
	_, err := OutboundRPC()
	logError(err)

	_, err2 := nonOutboundRetPtrError()

	return nil, &InternalErrorWrapper{
		sourceErr: err2,
	}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructPointer8() (*S, error) {
	_, err := OutboundRPC()
	return nil, &InternalErrorWrapper{err}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructPointer9() (*S, error) {
	_, err := OutboundRPC()
	return nil, &InternalErrorWrapper{
		sourceErr: wrapError(err),
	}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructPointer10() (*S, error) {
	_, err := OutboundRPC()
	return nil, &InternalErrorWrapper{
		wrapError(err),
	}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructPointer11() (*S, error) {
	_, err := OutboundRPC()
	logError(err)

	_, err2 := nonOutboundRetPtrError()

	return nil, &InternalErrorWrapper{err2}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructPointer12() (*S, error) {
	_, err := OutboundRPC()
	logError(err)

	_, err2 := nonOutboundRetPtrError()

	return nil, &InternalErrorWrapper{wrapError(err2)}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructPointer13() (*S, error) {
	_, err := OutboundRPC()
	logError(err)
	return nil, &InternalErrorWrapper{errors.New("some other error")}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructPointer14() (*S, error) {
	_, err := OutboundRPC()
	_, err = nonOutboundRetPtrError()
	return nil, &InternalErrorWrapper{err}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructPointer15() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		return nil, &InternalErrorWrapper{
			sourceErr: err,
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructByValue1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, InternalErrorWrapper2{
			sourceErr: err,
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testErrorImplementingStructByValue2() (*S, error) {
	_, err := OutboundRPC()
	return nil, InternalErrorWrapper2{
		sourceErr: err,
	}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorImplementingStructByValue3() (*S, error) {
	err2 := errors.New("some other error")

	_, err := OutboundRPC()
	logError(err)
	return nil, InternalErrorWrapper2{
		sourceErr: err2,
	}
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testErrorDefaultValue() (*S, error) {
	var err2 error
	resp, err := OutboundRPC()
	if err != nil {
		return nil, err2
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalReturnFromLoop() (map[string]*S, error) {
	mp := make(map[string]*S)
	for i := 0; i < 10; i++ {
		if _, ok := mp["abc"]; !ok {
			resp, err := OutboundRPC()
			if err != nil {
				return nil, err
			}
			mp["abc"] = resp
		}
	}
	return mp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNewError1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, NewInternalError("some other error")
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNewError2() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, NewInternalError(err.Error())
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testNewError3() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		return nil, NewError("some other error")
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testNewError4() (*S, error) {
	resp, _ := OutboundRPC()
	_, err2 := nonOutboundRetPtrError()
	if err2 != nil {
		return nil, NewInternalError("some other error")
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testNewError5() (*S, error) {
	resp, err := OutboundRPC()
	logError(err)
	return resp, NewInternalError("some other error")
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNewError6() (*S, error) {
	resp, err := OutboundRPC()
	return resp, NewError(err.Error())
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testNewError7() (*S, error) {
	resp, err := OutboundRPC()
	return resp, wrapError(wrapError(NewError(err.Error())))
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if _, ok := err.(*InternalErrorWrapper); ok {
			return nil, errors.New("recreated error")
		}
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation2() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond {
			return nil, errors.New("error1")
		} else {
			return nil, errors.New("error2")
		}
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testConditionalNilabilityPropagation3() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return nil, nil
	}
	if cond {
		return nil, errors.New("error")
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testConditionalNilabilityPropagation4() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		if cond {
			return nil, errors.New("error")
		}
		return nil, err
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testConditionalNilabilityPropagation5() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond {
			err = retAnotherError()
			return nil, err
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation6() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond && cond2 && cond3 {
			return nil, err
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testConditionalNilabilityPropagation7() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond {
			if cond2 {
				err = retAnotherError()
				if cond3 {
					return nil, err
				}
			}
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation8() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond {
			err = wrapError(err)
			if cond2 {
				if cond3 {
					return nil, err
				}
			}
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation9() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond {
			return nil, nil
		}
		if cond2 {
			return nil, wrapError(err)
		}
		if cond3 {
			return nil, nil
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testConditionalNilabilityPropagation10() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond {
			return nil, nil
		}
		if cond2 {
			resp, err = nonOutboundRetPtrError()
			if cond3 {
				return nil, wrapError(err)
			} else {
				return nil, err
			}
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation11() (*S, error) {
	resp, err := OutboundRPC()
	if cond {
		if err != nil {
			return nil, errors.New("error")
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation12() (*S, error) {
	resp, err := OutboundRPC()
	if cond {
		if err != nil {
			if cond2 {
				if cond3 {
					return nil, errors.New("error")
				}
			}
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation13() (*S, error) {
	resp, err := OutboundRPC()
	if cond {
		if cond2 {
			if err != nil {
				if cond2 && cond3 {
					return nil, errors.New("error")
				}
			}
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testConditionalNilabilityPropagation14() (*S, error) {
	resp, err := OutboundRPC()
	if cond && cond2 || cond3 {
		if err != nil {
			if cond {
				return nil, nil
			}
			if cond2 {
				resp, err = nonOutboundRetPtrError()
				if cond3 {
					if err != nil {
						return nil, wrapError(err)
					}
				} else {
					return nil, err
				}
			}
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testConditionalNilabilityPropagation15() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if err == nil {
			if cond {
				return nil, errors.New("recreated error")
			}
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testConditionalNilabilityPropagation16() (*S, error) {
	resp, err := OutboundRPC()
	for i := 0; i < 10; i++ {
		if err != nil {
			for j := 0; j < 10; j++ {
				if cond {
					return nil, errors.New("recreated error")
				}
			}
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testTypeSwitch1() (*S, error) {
	resp, err := OutboundRPC()
	switch err.(type) {
	case *InternalErrorWrapper:
		return resp, errors.New("recreated error")
	case *InternalErrorWrapper2:
		return nil, nil
	default:
		return nil, nil
	}

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testTypeSwitch2() (*S, error) {
	resp, err := OutboundRPC()
	switch err.(type) {
	case *InternalErrorWrapper:
		return nil, nil
	default:
		return nil, err
	}

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testTypeSwitch3() (*S, error) {
	resp, err := OutboundRPC()
	switch err.(type) {
	case *InternalErrorWrapper:
		return nil, nil
	default:
		return nil, fmt.Errorf("some error")
	}

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testTypeSwitch4() (*S, error) {
	resp, err := OutboundRPC()
	switch err.(type) {
	case *InternalErrorWrapper:
		return nil, wrapError(err)
	default:
		return nil, nil
	}

	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testTypeSwitch5() (*S, error) {
	resp, err := OutboundRPC()
	switch err.(type) {
	case *InternalErrorWrapper:
		return nil, nil
	case nil:
		return resp, nil
	}

	err2 := retAnotherError()
	return resp, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testTypeSwitch6() (*S, error) {
	resp, err := OutboundRPC()
	switch err.(type) {
	case *InternalErrorWrapper:
		return nil, errors.New("recreated error")
	case nil:
		return resp, nil
	}

	err2 := retAnotherError()
	return resp, err2
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalErr1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return resp, GlobalErrTrusted
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalErr2() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		return resp, nil
	}
	return resp, GlobalErrNewErrorPattern
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testGlobalErr3() (*S, error) {
	resp, err := OutboundRPC()
	if err == nil {
		return resp, err
	}
	return resp, _globalErrNil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalMultiDecl1() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return resp, GlobalErrMultiDecl1
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testGlobalUninitialized() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return resp, GlobalErrUninitialized
	}
	return resp, err
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalWrapped() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return resp, GlobalErrWrapped
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalInterface() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return resp, GlobalErrInterface
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalSwitch() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		switch cond {
		case true:
			return resp, GlobalErrTrusted
		default:
			return resp, GlobalErrNewErrorPattern
		}
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalNested() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if cond {
			if cond2 {
				return resp, GlobalErrTrusted
			}
			return resp, GlobalErrNewErrorPattern
		}
		return resp, _globalErrNil
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalTypeAssertion() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		if _, ok := GlobalErrInterface.(ErrorWrapper); ok {
			return resp, GlobalErrInterface
		}
		return resp, GlobalErrTrusted
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalWithLocal() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		localErr := errors.New("local")
		if cond {
			return resp, GlobalErrTrusted
		}
		return resp, localErr
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailClose
func testGlobalChain() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		return resp, wrapError(GlobalErrTrusted)
	}
	return resp, nil
}

// outbound_callsite: OutboundRPC
// expect_func_validity: FuncValid
// expect_status: FailOpen
func testGlobalReassign() (*S, error) {
	resp, err := OutboundRPC()
	if err != nil {
		localRef := _globalErrNil
		return resp, localRef
	}
	return resp, nil
}
