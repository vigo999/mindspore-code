package update

import (
	"os"
	"path/filepath"
	"runtime"
)

const defaultManifestURL = "https://github.com/vigo999/mindspore-code/releases/latest/download/manifest.json"

// InstallDir returns ~/.mscode/bin.
func InstallDir() string {
	return filepath.Join(ConfigDir(), "bin")
}

// BinaryPath returns the expected binary path.
func BinaryPath() string {
	name := "mscode"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(InstallDir(), name)
}

// ConfigDir returns ~/.mscode.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mscode")
}

// ManifestURL returns the manifest URL, overridable via MSCODE_MANIFEST_URL.
func ManifestURL() string {
	if u := os.Getenv("MSCODE_MANIFEST_URL"); u != "" {
		return u
	}
	return defaultManifestURL
}
