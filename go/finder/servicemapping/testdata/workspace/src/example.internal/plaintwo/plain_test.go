package plaintwo

import ubuildpb "gogoproto/example.internal/infra/ubuild"

var Module = fx.Options(
	fx.Provide(
		// This conflicts with the service name in plain.go. However, we should simply ignore it
		// since test files contain mocked services and the hint here is often invalid (e.g.,
		// "test-service" here).
		ubuildpb.NewFxBuildYARPCClient("test-service"),
	),
)
