package reporting

import (
	"bytes"
	_ "embed" // Using `embed` to embed the HTML template file
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/uber-research/rpcshield/go/output"
)

//go:embed htmltemplate.txt
var _htmlTemplate string

// WriteReportsToHTML writes the given reports to an HTML file.
func WriteReportsToHTML(outputDir string, service string, reports []output.Report, gitSHA string, timestamp string) (string, error) {
	if len(reports) == 0 {
		return "", errors.New("no reports to write")
	}

	t, err := template.New("report").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b }, // Helper function to add numbers
	}).Parse(_htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %v", err)
	}

	data := struct {
		Service      string
		GitSHA       string
		RunTimestamp string
		Reports      []output.Report
	}{
		Service:      service,
		GitSHA:       gitSHA,
		RunTimestamp: timestamp,
		Reports:      reports,
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %v", err)
	}

	// Create an HTML file in the output directory with the name `<service>.html`.
	htmlFilePath := filepath.Join(outputDir, getHTMLFileName(service))
	// Write to a temp file in the same directory and atomically rename into
	// place: a failed/canceled write cannot truncate an existing complete HTML
	// report.
	tmpFile, err := os.CreateTemp(outputDir, fmt.Sprintf(".%s.html.tmp-*", service))
	if err != nil {
		return "", fmt.Errorf("failed to create temp HTML file: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write HTML content to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp HTML file: %v", err)
	}
	// Match os.WriteFile's default permissions (CreateTemp creates 0600).
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return "", fmt.Errorf("failed to chmod temp HTML file: %v", err)
	}
	if err := os.Rename(tmpPath, htmlFilePath); err != nil {
		return "", fmt.Errorf("failed to finalize HTML file: %v", err)
	}
	return htmlFilePath, nil
}

func getHTMLFileName(service string) string {
	return fmt.Sprintf("%s.html", service)
}
