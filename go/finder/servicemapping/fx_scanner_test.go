package servicemapping

import (
	"cmp"
	"path/filepath"
	"testing"

	gocmp "github.com/google/go-cmp/cmp"
	gocmpopts "github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/grail"
)

func TestFxScanner(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(_testdata, "workspace")
	allServices := loadAllServices(t)
	allNetworkGroups := loadAllNetworkGroups(t)
	scanner, err := NewFxScanner(workspace, allServices, allNetworkGroups, "example.internal", nil /* logger */)
	require.NoError(t, err)

	expected := ServiceMapping{
		{PkgPath: "gogoproto/example.internal/part72/service-0024", Name: "LLCYARPCClient"}: {
			{
				Service:             grail.Service{Name: "service-0022", Tier: 1},
				ConstructorPkgPaths: []string{"example.internal/plaintwo"},
			},
			{
				Service:             grail.Service{Name: "service-0024", Tier: 5},
				ConstructorPkgPaths: []string{"example.internal/plaintwo"},
			},
			{
				Service:             grail.Service{Name: "test-service", Tier: 5},
				ConstructorPkgPaths: []string{"example.internal/plainone"},
			},
		},
		{PkgPath: "gogoproto/example.internal/part29/part112/part113", Name: "TestCatalogYARPCClient"}: {
			{
				Service: grail.Service{
					Name: "service-0052",
					Tier: 3,
				},
				ConstructorPkgPaths: []string{"example.internal/plainone"},
			},
		},
		{PkgPath: "gogoproto/example.internal/part29/part112/part113", Name: "ServiceTestConfigManagementServiceYARPCClient"}: {
			{
				Service: grail.Service{
					Name: "service-0052",
					Tier: 3,
				},
				ConstructorPkgPaths: []string{"example.internal/plainone"},
			},
		},
		{PkgPath: "gogoproto/example.internal/part29/part112/part113", Name: "TestCaseServiceYARPCClient"}: {
			{
				Service: grail.Service{
					Name: "service-0052",
					Tier: 3,
				},
				ConstructorPkgPaths: []string{"example.internal/plainone"},
			},
		},
		{PkgPath: "gogoproto/example.internal/part77/part108/part109", Name: "LoggingQueryYARPCClient"}: {
			{
				Service: grail.Service{
					Name: "service-0046",
					Tier: 0,
				},
				ConstructorPkgPaths: []string{"example.internal/plainone"},
			},
		},
		{PkgPath: "thriftrw/example.internal/part43/service-0057/part44/part45client", Name: "Interface"}: {
			{
				Service: grail.Service{
					Name: "service-0057",
					Tier: 1,
				},
				ConstructorPkgPaths: []string{"example.internal/plaintwo"},
			},
		},
		{PkgPath: "thriftrw/example.internal/part43/service-0057/part44/part117client", Name: "Interface"}: {
			{
				Service: grail.Service{
					Name: "service-0057",
					Tier: 1,
				},
				ConstructorPkgPaths: []string{"example.internal/plaintwo"},
			},
		},
		{PkgPath: "thriftrw/example.internal/part43/service-0057/part44/part118client", Name: "Interface"}: {
			{
				Service: grail.Service{
					Name: "service-0057",
					Tier: 1,
				},
				ConstructorPkgPaths: []string{"example.internal/plaintwo"},
			},
		},
		{PkgPath: "gogoproto/example.internal/part77/part119", Name: "BuildYARPCClient"}: {
			{
				Service: grail.Service{
					Name: "service-0059",
				},
				ConstructorPkgPaths: []string{"example.internal/plaintwo"},
			},
		},
		{PkgPath: "thriftrw/example.internal/part15/part93/part114/part115/part116client", Name: "Interface"}: {
			{
				Service: grail.Service{
					Name: "service-0018",
					Tier: 1,
				},
				ConstructorPkgPaths: []string{"example.internal/plainone"},
			},
		},
		{PkgPath: "thriftrw/example.internal/part77/service-0055/service-0055/part78client", Name: "Interface"}: {
			{
				Service: grail.Service{
					Name: "service-0056",
					Tier: 1,
				},
				ConstructorPkgPaths: []string{"example.internal/plainone", "example.internal/plaintwo"},
			},
		},
	}
	got, err := scanner.Mappings()
	require.NoError(t, err)
	if diff := gocmp.Diff(expected, got, gocmpopts.SortSlices(cmp.Less[string])); diff != "" {
		t.Errorf("mappings mismatch (-want +got):\n%s", diff)
		return
	}
}

func TestFxScanner_ServiceNotFoundError(t *testing.T) {
	t.Parallel()

	workspace := filepath.Join(_testdata, "workspace")

	// Use only a subset of services so that some service names in the test data won't be found
	limitedServices := []grail.Service{
		{Name: "service-0022", Tier: 1},
		{Name: "service-0024", Tier: 5},
		// Intentionally omit "test-service", "service-0052", "service-0046", etc.
	}

	allNetworkGroups := loadAllNetworkGroups(t)
	scanner, err := NewFxScanner(workspace, limitedServices, allNetworkGroups, "example.internal", nil /* logger */)
	require.NoError(t, err)

	got, err := scanner.Mappings()

	// Should get an error because some services referenced in the test data are not in limitedServices
	require.ErrorContains(t, err, "not found in all services")

	// The returned mapping should still contain the services that were found
	require.NotEmpty(t, got)

	// Verify we got mappings for the services that do exist
	expected := ServiceMapping{
		{PkgPath: "gogoproto/example.internal/part72/service-0024", Name: "LLCYARPCClient"}: {
			{
				Service:             grail.Service{Name: "service-0022", Tier: 1},
				ConstructorPkgPaths: []string{"example.internal/plaintwo"},
			},
			{
				Service:             grail.Service{Name: "service-0024", Tier: 5},
				ConstructorPkgPaths: []string{"example.internal/plaintwo"},
			},
		},
	}

	// Check that we got the expected mappings for services that exist
	if diff := gocmp.Diff(expected, got, gocmpopts.SortSlices(cmp.Less[string])); diff != "" {
		t.Errorf("mappings mismatch (-want +got):\n%s", diff)
		return
	}
}
