package main

// This app extends decoratortopval with the one structural element that fixture omitted:
// the real production pattern (campaign.controller.ValidateAdGroups in ads-campaign-manager)
// does not call the generic decorator with a single, statically-known callback -- it ranges
// over a slice of *several different* validator closures and calls the decorator once per
// element:
//
//	for _, validator := range validators {
//	    decorator.MetricDecorator(ctx, "ad_group", req, validator, ns)
//	}
//
// Every call in this loop is real, always-reached (barring an earlier validator's error) --
// there is no infeasible() call in this program. The companion test checks whether reaching
// Validator2's sink through the *loop variable* survives the analyzer's checkPathFeasibility,
// as it did (feasible) when Decorator was called with a single, direct callback reference in
// decoratortopval.

//go:noinline
func sink() {}

//go:noinline
func callSink() {
	sink()
}

type Gate interface {
	Allow() bool
}

type opaqueGate struct{}

//go:noinline
func (opaqueGate) Allow() bool {
	return true
}

type Req struct {
	Internal bool
}

//go:noinline
func Decorator[T any](gate Gate, input T, next func(T) error) error {
	if !gate.Allow() {
		return nil
	}
	return next(input)
}

// Validator1 is a sibling validator that never reaches the sink -- analogous to the other 52
// validators in ValidateAdGroups' slice that aren't ValidateTargetingCollection.
//
//go:noinline
func Validator1(req Req) error {
	return nil
}

// Validator2 is analogous to ValidateTargetingCollection: reachable only through the loop's
// second element.
//
//go:noinline
func Validator2(req Req) error {
	return validateInner(req)
}

//go:noinline
func validateInner(req Req) error {
	if !req.Internal {
		return nil
	}
	callSink()
	return nil
}

type Outer struct{}

//go:noinline
func (Outer) IsInternal() bool {
	return true
}

//go:noinline
func buildRequest(o Outer) Req {
	return Req{Internal: o.IsInternal()}
}

//go:noinline
func source() {
	o := Outer{}
	req := buildRequest(o)
	validators := []func(Req) error{Validator1, Validator2}
	for _, validator := range validators {
		_ = Decorator[Req](opaqueGate{}, req, validator)
	}
}

func main() {
	source()
}
