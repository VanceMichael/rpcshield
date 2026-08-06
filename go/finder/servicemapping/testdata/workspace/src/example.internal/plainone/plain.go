package plain

import (
	"example.internal/anotherpkg"
	"go.uber.org/fx"
	"go.uber.org/yarpc"
	testcatalogpb "gogoproto/example.internal/part29/part112/part113"
	llcpb "gogoproto/example.internal/part72/service-0024"
	loggingquerypb "gogoproto/example.internal/part77/part108/part109"
	ubuildpb "gogoproto/example.internal/part77/part119"
	logisticsfx "thriftrw/example.internal/part15/part93/part114/part115/part116fx"
	tripservicefx "thriftrw/example.internal/part77/service-0055/service-0055/part78fx"
)

func opts() fx.Option {
	return fx.Options(
		fx.Provide(
			func(clientConfig yarpc.ClientConfig) loggingquerypb.LoggingQueryYARPCClient {
				return loggingquerypb.NewFxLoggingQueryYARPCClient(_sawmillQuery)
			},
			// We will fail to resolve the following since the constant is defined in another
			// package, but that is OK - in example.internal/plaintwo package we can find a
			// constant string there. So we should not have any errors.
			ubuildpb.NewFxBuildYARPCClient(anotherpkg.UBuild),
			testcatalogpb.NewFxTestCaseServiceYARPCClient(_testCatalog),
			testcatalogpb.NewFxServiceTestConfigManagementServiceYARPCClient(_testCatalog),
			testcatalogpb.NewFxTestCatalogYARPCClient(_testCatalog),
			logisticsfx.Client("service-0018-alias"),
			tripservicefx.Client("service-0056"),
			llcpb.NewFxLLCYARPCClient("test-service"), // Conflicting service name under different packages.
		),
	)
}
