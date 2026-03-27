package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManifestURLsDefaultOrder(t *testing.T) {
	t.Setenv("MSCODE_MANIFEST_URL", "")

	got := ManifestURLs()
	if len(got) != 2 {
		t.Fatalf("ManifestURLs() len = %d, want 2", len(got))
	}
	if got[0] != defaultMirrorManifestURL {
		t.Fatalf("ManifestURLs()[0] = %q, want %q", got[0], defaultMirrorManifestURL)
	}
	if got[1] != defaultGitHubManifestURL {
		t.Fatalf("ManifestURLs()[1] = %q, want %q", got[1], defaultGitHubManifestURL)
	}
}

func TestManifestURLsEnvOverride(t *testing.T) {
	t.Setenv("MSCODE_MANIFEST_URL", "http://example.test/manifest.json")

	got := ManifestURLs()
	if len(got) != 1 {
		t.Fatalf("ManifestURLs() len = %d, want 1", len(got))
	}
	if got[0] != "http://example.test/manifest.json" {
		t.Fatalf("ManifestURLs()[0] = %q, want env override", got[0])
	}
}

func TestFetchManifestFallsBackToNextURL(t *testing.T) {
	success := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"latest":"0.5.0-beta.2","min_allowed":"","download_base":"http://mirror.example/mscode/releases"}`))
	}))
	defer success.Close()

	got, err := fetchManifestFromURLs(context.Background(), []string{
		"http://127.0.0.1:1/latest/manifest.json",
		success.URL,
	})
	if err != nil {
		t.Fatalf("fetchManifestFromURLs() error = %v", err)
	}
	if got.Latest != "0.5.0-beta.2" {
		t.Fatalf("manifest latest = %q, want %q", got.Latest, "0.5.0-beta.2")
	}
	if got.DownloadBase != "http://mirror.example/mscode/releases" {
		t.Fatalf("manifest download_base = %q", got.DownloadBase)
	}
}
