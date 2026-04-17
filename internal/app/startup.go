package app

import (
	"github.com/mindspore-lab/mindspore-cli/agent/loop"
	"github.com/mindspore-lab/mindspore-cli/integrations/skills"
	"github.com/mindspore-lab/mindspore-cli/ui/slash"
)

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

// builtinCommandSkills lists skill names that are already registered as
// built-in slash commands (e.g., /diagnose, /fix). These are not re-registered
// from the skill catalog to avoid duplicates. Backend agent skills (e.g.,
// failure-agent, accuracy-agent) are also excluded — they are invoked
// internally by the built-in commands, not by the user directly.
var hiddenSkills = map[string]bool{
	"failure-agent":     true,
	"accuracy-agent":    true,
	"performance-agent": true,
	"migrate-agent":     true,
	"algorithm-agent":   true,
	"operator-agent":    true,
	"readiness-agent":   true,
	"api-helper":        true,
}

func registerSkillCommands(summaries []skills.SkillSummary) {
	for _, s := range summaries {
		if hiddenSkills[s.Name] {
			continue
		}
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
	a.startupOnce.Do(func() {})
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
