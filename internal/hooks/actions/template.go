package actions

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// TemplateData holds the data available in templates.
type TemplateData struct {
	ContextName string
	BaseBranch  string
	VCS         string
}

// RenderTemplate reads a template file, executes it with data, and writes to destination.
func RenderTemplate(src, dst string, data *TemplateData) error {
	// Read template file
	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read template: %w", err)
	}

	// Parse and execute template
	tmpl, err := template.New("hook").Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Create destination directory
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Write result
	if err := os.WriteFile(dst, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write rendered template: %w", err)
	}

	return nil
}

// SimpleTemplate performs simple variable substitution without Go templates.
// Supports: {{ .ContextName }}, {{ .BaseBranch }}, {{ .VCS }}
func SimpleTemplate(content string, data *TemplateData) string {
	result := content
	result = strings.ReplaceAll(result, "{{ .ContextName }}", data.ContextName)
	result = strings.ReplaceAll(result, "{{ .BaseBranch }}", data.BaseBranch)
	result = strings.ReplaceAll(result, "{{ .VCS }}", data.VCS)
	return result
}
