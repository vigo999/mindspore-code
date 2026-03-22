package update

import (
	"os"
	"path/filepath"
	"runtime"
)

const defaultManifestURL = "https://github.com/vigo999/ms-cli/releases/latest/download/manifest.json"

// InstallDir returns ~/.ms-cli/bin.
func InstallDir() string {
	return filepath.Join(ConfigDir(), "bin")
}

// BinaryPath returns the expected binary path.
func BinaryPath() string {
	name := "mscli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(InstallDir(), name)
}

// ConfigDir returns ~/.ms-cli.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ms-cli")
}

// ManifestURL returns the manifest URL, overridable via MSCLI_MANIFEST_URL.
func ManifestURL() string {
	if u := os.Getenv("MSCLI_MANIFEST_URL"); u != "" {
		return u
	}
	return defaultManifestURL
}
