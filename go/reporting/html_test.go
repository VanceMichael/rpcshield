package reporting

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber-research/rpcshield/go/grail"
	"github.com/uber-research/rpcshield/go/output"
)

// Mock data and helper functions
const mockHTMLTemplate = `
<!DOCTYPE html>
<html>
<head><title>{{.Service}} Report</title></head>
<body>
    <h1>Service Report for {{.Service}}</h1>
    <p>Git SHA: {{.GitSHA}}</p>
    <p>Run Timestamp: {{.RunTimestamp}}</p>
    {{range $index, $report := .Reports}}
    <div>
        <h2>Report {{$index | add 1}}</h2>
        <p>Status: {{$report.Status}}</p>
    </div>
    {{end}}
</body>
</html>
`

func mockReports() []output.Report {
	return []output.Report{
		{
			Inbound: output.EndPoint{
				Service: grail.Service{Name: "ServiceA", Tier: 1},
				APIName: "API1",
			},
			Outbound: output.EndPoint{
				Service: grail.Service{Name: "ServiceB", Tier: 4},
				APIName: "API2",
			},
			Status:      "FailClose",
			PathResults: []output.PathResult{},
		},
		{
			Inbound: output.EndPoint{
				Service: grail.Service{Name: "ServiceA", Tier: 1},
				APIName: "API3",
			},
			Outbound: output.EndPoint{
				Service: grail.Service{Name: "ServiceC", Tier: 5},
				APIName: "API4",
			},
			Status:      "FailOpen",
			PathResults: []output.PathResult{},
		},
	}
}

// Unit tests
func TestWriteReportsToHTML_EmptyReports(t *testing.T) {
	_, err := WriteReportsToHTML(".", "ServiceA", []output.Report{}, "abc123", "2024-11-25T15:04:05Z")
	require.Error(t, err)
	require.Equal(t, "no reports to write", err.Error())
}

func TestWriteReportsToHTML_TemplateParsingError(t *testing.T) {
	// Backup original template
	originalTemplate := _htmlTemplate
	defer func() { _htmlTemplate = originalTemplate }()

	// Use an invalid template
	_htmlTemplate = "{{ .InvalidField }"

	_, err := WriteReportsToHTML(".", "ServiceA", mockReports(), "abc123", "2024-11-25T15:04:05Z")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to parse HTML template")
}

func TestWriteReportsToHTML_Success(t *testing.T) {
	// Backup original template and replace with a mock
	originalTemplate := _htmlTemplate
	_htmlTemplate = mockHTMLTemplate
	defer func() { _htmlTemplate = originalTemplate }()

	// Temporary directory for the output
	tempDir := t.TempDir()

	htmlFilePath, err := WriteReportsToHTML(tempDir, "ServiceA", mockReports(), "abc123", "2024-11-25T15:04:05Z")
	require.NoError(t, err)
	require.NotEmpty(t, htmlFilePath)

	// Verify the output file
	require.FileExists(t, htmlFilePath)

	// Read and verify content
	content, err := os.ReadFile(htmlFilePath)
	require.NoError(t, err)
	require.Contains(t, string(content), "<title>ServiceA Report</title>")
	require.Contains(t, string(content), "Git SHA: abc123")
	require.Contains(t, string(content), "Run Timestamp: 2024-11-25T15:04:05Z")
	require.Contains(t, string(content), "Report 1")
	require.Contains(t, string(content), "Report 2")
}

func TestWriteReportsToHTML_FileWriteError(t *testing.T) {
	// Backup original template and replace with a mock
	originalTemplate := _htmlTemplate
	_htmlTemplate = mockHTMLTemplate
	defer func() { _htmlTemplate = originalTemplate }()

	// Intentionally use an invalid output directory
	invalidDir := "/invalid-dir"

	_, err := WriteReportsToHTML(invalidDir, "ServiceA", mockReports(), "abc123", "2024-11-25T15:04:05Z")
	require.Error(t, err)
	// Writes are staged in a temp file in the output dir and atomically
	// renamed into place, so the failure is at temp-file creation here.
	require.Contains(t, err.Error(), "failed to create temp HTML file")
}
