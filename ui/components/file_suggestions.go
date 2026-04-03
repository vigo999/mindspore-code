package components

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type fileSuggestionProvider struct {
	workDir string
	cached  []string
	loaded  bool
}

func newFileSuggestionProvider(workDir string) *fileSuggestionProvider {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return nil
	}
	return &fileSuggestionProvider{workDir: workDir}
}

func (p *fileSuggestionProvider) suggestions(prefix string) []suggestionItem {
	if p == nil {
		return nil
	}
	if err := p.load(); err != nil {
		return nil
	}

	prefix = normalizeSuggestionPath(prefix)
	items := make([]suggestionItem, 0, len(p.cached))
	for _, path := range p.cached {
		if prefix != "" && !strings.HasPrefix(path, prefix) {
			continue
		}
		items = append(items, suggestionItem{
			Value:       path,
			Display:     path,
			Description: "file",
			Kind:        suggestionKindFile,
		})
	}
	return items
}

func (p *fileSuggestionProvider) load() error {
	if p.loaded {
		return nil
	}

	paths := make([]string, 0, 256)
	err := filepath.WalkDir(p.workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(p.workDir, path)
		if err != nil {
			return nil
		}
		paths = append(paths, normalizeSuggestionPath(rel))
		return nil
	})
	if err != nil {
		return err
	}

	sort.Strings(paths)
	p.cached = paths
	p.loaded = true
	return nil
}

func normalizeSuggestionPath(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(filepath.Clean(path)), "//", "/")
}
