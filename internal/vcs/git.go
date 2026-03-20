package vcs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Git implements VCS interface for Git worktrees.
type Git struct {
	RepoPath string
}

// Name returns the VCS name.
func (g *Git) Name() string {
	return "git"
}

// Detect checks if the repository uses git.
func (g *Git) Detect(repoPath string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	return cmd.Run() == nil
}

// CreateContext creates a new git worktree.
func (g *Git) CreateContext(name string, base string) (*Context, error) {
	ctxPath := filepath.Join(g.RepoPath, "..", name)

	if base == "" {
		base = "HEAD"
	}

	cmd := exec.Command("git", "worktree", "add", ctxPath, base)
	cmd.Dir = g.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create git worktree: %w\n%s", err, out)
	}

	// Try to get the branch name
	branch := base
	if base == "HEAD" {
		if b, err := g.getCurrentBranch(ctxPath); err == nil {
			branch = b
		}
	}

	return &Context{
		Name:       name,
		Path:       ctxPath,
		BaseBranch: branch,
		VCS:        "git",
	}, nil
}

// SwitchContext switches to an existing worktree.
func (g *Git) SwitchContext(name string) error {
	ctxPath := filepath.Join(g.RepoPath, "..", name)

	// Verify it exists and is a valid git worktree
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = ctxPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("worktree %s not found or invalid: %w", name, err)
	}

	return nil
}

// ListContexts lists all git worktrees.
func (g *Git) ListContexts() ([]Context, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = g.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var contexts []Context
	var currentCtx *Context

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" {
			// End of worktree entry
			if currentCtx != nil && currentCtx.Name != "" {
				contexts = append(contexts, *currentCtx)
			}
			currentCtx = nil
			continue
		}

		if currentCtx == nil {
			currentCtx = &Context{VCS: "git"}
		}

		if strings.HasPrefix(line, "worktree ") {
			currentCtx.Path = strings.TrimPrefix(line, "worktree ")
			// Derive name from path
			currentCtx.Name = filepath.Base(currentCtx.Path)
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch ")
			currentCtx.BaseBranch = filepath.Base(branch)
		}
	}

	// Don't forget the last entry
	if currentCtx != nil && currentCtx.Name != "" {
		contexts = append(contexts, *currentCtx)
	}

	return contexts, nil
}

// RemoveContext removes a worktree.
func (g *Git) RemoveContext(name string) error {
	ctxPath := filepath.Join(g.RepoPath, "..", name)

	cmd := exec.Command("git", "worktree", "remove", ctxPath)
	cmd.Dir = g.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w\n%s", err, out)
	}

	return nil
}

// CurrentContext returns the current worktree.
func (g *Git) CurrentContext() (*Context, error) {
	// Get current path
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = g.RepoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current path: %w", err)
	}

	path := strings.TrimSpace(string(out))
	name := filepath.Base(path)

	branch, _ := g.getCurrentBranch(path)

	return &Context{
		Name:       name,
		Path:       path,
		BaseBranch: branch,
		VCS:        "git",
	}, nil
}

// Init initializes a new Git repository.
func (g *Git) Init(repoPath string, bare bool) error {
	args := []string{"init"}
	if bare {
		args = append(args, "--bare")
	}
	args = append(args, repoPath)

	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to init repository: %w\n%s", err, out)
	}

	// If bare, set up the RepoPath for subsequent operations
	if bare {
		g.RepoPath = repoPath
	} else {
		g.RepoPath = repoPath
	}

	return nil
}

// Clone clones a remote Git repository.
func (g *Git) Clone(url string, dest string, branch string) error {
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, url, dest)

	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone repository: %w\n%s", err, out)
	}

	g.RepoPath = dest
	return nil
}

// InitAndSetup initializes a repo and creates main worktree for bare repos.
func (g *Git) InitAndSetup(projectDir string, bare bool) (string, error) {
	// Get absolute path for projectDir
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	if bare {
		// Create bare repository
		barePath := absProjectDir + ".git"
		if err := g.Init(barePath, true); err != nil {
			return "", err
		}

		// For bare repos, we need a different approach since worktrees require existing branches
		// We'll create a temporary repo to make an initial commit, then use that
		tmpDir := absProjectDir + ".tmp"
		if err := g.Init(tmpDir, false); err != nil {
			return "", err
		}
		defer os.RemoveAll(tmpDir)

		// Get current branch name
		branch, _ := g.getCurrentBranch(tmpDir)
		if branch == "" {
			branch = "main" // default
		}

		// Create initial commit
		cmd := exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
		cmd.Dir = tmpDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to create initial commit: %w\n%s", err, out)
		}

		// Push to bare repo
		cmd = exec.Command("git", "push", barePath, branch)
		cmd.Dir = tmpDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to push to bare repo: %w\n%s", err, out)
		}

		// Create main worktree
		mainPath := filepath.Join(absProjectDir, "main")
		if err := os.MkdirAll(mainPath, 0755); err != nil {
			return "", fmt.Errorf("failed to create main directory: %w", err)
		}

		// Add main worktree
		cmd = exec.Command("git", "worktree", "add", mainPath, branch)
		cmd.Dir = barePath
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to create main worktree: %w\n%s", err, out)
		}

		g.RepoPath = barePath
		return mainPath, nil
	}

	// Non-bare: simple init
	if err := g.Init(absProjectDir, false); err != nil {
		return "", err
	}

	// Create initial commit for non-bare repos too
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = absProjectDir
	cmd.Run() // Ignore error

	g.RepoPath = absProjectDir
	return absProjectDir, nil
}

func (g *Git) getCurrentBranch(path string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// HasChanges checks if there are uncommitted changes.
func (g *Git) HasChanges(path string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = path
	out, _ := cmd.Output()
	return len(strings.TrimSpace(string(out))) > 0
}
