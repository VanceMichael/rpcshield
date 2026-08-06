package testdata

import loggingquerypb "gogoproto/example.internal/infra/logging/loggingquery"

func main() {
	// This gives a conflict name for "logging/loggingquery" package, but it does not matter since
	// anything under `testdata` folder should be ignored.
	loggingquerypb.NewFxLoggingQueryYARPCClient("should-be-ignored"),
}
