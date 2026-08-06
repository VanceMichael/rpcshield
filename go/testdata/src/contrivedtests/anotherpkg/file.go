package anotherpkg

import "errors"

type A struct{}

var cond bool

const IsInEmergencyAreaFailOpen = true

func OutsideRPC() (*A, error) {
	if cond {
		return nil, errors.New("error")
	}
	return &A{}, nil
}

func IsDependencyFailureError(err error) bool {
	return true
}
