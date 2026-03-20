package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Session represents an AI session for a context.
type Session struct {
	ContextName string    `toml:"context_name"`
	VCS         string    `toml:"vcs"`
	CreatedAt   time.Time `toml:"created_at"`
	UpdatedAt   time.Time `toml:"updated_at"`
	
	// Context files
	ContextPath string `toml:"context_path"`  // path to context.md
	TodoPath    string `toml:"todo_path"`     // path to todo.md
	DecisionsPath string `toml:"decisions_path"` // path to decisions.md
	
	// Session state
	LastAIInteraction time.Time `toml:"last_ai_interaction"`
	TaskCount         int       `toml:"task_count"`
	CompletedTasks    int       `toml:"completed_tasks"`
}

// Store manages session storage.
type Store struct {
	ProjectPath string
}

// NewStore creates a new session store for a project.
func NewStore(projectPath string) *Store {
	return &Store{
		ProjectPath: projectPath,
	}
}

// BaseDir returns the sessions directory for the project.
func (s *Store) BaseDir() string {
	return filepath.Join(s.ProjectPath, ".dcell", "sessions")
}

// sanitizeContextName replaces path separators to create safe filenames.
func sanitizeContextName(ctxName string) string {
	return strings.ReplaceAll(ctxName, "/", "-")
}

// GetSessionDir returns the directory for a context's session.
func (s *Store) GetSessionDir(ctxName string) string {
	safeName := sanitizeContextName(ctxName)
	return filepath.Join(s.BaseDir(), safeName)
}

// Create creates a new session for a context.
func (s *Store) Create(ctxName string, vcs string, ctxPath string) (*Session, error) {
	sessionDir := s.GetSessionDir(ctxName)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	now := time.Now()
	session := &Session{
		ContextName: ctxName,
		VCS:         vcs,
		CreatedAt:   now,
		UpdatedAt:   now,
		ContextPath: filepath.Join(ctxPath, ".dcell-session", "context.md"),
		TodoPath:    filepath.Join(ctxPath, ".dcell-session", "todo.md"),
		DecisionsPath: filepath.Join(ctxPath, ".dcell-session", "decisions.md"),
	}

	// Create context files
	if err := os.MkdirAll(filepath.Dir(session.ContextPath), 0755); err != nil {
		return nil, err
	}

	// Create initial context.md
	if err := s.createContextFile(session.ContextPath, ctxName); err != nil {
		return nil, err
	}

	// Create initial todo.md
	if err := s.createTodoFile(session.TodoPath); err != nil {
		return nil, err
	}

	// Create initial decisions.md
	if err := s.createDecisionsFile(session.DecisionsPath); err != nil {
		return nil, err
	}

	// Save session metadata
	if err := s.Save(session); err != nil {
		return nil, err
	}

	// Create AGENTS.md for Kimi CLI
	if err := s.CreateAGENTSMD(ctxPath, ctxName); err != nil {
		// Non-fatal error
		fmt.Fprintf(os.Stderr, "Warning: Failed to create AGENTS.md: %v\n", err)
	}

	return session, nil
}

// Load loads a session for a context.
func (s *Store) Load(ctxName string) (*Session, error) {
	sessionFile := filepath.Join(s.GetSessionDir(ctxName), "session.toml")
	
	var session Session
	if _, err := toml.DecodeFile(sessionFile, &session); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session not found for context: %s", ctxName)
		}
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	return &session, nil
}

// Save saves a session.
func (s *Store) Save(session *Session) error {
	sessionDir := s.GetSessionDir(session.ContextName)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	sessionFile := filepath.Join(sessionDir, "session.toml")
	f, err := os.Create(sessionFile)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	if err := enc.Encode(session); err != nil {
		return fmt.Errorf("failed to encode session: %w", err)
	}

	return nil
}

// Update updates the session timestamp.
func (s *Store) Update(ctxName string) error {
	session, err := s.Load(ctxName)
	if err != nil {
		return err
	}

	session.UpdatedAt = time.Now()
	return s.Save(session)
}

// Remove removes a session.
func (s *Store) Remove(ctxName string) error {
	sessionDir := s.GetSessionDir(ctxName)
	return os.RemoveAll(sessionDir)
}

// List lists all sessions.
func (s *Store) List() ([]Session, error) {
	entries, err := os.ReadDir(s.BaseDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []Session{}, nil
		}
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if entry.IsDir() {
			if session, err := s.Load(entry.Name()); err == nil {
				sessions = append(sessions, *session)
			}
		}
	}

	return sessions, nil
}

func (s *Store) createContextFile(path string, ctxName string) error {
	content := fmt.Sprintf(`# dcell Context: %s

## Purpose
<!-- Describe what you're working on in this context -->

## Goals
- [ ] 

## Constraints
<!-- Any limitations or requirements -->

## References
<!-- Links to relevant issues, PRs, docs -->
`, ctxName)

	return os.WriteFile(path, []byte(content), 0644)
}

func (s *Store) CreateAGENTSMD(ctxPath string, ctxName string) error {
	agentsPath := filepath.Join(ctxPath, "AGENTS.md")
	
	content := fmt.Sprintf(`# dcell Session: %s

## コンテキストファイル

以下のファイルを読み込んでください：

1. **Session Context**: ./.dcell-session/context.md
   - このセッションの目的、目標、制約事項

2. **Todo List**: ./.dcell-session/todo.md
   - 現在のタスクリスト（進行中、保留中、完了済み）

3. **Decisions**: ./.dcell-session/decisions.md
   - アーキテクチャ決定記録（ADR）

## 指示

上記のコンテキストファイルを読み込んだ上で、開発を続けてください。
セッション作成時刻: %s
`, ctxName, time.Now().Format("2006-01-02 15:04"))

	return os.WriteFile(agentsPath, []byte(content), 0644)
}

func (s *Store) createTodoFile(path string) error {
	content := `# Todo

## In Progress
- [ ] 

## Pending
- [ ] 

## Completed
- [x] Session initialized

## Notes
<!-- Additional notes -->
`

	return os.WriteFile(path, []byte(content), 0644)
}

func (s *Store) createDecisionsFile(path string) error {
	content := `# Decisions

## Architecture Decisions

### ADR-001: 
**Status:** Proposed | Accepted | Deprecated

**Context:**

**Decision:**

**Consequences:**

## Technical Notes
<!-- Implementation details, lessons learned -->
`

	return os.WriteFile(path, []byte(content), 0644)
}
