package reporting

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/constant"
	"github.com/uber-research/rpcshield/go/grail"
)

// assertNoTmpFiles ensures no leftover temp artifacts in dir after a write.
func assertNoTmpFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".*.tmp-*"))
	require.NoError(t, err)
	require.Empty(t, matches, "temporary artifacts left behind: %v", matches)
}

// TestWriteReportsToCSV_AtomicReplace verifies a successful write atomically
// replaces a previously generated (possibly complete) CSV and leaves no temp
// files behind.
func TestWriteReportsToCSV_AtomicReplace(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "svc.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte("previous trusted complete report\n"), 0644))

	path, rows, err := WriteReportsToCSV(dir, grail.Service{Name: "svc"}, mockReports(), "sha", "ts", constant.SourceForCSV, "repo", "")
	require.NoError(t, err)
	require.Equal(t, csvPath, path)
	require.Greater(t, rows, 0)

	got, err := os.ReadFile(csvPath)
	require.NoError(t, err)
	require.NotContains(t, string(got), "previous trusted complete report")
	require.Contains(t, string(got), "svc", "sanity: header/data present")
	assertNoTmpFiles(t, dir)
}

// TestWriteReportsToHTML_AtomicReplace verifies the same atomic-rename
// guarantee for the HTML artifact.
func TestWriteReportsToHTML_AtomicReplace(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "ServiceA.html")
	require.NoError(t, os.WriteFile(htmlPath, []byte("<html>previous</html>"), 0644)

	path, err := WriteReportsToHTML(dir, "ServiceA", mockReports(), "abc123", "2024-11-25T15:04:05Z")
	require.NoError(t, err)
	require.Equal(t, htmlPath, path)

	got, err := os.ReadFile(htmlPath)
	require.NoError(t, err)
	require.NotContains(t, string(got), "previous")
	assertNoTmpFiles(t, dir)
}
