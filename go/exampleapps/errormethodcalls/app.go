package main

import (
	_ "context"
	_ "sync"
)

// OtherReturn is an interface.
type OtherReturn interface {
	Error() int
}

// OtherReturnImpl implements OtherReturn
type OtherReturnImpl struct {
}

// TakesArg is an interface that has a method Error(int) string
type TakesArg interface {
	Error(int) string
}

// TakesArgImpl implements TakesArg
type TakesArgImpl struct {
}

// NoReturn is an interface that has a method Error()
type NoReturn interface {
	Error()
}

// NoReturnImpl implements NoReturn
type NoReturnImpl struct {
}

// NameMismatch is an interface that has a method OtherError() string
type NameMismatch interface {
	OtherError() string
}

// NameMismatchImpl implements NameMismatch
type NameMismatchImpl struct {
}

// EType is an interface that has a method Error() string
type EType struct {
}

// OtherError implements NameMismatch.
func (e *NameMismatchImpl) OtherError() string {
	return ""
}

// NotETypeImpl implements NoReturn.
func (e *NoReturnImpl) Error() {
}

// OtherReturnImpl implements OtherReturn.
func (e *OtherReturnImpl) Error() int {
	return 0
}

// EType is an interface that has a method Error() string
func (e *EType) Error() string {
	return ""
}

func (e *TakesArgImpl) Error(_ int) string {
	return ""
}

func successError1(e error) {
	_ = e.Error()
}

func successError2(e EType) {
	_ = e.Error()
}

func successError3(e *EType) {
	_ = e.Error()
}

func failureError1(e TakesArg) {
	e.Error(1)
}

func failureError2(e TakesArgImpl) {
	e.Error(2)
}

func failureError3(e NoReturn) {
	e.Error()
}

func failureError4(e NoReturnImpl) {
	e.Error()
}

func failureError5(e OtherReturn) {
	e.Error()
}

func failureError6(e OtherReturnImpl) {
	e.Error()
}

func failureError7(e NameMismatch) {
	e.OtherError()
}

func failureError9(e NameMismatchImpl) {
	e.OtherError()
}

func main() {
}
