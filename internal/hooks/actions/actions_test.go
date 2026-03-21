package actions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "source.txt")
	dst := filepath.Join(tempDir, "dest.txt")

	// Create source file
	content := []byte("hello world")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy
	if err := Copy(src, dst); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	// Verify
	copied, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(copied) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", string(copied), string(content))
	}
}

func TestSymlink(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "target.txt")
	link := filepath.Join(tempDir, "link.txt")

	// Create target file
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create target file: %v", err)
	}

	// Create symlink
	if err := Symlink(target, link); err != nil {
		t.Fatalf("Symlink failed: %v", err)
	}

	// Verify
	if !IsSymlink(link) {
		t.Error("link is not a symlink")
	}

	content, err := os.ReadFile(link)
	if err != nil {
		t.Fatalf("failed to read through symlink: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("content mismatch: got %q", string(content))
	}
}

func TestRenderTemplate(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "template.txt")
	dst := filepath.Join(tempDir, "output.txt")

	// Create template file
	template := "Hello {{ .ContextName }} from {{ .VCS }}!"
	if err := os.WriteFile(src, []byte(template), 0644); err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Render
	data := &TemplateData{
		ContextName: "test-ctx",
		BaseBranch:  "main",
		VCS:         "git",
	}
	if err := RenderTemplate(src, dst, data); err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	// Verify
	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	expected := "Hello test-ctx from git!"
	if string(content) != expected {
		t.Errorf("content mismatch: got %q, want %q", string(content), expected)
	}
}
