package predefined

import (
	"fmt"
	"regexp"

	"github.com/uber-research/rpcshield/go/constant"
	"golang.org/x/tools/go/ssa"
)

var failCloseByFuctionNameRegex = map[*regexp.Regexp]struct{}{
	regexp.MustCompile("^.*\\$bound$"): {},
	regexp.MustCompile("^.*\\$thunk$"): {},
}

// AnalyzeCall analyzes a call instruction in a function and returns the failness of the function.
// It uses the function name and its synthetic flag to determine the failness.
// It returns constant.FailUnknown if the function is not a fail-close or fail-open function.
// It returns an error if the function is nil.
func AnalyzeCall(f *ssa.Function, call ssa.CallInstruction) (constant.Failness, error) {
	if f == nil {
		return constant.FailUnknown, fmt.Errorf("nil function")
	}

	// Any synthetic function is considered to be a fail-close function.
	if f.Synthetic != "" {
		return constant.FailClose, nil
	}

	// Check if the function is a fail-close function based on its name.
	if v := failNessByFunctionName(f.Name()); v != constant.FailUnknown {
		return v, nil
	}

	return constant.FailUnknown, nil
}

// failNessByFunctionName returns the failness of a function based on its name.
func failNessByFunctionName(name string) constant.Failness {
	// Check if it is a fail-close function based on its name.
	for r := range failCloseByFuctionNameRegex {
		if r.MatchString(name) {
			return constant.FailClose
		}
	}
	// TODO: Check if it is a fail-open function based on its name.
	// TODO: Add more rules here.
	return constant.FailUnknown
}
