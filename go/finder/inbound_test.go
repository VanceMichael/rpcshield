package finder

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/grail"
	"github.com/uber-research/rpcshield/go/loader"
	"github.com/uber-research/rpcshield/go/rpcshieldtest"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestFinder_FindInbounds(t *testing.T) {
	t.Parallel()

	inboundService := grail.Service{Name: "rpcshielddemo", Tier: -1}
	mapper := servicemapping.New([]grail.Service{{Name: "lottery", Tier: 5}}, nil /* provider */, nil /* logger */)
	finder := New(inboundService, mapper, nil /* logger */, nil /* keepIBPkgs */, "" /* codebasePkgPrefix: match everything outside the monorepo */, "" /* kafkaConsumerProxyPkgPath */, nil /* infraPkgPrefixes */, false /* enableGenericGRPCThriftDetection */)

	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	entryPkg, allPkgs, ssaProg := testPkg.Entry, testPkg.All, testPkg.SSAProg

	inbounds, err := finder.Inbound(entryPkg, allPkgs, ssaProg)
	require.NoError(t, err)
	require.NotEmpty(t, inbounds)

	type InboundRepr struct {
		PkgPath       string
		FullFuncName  string
		CanonicalName string
		Type          fty.EndpointType
	}

	expected := []InboundRepr{
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler",
			FullFuncName:  "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler.handler).Create",
			CanonicalName: "rpcshielddemo.rpcshielddemo__create",
			Type:          fty.TypeGRPC,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler",
			FullFuncName:  "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler.handler).Get",
			CanonicalName: "rpcshielddemo.rpcshielddemo__get",
			Type:          fty.TypeGRPC,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler/thrifthandler",
			FullFuncName:  "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler/thrifthandler.handler).Create",
			CanonicalName: "simpleservice__create",
			Type:          fty.TypeThrift,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler",
			FullFuncName:  "(github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler.valueReceiverHandler).Create",
			CanonicalName: "rpcshielddemo.rpcshielddemo__create",
			Type:          fty.TypeGRPC,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/handler",
			FullFuncName:  "(*github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/pkg/nopservice.Server).Get",
			CanonicalName: "rpcshielddemo.rpcshielddemo__get",
			Type:          fty.TypeGRPC,
		},
	}

	got := make([]InboundRepr, 0, len(inbounds))
	for _, point := range inbounds {
		got = append(got, InboundRepr{PkgPath: point.Pkg.PkgPath, FullFuncName: point.Func.FullName(), CanonicalName: point.CanonicalName, Type: point.Type})
	}
	require.ElementsMatch(t, expected, got)
}

func TestFinder_FindInbounds_GenericGRPCThrift(t *testing.T) {
	t.Parallel()

	inboundService := grail.Service{Name: "genericgrpcthrift", Tier: -1}
	mapper := servicemapping.New(nil /* services */, nil /* provider */, nil /* logger */)
	finder := New(inboundService, mapper, nil /* logger */, nil /* keepIBPkgs */, "" /* codebasePkgPrefix */, "" /* kafkaConsumerProxyPkgPath */, nil /* infraPkgPrefixes */, true /* enableGenericGRPCThriftDetection */)

	entryPkg, allPkgs, err := loader.LoadWithGoPath("rpcshield", []string{"exampleapps", "outbounds", "genericgrpcthrift", "genericgrpcthrift-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift")
	require.NoError(t, err)

	ssaProg, _ := ssautil.AllPackages(allPkgs, ssa.InstantiateGenerics|ssa.GlobalDebug)
	ssaProg.Build()

	inbounds, err := finder.Inbound(entryPkg, allPkgs, ssaProg)
	require.NoError(t, err)

	type InboundRepr struct {
		PkgPath       string
		FullFuncName  string
		CanonicalName string
		Type          fty.EndpointType
	}
	got := make([]InboundRepr, 0, len(inbounds))
	for _, point := range inbounds {
		got = append(got, InboundRepr{PkgPath: point.Pkg.PkgPath, FullFuncName: point.Func.FullName(), CanonicalName: point.CanonicalName, Type: point.Type})
	}

	expected := []InboundRepr{
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift",
			FullFuncName:  "(github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift.fooServerImpl).Foo",
			CanonicalName: "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift/pb__foo",
			Type:          fty.TypeGRPC,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift",
			FullFuncName:  "(github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift.barHandlerImpl).Bar",
			CanonicalName: "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift/pb__bar",
			Type:          fty.TypeThrift,
		},
	}
	require.ElementsMatch(t, expected, got)
}

// Tests requiring Go SDK in runtime need a real `go` binary on PATH -- see
// modelfx/model_test.go's goSDKBinDir for the pattern this mirrors.
func TestMain(m *testing.M) {
	goBin := goSDKBinDir()
	os.Setenv("PATH", goBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if os.Getenv("GOCACHE") == "" {
		if dir, err := os.MkdirTemp("", "finder-gocache"); err == nil {
			os.Setenv("GOCACHE", dir)
		}
	}
	os.Exit(m.Run())
}

func goSDKBinDir() string {
	runfilesRoot := os.Getenv("RUNFILES_DIR")
	if runfilesRoot == "" {
		runfilesRoot = os.Getenv("TEST_SRCDIR")
	}
	if runfilesRoot != "" {
		if goBin := filepath.Join(runfilesRoot, "go_sdk", "bin", "go"); fileExists(goBin) {
			return filepath.Dir(goBin)
		}
	}
	return filepath.Join(runtime.GOROOT(), "bin")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
