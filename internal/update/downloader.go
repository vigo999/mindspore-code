package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const maxBinarySize = 500 << 20 // 500 MiB

// Download fetches the binary at url into ~/.ms-cli/tmp/ and returns the temp file path.
func Download(ctx context.Context, url string) (string, error) {
	tmpDir := filepath.Join(ConfigDir(), "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("create tmp dir: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp(tmpDir, "ms-cli-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	limited := &io.LimitedReader{R: resp.Body, N: maxBinarySize}
	if _, err := io.Copy(tmpFile, limited); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write binary: %w", err)
	}

	// Check if we hit the limit (stream had more data).
	if limited.N == 0 {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("download exceeds maximum size (%d MiB)", maxBinarySize>>20)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close temp file: %w", err)
	}

	return tmpPath, nil
}
