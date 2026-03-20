package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
func (k *Kimi) Start(ctxPath string, session *Session, loader *ContextLoader) error {
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

	// Kimi doesn't have a direct prompt flag, so we'll use -c flag
	cmd := exec.Command("kimi", "-c", contextContent)
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

// GetAI returns an AI instance by name.
func GetAI(name string) (AI, error) {
	switch name {
	case "claude":
		return &Claude{}, nil
	case "kimi":
		return &Kimi{}, nil
	default:
		return nil, fmt.Errorf("unknown AI: %s", name)
	}
}

// DetectAI tries to detect available AI tools.
func DetectAI() (AI, error) {
	aiList := []AI{&Claude{}, &Kimi{}}
	
	for _, ai := range aiList {
		if ai.IsAvailable() {
			return ai, nil
		}
	}
	
	return nil, fmt.Errorf("no AI tool found (install claude or kimi)")
}
