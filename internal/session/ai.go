package session

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/heelune/dcell/internal/config"
	"github.com/heelune/dcell/internal/devcontainer"
)

// AI defines the interface for AI assistants.
type AI interface {
	Name() string
	IsAvailable() bool
	Start(ctxPath string, session *Session, loader *ContextLoader) error
	Continue(ctxPath string, session *Session, loader *ContextLoader) error
	Execute(ctxPath string, session *Session, prompt string, loader *ContextLoader) error
}

// Claude implements AI interface for Claude Code.
type Claude struct{}

// Name returns the AI name.
func (c *Claude) Name() string {
	return "claude"
}

// IsAvailable checks if Claude Code is installed.
func (c *Claude) IsAvailable() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

// Start starts a new Claude Code session.
func (c *Claude) Start(ctxPath string, session *Session, loader *ContextLoader) error {
	// Load layered context
	var contextContent string
	if loader != nil {
		loadedCtx, err := loader.LoadContext()
		if err == nil && loadedCtx != "" {
			contextContent = loadedCtx
		}
	}

	// Fallback to session files if no layered context
	if contextContent == "" {
		if data, err := os.ReadFile(session.ContextPath); err == nil {
			contextContent += string(data) + "\n\n"
		}
		if data, err := os.ReadFile(session.TodoPath); err == nil {
			contextContent += string(data) + "\n\n"
		}
	}

	// Write a temporary file with context (sanitize filename)
	safeName := strings.ReplaceAll(session.ContextName, "/", "-")
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("dcell-%s-prompt.txt", safeName))
	if err := os.WriteFile(tmpFile, []byte(contextContent), 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}
	defer os.Remove(tmpFile)

	// Start Claude with context
	cmd := exec.Command("claude", "--prompt", tmpFile)
	cmd.Dir = ctxPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Continue continues an existing Claude Code session.
func (c *Claude) Continue(ctxPath string, session *Session, loader *ContextLoader) error {
	// For now, just start fresh - Claude maintains its own history
	return c.Start(ctxPath, session, loader)
}

// Execute sends a one-off command to Claude.
func (c *Claude) Execute(ctxPath string, session *Session, prompt string, loader *ContextLoader) error {
	cmd := exec.Command("claude", "--prompt", "-")
	cmd.Dir = ctxPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Write prompt to stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		fmt.Fprint(stdin, prompt)
	}()

	return cmd.Run()
}

// Kimi implements AI interface for Kimi CLI.
type Kimi struct{}

// Name returns the AI name.
func (k *Kimi) Name() string {
	return "kimi"
}

// IsAvailable checks if Kimi CLI is installed.
func (k *Kimi) IsAvailable() bool {
	_, err := exec.LookPath("kimi")
	return err == nil
}

// Start starts a new Kimi session.
// If Dev Container is configured, it runs kimi inside the container.
func (k *Kimi) Start(ctxPath string, session *Session, loader *ContextLoader) error {
	// Check if Dev Container is configured
	if devcontainer.IsDevContainerConfigured(ctxPath) {
		// Get project name for container naming
		projectName := filepath.Base(filepath.Dir(ctxPath))
		containerName := devcontainer.GetContainerName(projectName, session.ContextName)
		
		// Try to run AI in container
		if err := devcontainer.RunAIInContainer(ctxPath, containerName, "kimi"); err == nil {
			return nil
		} else {
			// Container execution failed, fall back to host mode
			fmt.Fprintf(os.Stderr, "Warning: Failed to run in Dev Container: %v\n", err)
			fmt.Fprintf(os.Stderr, "Falling back to host mode...\n")
		}
	}

	// Host mode: Create agent spec files in .dcell-session/
	sessionDir := filepath.Dir(session.ContextPath) // .dcell-session/
	agentPath := filepath.Join(sessionDir, "dcell-agent.yaml")
	promptPath := filepath.Join(sessionDir, "system-prompt.md")
	
	// Get global and project context paths
	globalDir := filepath.Dir(config.GlobalConfigPath())
	globalCtxPath := filepath.Join(globalDir, "context.md")
	
	// Get project root from session path (flat structure)
	projectRoot := filepath.Dir(ctxPath)
	projectCtxPath := filepath.Join(projectRoot, ".context.md")
	
	// Ensure system prompt file exists
	if _, err := os.Stat(promptPath); os.IsNotExist(err) {
		promptContent := fmt.Sprintf(`# dcell Session: %s

## 階層型コンテキストファイル

このセッションの作業を開始する前に、以下のファイルを**順番に** ReadFile ツールで読み込んでください：

### 1. Global Context
**パス**: %s
- 共通設定、グローバルな制約事項、個人の好み

### 2. Project Context  
**パス**: %s
- プロジェクト固有の設定、技術スタック、コーディング規約

### 3. Session Context
**パス**: %s/context.md
- このセッションの目的、目標、作業範囲

### 4. Todo List
**パス**: %s/todo.md
- 現在のタスクリスト（進行中、保留中、完了済み）

### 5. Decisions
**パス**: %s/decisions.md
- アーキテクチャ決定記録（ADR）

## 指示

上記のコンテキストファイルを**必ずすべて読み込んで**から、開発を続けてください。
各ファイルの内容を理解し、優先順位と制約を考慮して作業してください。
`, session.ContextName, globalCtxPath, projectCtxPath, sessionDir, sessionDir, sessionDir)
		os.WriteFile(promptPath, []byte(promptContent), 0644)
	}
	
	// Ensure agent spec exists
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		// Use relative path from agent spec to prompt file
		agentContent := `version: 1
agent:
  name: dcell-session
  extend: default
  system_prompt_path: ./system-prompt.md
`
		os.WriteFile(agentPath, []byte(agentContent), 0644)
	}

	// Start kimi with agent file from .dcell-session/
	cmd := exec.Command("kimi", "--agent-file", agentPath)
	cmd.Dir = ctxPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Continue continues an existing Kimi session.
func (k *Kimi) Continue(ctxPath string, session *Session, loader *ContextLoader) error {
	return k.Start(ctxPath, session, loader)
}

// Execute sends a one-off command to Kimi.
func (k *Kimi) Execute(ctxPath string, session *Session, prompt string, loader *ContextLoader) error {
	cmd := exec.Command("kimi", "-c", prompt)
	cmd.Dir = ctxPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// Codex implements AI interface for OpenAI Codex CLI.
type Codex struct{}

// Name returns the AI name.
func (c *Codex) Name() string {
	return "codex"
}

// IsAvailable checks if Codex CLI is installed.
func (c *Codex) IsAvailable() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

// Start starts a new Codex session.
func (c *Codex) Start(ctxPath string, session *Session, loader *ContextLoader) error {
	// Load layered context
	var contextContent string
	if loader != nil {
		loadedCtx, err := loader.LoadContext()
		if err == nil && loadedCtx != "" {
			contextContent = loadedCtx
		}
	}

	// Fallback to session files
	if contextContent == "" {
		if data, err := os.ReadFile(session.ContextPath); err == nil {
			contextContent += string(data) + "\n\n"
		}
		if data, err := os.ReadFile(session.TodoPath); err == nil {
			contextContent += string(data) + "\n\n"
		}
	}

	// Write a temporary file with context
	safeName := strings.ReplaceAll(session.ContextName, "/", "-")
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("dcell-codex-%s-prompt.txt", safeName))
	if err := os.WriteFile(tmpFile, []byte(contextContent), 0644); err != nil {
		return fmt.Errorf("failed to write prompt file: %w", err)
	}
	defer os.Remove(tmpFile)

	// Start Codex with context
	// Codex supports --image flag for context, but we'll use stdin for text
	cmd := exec.Command("codex")
	cmd.Dir = ctxPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Create pipe for stdin to send context first
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start codex: %w", err)
	}

	// Send context as first input
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, contextContent)
		io.WriteString(stdin, "\n\n---\n\nAbove is the project context. Please continue development.\n\n")
		io.Copy(stdin, os.Stdin)
	}()

	return cmd.Wait()
}

// Continue continues an existing Codex session.
func (c *Codex) Continue(ctxPath string, session *Session, loader *ContextLoader) error {
	return c.Start(ctxPath, session, loader)
}

// Execute sends a one-off command to Codex.
func (c *Codex) Execute(ctxPath string, session *Session, prompt string, loader *ContextLoader) error {
	cmd := exec.Command("codex", "-q", prompt)
	cmd.Dir = ctxPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// GetAI returns an AI instance by name.
func GetAI(name string) (AI, error) {
	switch name {
	case "claude":
		return &Claude{}, nil
	case "kimi":
		return &Kimi{}, nil
	case "codex":
		return &Codex{}, nil
	default:
		return nil, fmt.Errorf("unknown AI: %s", name)
	}
}

// DetectAI tries to detect available AI tools.
func DetectAI() (AI, error) {
	aiList := []AI{&Claude{}, &Kimi{}, &Codex{}}
	
	for _, ai := range aiList {
		if ai.IsAvailable() {
			return ai, nil
		}
	}
	
	return nil, fmt.Errorf("no AI tool found (install claude, kimi, or codex)")
}
