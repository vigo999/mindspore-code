package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Check fetches the manifest and compares the current version.
// Returns nil, nil for dev builds.
func Check(ctx context.Context, currentVersion string) (*CheckResult, error) {
	if currentVersion == "dev" || currentVersion == "" {
		return nil, nil
	}

	manifest, err := fetchManifest(ctx)
	if err != nil {
		return nil, err
	}

	result := &CheckResult{
		CurrentVersion: currentVersion,
		LatestVersion:  manifest.Latest,
	}

	if compareSemver(currentVersion, manifest.Latest) < 0 {
		result.UpdateAvailable = true
		result.DownloadURL = buildDownloadURL(manifest.DownloadBase, manifest.Latest)
	}

	if manifest.MinAllowed != "" && compareSemver(currentVersion, manifest.MinAllowed) < 0 {
		result.ForceUpdate = true
		result.UpdateAvailable = true
		result.DownloadURL = buildDownloadURL(manifest.DownloadBase, manifest.Latest)
	}

	return result, nil
}

func fetchManifest(ctx context.Context) (*Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ManifestURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest fetch returned %d", resp.StatusCode)
	}

	var m Manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return &m, nil
}

// compareSemver compares two semver strings (major.minor.patch).
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareSemver(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		result[i] = n
	}
	return result
}

// FetchReleaseNotes fetches the body of a GitHub release by tag.
// Returns empty string on any failure (non-fatal).
func FetchReleaseNotes(ctx context.Context, version string) string {
	version = strings.TrimPrefix(version, "v")
	url := fmt.Sprintf("https://api.github.com/repos/vigo999/ms-cli/releases/tags/v%s", version)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var release struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}
	return release.Body
}

func buildDownloadURL(base, version string) string {
	version = strings.TrimPrefix(version, "v")
	name := fmt.Sprintf("ms-cli-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return fmt.Sprintf("%s/v%s/%s", strings.TrimRight(base, "/"), version, name)
}
