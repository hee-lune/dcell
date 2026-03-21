// Package hooks provides lifecycle hook execution for dcell.
package hooks

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/heelune/dcell/internal/hooks/actions"
)

// Runner executes hook actions.
type Runner struct {
	ctx    *Context
	stdout io.Writer
	stderr io.Writer
}

// NewRunner creates a new hook runner.
func NewRunner(ctx *Context) *Runner {
	return &Runner{
		ctx:    ctx,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// NewRunnerWithOutput creates a runner with custom output.
func NewRunnerWithOutput(ctx *Context, stdout, stderr io.Writer) *Runner {
	return &Runner{
		ctx:    ctx,
		stdout: stdout,
		stderr: stderr,
	}
}

// ExecutePostCreate runs all post-create hooks.
func (r *Runner) ExecutePostCreate(actions []Action) error {
	if len(actions) == 0 {
		return nil
	}

	fmt.Fprintf(r.stdout, "Running %d post-create hook(s)...\n", len(actions))

	for i, action := range actions {
		if err := r.executeAction(i+1, len(actions), &action); err != nil {
			return err
		}
	}

	return nil
}


// ExecutePreRemove runs all pre-remove hooks.
func (r *Runner) ExecutePreRemove(actions []Action) error {
	if len(actions) == 0 {
		return nil
	}

	fmt.Fprintf(r.stdout, "Running %d pre-remove hook(s)...\n", len(actions))

	for i, action := range actions {
		if err := r.executeAction(i+1, len(actions), &action); err != nil {
			return err
		}
	}

	return nil
}

// executeAction executes a single action.
func (r *Runner) executeAction(index, total int, action *Action) error {
	// Print description if available
	if action.Description != "" {
		fmt.Fprintf(r.stdout, "  [%d/%d] %s\n", index, total, action.Description)
	} else {
		fmt.Fprintf(r.stdout, "  [%d/%d] %s", index, total, action.Type)
		if action.Command != "" {
			fmt.Fprintf(r.stdout, ": %s", action.Command)
		} else if action.From != "" {
			fmt.Fprintf(r.stdout, ": %s -> %s", action.From, action.To)
		}
		fmt.Fprintln(r.stdout)
	}

	// Determine action type
	actionType := action.Type
	if actionType == "" {
		// Infer type from fields
		if action.Command != "" {
			actionType = ActionCommand
		} else if action.From != "" && action.To != "" {
			actionType = ActionCopy
		} else {
			return fmt.Errorf("cannot determine action type")
		}
	}

	// Execute based on type
	var err error
	switch actionType {
	case ActionCopy:
		err = r.executeCopy(action)
	case ActionSymlink:
		err = r.executeSymlink(action)
	case ActionCommand:
		err = r.executeCommand(action)
	case ActionTemplate:
		err = r.executeTemplate(action)
	default:
		return fmt.Errorf("unknown action type: %s", actionType)
	}

	if err != nil {
		onError := action.OnError
		if onError == "" {
			onError = OnErrorContinue
		}

		if onError == OnErrorAbort {
			return fmt.Errorf("hook failed: %w", err)
		}

		// Continue on error
		fmt.Fprintf(r.stderr, "    Warning: %v\n", err)
	}

	return nil
}

// executeCopy executes a copy action.
func (r *Runner) executeCopy(action *Action) error {
	src := r.resolveSourcePath(action.From)
	dst := r.resolveDestPath(action.To)

	// Check condition
	srcExists := fileExists(src)
	dstExists := fileExists(dst)

	if !action.ShouldExecute(srcExists, dstExists) {
		fmt.Fprintf(r.stdout, "    (skipped: condition not met)\n")
		return nil
	}

	if !srcExists {
		return fmt.Errorf("source does not exist: %s", src)
	}

	return actions.Copy(src, dst)
}

// executeSymlink executes a symlink action.
func (r *Runner) executeSymlink(action *Action) error {
	target := r.resolveSourcePath(action.From)
	link := r.resolveDestPath(action.To)

	// Check condition
	srcExists := fileExists(target)
	dstExists := fileExists(link)

	if !action.ShouldExecute(srcExists, dstExists) {
		fmt.Fprintf(r.stdout, "    (skipped: condition not met)\n")
		return nil
	}

	if !srcExists {
		return fmt.Errorf("target does not exist: %s", target)
	}

	return actions.Symlink(target, link)
}

// executeCommand executes a command action.
func (r *Runner) executeCommand(action *Action) error {
	cmd := &actions.Command{
		Cmd:    action.Command,
		Dir:    r.ctx.WorktreePath,
		Stdout: r.stdout,
		Stderr: r.stderr,
	}
	return cmd.Execute()
}

// executeTemplate executes a template action.
func (r *Runner) executeTemplate(action *Action) error {
	src := r.resolveSourcePath(action.From)
	dst := r.resolveDestPath(action.To)

	// Check condition
	srcExists := fileExists(src)
	dstExists := fileExists(dst)

	if !action.ShouldExecute(srcExists, dstExists) {
		fmt.Fprintf(r.stdout, "    (skipped: condition not met)\n")
		return nil
	}

	if !srcExists {
		return fmt.Errorf("template source does not exist: %s", src)
	}

	// Prepare template data
	data := &actions.TemplateData{
		ContextName: r.ctx.ContextName,
		BaseBranch:  r.ctx.BaseBranch,
		VCS:         r.ctx.VCS,
	}

	return actions.RenderTemplate(src, dst, data)
}

// resolveDestPath resolves destination path (relative to worktree).
func (r *Runner) resolveDestPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	// ./ prefix means worktree-relative
	if len(path) >= 2 && path[:2] == "./" {
		return filepath.Join(r.ctx.WorktreePath, path[2:])
	}

	// Default: relative to worktree
	return filepath.Join(r.ctx.WorktreePath, path)
}

// resolveSourcePath resolves source path (relative to project root).
func (r *Runner) resolveSourcePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	// ../ prefix means project-root-relative
	if len(path) >= 3 && path[:3] == "../" {
		return filepath.Join(r.ctx.ProjectRoot, path[3:])
	}

	// Default: relative to project root
	return filepath.Join(r.ctx.ProjectRoot, path)
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
