// Package vcs provides abstraction for version control systems.
package vcs

import (
	"fmt"
	"path/filepath"
)

// Context represents a development context (workspace/worktree).
type Context struct {
	Name       string
	Path       string
	BaseBranch string
	VCS        string // "jj" or "git"
}

// VCS defines the interface for version control operations.
type VCS interface {
	Name() string
	Detect(repoPath string) bool
	CreateContext(name string, base string) (*Context, error)
	SwitchContext(name string) error
	ListContexts() ([]Context, error)
	RemoveContext(name string) error
	CurrentContext() (*Context, error)

	// Init initializes a new repository (bare or non-bare).
	Init(repoPath string, bare bool) error
	// Clone clones a remote repository.
	Clone(url string, dest string, branch string) error
}

// NewAuto detects the VCS type and returns the appropriate implementation.
func NewAuto(repoPath string) (VCS, error) {
	// Try jj first
	jj := &JJ{RepoPath: repoPath}
	if jj.Detect(repoPath) {
		return jj, nil
	}

	// Fall back to git
	git := &Git{RepoPath: repoPath}
	if git.Detect(repoPath) {
		return git, nil
	}

	return nil, fmt.Errorf("no supported VCS found in %s", repoPath)
}

// New creates a VCS instance with explicit type.
func New(vcsType string, repoPath string) (VCS, error) {
	switch vcsType {
	case "jj":
		return &JJ{RepoPath: repoPath}, nil
	case "git":
		return &Git{RepoPath: repoPath}, nil
	default:
		return nil, fmt.Errorf("unknown VCS type: %s", vcsType)
	}
}

// ContextPath returns the path for a context.
func ContextPath(repoPath string, name string) string {
	return filepath.Join(repoPath, "..", name)
}
