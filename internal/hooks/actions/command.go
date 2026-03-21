package actions

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Command executes a shell command.
type Command struct {
	Cmd    string
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
	Env    []string
}

// Execute runs the command.
func (c *Command) Execute() error {
	// Use sh -c for shell commands
	var cmd *exec.Cmd
	if strings.Contains(c.Cmd, "|") || strings.Contains(c.Cmd, ">") || strings.Contains(c.Cmd, "<") {
		// Complex shell command
		cmd = exec.Command("sh", "-c", c.Cmd)
	} else {
		// Simple command
		parts := strings.Fields(c.Cmd)
		if len(parts) == 0 {
			return fmt.Errorf("empty command")
		}
		cmd = exec.Command(parts[0], parts[1:]...)
	}

	if c.Dir != "" {
		cmd.Dir = c.Dir
	}

	if c.Env != nil {
		cmd.Env = c.Env
	}

	if c.Stdout != nil {
		cmd.Stdout = c.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}

	if c.Stderr != nil {
		cmd.Stderr = c.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// ExecuteShell runs a command with shell.
func ExecuteShell(command, dir string) error {
	cmd := &Command{
		Cmd: command,
		Dir: dir,
	}
	return cmd.Execute()
}
