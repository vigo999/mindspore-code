package workspacefile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// DefaultMaxInlineBytes is the default size cap for inline text expansion.
const DefaultMaxInlineBytes = 64 * 1024

// ResolvePath validates a workspace-relative path and returns its absolute path.
func ResolvePath(workDir, input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("path is required")
	}

	cleaned := filepath.Clean(input)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", input)
	}

	baseAbs, err := filepath.Abs(workDir)
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	fullAbs, err := filepath.Abs(filepath.Join(baseAbs, cleaned))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	rel, err := filepath.Rel(baseAbs, fullAbs)
	if err != nil {
		return "", fmt.Errorf("check path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes working directory: %s", input)
	}

	return fullAbs, nil
}

// ResolveExistingFilePath validates a workspace-relative path and confirms it exists as a file.
func ResolveExistingFilePath(workDir, input string) (string, error) {
	fullPath, err := ResolvePath(workDir, input)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", input)
		}
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory: %s", input)
	}

	return fullPath, nil
}

// ReadTextFile reads a validated workspace-relative file and applies text safety checks.
func ReadTextFile(workDir, input string, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxInlineBytes
	}

	fullPath, err := ResolveExistingFilePath(workDir, input)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > int64(maxBytes) {
		return "", fmt.Errorf("file too large: %s exceeds %d bytes", input, maxBytes)
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	if len(data) > maxBytes {
		return "", fmt.Errorf("file too large: %s exceeds %d bytes", input, maxBytes)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("file is not valid text (contains NUL bytes): %s", input)
	}
	if !utf8.Valid(data) {
		return "", fmt.Errorf("file is not valid UTF-8 text: %s", input)
	}

	return string(data), nil
}
