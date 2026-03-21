// Package tmux provides tmux session management for dcell.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Session represents a tmux session.
type Session struct {
	Name    string
	Attached bool
	Windows  int
}

// HasTmux checks if tmux is installed.
func HasTmux() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// SessionExists checks if a tmux session exists.
func SessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// CreateSession creates a new tmux session.
func CreateSession(name string, path string) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", path)
	return cmd.Run()
}

// AttachSession attaches to an existing tmux session.
// This replaces the current process with tmux.
func AttachSession(name string) error {
	cmd := exec.Command("tmux", "attach-session", "-t", name)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// SwitchSession switches to another tmux session from within tmux.
func SwitchSession(name string) error {
	cmd := exec.Command("tmux", "switch-client", "-t", name)
	return cmd.Run()
}

// KillSession kills a tmux session.
func KillSession(name string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	return cmd.Run()
}

// ListSessions lists all tmux sessions.
func ListSessions() ([]Session, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}|#{session_attached}|#{session_windows}")
	out, err := cmd.Output()
	if err != nil {
		// No sessions is not an error
		if strings.Contains(err.Error(), "no server running") {
			return []Session{}, nil
		}
		return nil, err
	}

	var sessions []Session
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			sessions = append(sessions, Session{
				Name:     parts[0],
				Attached: parts[1] == "1",
				Windows:  parseInt(parts[2]),
			})
		}
	}
	return sessions, nil
}

// InTmux checks if currently running inside tmux.
func InTmux() bool {
	return os.Getenv("TMUX") != ""
}

// GetSessionForContext returns the tmux session name for a context.
func GetSessionForContext(ctxName string) string {
	return fmt.Sprintf("dcell-%s", ctxName)
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
