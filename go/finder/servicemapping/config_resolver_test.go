package servicemapping

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// The main logic (happy path) is directly tested in the tests of the service mapper. Here we test
// some error cases for config resolver, which is hard to test in the service mapper tests.

func TestConfigResolver_NonExistentWorkspace(t *testing.T) {
	t.Parallel()

	resolver, err := newConfigAliasResolver("nonexistent", "example.internal", nil /* logger */)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Empty(t, resolver)
}

func TestConfigResolver_CorruptProductionYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create a corrupt production.yaml file.
	err := os.MkdirAll(tmpDir+"/udeploy/pinocchio/foo", 0o755)
	require.NoError(t, err)
	err = os.WriteFile(tmpDir+"/udeploy/pinocchio/foo/production.yaml", []byte(`invalid: "missing-quote`), 0o644)
	require.NoError(t, err)

	resolver, err := newConfigAliasResolver(tmpDir, "example.internal", nil /* logger */)
	require.Error(t, err)
	require.Empty(t, resolver)
}
