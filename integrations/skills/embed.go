package skills

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed builtin
var builtinSkills embed.FS

// ExtractBuiltin writes the embedded skill files to destDir if not
// already present. Existing files on disk are not overwritten, so
// user-installed skills in the same directory take precedence.
func ExtractBuiltin(destDir string) error {
	return fs.WalkDir(builtinSkills, "builtin", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Strip the "builtin/" prefix to get the relative path.
		rel, err := filepath.Rel("builtin", path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		dest := filepath.Join(destDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		// Skip if file already exists on disk.
		if _, err := os.Stat(dest); err == nil {
			return nil
		}

		data, err := builtinSkills.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
}
