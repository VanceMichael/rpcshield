package plaintwo

import (
	"go.uber.org/fx"
	llcpb "gogoproto/example.internal/part72/service-0024"
	ubuildpb "gogoproto/example.internal/part77/part119"
	paymentservicefx "thriftrw/example.internal/part43/service-0057/part44/part117fx"
	programservicefx "thriftrw/example.internal/part43/service-0057/part44/part118fx"
	u4bgateway_organizationfx "thriftrw/example.internal/part43/service-0057/part44/part45fx"
	tripservicefx "thriftrw/example.internal/part77/service-0055/service-0055/part78fx"
)

const u4bGatewayName = "service-0057"

// Module returns the output of this package.
var Module = fx.Options(
	fx.Provide(
		ubuildpb.NewFxBuildYARPCClient("service-0059"),
		u4bgateway_employeefx.Client(u4bGatewayName),
		u4bgateway_organizationfx.Client(u4bGatewayName),
		paymentservicefx.Client(u4bGatewayName),
		programservicefx.Client(u4bGatewayName),
		// This tests our ability to handle network group resolution, the "service-0055" here is the
		// network group name, where the actual service name is "service-0056".
		tripservicefx.Client("service-0055"),
		llcpb.NewFxLLCYARPCClient("service-0024"), // Conflicting service name under different packages.
		llcpb.NewFxLLCYARPCClient("service-0022"), // Conflicting service name under the same package.
	),
)
