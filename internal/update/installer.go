package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Install replaces the current binary with the downloaded one.
// It backs up the existing binary to .bak and rolls back on failure.
func Install(downloadedPath string) error {
	target := BinaryPath()
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}

	backup := target + ".bak"

	// Back up existing binary if it exists.
	os.Remove(backup)
	if err := os.Rename(target, backup); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("backup existing binary: %w", err)
	}

	// Move downloaded binary into place.
	if err := os.Rename(downloadedPath, target); err != nil {
		rollback(backup, target)
		return fmt.Errorf("install binary: %w", err)
	}

	// Set executable permissions.
	if err := os.Chmod(target, 0755); err != nil {
		os.Remove(target)
		rollback(backup, target)
		return fmt.Errorf("chmod binary: %w", err)
	}

	// Clean up backup.
	os.Remove(backup)

	return nil
}

// rollback restores the backup file to the target path if the backup exists.
func rollback(backup, target string) {
	if err := os.Rename(backup, target); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Best effort; nothing more we can do.
	}
}
