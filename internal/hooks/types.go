// Package hooks provides lifecycle hook execution for dcell.
package hooks

// ActionType represents the type of hook action.
type ActionType string

const (
	// ActionCopy copies a file or directory.
	ActionCopy ActionType = "copy"
	// ActionSymlink creates a symbolic link.
	ActionSymlink ActionType = "symlink"
	// ActionCommand executes a shell command.
	ActionCommand ActionType = "command"
	// ActionTemplate renders a template file.
	ActionTemplate ActionType = "template"
)

// Condition represents when the hook should execute.
type Condition string

const (
	// ConditionAlways always executes the hook.
	ConditionAlways Condition = "always"
	// ConditionExists executes only if source exists.
	ConditionExists Condition = "exists"
	// ConditionMissing executes only if destination is missing.
	ConditionMissing Condition = "missing"
	// ConditionIfFileChanged executes if source file changed (for templates).
	ConditionIfFileChanged Condition = "if-file-changed"
)

// OnError represents error handling behavior.
type OnError string

const (
	// OnErrorContinue continues execution on error.
	OnErrorContinue OnError = "continue"
	// OnErrorAbort stops execution on error.
	OnErrorAbort OnError = "abort"
)

// Action represents a single hook action.
type Action struct {
	// Type is the action type: copy, symlink, command, template
	Type ActionType `toml:"type"`

	// From is the source path (for copy, symlink, template)
	From string `toml:"from,omitempty"`

	// To is the destination path (for copy, symlink, template)
	To string `toml:"to,omitempty"`

	// Command is the shell command to execute (for command type)
	Command string `toml:"command,omitempty"`

	// Description is a human-readable description of the action
	Description string `toml:"description,omitempty"`

	// Condition determines when to execute: always, exists, missing, if-file-changed
	Condition Condition `toml:"condition,omitempty"`

	// OnError determines error handling: continue (default), abort
	OnError OnError `toml:"on_error,omitempty"`
}

// ShouldExecute returns true if the action should execute based on its condition.
func (a *Action) ShouldExecute(srcExists, dstExists bool) bool {
	switch a.Condition {
	case ConditionExists, "": // default
		return srcExists
	case ConditionMissing:
		return !dstExists
	case ConditionAlways:
		return true
	default:
		return true
	}
}

// Config holds all lifecycle hooks.
type Config struct {
	// PostCreate runs after creating a new worktree
	PostCreate []Action `toml:"post-create"`

	// PreRemove runs before removing a context
	PreRemove []Action `toml:"pre-remove"`
}

// Context provides execution context for hooks.
type Context struct {
	// ProjectRoot is the dcell project root (where .bare/ is located)
	ProjectRoot string

	// WorktreePath is the target worktree directory
	WorktreePath string

	// ContextName is the name of the context being created/switched
	ContextName string

	// BaseBranch is the base branch for new contexts
	BaseBranch string

	// VCS is the version control system type (git or jj)
	VCS string
}
