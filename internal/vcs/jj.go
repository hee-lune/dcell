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
	RepoPath string // Path to .bare/ directory
}

// Name returns the VCS name.
func (j *JJ) Name() string {
	return "jj"
}

// Detect checks if the repository uses jj.
// For bare-first design, looks for .bare directory.
func (j *JJ) Detect(repoPath string) bool {
	// First, check if there's a .bare directory with jj
	barePath := filepath.Join(repoPath, ".bare")
	if _, err := os.Stat(barePath); err == nil {
		cmd := exec.Command("jj", "root", "--ignore-working-copy")
		cmd.Dir = barePath
		if err := cmd.Run(); err == nil {
			j.RepoPath = barePath
			return true
		}
	}

	// Fallback: check if current dir is a jj workspace
	cmd := exec.Command("jj", "root", "--ignore-working-copy")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		return false
	}

	// Try to find .bare/ from jj config
	// JJ workspaces link to a common repo
	cmd = exec.Command("jj", "root")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	root := strings.TrimSpace(string(out))
	parent := filepath.Dir(root)
	if _, err := os.Stat(filepath.Join(parent, ".bare")); err == nil {
		j.RepoPath = filepath.Join(parent, ".bare")
		return true
	}

	return false
}

// DetectBare looks for .bare directory with jj in current or parent directories.
func (j *JJ) DetectBare(startPath string) (string, error) {
	current := startPath
	for {
		barePath := filepath.Join(current, ".bare")
		if info, err := os.Stat(barePath); err == nil && info.IsDir() {
			cmd := exec.Command("jj", "root", "--ignore-working-copy")
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

	return "", fmt.Errorf("no .bare directory with jj found in %s or its parents", startPath)
}

// CreateContext creates a new jj workspace.
func (j *JJ) CreateContext(name string, base string) (*Context, error) {
	if j.RepoPath == "" {
		return nil, fmt.Errorf("repository not detected")
	}

	projectRoot := filepath.Dir(j.RepoPath)
	ctxPath := filepath.Join(projectRoot, name)

	args := []string{"workspace", "create", "--name", name}
	if base != "" {
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
	if j.RepoPath == "" {
		return fmt.Errorf("repository not detected")
	}

	projectRoot := filepath.Dir(j.RepoPath)
	ctxPath := filepath.Join(projectRoot, name)

	cmd := exec.Command("jj", "root")
	cmd.Dir = ctxPath
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("workspace %s not found or invalid: %w", name, err)
	}

	return nil
}

// ListContexts lists all jj workspaces.
func (j *JJ) ListContexts() ([]Context, error) {
	if j.RepoPath == "" {
		return nil, fmt.Errorf("repository not detected")
	}

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

		if base, err := j.getBaseRevision(path); err == nil {
			ctx.BaseBranch = base
		}

		contexts = append(contexts, ctx)
	}

	return contexts, nil
}

// RemoveContext removes a workspace.
func (j *JJ) RemoveContext(name string, force bool) error {
	if j.RepoPath == "" {
		return fmt.Errorf("repository not detected")
	}

	cmd := exec.Command("jj", "workspace", "forget", name)
	cmd.Dir = j.RepoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to forget workspace: %w\n%s", err, out)
	}

	projectRoot := filepath.Dir(j.RepoPath)
	ctxPath := filepath.Join(projectRoot, name)
	if err := os.RemoveAll(ctxPath); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	return nil
}

// CurrentContext returns the current workspace.
func (j *JJ) CurrentContext() (*Context, error) {
	cmd := exec.Command("jj", "workspace", "name")
	cmd.Dir = "."
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current workspace: %w", err)
	}

	name := strings.TrimSpace(string(out))

	cmd = exec.Command("jj", "root")
	cmd.Dir = "."
	out, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	path := strings.TrimSpace(string(out))

	return &Context{
		Name: name,
		Path: path,
		VCS:  "jj",
	}, nil
}

// Init initializes a new Jujutsu repository.
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

	cmd = exec.Command("jj", "describe", "-m", "Initial commit")
	cmd.Dir = repoPath
	cmd.Run()

	j.RepoPath = repoPath
	return nil
}

// Clone clones a remote repository with Jujutsu.
func (j *JJ) Clone(url string, dest string, branch string) error {
	git := &Git{}
	if err := git.Clone(url, dest, branch); err != nil {
		return err
	}

	cmd := exec.Command("jj", "git", "init", "--colocate")
	cmd.Dir = dest
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to initialize jj: %w\n%s", err, out)
	}

	j.RepoPath = dest
	return nil
}

// InitBareProject initializes a bare repo project structure with JJ.
func (j *JJ) InitBareProject(projectDir string) (string, error) {
	absProjectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	barePath := filepath.Join(absProjectDir, ".bare")

	// Create bare git repo with jj
	if err := j.Init(barePath, true); err != nil {
		return "", err
	}

	// Add main workspace
	mainPath := filepath.Join(absProjectDir, "main")
	if err := j.AddMainWorktree(barePath, mainPath); err != nil {
		return "", err
	}

	j.RepoPath = barePath
	return mainPath, nil
}

// CloneBare clones a repository as bare and adds main workspace.
func (j *JJ) CloneBare(url string, barePath string, branch string) error {
	// Use git to clone bare
	git := &Git{}
	if err := git.CloneBare(url, barePath, branch); err != nil {
		return err
	}

	// Add jj colocate
	cmd := exec.Command("jj", "git", "init", "--colocate")
	cmd.Dir = barePath
	if out, err := cmd.CombinedOutput(); err != nil {
		// If jj init fails, the bare repo still exists
		return fmt.Errorf("failed to initialize jj: %w\n%s", err, out)
	}

	j.RepoPath = barePath
	return nil
}

// AddMainWorktree adds a main workspace to a bare repository.
func (j *JJ) AddMainWorktree(barePath string, mainPath string) error {
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		return fmt.Errorf("failed to create main directory: %w", err)
	}

	cmd := exec.Command("jj", "workspace", "create", "--name", "main", mainPath)
	cmd.Dir = barePath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create main workspace: %w\n%s", err, out)
	}

	return nil
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
