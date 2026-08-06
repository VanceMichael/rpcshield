package deadmethod

// deadProvider is an unexported type whose method is never constructed or
// called anywhere in the built program (its only caller lives in test code).
// This mirrors the real-world case where an RPC call sits inside a method of an
// unexported type wired up only in tests: ssautil.AllFunctions omits such a
// method (its exportedTypeHack only visits methods of exported named types), so
// the AST->SSA map must add it back explicitly via prog.FuncValue.
type deadProvider struct {
	id int
}

// Fetch contains a nested function literal to also exercise anonymous-function
// recursion for a dead method.
func (p *deadProvider) Fetch() int {
	f := func() int {
		return p.id
	}
	return f()
}
