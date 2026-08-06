// Command rpcshielddemo is a whole-program test fixture for rpcshield: it
// reproduces the inbound/outbound/fail-open/fail-close call-graph shapes of
// an internal cff-demo-style service using only OSS dependencies
// (go.uber.org/cff, go.uber.org/yarpc, genuine thriftrw/protoc-generated
// code) so it can be analyzed outside a monorepo.
package main

import (
	"context"
	"fmt"

	"gogoproto/rpcshielddemo"

	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/controller"
	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/httpservicefoo"
	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/lottery"
	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler"
	"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/httpclient"
)

func main() {
	l := lottery.New(nil)
	h := httpservicefoo.New(httpclient.New())
	ctrl := controller.New(l, h)
	hdlr := handler.New(ctrl)

	ctx := context.Background()
	resp, err := hdlr.Create(ctx, &rpcshielddemo.CreateRequest{Name: "lucky"})
	fmt.Println(resp, err)
}
