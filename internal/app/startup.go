package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/agent/loop"
	"github.com/vigo999/ms-cli/integrations/skills"
	"github.com/vigo999/ms-cli/ui/model"
	"github.com/vigo999/ms-cli/ui/slash"
)

const skillsNoteReadyDuration = 5 * time.Minute

const bootReadyToken = "__boot_ready__"

func buildSystemPrompt(summaries []skills.SkillSummary) string {
	systemPrompt := loop.DefaultSystemPrompt()
	if len(summaries) == 0 {
		return systemPrompt
	}
	return systemPrompt + "\n\n## Available Skills\n\n" +
		"Use the load_skill tool to load a skill when the user's task matches one:\n\n" +
		skills.FormatSummaries(summaries)
}

func registerSkillCommands(summaries []skills.SkillSummary) {
	for _, s := range summaries {
		slash.Register(slash.Command{
			Name:        "/" + s.Name,
			Description: s.Description,
			Usage:       "/" + s.Name + " [request...]",
		})
	}
}

func (a *Application) startDeferredStartup() {
	if a == nil {
		return
	}
	a.startupOnce.Do(func() {
		if strings.TrimSpace(a.skillsHomeDir) != "" {
			go a.syncSharedSkills()
		}
	})
}

func (a *Application) syncSharedSkills() {
	if a == nil || strings.TrimSpace(a.skillsHomeDir) == "" {
		return
	}

	repoSync := skills.NewRepoSync(skills.RepoSyncConfig{
		HomeDir:   a.skillsHomeDir,
		LogWriter: io.Discard,
	})

	repoDir := skills.SyncedRepoDir(a.skillsHomeDir)
	firstClone := !dirExistsCheck(repoDir)

	if err := repoSync.Sync(); err != nil {
		a.emitToolError("skills", "sync shared skills repo: %v", err)
		return
	}

	if err := a.installRepoSkills(); err != nil {
		a.emitToolError("skills", "install repo skills: %v", err)
	}

	a.refreshSkillCatalog()

	if firstClone {
		version := skills.ReadVersion(skills.SyncedRepoDir(a.skillsHomeDir))
		a.emitSkillsNote(fmt.Sprintf("mindspore-skills %s loaded", fmtVersion(version)))
		a.clearSkillsNoteAfter(skillsNoteReadyDuration)
		return
	}

	info, err := repoSync.CheckUpdate()
	if err != nil {
		// Network error checking for updates is not fatal — skills are loaded.
		return
	}
	if info.Available {
		a.emitSkillsNote(fmt.Sprintf(
			"mindspore-skills update: %s → %s (/skill-update)",
			fmtVersion(info.LocalVersion), fmtVersion(info.RemoteVersion),
		))
	}
}

func (a *Application) refreshSkillCatalog() {
	if a == nil || a.skillLoader == nil {
		return
	}

	summaries := a.skillLoader.List()
	registerSkillCommands(summaries)

	if a.ctxManager != nil {
		a.ctxManager.SetSystemPrompt(buildSystemPrompt(summaries))
	}
	if err := a.persistSessionSnapshot(); err != nil {
		a.emitToolError("session", "Failed to persist session snapshot: %v", err)
	}
}

func (a *Application) emitSkillsNote(note string) {
	if a == nil || a.EventCh == nil || strings.TrimSpace(note) == "" {
		return
	}
	a.EventCh <- model.Event{
		Type:    model.SkillsNoteUpdate,
		Message: note,
	}
}

func (a *Application) clearSkillsNoteAfter(d time.Duration) {
	if a == nil || a.EventCh == nil {
		return
	}
	go func() {
		time.Sleep(d)
		a.EventCh <- model.Event{
			Type:    model.SkillsNoteUpdate,
			Message: "",
		}
	}()
}

func (a *Application) cmdSkillUpdate() {
	if a == nil || strings.TrimSpace(a.skillsHomeDir) == "" {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Skills home directory not configured.",
		}
		return
	}

	repoSync := skills.NewRepoSync(skills.RepoSyncConfig{
		HomeDir:   a.skillsHomeDir,
		LogWriter: io.Discard,
	})

	if err := repoSync.ApplyUpdate(); err != nil {
		a.emitToolError("skills", "update shared skills repo: %v", err)
		return
	}

	if err := a.installRepoSkills(); err != nil {
		a.emitToolError("skills", "install repo skills: %v", err)
	}

	a.refreshSkillCatalog()

	version := skills.ReadVersion(skills.SyncedRepoDir(a.skillsHomeDir))
	a.emitSkillsNote(fmt.Sprintf("mindspore-skills %s loaded", fmtVersion(version)))
	a.clearSkillsNoteAfter(skillsNoteReadyDuration)
}

// fmtVersion prefixes "v" only when the version is a real semver string.
func fmtVersion(v string) string {
	if v == "" || v == "unknown" {
		return v
	}
	return "v" + v
}

// installRepoSkills copies skill directories from the synced repo into ~/.ms-cli/skills/.
func (a *Application) installRepoSkills() error {
	repoSkillsDir := skills.SyncedSkillsDir(a.skillsHomeDir)
	if !dirExistsCheck(repoSkillsDir) {
		return nil
	}

	destDir := filepath.Join(a.skillsHomeDir, ".ms-cli", "skills")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}

	entries, err := os.ReadDir(repoSkillsDir)
	if err != nil {
		return fmt.Errorf("read repo skills dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		src := filepath.Join(repoSkillsDir, entry.Name())
		dst := filepath.Join(destDir, entry.Name())
		// Remove old copy and replace with fresh one.
		_ = os.RemoveAll(dst)
		if err := copyDir(src, dst); err != nil {
			return fmt.Errorf("copy skill %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// dirExistsCheck is a local helper to avoid importing skills.dirExists (unexported).
func dirExistsCheck(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
