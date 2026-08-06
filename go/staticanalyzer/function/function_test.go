package function

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/common/pkgdeps"
	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/rpcshieldtest"
	"github.com/uber-research/rpcshield/go/rta/rtatest"
	"github.com/uber-research/rpcshield/go/staticanalyzer/asthelper"
	"golang.org/x/tools/go/packages"
)

const _contrivedTestsDirPath = "../../testdata/src/contrivedtests"

func TestAnalyzeFunctionForErrorPropagation(t *testing.T) {
	t.Parallel()

	data := rpcshieldtest.SetupTest(t, _contrivedTestsDirPath, []string{rpcshieldtest.OutboundCallSite, rpcshieldtest.OutboundCallSitePos, rpcshieldtest.WantFuncValidityPrefix, rpcshieldtest.WantFuncStatusPrefix}...)
	allPkgsMap := make(map[string]*packages.Package)
	for _, pkgTestData := range data {
		allPkgsMap[pkgTestData.Pkg.PkgPath] = pkgTestData.Pkg
	}

	for _, pkgTestData := range data {
		for funcDecl, expectedValues := range pkgTestData.FuncExpectedValues {
			require.NotNilf(t, expectedValues[rpcshieldtest.OutboundCallSite], "Expected an outbound sink site to be defined")
			require.NotNilf(t, expectedValues[rpcshieldtest.WantFuncValidityPrefix], "Expected a function validity value to be defined")
			require.NotNilf(t, expectedValues[rpcshieldtest.WantFuncStatusPrefix], "Expected a function status value to be defined")

			// Check if the outbound sink site is present for the function under test.
			calleeList := rpcshieldtest.ParseOutboundSite(pkgTestData.Pkg, allPkgsMap, expectedValues[rpcshieldtest.OutboundCallSite], funcDecl)
			require.NotNilf(t, calleeList, "Outbound sink site `%s` not found for function `%s`", expectedValues[rpcshieldtest.OutboundCallSite], funcDecl.Name.Name)

			// Check if the function validity is as expected.
			expectedFuncValidity := constant.GetFuncValidityFromString(expectedValues[rpcshieldtest.WantFuncValidityPrefix])
			pkgInfo := &pkgdeps.Info{Pkg: pkgTestData.Pkg, Deps: allPkgsMap}

			// If the outbound call site position is not specified, use the first outbound call site position by default.
			outboundCallSite := calleeList[0]
			if v, ok := expectedValues[rpcshieldtest.OutboundCallSitePos]; ok && v != "" {
				expectedPosInt, err := strconv.Atoi(v)
				require.NoErrorf(t, err, "Error converting expected outbound call position `%s` to integer", v)
				require.GreaterOrEqualf(t, expectedPosInt, 1, "Expected outbound call position to be greater than or equal to 1")
				require.LessOrEqualf(t, expectedPosInt, len(calleeList), "Expected outbound call position to be less than or equal to the number of outbound call sites")
				outboundCallSite = calleeList[expectedPosInt-1] // Convert to 0-based index
			}
			astContext := asthelper.New(pkgInfo, funcDecl)
			f := &funcAnalyzer{
				callerCalleeInfo: &callerCalleeInfo{
					caller:    funcDecl,
					callee:    outboundCallSite,
					callerObj: astContext.FuncObjectFromFuncDecl(funcDecl),
				},
				astContext: astContext,
				logger:     nil,
			}
			actualFuncValidity := f.isAnalyzable()
			require.Equal(t, expectedFuncValidity.String(), actualFuncValidity.String(), fmt.Sprintf("Function validity mismatch for function `%s`", funcDecl.Name.Name))

			// Check if the function's fail-ness status is as expected
			expectedFuncStatus := constant.Failness(expectedValues[rpcshieldtest.WantFuncStatusPrefix])
			actualFuncMetadata, err := Run(f.caller, f.callee, astContext, nil)
			require.NoErrorf(t, err, "Error analyzing function `%s`", funcDecl.Name.Name)
			require.Equalf(t, expectedFuncStatus, actualFuncMetadata.Status, fmt.Sprintf("Function status mismatch for function `%s`", funcDecl.Name.Name))
		}
	}
}

func TestRecovery(t *testing.T) {
	t.Parallel()

	// Intentionally give a nil pass variable to trigger a panic, but we should recover from it
	// and convert it to an error via the result struct.
	_, err := Run(nil, nil, nil, nil)
	require.ErrorContains(t, err, "INTERNAL PANIC")
}

// Tests requiring Go SDK in runtime need rtatest.RunWithGoSDK, mirroring the monorepo's
// testutil.RunWithGoSDK (see https://github.com/bazel-contrib/rules_go/issues/2370). cfg_test.go's
// TestBuildCFG additionally needs the stricter re-exec variant (not just SetupGoSDK) since it uses
// analysistest.Run/importer.Default(), which compare `go env GOROOT` against runtime.GOROOT()
// directly within this process.
func TestMain(m *testing.M) {
	rtatest.RunWithGoSDK(m)
}
