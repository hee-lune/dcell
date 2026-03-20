package vcs

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// JJ implements VCS interface for Jujutsu.
type JJ struct {
	RepoPath string
}

// Name returns the VCS name.
func (j *JJ) Name() string {
	return "jj"
}

// Detect checks if the repository uses jj.
func (j *JJ) Detect(repoPath string) bool {
	cmd := exec.Command("jj", "root", "--ignore-working-copy")
	cmd.Dir = repoPath
	return cmd.Run() == nil
}

// CreateContext creates a new jj workspace.
func (j *JJ) CreateContext(name string, base string) (*Context, error) {
	ctxPath := filepath.Join(j.RepoPath, "..", name)

	args := []string{"workspace", "create", "--name", name}
	if base != "" {
		// In jj, we can set the revision to work from
		args = append(args, "-r", base)
	}
	args = append(args, ctxPath)

	cmd := exec.Command("jj", args...)
	cmd.Dir = j.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create jj workspace: %w\n%s", err, out)
	}

	return &Context{
		Name:       name,
		Path:       ctxPath,
		BaseBranch: base,
		VCS:        "jj",
	}, nil
}

// SwitchContext switches to an existing workspace.
func (j *JJ) SwitchContext(name string) error {
	// In jj, we just need to cd to the workspace directory
	// The workspace is already independent
	ctxPath := filepath.Join(j.RepoPath, "..", name)

	// Verify it exists and is a valid jj workspace
	cmd := exec.Command("jj", "root")
	cmd.Dir = ctxPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("workspace %s not found or invalid: %w", name, err)
	}

	return nil
}

// ListContexts lists all jj workspaces.
func (j *JJ) ListContexts() ([]Context, error) {
	cmd := exec.Command("jj", "workspace", "list")
	cmd.Dir = j.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}

	var contexts []Context
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse: "name: path" format
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		path := strings.TrimSpace(parts[1])

		ctx := Context{
			Name: name,
			Path: path,
			VCS:  "jj",
		}

		// Try to get the base branch/revision
		if base, err := j.getBaseRevision(path); err == nil {
			ctx.BaseBranch = base
		}

		contexts = append(contexts, ctx)
	}

	return contexts, nil
}

// RemoveContext removes a workspace.
func (j *JJ) RemoveContext(name string) error {
	// First forget the workspace in jj
	cmd := exec.Command("jj", "workspace", "forget", name)
	cmd.Dir = j.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to forget workspace: %w\n%s", err, out)
	}

	// Then remove the directory
	ctxPath := filepath.Join(j.RepoPath, "..", name)
	if err := exec.Command("rm", "-rf", ctxPath).Run(); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	return nil
}

// CurrentContext returns the current workspace.
func (j *JJ) CurrentContext() (*Context, error) {
	cmd := exec.Command("jj", "workspace", "name")
	cmd.Dir = j.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current workspace: %w", err)
	}

	name := strings.TrimSpace(string(out))

	return &Context{
		Name: name,
		Path: j.RepoPath,
		VCS:  "jj",
	}, nil
}

// Init initializes a new Jujutsu repository.
// JJ works with git repositories, so we use git init + jj colocate.
func (j *JJ) Init(repoPath string, bare bool) error {
	// First, git init
	git := &Git{}
	if err := git.Init(repoPath, bare); err != nil {
		return err
	}

	// Then colocate jj
	cmd := exec.Command("jj", "git", "init", "--colocate")
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to initialize jj: %w\n%s", err, out)
	}

	// Create initial empty change
	cmd = exec.Command("jj", "describe", "-m", "Initial commit")
	cmd.Dir = repoPath
	cmd.Run() // Ignore error for initial empty repo

	j.RepoPath = repoPath
	return nil
}

// Clone clones a remote repository with Jujutsu.
func (j *JJ) Clone(url string, dest string, branch string) error {
	// Use git clone first
	git := &Git{}
	if err := git.Clone(url, dest, branch); err != nil {
		return err
	}

	// Then colocate jj
	cmd := exec.Command("jj", "git", "init", "--colocate")
	cmd.Dir = dest
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to initialize jj: %w\n%s", err, out)
	}

	j.RepoPath = dest
	return nil
}

// InitAndSetup initializes a repo and creates main workspace for bare repos.
func (j *JJ) InitAndSetup(projectDir string, bare bool) (string, error) {
	if bare {
		// Create bare repository with git
		barePath := projectDir + ".git"
		if err := j.Init(barePath, true); err != nil {
			return "", err
		}

		// Create main workspace
		mainPath := filepath.Join(projectDir, "main")
		if err := os.MkdirAll(mainPath, 0755); err != nil {
			return "", fmt.Errorf("failed to create main directory: %w", err)
		}

		// Add main workspace
		cmd := exec.Command("jj", "workspace", "create", "--name", "main", mainPath)
		cmd.Dir = barePath
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to create main workspace: %w\n%s", err, out)
		}

		return mainPath, nil
	}

	// Non-bare: simple init
	if err := j.Init(projectDir, false); err != nil {
		return "", err
	}

	return projectDir, nil
}

func (j *JJ) getBaseRevision(path string) (string, error) {
	cmd := exec.Command("jj", "log", "-r", "@", "--no-graph", "-T", "change_id.short()")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// GetChangeStatus returns the status of changes in the workspace.
func (j *JJ) GetChangeStatus(path string) (string, error) {
	cmd := exec.Command("jj", "st")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// HasChanges checks if there are uncommitted changes.
func (j *JJ) HasChanges(path string) bool {
	cmd := exec.Command("jj", "diff", "--stat")
	cmd.Dir = path
	out, _ := cmd.Output()
	return len(bytes.TrimSpace(out)) > 0
}
