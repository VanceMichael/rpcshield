package main

import (
	"github.com/uber-research/rpcshield/go/exampleapps/invokeprobability/a/b/c/d"
	"github.com/uber-research/rpcshield/go/exampleapps/invokeprobability/a/b/c/x"
	"github.com/uber-research/rpcshield/go/exampleapps/invokeprobability/a/m/o/p"
)

func dCaller(dc d.Controller) {
	dc.F1(nil, 0)
}

func xCaller(xc x.Controller) {
	xc.F1(nil, 0)
}

func pCaller(pc p.Controller) {
	pc.F1(nil, 0)
}

func main() {
	// Create a d controller
	dc := d.NewController()
	xc := x.NewController()
	xo := x.NewOtherName()
	pc := p.NewController()
	pc.F1(nil, 0)
	po := p.NewOtherName()
	po.F1(nil, 0)
	dCaller(dc)
	xCaller(xc)
	xCaller(xo)
}
