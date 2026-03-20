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
	RepoPath string // Path to .bare/ directory
}

// Name returns the VCS name.
func (g *Git) Name() string {
	return "git"
}

// Detect checks if the repository uses git.
// For bare-first design, looks for .bare/ directory.
func (g *Git) Detect(repoPath string) bool {
	// First, check if there's a .bare directory
	barePath := filepath.Join(repoPath, ".bare")
	if _, err := os.Stat(barePath); err == nil {
		// Verify it's a valid git repo
		cmd := exec.Command("git", "rev-parse", "--git-dir")
		cmd.Dir = barePath
		if cmd.Run() == nil {
			g.RepoPath = barePath
			return true
		}
	}

	// Fallback: check if current dir is a worktree
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return false
	}

	// Try to find .bare/ from gitdir link
	gitDir := filepath.Join(repoPath, ".git")
	if data, err := os.ReadFile(gitDir); err == nil {
		content := string(data)
		if strings.HasPrefix(content, "gitdir: ") {
			// This is a worktree, try to find the bare repo
			gitDirPath := strings.TrimSpace(strings.TrimPrefix(content, "gitdir: "))
			// gitdir points to .bare/worktrees/<name>
			parent := filepath.Dir(filepath.Dir(gitDirPath))
			if _, err := os.Stat(filepath.Join(parent, "config")); err == nil {
				g.RepoPath = parent
				return true
			}
		}
	}

	return false
}

// DetectBare looks for .bare directory in current or parent directories.
func (g *Git) DetectBare(startPath string) (string, error) {
	current := startPath
	for {
		barePath := filepath.Join(current, ".bare")
		if info, err := os.Stat(barePath); err == nil && info.IsDir() {
			// Verify it's a valid bare git repo
			cmd := exec.Command("git", "rev-parse", "--git-dir")
			cmd.Dir = barePath
			if err := cmd.Run(); err == nil {
				return barePath, nil
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}

	return "", fmt.Errorf("no .bare directory found in %s or its parents", startPath)
}

// CreateContext creates a new git worktree with a new branch.
func (g *Git) CreateContext(name string, base string) (*Context, error) {
	if g.RepoPath == "" {
		return nil, fmt.Errorf("repository not detected")
	}

	// Get project root from .bare path
	projectRoot := filepath.Dir(g.RepoPath)
	ctxPath := filepath.Join(projectRoot, name)

	if base == "" {
		base = "HEAD"
	}

	// Create new branch and worktree: git worktree add -b <name> <path> <base>
	cmd := exec.Command("git", "worktree", "add", "-b", name, ctxPath, base)
	cmd.Dir = g.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create git worktree: %w\n%s", err, out)
	}

	return &Context{
		Name:       name,
		Path:       ctxPath,
		BaseBranch: base,
		VCS:        "git",
	}, nil
}

// SwitchContext switches to an existing worktree.
func (g *Git) SwitchContext(name string) error {
	if g.RepoPath == "" {
		return fmt.Errorf("repository not detected")
	}

	projectRoot := filepath.Dir(g.RepoPath)
	ctxPath := filepath.Join(projectRoot, name)

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
	if g.RepoPath == "" {
		return nil, fmt.Errorf("repository not detected")
	}

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
func (g *Git) RemoveContext(name string, force bool) error {
	if g.RepoPath == "" {
		return fmt.Errorf("repository not detected")
	}

	projectRoot := filepath.Dir(g.RepoPath)
	ctxPath := filepath.Join(projectRoot, name)

	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, ctxPath)

	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w\n%s", err, out)
	}

	return nil
}

// CurrentContext returns the current worktree.
func (g *Git) CurrentContext() (*Context, error) {
	if g.RepoPath == "" {
		return nil, fmt.Errorf("repository not detected")
	}

	// Get current path
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = "."
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

	g.RepoPath = repoPath
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

// InitBareProject initializes a bare repo project structure.
// Creates: projectDir/.bare/, projectDir/main/, and initial commit
func (g *Git) InitBareProject(projectDir string) (string, error) {
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	barePath := filepath.Join(absProjectDir, ".bare")

	// Create bare repository
	if err := g.Init(barePath, true); err != nil {
		return "", err
	}

	// Create a temporary repo to make initial commit
	tmpDir := filepath.Join(os.TempDir(), fmt.Sprintf("dcell-init-%d", os.Getpid()))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// Init temp repo
	tmpGit := &Git{}
	if err := tmpGit.Init(tmpDir, false); err != nil {
		return "", err
	}

	// Create initial commit
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create initial commit: %w\n%s", err, out)
	}

	// Get branch name
	branch, _ := tmpGit.getCurrentBranch(tmpDir)
	if branch == "" {
		branch = "main"
	}

	// Push to bare repo
	cmd = exec.Command("git", "push", barePath, branch)
	cmd.Dir = tmpDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to push to bare repo: %w\n%s", err, out)
	}

	// Add main worktree directly under project root
	mainPath := filepath.Join(absProjectDir, "main")
	if err := g.AddMainWorktree(barePath, mainPath); err != nil {
		return "", err
	}

	g.RepoPath = barePath
	return mainPath, nil
}

// CloneBare clones a repository as bare and adds main worktree.
func (g *Git) CloneBare(url string, barePath string, branch string) error {
	args := []string{"clone", "--bare"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, barePath)

	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone repository: %w\n%s", err, out)
	}

	g.RepoPath = barePath
	return nil
}

// AddMainWorktree adds a main worktree to a bare repository.
func (g *Git) AddMainWorktree(barePath string, mainPath string) error {
	// Ensure main directory exists
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		return fmt.Errorf("failed to create main directory: %w", err)
	}

	// Determine default branch
	cmd := exec.Command("git", "symbolic-ref", "HEAD")
	cmd.Dir = barePath
	out, err := cmd.Output()
	if err != nil {
		// Default to main
		cmd = exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/main")
		cmd.Dir = barePath
		cmd.Run()
	}

	branch := strings.TrimPrefix(strings.TrimSpace(string(out)), "refs/heads/")
	if branch == "" {
		branch = "main"
	}

	// Add worktree
	cmd = exec.Command("git", "worktree", "add", mainPath, branch)
	cmd.Dir = barePath
	if _, err := cmd.CombinedOutput(); err != nil {
		// Try with force if branch doesn't exist
		cmd = exec.Command("git", "worktree", "add", "-B", branch, mainPath)
		cmd.Dir = barePath
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create main worktree: %w\n%s", err, out)
		}
	}

	return nil
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
