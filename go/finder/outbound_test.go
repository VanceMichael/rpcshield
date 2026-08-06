package finder

import (
	"bytes"
	"go/printer"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/finder/servicemapping"
	fty "github.com/uber-research/rpcshield/go/finder/types"
	"github.com/uber-research/rpcshield/go/grail"
	"github.com/uber-research/rpcshield/go/loader"
	"github.com/uber-research/rpcshield/go/rpcshieldtest"
	"golang.org/x/tools/go/packages"
)

func TestFinder_Outbound(t *testing.T) {
	t.Parallel()

	inboundService := grail.Service{Name: "rpcshielddemo", Tier: -1}
	lotteryService := grail.Service{Name: "lottery", Tier: 5}
	clientTypeToService := servicemapping.ServiceMapping{}
	clientTypeToService.AddMappings(
		servicemapping.FullyQualifiedType{PkgPath: "thriftrw/lottery/lotteryclient", Name: "Interface"},
		&servicemapping.ServiceWithIDLInfo{Service: lotteryService},
	)
	mapper := servicemapping.New([]grail.Service{lotteryService, {Name: "cadence", Tier: 1}}, clientTypeToService, nil /* logger */)
	finder := New(inboundService, mapper, nil /* logger */, make(map[string]bool) /* keepIBPkgs */, "" /* codebasePkgPrefix */, "" /* kafkaConsumerProxyPkgPath */, nil /* infraPkgPrefixes */, false /* enableGenericGRPCThriftDetection */)

	type OutboundRepr struct {
		PkgPath       string
		CallExprStr   string
		Service       grail.Service
		CanonicalName string
		Type          fty.OutboundType
	}

	testcases := []struct {
		desc        string
		pkgProvider func() (entryPkg *packages.Package, allPkgs []*packages.Package, err error)
		expected    []OutboundRepr
	}{
		{
			desc: "rpcshielddemo",
			pkgProvider: func() (entryPkg *packages.Package, allPkgs []*packages.Package, err error) {
				testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
				return testPkg.Entry, testPkg.All, nil
			},
			expected: []OutboundRepr{
				{
					PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/lottery",
					CallExprStr:   "g.client.Play",
					Service:       lotteryService,
					CanonicalName: "lottery__play",
					Type:          fty.TypeThrift,
				},
			},
		},
		{
			desc: "cadence",
			pkgProvider: func() (entryPkg *packages.Package, allPkgs []*packages.Package, err error) {
				return loader.LoadWithGoPath("rpcshield", []string{"exampleapps", "outbounds", "cadence", "cadence-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/outbounds/cadence")
			},
			expected: []OutboundRepr{
				{
					PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/outbounds/cadence",
					CallExprStr:   "c.ExecuteWorkflow",
					Service:       grail.Service{Name: "cadence", Tier: 1},
					CanonicalName: "cadence__executeworkflow",
					Type:          fty.TypeCadence,
				},
			},
		},
	}

	for index, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			entryPkg, allPkgs, err := tc.pkgProvider()
			require.NoError(t, err)
			outbounds, _, err := finder.Outbound(entryPkg, allPkgs)
			var filteredOutbounds []fty.OutboundPoint
			for _, ob := range outbounds {
				if ob.Type != fty.TypeHTTP {
					filteredOutbounds = append(filteredOutbounds, ob)
				}
			}
			outbounds = filteredOutbounds
			require.NoError(t, err)
			require.NotEmpty(t, outbounds)

			got := make([]OutboundRepr, 0, len(outbounds))
			for _, point := range outbounds {
				var buf bytes.Buffer
				err := printer.Fprint(&buf, point.Pkg.Fset, point.Call.Fun)
				require.NoError(t, err)
				if point.Type == fty.TypeHTTP {
					continue
				}
				got = append(got, OutboundRepr{PkgPath: point.Pkg.PkgPath, CallExprStr: buf.String(), Service: point.Service, CanonicalName: point.CanonicalName, Type: point.Type})
			}

			if tc.expected == nil {
				t.Logf("DISCOVERY got=%#v", got)
			}
			require.ElementsMatch(t, tc.expected, got)

			// Only test the error case once.
			if index == 0 {
				// Trigger the error case of duplicate packages.
				allPkgsWithDuplicates := append(allPkgs, allPkgs[0])
				_, _, err = finder.Outbound(entryPkg, allPkgsWithDuplicates)
				require.Error(t, err)
			}
		})
	}
}

func TestFinder_Outbound_GenericGRPCThrift(t *testing.T) {
	t.Parallel()

	inboundService := grail.Service{Name: "genericgrpcthrift", Tier: -1}
	grpcService := grail.Service{Name: "generic-grpc-service", Tier: 5}
	thriftService := grail.Service{Name: "generic-thrift-service", Tier: 5}
	clientTypeToService := servicemapping.ServiceMapping{}
	clientTypeToService.AddMappings(
		servicemapping.FullyQualifiedType{PkgPath: "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift/pb", Name: "FooClient"},
		&servicemapping.ServiceWithIDLInfo{Service: grpcService},
	)
	clientTypeToService.AddMappings(
		servicemapping.FullyQualifiedType{PkgPath: "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift/pb", Name: "BarClient"},
		&servicemapping.ServiceWithIDLInfo{Service: thriftService},
	)
	mapper := servicemapping.New([]grail.Service{grpcService, thriftService}, clientTypeToService, nil /* logger */)
	finder := New(inboundService, mapper, nil /* logger */, make(map[string]bool) /* keepIBPkgs */, "" /* codebasePkgPrefix */, "" /* kafkaConsumerProxyPkgPath */, nil /* infraPkgPrefixes */, true /* enableGenericGRPCThriftDetection */)

	entryPkg, allPkgs, err := loader.LoadWithGoPath("rpcshield", []string{"exampleapps", "outbounds", "genericgrpcthrift", "genericgrpcthrift-anal"}, "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift")
	require.NoError(t, err)

	outbounds, _, err := finder.Outbound(entryPkg, allPkgs)
	require.NoError(t, err)

	type OutboundRepr struct {
		PkgPath       string
		CallExprStr   string
		Service       grail.Service
		CanonicalName string
		Type          fty.OutboundType
	}
	got := make([]OutboundRepr, 0, len(outbounds))
	for _, point := range outbounds {
		if point.Type == fty.TypeHTTP {
			continue
		}
		var buf bytes.Buffer
		require.NoError(t, printer.Fprint(&buf, point.Pkg.Fset, point.Call.Fun))
		got = append(got, OutboundRepr{PkgPath: point.Pkg.PkgPath, CallExprStr: buf.String(), Service: point.Service, CanonicalName: point.CanonicalName, Type: point.Type})
	}

	expected := []OutboundRepr{
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift",
			CallExprStr:   "c.Foo",
			Service:       grpcService,
			CanonicalName: "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift/pb__foo",
			Type:          fty.TypeGRPC,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift",
			CallExprStr:   "c.Bar",
			Service:       thriftService,
			CanonicalName: "github.com/uber-research/rpcshield/go/exampleapps/outbounds/genericgrpcthrift/pb__bar",
			Type:          fty.TypeThrift,
		},
	}
	require.ElementsMatch(t, expected, got)
}

func TestFinder_HTTP_OB(t *testing.T) {
	t.Parallel()
	type OutboundRepr struct {
		PkgPath       string
		CallExprStr   string
		CanonicalName string
		Type          fty.OutboundType
	}

	inboundService := grail.Service{Name: "rpcshielddemo", Tier: -1}
	mapper := servicemapping.New([]grail.Service{{Name: "fake", Tier: 5}}, nil /* provider */, nil /* logger */)
	finder := New(inboundService, mapper, nil /* logger */, make(map[string]bool) /* keepIBPkgs */, "" /* codebasePkgPrefix */, "" /* kafkaConsumerProxyPkgPath */, nil, /* infraPkgPrefixes */
		false, /* enableGenericGRPCThriftDetection */
		"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/httpclient.Client",
		"github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/wrappedclient.Client",
	)

	testPkg := rpcshieldtest.LoadrpcshieldDemoPackages()
	outbounds, _, err := finder.Outbound(testPkg.Entry, testPkg.All)
	require.NoError(t, err)
	require.NotEmpty(t, outbounds)
	got := make([]OutboundRepr, 0, len(outbounds))
	for _, point := range outbounds {
		if point.Type != fty.TypeHTTP {
			continue
		}
		var buf bytes.Buffer
		err := printer.Fprint(&buf, point.Pkg.Fset, point.Call.Fun)
		require.NoError(t, err)
		got = append(got, OutboundRepr{PkgPath: point.Pkg.PkgPath, CallExprStr: buf.String(), CanonicalName: point.CanonicalName, Type: point.Type})
	}
	t.Logf("DISCOVERY got=%#v", got)
	expected := []OutboundRepr{
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/httpclient",
			CallExprStr:   "c.inner.Do",
			CanonicalName: "net/http__do",
			Type:          fty.TypeHTTP,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/wrappedclient",
			CallExprStr:   "c.inner.Do",
			CanonicalName: "net/http__do",
			Type:          fty.TypeHTTP,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/httpservicefoo",
			CallExprStr:   "cl.Do",
			CanonicalName: "net/http__do",
			Type:          fty.TypeHTTP,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/httpservicefoo",
			CallExprStr:   "wrappedclient.DefaultClient.Do",
			CanonicalName: "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/wrappedclient__do",
			Type:          fty.TypeHTTP,
		},
		{
			PkgPath:       "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/gateway/httpservicefoo",
			CallExprStr:   "g.client.Do",
			CanonicalName: "github.com/uber-research/rpcshield/go/exampleapps/rpcshielddemo/util/httpclient__do",
			Type:          fty.TypeHTTP,
		},
	}
	require.ElementsMatch(t, expected, got)
}
