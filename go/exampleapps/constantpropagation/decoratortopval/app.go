package main

// This app reproduces a suspected analyzer limitation seen in production
// (decorator.MetricDecorator[T] in ads-campaign-manager): a generic decorator gates a
// callback behind an opaque capability check (modeling a feature-flag/experimentation
// client whose result cannot be constant-folded), and the value ultimately checked deep
// inside the callback chain is a struct field derived from a *method call* on a different,
// outer struct, threaded across several interprocedural hops before being tested.
//
// Unlike ifelse/functionpassing, no infeasible() call is placed on the branch that should
// never be taken -- from a program-semantics perspective this program's sink call IS always
// reachable (Outer.IsInternal and opaqueGate.Allow both unconditionally return true). The
// companion test documents whether the analyzer agrees or disagrees (a false negative would
// mean it doesn't), and which *ssa.If it attributes the disagreement to.

//go:noinline
func sink() {}

//go:noinline
func callSink() {
	sink()
}

// Gate models an external/opaque capability check (e.g. a feature-flag client): its result
// cannot be constant-folded by the analyzer because it is reached through an interface method.
type Gate interface {
	Allow() bool
}

type opaqueGate struct{}

//go:noinline
func (opaqueGate) Allow() bool {
	return true
}

// Req is the input threaded through the decorator, analogous to entity.ValidateAdGroupRequest.
type Req struct {
	Internal bool
}

// Decorator mirrors decorator.MetricDecorator[T]: it gates the call to next behind an
// opaque, non-constant-foldable check before forwarding to the caller-supplied callback.
//
//go:noinline
func Decorator[T any](gate Gate, input T, next func(T) error) error {
	if !gate.Allow() {
		return nil
	}
	return next(input)
}

// Validator is analogous to ValidateTargetingCollection: it forwards to a nested helper
// rather than checking the field itself.
//
//go:noinline
func Validator(req Req) error {
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

// Outer models entity.ValidateCampaignRequest: the boolean that eventually gates the sink
// originates from a method call on a *different* struct than the one passed to Decorator,
// not from a struct-literal constant.
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
	_ = Decorator[Req](opaqueGate{}, req, Validator)
}

func main() {
	source()
}
