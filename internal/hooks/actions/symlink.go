package actions

import (
	"fmt"
	"os"
	"path/filepath"
)

// Symlink creates a symbolic link.
// On Windows, it may create a junction for directories.
func Symlink(target, link string) error {
	// Ensure link parent directory exists
	linkDir := filepath.Dir(link)
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		return fmt.Errorf("failed to create link directory: %w", err)
	}

	// Remove existing link if it exists
	if _, err := os.Lstat(link); err == nil {
		if err := os.Remove(link); err != nil {
			return fmt.Errorf("failed to remove existing link: %w", err)
		}
	}

	// Create the symlink
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// IsSymlink checks if path is a symbolic link.
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}
