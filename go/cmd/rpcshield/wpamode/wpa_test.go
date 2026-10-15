package wpamode

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/serviceregistry"
)

func TestRegistryMetricValues_CustomProvider(t *testing.T) {
	t.Parallel()

	// A custom ServiceRegistryProvider supplies no generation metadata.
	values := registryMetricValues(serviceregistry.RegistryMeta{})
	require.Equal(t, "custom", values["registry_generation"])
	require.Equal(t, "custom", values["registry_services_source"])
	require.Equal(t, "custom", values["registry_network_groups_source"])
	require.Equal(t, "custom", values["registry_fx_mappings_source"])
	require.Equal(t, "false", values["registry_fallback"])
	require.Empty(t, values["registry_fallback_reason"])
}

func TestRegistryMetricValues_GenerationWithFallback(t *testing.T) {
	t.Parallel()

	meta := serviceregistry.RegistryMeta{
		Generation:     "gen-20260101T000000-abcdef",
		FellBack:       true,
		FallbackReason: "component \"services\": checksum mismatch",
		Components: map[string]serviceregistry.ComponentProvenance{
			serviceregistry.ComponentServices:      {Source: "generation"},
			serviceregistry.ComponentNetworkGroups: {Source: "generation"},
			serviceregistry.ComponentFxMappings:    {Source: "legacy"},
		},
	}
	values := registryMetricValues(meta)
	require.Equal(t, "gen-20260101T000000-abcdef", values["registry_generation"])
	require.Equal(t, "generation", values["registry_services_source"])
	require.Equal(t, "generation", values["registry_network_groups_source"])
	require.Equal(t, "legacy", values["registry_fx_mappings_source"])
	require.Equal(t, "true", values["registry_fallback"])
	require.Contains(t, values["registry_fallback_reason"], "checksum mismatch")
}
