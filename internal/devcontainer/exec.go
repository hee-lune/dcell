// Package devcontainer provides Dev Container integration.
package devcontainer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// IsDevContainerConfigured checks if .devcontainer exists in the given path.
func IsDevContainerConfigured(ctxPath string) bool {
	dcPath := filepath.Join(ctxPath, ".devcontainer", "devcontainer.json")
	_, err := os.Stat(dcPath)
	return err == nil
}

// GetContainerName returns the expected container name for a context.
func GetContainerName(projectName, ctxName string) string {
	// Docker Compose generates container names like: projectname-servicename-1
	// We use a simplified naming convention
	return fmt.Sprintf("%s-%s", projectName, ctxName)
}

// IsContainerRunning checks if a container is running.
func IsContainerRunning(containerName string) bool {
	cmd := exec.Command("docker", "ps", "-q", "-f", "name="+containerName)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(output) > 0
}

// StartContainer starts the Dev Container for the given context.
func StartContainer(ctxPath string) error {
	// Try docker compose first, then docker-compose
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = ctxPath
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("docker-compose", "up", "-d")
		cmd.Dir = ctxPath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start container: %w", err)
		}
	}
	return nil
}

// ExecInContainer executes a command inside the Dev Container.
func ExecInContainer(containerName string, args []string, stdin *os.File, stdout *os.File, stderr *os.File) error {
	// Build docker exec command
	execArgs := []string{"exec", "-it", containerName}
	execArgs = append(execArgs, args...)

	cmd := exec.Command("docker", execArgs...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

// RunAIInContainer runs the AI CLI inside the Dev Container.
func RunAIInContainer(ctxPath, containerName, aiName string) error {
	// Ensure container is running
	if !IsContainerRunning(containerName) {
		fmt.Printf("Starting Dev Container: %s\n", containerName)
		if err := StartContainer(ctxPath); err != nil {
			return fmt.Errorf("failed to start dev container: %w", err)
		}
		// Wait a bit for container to be ready
		// In production, we should poll for readiness
	}

	// Check if AI CLI is available in container
	checkCmd := exec.Command("docker", "exec", containerName, "which", aiName)
	if err := checkCmd.Run(); err != nil {
		// AI CLI not found in container, show instructions
		fmt.Fprintf(os.Stderr, "Warning: %s is not installed in the Dev Container.\n", aiName)
		fmt.Fprintf(os.Stderr, "Please install it in the container or use host mode.\n")
		return fmt.Errorf("%s not found in container", aiName)
	}

	// Run AI CLI in container
	fmt.Printf("Running %s inside Dev Container: %s\n", aiName, containerName)
	cmd := exec.Command("docker", "exec", "-it", containerName, aiName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
