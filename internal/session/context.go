// Package session provides AI session management with layered context.
package session

import (
	"fmt"
	"os"
	"path/filepath"
)

// ContextLoader manages layered context loading.
type ContextLoader struct {
	GlobalDir     string // ~/.config/dcell/
	ProjectDir    string // dcell/ (where .bare is)
	SessionDir    string // .dcell-session/
}

// NewContextLoader creates a new context loader.
func NewContextLoader(globalDir, projectDir, sessionDir string) *ContextLoader {
	return &ContextLoader{
		GlobalDir:  globalDir,
		ProjectDir: projectDir,
		SessionDir: sessionDir,
	}
}

// LoadContext loads all context files in order:
// 1. Global (~/.config/dcell/context.md)
// 2. Project (dcell/.context.md)
// 3. Session (.dcell-session/context.md)
func (cl *ContextLoader) LoadContext() (string, error) {
	var content string

	// 1. Load global context
	if globalCtx, err := cl.loadFile(filepath.Join(cl.GlobalDir, "context.md")); err == nil {
		content += "## Global Context\n\n" + globalCtx + "\n\n"
	}

	// 2. Load project context
	if projectCtx, err := cl.loadFile(filepath.Join(cl.ProjectDir, ".context.md")); err == nil {
		content += "## Project Context\n\n" + projectCtx + "\n\n"
	}

	// 3. Load session context
	if sessionCtx, err := cl.loadFile(filepath.Join(cl.SessionDir, "context.md")); err == nil {
		content += "## Session Context\n\n" + sessionCtx + "\n\n"
	}

	// Also load todo files
	content += cl.loadTodoFiles()

	return content, nil
}

// loadTodoFiles loads all todo files from the hierarchy.
func (cl *ContextLoader) loadTodoFiles() string {
	var content string

	// Global todo
	if globalTodo, err := cl.loadFile(filepath.Join(cl.GlobalDir, "todo.md")); err == nil {
		content += "## Global Todo\n\n" + globalTodo + "\n\n"
	}

	// Project todo
	if projectTodo, err := cl.loadFile(filepath.Join(cl.ProjectDir, ".todo.md")); err == nil {
		content += "## Project Todo\n\n" + projectTodo + "\n\n"
	}

	// Session todo
	if sessionTodo, err := cl.loadFile(filepath.Join(cl.SessionDir, "todo.md")); err == nil {
		content += "## Session Todo\n\n" + sessionTodo + "\n\n"
	}

	return content
}

// loadFile reads a file if it exists.
func (cl *ContextLoader) loadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// InitGlobalContext creates global context files if they don't exist.
func InitGlobalContext(globalDir string) error {
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return fmt.Errorf("failed to create global dir: %w", err)
	}

	// Create global context.md template
	globalContextPath := filepath.Join(globalDir, "context.md")
	if _, err := os.Stat(globalContextPath); os.IsNotExist(err) {
		template := `# Global Context

## Your Preferences
- Preferred languages: 
- Coding style: 

## Common Tasks
- バグ修正時の手順
- 新機能追加時の手順

## Rules
- 必ずテストを書く
- コメントは日本語で
`
		if err := os.WriteFile(globalContextPath, []byte(template), 0644); err != nil {
			return err
		}
	}

	// Create global todo.md
	globalTodoPath := filepath.Join(globalDir, "todo.md")
	if _, err := os.Stat(globalTodoPath); os.IsNotExist(err) {
		if err := os.WriteFile(globalTodoPath, []byte("# Global Todo\n\n"), 0644); err != nil {
			return err
		}
	}

	return nil
}

// InitProjectContext creates project-level context files.
func InitProjectContext(projectDir string) error {
	// Create .context.md
	contextPath := filepath.Join(projectDir, ".context.md")
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		template := `# Project Context

## Tech Stack
- Language: 
- Framework: 
- Database: 

## Architecture
- 設計方針
- ディレクトリ構成

## Development Rules
- コーディング規約
- テスト方針
`
		if err := os.WriteFile(contextPath, []byte(template), 0644); err != nil {
			return err
		}
	}

	// Create .todo.md
	todoPath := filepath.Join(projectDir, ".todo.md")
	if _, err := os.Stat(todoPath); os.IsNotExist(err) {
		if err := os.WriteFile(todoPath, []byte("# Project Todo\n\n"), 0644); err != nil {
			return err
		}
	}

	return nil
}
