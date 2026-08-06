package main

import (
	"context"
)

type holdsContext struct {
	x   int
	ctx context.Context
	y   float32
}

type holdsError struct {
	x   int
	ctx context.Context
	err error
	y   float32
}

type holdsContextAndError struct {
	x   int
	ctx context.Context
	err error
	y   float32
}

func foo() error {
	return nil
}

type dummy1 struct{}

type dummy2 struct {
	x int
	y float32
}

func (d *dummy1) noCtxt(_ int) error {
	return nil
}

func (d *dummy1) missingSecondArg(_ context.Context) error {
	return nil
}

func (d *dummy1) missingErrRet(_ context.Context, _ int) string {
	return ""
}

func (d *dummy1) structWithoutCtxt(_ dummy2) error {
	return nil
}

func (d *dummy1) myRPC(_ context.Context, _ int) error {
	return nil
}

func (d *dummy1) myRPC2(_ *context.Context, _ int) error {
	return nil
}

func (d *dummy1) structWithCtxt1(_ int, _ holdsContext) error {
	return nil
}

func (d *dummy1) structWithCtxt2(_ int, _ *holdsContext) error {
	return nil
}

func (d *dummy1) retWithCtxt1(_ context.Context, _ int) holdsError {
	return holdsError{}
}

func (d *dummy1) retWithCtxt2(_ context.Context, _ int) *holdsError {
	return nil
}

func (h *holdsContext) ctxtInRecv(_ int) error {
	return nil
}

func (h *holdsContext) ctxtInRecvButNoErrRet(_ int) {
}

func (h *holdsError) errInRecv(c context.Context, _ int) {
}

func (h *holdsContextAndError) bothInRecv(_ int) {
}

// bothInRecvNoArgs captures both context and error via receiver with no method args.
func (h *holdsContextAndError) bothInRecvNoArgs() {
}

// func failureNoInvoke() {
// 	_ = foo()
// }

func failureNoContext() {
	var d dummy1
	_ = d.noCtxt(1)
}

func failureContextButNoOtherArg(c context.Context) {
	var d dummy1
	_ = d.missingSecondArg(c)
}

func failureNoErrorRet(c context.Context) {
	var d dummy1
	d.missingErrRet(c, 1)
}

func failureStructWithoutCtxt() {
	var d dummy1
	_ = d.structWithoutCtxt(dummy2{})
}

func successStructWithCtxt() {
	var d dummy1
	_ = d.structWithCtxt1(1, holdsContext{})
}

func successStructWithPtrCtxt() {
	var d dummy1
	_ = d.structWithCtxt2(1, &holdsContext{})
}

func successRetWithCtxt1(c context.Context) {
	var d dummy1
	_ = d.retWithCtxt1(c, 1)
}

func successRetWithCtxt2(c context.Context) {
	var d dummy1
	_ = d.retWithCtxt2(c, 1)
}

func successViaReceiver() {
	var h holdsContext
	_ = h.ctxtInRecv(1)
}

func failureCtxtViaReceiverButNoRet() {
	var h holdsContext
	h.ctxtInRecvButNoErrRet(1)
}

func successRetViaReceiver(c context.Context) {
	var h holdsError
	h.errInRecv(c, 1)
}

func successBothViaReceiver() {
	var h holdsContextAndError
	h.bothInRecv(1)
}

func successPassViaFreeVar() {
	var ctxt context.Context
	var err error
	f := func() {
		println(ctxt, err)
	}
	f()
}

func successPassViaFreeVarStruct() {
	type tmytype struct {
		ctxt context.Context
		err  error
	}
	var t tmytype
	f := func() {
		println(t.ctxt, t.err)
	}
	f()
}

// // Interface used to invoke method through interface call.
type bothInRecvNoArgsIFace interface {
	bothInRecvNoArgs()
}

// successBothViaReceiverNoArgsDirect: direct method call, no args; receiver carries context and error.
func successBothViaReceiverNoArgsDirect() {
	var h holdsContextAndError
	h.bothInRecvNoArgs()
}

// successBothViaReceiverNoArgsInterface: interface invoke call, no args; receiver carries context and error.
func successBothViaReceiverNoArgsInterface() {
	var h holdsContextAndError
	var i bothInRecvNoArgsIFace = &h
	i.bothInRecvNoArgs()
}

func direct1(_ context.Context, _ int) error {
	return nil
}

func direct2(_ holdsContext) holdsError {
	return holdsError{}
}

func direct3(_ *holdsContext) *holdsError {
	return nil
}

func successDirectFunctionCall(c context.Context) {
	_ = direct1(c, 1)
}

func successDirectFunctionCallStruct(c context.Context) {
	_ = direct2(holdsContext{})
}

func successDirectFunctionCallStructPtr(c context.Context) {
	_ = direct3(&holdsContext{})
}

func main() {
}
