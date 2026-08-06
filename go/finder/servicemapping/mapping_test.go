package servicemapping

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/grail"
	"golang.org/x/tools/go/packages"
)

const _testdata = "testdata"

func loadAllServices(t *testing.T) []grail.Service {
	t.Helper()

	// Load the pre-fetched list of all services from the testdata file (gzipped since there are
	// many services that lead to big JSON files, which is forbidden to check into the Go monorepo).
	//
	// It can be refreshed by authenticating to Grail, POSTing query_all.txt to its /yql endpoint,
	// and gzipping the resulting {name, tier} list into testdata/services.json.gz.
	f, err := os.Open(filepath.Join(_testdata, "services.json.gz"))
	require.NoError(t, err)
	defer f.Close()
	reader, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer reader.Close()

	var allServices []grail.Service
	err = json.NewDecoder(reader).Decode(&allServices)
	require.NoError(t, err)
	return allServices
}

func loadAllNetworkGroups(t *testing.T) []grail.NetworkGroup {
	t.Helper()

	// Load the pre-fetched list of all services from the testdata file (gzipped since there are
	// many services that lead to big JSON files, which is forbidden to check into the Go monorepo).
	//
	// It can be refreshed by authenticating to Grail, POSTing query_network_group.txt to its /yql
	// endpoint, stripping each group id down to its last ":"-separated segment, and gzipping the
	// result into testdata/network_groups.json.gz.
	f, err := os.Open(filepath.Join(_testdata, "network_groups.json.gz"))
	require.NoError(t, err)
	defer f.Close()
	reader, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer reader.Close()

	var allNetworkGroups []grail.NetworkGroup
	err = json.NewDecoder(reader).Decode(&allNetworkGroups)
	require.NoError(t, err)
	return allNetworkGroups
}

func TestMapper_InferService(t *testing.T) {
	t.Parallel()

	testcases := map[FullyQualifiedType]string{
		{PkgPath: constant.CadenceClientPkg, Name: "Client"}:                                                                          constant.CadenceServiceName,
		{PkgPath: "thriftrw/example.internal/part1/service-0060/part2/part3", Name: "Interface"}:                                      "service-0060",
		{PkgPath: "thriftrw/example.internal/part4/part5/service-0038/part6/part7", Name: "Interface"}:                                "service-0038",
		{PkgPath: "thriftrw/example.internal/part8/service-0044/part9/part10", Name: "Interface"}:                                     "service-0044",
		{PkgPath: "thriftrw/example.internal/part11/service-0015/part12/part13", Name: "Interface"}:                                   "service-0015",
		{PkgPath: "gogoproto/example.internal/part14/service-0020", Name: "GiftingOrchestratorYARPCClient"}:                           "service-0020",
		{PkgPath: "thriftrw/example.internal/part15/service-0017/part16/part17/part18", Name: "Interface"}:                            "service-0017",
		{PkgPath: "gogoproto/example.internal/part19/part20/service-0025", Name: "LocalGraphDbYARPCClient"}:                           "service-0025",
		{PkgPath: "gogoproto/example.internal/part4/service-0039", Name: "PaymentOrderEligibilityGatewayYARPCClient"}:                 "service-0039",
		{PkgPath: "thriftrw/example.internal/part22/service-0010/service-0010/part23", Name: "Interface"}:                             "service-0010",
		{PkgPath: "gogoproto/example.internal/part24/service-0040/part25", Name: "PlaceGatewayYARPCClient"}:                           "service-0040",
		{PkgPath: "thriftrw/example.internal/part26/service-0030/service-0030/part27", Name: "Interface"}:                             "service-0030",
		{PkgPath: "gogoproto/example.internal/part4/part28/service-0013", Name: "PaymentPolicyEnforcerYARPCClient"}:                   "service-0013",
		{PkgPath: "gogoproto/example.internal/part29/service-0048", Name: "ShadowManagerYARPCClient"}:                                 "service-0048",
		{PkgPath: "thriftrw/example.internal/part30/service-0009/service-0009/part31", Name: "Interface"}:                             "service-0009",
		{PkgPath: "thriftrw/example.internal/part32/service-0031/part33/part34", Name: "Interface"}:                                   "service-0031",
		{PkgPath: "gogoproto/example.internal/part35/part36/service-0007", Name: "CustomerGatewayYARPCClient"}:                        "service-0007",
		{PkgPath: "gogoproto/example.internal/part37/service-0008", Name: "DriverOnJobYARPCClient"}:                                   "service-0008",
		{PkgPath: "thriftrw/example.internal/part1/service-0012/part38/part39", Name: "Interface"}:                                    "service-0012",
		{PkgPath: "thriftrw/example.internal/part40/service-0026/part41/part42", Name: "Interface"}:                                   "service-0026",
		{PkgPath: "thriftrw/example.internal/part43/service-0057/part44/part45client", Name: "Interface"}:                             "service-0057",
		{PkgPath: "gogoproto/example.internal/part4/service-0050/part46", Name: "TaxServiceYARPCClient"}:                              "service-0050",
		{PkgPath: "thriftrw/example.internal/part15/part47/service-0023/part48", Name: "Interface"}:                                   "service-0023",
		{PkgPath: "thriftrw/example.internal/part37/service-0008/part49/part50", Name: "Interface"}:                                   "service-0008",
		{PkgPath: "thriftrw/example.internal/part51/service-0042/part52/part53", Name: "Interface"}:                                   "service-0042",
		{PkgPath: "gogoproto/example.internal/part35/service-0051", Name: "TaxonomyYARPCClient"}:                                      "service-0051",
		{PkgPath: "thriftrw/example.internal/part15/service-0053/service-0053/part54", Name: "Interface"}:                             "service-0053",
		{PkgPath: "thriftrw/example.internal/part1/service-0033/service-0033/part55", Name: "Interface"}:                              "service-0033",
		{PkgPath: "thriftrw/example.internal/part30/part56/service-0035/part58", Name: "Interface"}:                                   "service-0035",
		{PkgPath: "gogoproto/example.internal/part15/part59/service-0037", Name: "ParameterServingYARPCClient"}:                       "service-0037",
		{PkgPath: "gogoproto/example.internal/part4/part60/service-0064", Name: "WalletApiGatewayYARPCClient"}:                        "service-0064",
		{PkgPath: "gogoproto/example.internal/part1/service-0041/part61", Name: "OTTMerchantYARPCClient"}:                             "service-0041",
		{PkgPath: "gogoproto/example.internal/part15/service-0019/part62", Name: "ThirdPartyTransportationYARPCClient"}:               "service-0019",
		{PkgPath: "gogoproto/example.internal/part1/service-0006", Name: "CourierDispatchIncentiveYARPCClient"}:                       "service-0006",
		{PkgPath: "thriftrw/example.internal/part1/service-0036/part63/part64", Name: "Interface"}:                                    "service-0036",
		{PkgPath: "thriftrw/example.internal/part1/service-0065/service-0065/part65", Name: "Interface"}:                              "service-0065",
		{PkgPath: "gogoproto/example.internal/part15/part66/service-0032", Name: "OmniviewServiceYARPCClient"}:                        "service-0032",
		{PkgPath: "thriftrw/example.internal/part67/service-0049/service-0049/part68", Name: "Interface"}:                             "service-0049",
		{PkgPath: "thriftrw/example.internal/part4/service-0050/service-0050/part69", Name: "Interface"}:                              "service-0050",
		{PkgPath: "thriftrw/example.internal/part70/service-0063/service-0063/part71", Name: "Interface"}:                             "service-0063",
		{PkgPath: "gogoproto/example.internal/part72/part73/service-0031", Name: "DeployServiceYARPCClient"}:                          "service-0031",
		{PkgPath: "gogoproto/example.internal/part4/service-0050", Name: "EstablishmentLocationServiceYARPCClient"}:                   "service-0050",
		{PkgPath: "gogoproto/example.internal/part15/service-0021", Name: "ArrivalTimeServiceYARPCClient"}:                            "service-0021",
		{PkgPath: "gogoproto/example.internal/part75/part11/part76/service-0028", Name: "MerchantPricingGatewayYARPCClient"}:          "service-0028",
		{PkgPath: "thriftrw/example.internal/part77/service-0055/service-0055/part78client", Name: "Interface"}:                       "service-0056",
		{PkgPath: "thriftrw/example.internal/part1/service-0034/part79/part80", Name: "Interface"}:                                    "service-0034",
		{PkgPath: "thriftrw/example.internal/part77/service-0062/service-0062/part81", Name: "Interface"}:                             "service-0062",
		{PkgPath: "thriftrw/example.internal/part1/service-0041/part82/part83", Name: "Interface"}:                                    "service-0041",
		{PkgPath: "thriftrw/example.internal/part1/service-0011/part84/part85", Name: "Interface"}:                                    "service-0011",
		{PkgPath: "thriftrw/example.internal/part86/service-0016/part87/part13", Name: "Interface"}:                                   "service-0016",
		{PkgPath: "thriftrw/example.internal/part88/service-0054/service-0054/part89", Name: "Interface"}:                             "service-0054",
		{PkgPath: "gogoproto/example.internal/part43/service-0058/part90", Name: "BusinessProductServiceYARPCClient"}:                 "service-0058",
		{PkgPath: "thriftrw/example.internal/part1/service-0041/part91/part92", Name: "Interface"}:                                    "service-0041",
		{PkgPath: "gogoproto/example.internal/part93/part94/part95/service-0005/part96", Name: "ConsumerBenefitProvidersYARPCClient"}: "service-0005",
		{PkgPath: "gogoproto/example.internal/part15/service-0029", Name: "MomaYARPCClient"}:                                          "service-0029",
		{PkgPath: "gogoproto/example.internal/part40/part97/service-0043", Name: "GeofenceYARPCClient"}:                               "service-0043",
		{PkgPath: "gogoproto/example.internal/part93/part15/part66/service-0027", Name: "XPlusProductYARPCClient"}:                    "service-0027",
		{PkgPath: "thriftrw/example.internal/part30/service-0045/part98/part99", Name: "Interface"}:                                   "service-0045",
		{PkgPath: "thriftrw/example.internal/part11/service-0015/part100/part101", Name: "Interface"}:                                 "service-0015",
		{PkgPath: "gogoproto/example.internal/part77/part102/part103/service-0061", Name: "UpFaasInspectorYARPCClient"}:               "service-0061",
		{PkgPath: "gogoproto/example.internal/part77/part102/part32/service-0003", Name: "PlannerYARPCClient"}:                        "service-0003",
		{PkgPath: "gogoproto/example.internal/part32/part106/part96/service-0004", Name: "QueryYARPCClient"}:                          "service-0004",
		{PkgPath: "gogoproto/example.internal/part77/part108/part109", Name: "LoggingQueryYARPCClient"}:                               "service-0046",
		{PkgPath: "gogoproto/example.internal/part75/part11/part76/service-0028-v2", Name: "MerchantPricingGatewayYARPCClient"}:       "service-0028",
		{PkgPath: "thriftrw/example.internal/part43/service-0014/part111", Name: "Interface"}:                                         "service-0014",
		{PkgPath: "gogoproto/example.internal/part29/part112/part113", Name: "TestCaseServiceYARPCClient"}:                            "service-0052",
		{PkgPath: "thriftrw/example.internal/part15/part93/part114/part115/part116client", Name: "Interface"}:                         "service-0018",
		// {PkgPath: "gogoproto/example.internal/part15/part93/part114", Name: ""}:                                                                               "service-0018",

		// There are conflicting mappings (fx constructors in different packages) in our test
		// workspace. This tests our ability to handle that (i.e., pick the one with most relevant
		// and highest tier).
		{PkgPath: "gogoproto/example.internal/part72/service-0024", Name: "LLCYARPCClient"}: "service-0022",
	}

	allServices := loadAllServices(t)
	allNetworkGroups := loadAllNetworkGroups(t)
	scanner, err := NewFxScanner(filepath.Join(_testdata, "workspace"), allServices, allNetworkGroups, "example.internal", nil /* logger */)
	require.NoError(t, err)
	clientTypeToService, err := scanner.Mappings()
	require.NoError(t, err)
	mapper := New(allServices, clientTypeToService, nil /* logger */)
	mapper.RankCachedServices([]*packages.Package{{PkgPath: "example.internal/plaintwo"}})
	for clientType, serviceName := range testcases {
		t.Run(serviceName, func(t *testing.T) {
			t.Parallel()

			inferred := mapper.InferService(clientType)
			require.NotNil(t, inferred, "service inference failed for client: %q", clientType)
			require.Equal(t, serviceName, inferred.Name, "client type: %s", clientType)
		})
	}
}

func TestMapper_NoExtraMapping(t *testing.T) {
	t.Parallel()

	allServices := loadAllServices(t)
	mapper := New(allServices, nil /* pkgPathToService */, nil /* logger */)

	clientType := FullyQualifiedType{PkgPath: "thriftrw/example.internal/part22/service-0010/service-0010/part23", Name: "Interface"}
	inferred := mapper.InferService(clientType)
	require.NotNil(t, inferred)
	require.Equal(t, "service-0010", inferred.Name)
}

func TestFullyQualifiedType_Codec(t *testing.T) {
	t.Parallel()

	// FullyQualifiedType's text marshalling capabilities are implemented mostly for JSON
	// marshalling when it is used in map keys. So here we test that.
	inp := map[FullyQualifiedType]string{
		{PkgPath: "github.com/uber-research/rpcshield/go/grail", Name: "Foo"}: "foo",
		{PkgPath: "github.com/uber-research/rpcshield/go/grail", Name: "Bar"}: "bar",
	}

	jsonString := `{
  "github.com/uber-research/rpcshield/go/grail.Foo": "foo",
  "github.com/uber-research/rpcshield/go/grail.Bar": "bar"
}`

	// Test marshalling.
	data, err := json.Marshal(inp)
	require.NoError(t, err)
	require.JSONEq(t, jsonString, string(data))

	// Test Unmarshalling.
	var out map[FullyQualifiedType]string
	err = json.Unmarshal([]byte(jsonString), &out)
	require.NoError(t, err)
	require.Equal(t, inp, out)
}
