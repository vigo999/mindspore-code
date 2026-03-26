package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/vigo999/ms-cli/integrations/llm"
	"github.com/vigo999/ms-cli/permission"
	"github.com/vigo999/ms-cli/ui/model"
)

func (a *Application) handleCommand(input string) {
	cmd, ok := splitRawCommand(input)
	if !ok {
		return
	}
	args := strings.Fields(cmd.Remainder)

	switch cmd.Name {
	case "/model":
		a.cmdModel(args)
	case "/exit":
		a.cmdExit()
	case "/compact":
		a.cmdCompact()
	case "/clear":
		a.cmdClear()
	case "/test":
		a.cmdTest()
	case "/permission":
		a.cmdPermission(args)
	case "/yolo":
		a.cmdYolo()
	case "/train":
		a.cmdTrain(args)
	case "/project":
		a.cmdProjectInput(cmd.Remainder)
	case "/login":
		a.cmdLogin(args)
	case "/report":
		expanded, err := a.expandReportInput(cmd.Remainder)
		if err != nil {
			a.emitInputExpansionError(err)
			return
		}
		a.cmdUnifiedReport(expanded)
	case "/issues":
		a.cmdIssues(args)
	case "/__issue_detail":
		a.cmdIssueDetail(args)
	case "/__issue_note":
		a.cmdIssueNoteInput(cmd.Remainder)
	case "/__issue_claim":
		a.cmdIssueClaim(args)
	case "/status":
		a.cmdIssueStatus(args)
	case "/diagnose":
		expanded, err := a.expandIssueCommandInput(cmd.Remainder)
		if err != nil {
			a.emitInputExpansionError(err)
			return
		}
		a.cmdDiagnose(expanded)
	case "/fix":
		expanded, err := a.expandIssueCommandInput(cmd.Remainder)
		if err != nil {
			a.emitInputExpansionError(err)
			return
		}
		a.cmdFix(expanded)
	case "/bugs":
		a.cmdBugs(args)
	case "/__bug_detail":
		a.cmdBugDetail(args)
	case "/claim":
		a.cmdClaim(args)
	case "/close":
		a.cmdClose(args)
	case "/dock":
		a.cmdDock()
	case "/skill":
		if err := a.handleRawSkillCommand(cmd.Remainder); err != nil {
			a.emitInputExpansionError(err)
		}
	case "/skill-add":
		a.cmdSkillAddInput(cmd.Remainder)
	case "/skill-update":
		a.cmdSkillUpdate()
	case "/help":
		a.cmdHelp()
	default:
		if handled, err := a.handleSkillAliasCommand(cmd.Name, cmd.Remainder); handled {
			if err != nil {
				a.emitInputExpansionError(err)
			}
			return
		}
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: fmt.Sprintf("Unknown command: %s. Type /help for available commands.", cmd.Name),
		}
	}
}

func (a *Application) handleRawSkillCommand(rawInput string) error {
	if strings.TrimSpace(rawInput) == "" {
		a.cmdSkill(nil)
		return nil
	}

	skillName, request := splitFirstToken(rawInput)
	if skillName == "" {
		a.cmdSkill(nil)
		return nil
	}

	if request != "" {
		expanded, err := a.expandInputText(request)
		if err != nil {
			return err
		}
		request = expanded
	}

	a.runSkillCommand(skillName, request)
	return nil
}

func (a *Application) handleSkillAliasCommand(commandName, rawRemainder string) (bool, error) {
	if a.skillLoader == nil {
		return false, nil
	}

	skillName := strings.TrimPrefix(strings.TrimSpace(commandName), "/")
	if skillName == "" {
		return false, nil
	}
	if _, err := a.skillLoader.Load(skillName); err != nil {
		return false, nil
	}

	request := strings.TrimSpace(rawRemainder)
	if request != "" {
		expanded, err := a.expandInputText(request)
		if err != nil {
			return true, err
		}
		request = expanded
	}

	a.runSkillCommand(skillName, request)
	return true, nil
}

func (a *Application) cmdModel(args []string) {
	if len(args) == 0 {
		a.showCurrentModel()
		return
	}

	modelArg := args[0]
	if strings.Contains(modelArg, ":") {
		parts := strings.SplitN(modelArg, ":", 2)
		providerName := llm.NormalizeProvider(parts[0])
		modelName := strings.TrimSpace(parts[1])
		if !llm.IsSupportedProvider(providerName) {
			a.EventCh <- model.Event{
				Type:    model.AgentReply,
				Message: fmt.Sprintf("Unsupported provider prefix: %s (supported: openai-completion, openai-responses, anthropic)", providerName),
			}
			return
		}
		a.switchModel(providerName, modelName)
		return
	}

	a.switchModel("", modelArg)
}

func (a *Application) showCurrentModel() {
	providerName := a.Config.Model.Provider
	if providerName == "" {
		providerName = "openai-completion"
	}
	modelName := a.Config.Model.Model
	url := a.Config.Model.URL
	if url == "" {
		url = "https://api.openai.com/v1"
	}

	apiKeyStatus := "not set"
	if strings.TrimSpace(a.Config.Model.Key) != "" {
		apiKeyStatus = "set"
	}

	msg := fmt.Sprintf(`Current Model Configuration:

  Provider: %s
  URL:   %s
  Model: %s
  Key:   %s

To switch model:
  /model <model-name>
  /model <provider>:<model>

Examples:
  /model gpt-4o
  /model openai-completion:gpt-4o-mini
  /model openai-responses:gpt-4o
  /model anthropic:claude-3-5-sonnet`, providerName, url, modelName, apiKeyStatus)

	a.EventCh <- model.Event{Type: model.AgentReply, Message: msg}
}

func (a *Application) switchModel(providerName, modelName string) {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	err := a.SetProvider(providerName, modelName, "")
	if err != nil {
		a.EventCh <- model.Event{
			Type:     model.ToolError,
			ToolName: "model",
			Message:  fmt.Sprintf("Failed to switch model: %v", err),
		}
		return
	}

	a.EventCh <- model.Event{
		Type:    model.ModelUpdate,
		Message: a.Config.Model.Model,
		CtxMax:  a.Config.Context.Window,
	}

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Model switched to: %s", a.Config.Model.Model),
	}
}

func (a *Application) cmdExit() {
	a.EventCh <- model.Event{Type: model.AgentReply, Message: "Goodbye!"}
	go func() {
		time.Sleep(100 * time.Millisecond)
		a.EventCh <- model.Event{Type: model.Done}
	}()
}

func (a *Application) cmdCompact() {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	if a.Engine != nil {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Context compacted. Conversation summary has been created to save tokens.",
		}
	} else {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Context compaction is not available.",
		}
	}
}

func (a *Application) cmdClear() {
	a.EventCh <- model.Event{Type: model.ClearScreen, Message: "Chat history cleared."}
}

func (a *Application) cmdTest() {
	a.EventCh <- model.Event{Type: model.AgentThinking}

	modelName := a.Config.Model.Model
	url := a.Config.Model.URL
	if url == "" {
		url = "https://api.openai.com/v1"
	}
	apiKeyStatus := "not set"
	if a.Config.Model.Key != "" {
		apiKeyStatus = fmt.Sprintf("set (%d chars)", len(a.Config.Model.Key))
	}

	msg := fmt.Sprintf("API Connection Test:\n\n  URL:     %s\n  Model:   %s\n  API Key: %s\n\nTesting connectivity...",
		url, modelName, apiKeyStatus)
	a.EventCh <- model.Event{Type: model.AgentReply, Message: msg}

	if a.Engine != nil && a.llmReady {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "API configuration looks correct. Send a message to test the connection.",
		}
	} else {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: provideAPIKeyFirstMsg}
	}
}

func (a *Application) cmdPermission(args []string) {
	permSvc, ok := a.permService.(*permission.DefaultPermissionService)
	if !ok {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Permission management not available in current mode.",
		}
		return
	}

	if len(args) == 0 {
		policies := permSvc.GetPolicies()
		msg := "Current Permission Settings:\n\n"
		if len(policies) == 0 {
			msg += "  No custom permissions set.\n"
			msg += "  Default: ask for destructive operations (write, edit, shell)\n"
		} else {
			for tool, level := range policies {
				msg += fmt.Sprintf("  %s: %s\n", tool, level)
			}
		}
		msg += "\nUsage:\n  /permission <tool> <level>\n"
		msg += "\nLevels: ask, allow_once, allow_session, allow_always, deny\n"
		msg += "Tools: read, write, edit, grep, glob, shell\n"
		msg += "\nExamples:\n  /permission shell ask\n  /permission write allow_always"
		a.EventCh <- model.Event{Type: model.AgentReply, Message: msg}
		return
	}

	if len(args) < 2 {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /permission <tool> <level>\nExample: /permission shell ask",
		}
		return
	}

	tool := args[0]
	level := permission.ParsePermissionLevel(args[1])
	permSvc.Grant(tool, level)

	a.EventCh <- model.Event{
		Type:    model.AgentReply,
		Message: fmt.Sprintf("Permission for '%s' set to: %s", tool, level),
	}
}

func (a *Application) cmdYolo() {
	permSvc, ok := a.permService.(*permission.DefaultPermissionService)
	if !ok {
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "YOLO mode not available in current configuration.",
		}
		return
	}

	current := permSvc.Check("shell", "")
	if current == permission.PermissionAllowAlways {
		permSvc.Grant("shell", permission.PermissionAsk)
		permSvc.Grant("write", permission.PermissionAsk)
		permSvc.Grant("edit", permission.PermissionAsk)
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "YOLO mode disabled. Will ask for confirmation on destructive operations.",
		}
	} else {
		permSvc.Grant("shell", permission.PermissionAllowAlways)
		permSvc.Grant("write", permission.PermissionAllowAlways)
		permSvc.Grant("edit", permission.PermissionAllowAlways)
		permSvc.Grant("read", permission.PermissionAllowAlways)
		permSvc.Grant("grep", permission.PermissionAllowAlways)
		permSvc.Grant("glob", permission.PermissionAllowAlways)
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "YOLO mode enabled! All operations will be auto-approved. Use with caution!",
		}
	}
}

func (a *Application) cmdSkill(args []string) {
	if a.skillLoader == nil {
		a.EventCh <- model.Event{Type: model.AgentReply, Message: "Skills not available."}
		return
	}
	if len(args) == 0 {
		a.emitAvailableSkills(true)
		return
	}

	skillName := args[0]
	userRequest := strings.TrimSpace(strings.Join(args[1:], " "))
	a.runSkillCommand(skillName, userRequest)
}

func (a *Application) runSkillCommand(skillName, userRequest string) {
	content, err := a.skillLoader.Load(skillName)
	if err != nil {
		a.EventCh <- model.Event{
			Type:    model.ToolError,
			Message: fmt.Sprintf("Failed to load skill %q: %v", skillName, err),
		}
		return
	}

	// Inject a synthetic assistant tool_call + tool result into context so the
	// model sees the skill as already loaded and won't call load_skill again.
	toolCallID := "slash_skill_" + skillName
	argBytes, _ := json.Marshal(map[string]string{"name": skillName})
	assistantMsg := llm.Message{
		Role: "assistant",
		ToolCalls: []llm.ToolCall{
			{
				ID:   toolCallID,
				Type: "function",
				Function: llm.ToolCallFunc{
					Name:      "load_skill",
					Arguments: json.RawMessage(argBytes),
				},
			},
		},
	}
	if err := a.addContextMessages(assistantMsg, llm.NewToolMessage(toolCallID, content)); err != nil {
		a.emitToolError("load_skill", "Failed to activate skill %q: %v", skillName, err)
		return
	}
	if a.session != nil {
		if err := a.session.AppendSkillActivation(skillName); err != nil {
			a.emitToolError("session", "Failed to persist skill activation: %v", err)
		}
		if err := a.persistSessionSnapshot(); err != nil {
			a.emitToolError("session", "Failed to persist session snapshot: %v", err)
		}
	}
	a.EventCh <- model.Event{
		Type:     model.ToolSkill,
		ToolName: "load_skill",
		Message:  skillName,
		Summary:  fmt.Sprintf("loaded skill: %s", skillName),
	}

	if userRequest == "" {
		userRequest = defaultSkillRequest(skillName)
	}
	go a.runTask(userRequest)
}

func defaultSkillRequest(skillName string) string {
	return fmt.Sprintf(
		`The %q skill is already loaded. Start following that skill now using the current workspace and conversation context. Begin with the first concrete step immediately, keep gathering evidence with tools, and only stop to ask the user if the skill cannot proceed without missing information.`,
		skillName,
	)
}

func (a *Application) cmdHelp() {
	helpText := `Available commands:

  /skill [name] [request] Load and run a skill; omit request to start immediately
  /skill-add <path|git-url|owner/repo>  Add skills into ~/.ms-cli/skills
  /skill-update              Update shared skills repo
  /train <model> <method> Start train workflow (e.g. /train qwen3 lora)
  /train <action>         Control active train HUD (start, stop, analyze, apply fix, retry, view diff, exit)
  /project [status]        Show project status snapshot (server + git status)
  /project add <section> "<title>" [--owner o] [--progress p]  Add a task
  /project update <section> <id> [--title t] [--owner o] [--progress p] [--status s]  Update a task
  /project rm <section> <id>  Remove a task
  /login <token>          Log in to the bug server
  /report [ui,train] <title>  Report a new bug with optional tags
  /issues [status]         List issues (optional status filter: ready, doing, closed)
  /status <ISSUE-id> <ready|doing|closed>  Update an issue status
  /diagnose <problem text|ISSUE-id>  Diagnose a problem or issue
  /fix <problem text|ISSUE-id>  Run fix flow for a problem or issue
  /bugs [status]          List bugs (optional status filter: open, doing)
  /claim <id>             Claim a bug as your lead
  /dock                   Show bug dashboard (open count, ready, recent)
  /model [model-name]     Show or switch model
  /test                   Test API connectivity
  /permission [tool] [level]  Manage tool permissions
  /yolo                   Toggle auto-approve mode
  /exit                   Exit the application
  /compact                Compact conversation context to save tokens
  /clear                  Clear chat history
  /help                   Show this help message

Model Commands:
  /model                  Show current configuration
  /model gpt-4o           Switch to gpt-4o
  /model openai-completion:gpt-4o
  /model openai-responses:gpt-4o
  /model anthropic:claude-3-5-sonnet

Permission Commands:
  /permission             Show current permission settings
  /permission shell ask   Set permission level for a tool
  /yolo                   Toggle auto-approve for all operations

Permission Levels:
  ask          - Ask each time (default)
  allow_once   - Allow once
  allow_session - Allow for this session
  allow_always - Always allow
  deny         - Always deny

Keybindings:
  enter      Send input
  shift+drag Select terminal text in compatible terminals
  mouse wheel Scroll chat
  pgup/pgdn  Scroll chat
  home/end   Jump to top/bottom
  /          Start a slash command
  ctrl+c     Cancel/Quit (press twice to exit)

@file Input Expansion:
  Plain chat and /report, /diagnose, /fix, /skill, /<skill> alias support standalone @relative/path
  Typing @path in the composer shows file completion candidates before submit
  Use @@name to keep a literal @name token
  @path injects a workspace file reference as an absolute path marker; the agent can read it if needed
  Referenced paths must stay inside the workspace and point to an existing file
  Invalid @file references fail the whole input

Environment Variables:
  MSCLI_PROVIDER          Provider (openai-completion/openai-responses/anthropic)
  MSCLI_BASE_URL          Base URL
  MSCLI_MODEL             Default model
  MSCLI_API_KEY           API key
  MSCLI_TEMPERATURE       Temperature
  MSCLI_MAX_TOKENS        Max completion tokens
  MSCLI_CONTEXT_WINDOW    Context window tokens
  MSCLI_TIMEOUT           Request timeout seconds`

	a.EventCh <- model.Event{Type: model.AgentReply, Message: helpText}
}
