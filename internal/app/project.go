package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	projectpkg "github.com/vigo999/ms-cli/internal/project"
	"github.com/vigo999/ms-cli/ui/model"
	"gopkg.in/yaml.v3"
)

var runProjectGit = func(workDir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", workDir}, args...)...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			return "", err
		}
		return text, fmt.Errorf("%w: %s", err, text)
	}
	return text, nil
}

type projectCard struct {
	Status      model.ProjectStatusView
	Doc         *yaml.Node
	Overview    []string
	Progress    string
	ProgressPct int
	TodayTasks  []projectTask
	Today       []string
	Tomorrow    []string
	WeekGoals   []string
}

type projectTask struct {
	Title    string `yaml:"title"`
	Status   string `yaml:"status"`
	Progress int    `yaml:"progress"`
	Owner    string `yaml:"owner"`
}

type projectDoc struct {
	Overview struct {
		Phase string `yaml:"phase"`
		Owner string `yaml:"owner"`
		Focus string `yaml:"focus"`
	} `yaml:"overview"`
	ProgressPct int           `yaml:"progress_pct"`
	TodayTasks  []projectTask `yaml:"today_tasks"`
	Today       []string      `yaml:"today"`
	Tomorrow    []string      `yaml:"tomorrow"`
	WeekGoals   []string      `yaml:"week_goals"`
}

func (a *Application) cmdProject(args []string) {
	action := "status"
	if len(args) > 0 {
		action = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch action {
	case "", "status", "show", "open", "refresh":
		card, err := collectProjectCard(a.WorkDir)
		if err != nil {
			a.EventCh <- model.Event{
				Type:     model.ToolError,
				ToolName: "project",
				Message:  fmt.Sprintf("project status failed: %v", err),
			}
			return
		}
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: renderProjectCard(card),
		}
	case "close", "exit":
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "project status is stream-only now. Run /project again to refresh the snapshot.",
		}
	default:
		a.EventCh <- model.Event{
			Type:    model.AgentReply,
			Message: "Usage: /project [status]",
		}
	}
}

func collectProjectCard(workDir string) (projectCard, error) {
	status, err := collectProjectStatus(workDir)
	if err != nil {
		return projectCard{}, err
	}

	card := projectCard{
		Status: status,
	}
	docNode, err := loadProjectDocNode(status.Root)
	if err != nil {
		return projectCard{}, err
	}
	if docNode != nil {
		card.Doc = docNode
		return card, nil
	}
	if doc := loadProjectDoc(status.Root); doc != nil {
		card.Overview = docOverviewLines(doc)
		if doc.ProgressPct > 0 {
			card.ProgressPct = doc.ProgressPct
			card.Progress = progressBar(doc.ProgressPct, 10)
		}
		if len(doc.TodayTasks) > 0 {
			card.TodayTasks = normalizeProjectTasks(doc.TodayTasks)
		}
		if len(doc.Today) > 0 {
			card.Today = normalizeProjectItems(doc.Today)
		}
		if len(doc.Tomorrow) > 0 {
			card.Tomorrow = normalizeProjectItems(doc.Tomorrow)
		}
		if len(doc.WeekGoals) > 0 {
			card.WeekGoals = normalizeProjectItems(doc.WeekGoals)
		}
	}
	if roadmap := loadProjectRoadmap(status.Root); roadmap != nil {
		if card.Progress == "" {
			card.ProgressPct = roadmap.Overall.Pct
			card.Progress = progressBar(roadmap.Overall.Pct, 10)
		}
		if len(card.Today) == 0 {
			card.Today = roadmapToday(roadmap.rm)
		}
		if len(card.Tomorrow) == 0 {
			card.Tomorrow = roadmapTomorrow(roadmap.rm)
		}
		if len(card.WeekGoals) == 0 {
			card.WeekGoals = roadmapWeekGoals(roadmap.rm)
		}
	}
	if card.Progress == "" {
		card.Progress = progressBar(0, 10)
	}
	if len(card.TodayTasks) == 0 && len(card.Today) == 0 {
		card.Today = fallbackToday(status)
	}
	if len(card.Today) == 0 {
		if len(card.TodayTasks) == 0 {
			card.Today = fallbackToday(status)
		}
	}
	if len(card.Tomorrow) == 0 {
		card.Tomorrow = fallbackTomorrow(status)
	}
	if len(card.Overview) == 0 {
		card.Overview = fallbackOverview(status)
	}
	if len(card.WeekGoals) == 0 {
		card.WeekGoals = fallbackWeekGoals(status)
	}
	return card, nil
}

func collectProjectStatus(workDir string) (model.ProjectStatusView, error) {
	root, err := runProjectGit(workDir, "rev-parse", "--show-toplevel")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not a git repository") {
			absRoot, absErr := filepath.Abs(workDir)
			if absErr == nil {
				workDir = absRoot
			}
			return model.ProjectStatusView{
				Name:    filepath.Base(workDir),
				Root:    workDir,
				Branch:  "-",
				Summary: "not a git repository",
			}, nil
		}
		return model.ProjectStatusView{}, err
	}

	branch, err := runProjectGit(root, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		branch = "detached"
	}

	shortStatus, err := runProjectGit(root, "status", "--short")
	if err != nil {
		return model.ProjectStatusView{}, err
	}

	modified, staged, untracked := parseShortStatus(shortStatus)
	changed, docs, code, tests := classifyChangedFiles(shortStatus)
	ahead, behind := parseAheadBehind(root)
	summary, dirty := formatProjectSummary(modified, staged, untracked, ahead, behind)

	return model.ProjectStatusView{
		Name:      filepath.Base(root),
		Root:      root,
		Branch:    branch,
		Summary:   summary,
		Dirty:     dirty,
		Modified:  modified,
		Staged:    staged,
		Untracked: untracked,
		Ahead:     ahead,
		Behind:    behind,
		Changed:   changed,
		Docs:      docs,
		Code:      code,
		Tests:     tests,
	}, nil
}

func parseShortStatus(status string) (modified, staged, untracked int) {
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "??") {
			untracked++
			continue
		}
		if len(line) >= 1 && line[0] != ' ' {
			staged++
		}
		if len(line) >= 2 && line[1] != ' ' {
			modified++
		}
	}
	return modified, staged, untracked
}

func classifyChangedFiles(status string) (changed, docs, code, tests int) {
	for _, line := range strings.Split(status, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		path := parseStatusPath(line)
		if path == "" {
			continue
		}
		changed++
		switch {
		case isTestPath(path):
			tests++
		case isDocPath(path):
			docs++
		default:
			code++
		}
	}
	return changed, docs, code, tests
}

func parseStatusPath(line string) string {
	if len(line) <= 3 {
		return ""
	}
	path := strings.TrimSpace(line[3:])
	if idx := strings.LastIndex(path, " -> "); idx >= 0 {
		path = strings.TrimSpace(path[idx+4:])
	}
	return path
}

func isDocPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasPrefix(lower, "docs/") ||
		strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".rst")
}

func isTestPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "/testdata/") ||
		strings.HasSuffix(lower, "_test.go") ||
		strings.HasPrefix(lower, "test/") ||
		strings.HasPrefix(lower, "tests/")
}

func parseAheadBehind(workDir string) (ahead, behind int) {
	out, err := runProjectGit(workDir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	if err != nil {
		return 0, 0
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0
	}
	fmt.Sscanf(fields[0], "%d", &behind)
	fmt.Sscanf(fields[1], "%d", &ahead)
	return ahead, behind
}

func formatProjectSummary(modified, staged, untracked, ahead, behind int) (string, bool) {
	parts := make([]string, 0, 5)
	dirty := modified > 0 || staged > 0 || untracked > 0
	if staged > 0 {
		parts = append(parts, fmt.Sprintf("%d staged", staged))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", modified))
	}
	if untracked > 0 {
		parts = append(parts, fmt.Sprintf("%d untracked", untracked))
	}
	if ahead > 0 {
		parts = append(parts, fmt.Sprintf("ahead %d", ahead))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("behind %d", behind))
	}
	if len(parts) == 0 {
		return "clean working tree", false
	}
	return strings.Join(parts, " · "), dirty
}

type roadmapSnapshot struct {
	rm      *projectpkg.Roadmap
	Overall projectpkg.Progress
}

func loadProjectRoadmap(root string) *roadmapSnapshot {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	path := filepath.Join(root, "roadmap.yaml")
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	rm, err := projectpkg.LoadRoadmapFromFile(path)
	if err != nil {
		return nil
	}
	status, err := projectpkg.ComputeRoadmapStatus(rm, time.Now())
	if err != nil {
		return nil
	}
	return &roadmapSnapshot{rm: rm, Overall: status.Overall}
}

func loadProjectDoc(root string) *projectDoc {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	path := filepath.Join(root, "docs", "project.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc projectDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	return &doc
}

func loadProjectDocNode(root string) (*yaml.Node, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	path := filepath.Join(root, "docs", "project.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &node, nil
}

func docOverviewLines(doc *projectDoc) []string {
	if doc == nil {
		return nil
	}
	lines := []string{}
	if strings.TrimSpace(doc.Overview.Phase) != "" {
		lines = append(lines, "phase: "+doc.Overview.Phase)
	}
	if strings.TrimSpace(doc.Overview.Owner) != "" {
		lines = append(lines, "owner: "+doc.Overview.Owner)
	}
	if strings.TrimSpace(doc.Overview.Focus) != "" {
		lines = append(lines, "focus: "+doc.Overview.Focus)
	}
	return lines
}

func normalizeProjectItems(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		lower := strings.ToLower(item)
		if strings.HasPrefix(lower, "[x]") || strings.HasPrefix(lower, "[ ]") {
			out = append(out, item)
			continue
		}
		out = append(out, "[ ] "+item)
	}
	return out
}

func normalizeProjectTasks(tasks []projectTask) []projectTask {
	out := make([]projectTask, 0, len(tasks))
	for _, task := range tasks {
		task.Title = strings.TrimSpace(task.Title)
		if task.Title == "" {
			continue
		}
		task.Status = normalizeTaskStatus(task.Status)
		if task.Progress < 0 {
			task.Progress = 0
		}
		if task.Progress > 100 {
			task.Progress = 100
		}
		task.Owner = strings.TrimSpace(task.Owner)
		out = append(out, task)
	}
	return out
}

func normalizeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done":
		return "done"
	case "block", "blocked":
		return "block"
	case "doing", "in_progress", "in-progress":
		return "doing"
	default:
		return "doing"
	}
}

func roadmapToday(rm *projectpkg.Roadmap) []string {
	if rm == nil {
		return nil
	}
	phase := currentRoadmapPhase(rm)
	if phase == nil {
		return nil
	}
	items := make([]string, 0, 3)
	for _, ms := range phase.Milestones {
		switch strings.ToLower(strings.TrimSpace(ms.Status)) {
		case "done":
			items = append(items, "[x] "+ms.Title)
		case "in_progress":
			items = append(items, "[ ] "+ms.Title)
		}
		if len(items) == 3 {
			return items
		}
	}
	return items
}

func roadmapTomorrow(rm *projectpkg.Roadmap) []string {
	if rm == nil {
		return nil
	}
	phase := currentRoadmapPhase(rm)
	if phase == nil {
		return nil
	}
	items := make([]string, 0, 3)
	for _, ms := range phase.Milestones {
		if strings.ToLower(strings.TrimSpace(ms.Status)) == "pending" {
			items = append(items, "[ ] "+ms.Title)
		}
		if len(items) == 3 {
			return items
		}
	}
	if len(items) > 0 {
		return items
	}
	for _, next := range rm.Phases {
		if next.ID == phase.ID {
			continue
		}
		for _, ms := range next.Milestones {
			if strings.ToLower(strings.TrimSpace(ms.Status)) == "pending" {
				items = append(items, "[ ] "+ms.Title)
			}
			if len(items) == 3 {
				return items
			}
		}
		if len(items) > 0 {
			return items
		}
	}
	return items
}

func roadmapWeekGoals(rm *projectpkg.Roadmap) []string {
	if rm == nil {
		return nil
	}
	phase := currentRoadmapPhase(rm)
	if phase == nil {
		return nil
	}
	items := make([]string, 0, 4)
	for _, ms := range phase.Milestones {
		status := strings.ToLower(strings.TrimSpace(ms.Status))
		if status == "done" {
			continue
		}
		items = append(items, "[ ] "+ms.Title)
		if len(items) == 4 {
			return items
		}
	}
	return items
}

func currentRoadmapPhase(rm *projectpkg.Roadmap) *projectpkg.Phase {
	if rm == nil {
		return nil
	}
	for i := range rm.Phases {
		phase := &rm.Phases[i]
		hasInFlight := false
		hasPending := false
		for _, ms := range phase.Milestones {
			switch strings.ToLower(strings.TrimSpace(ms.Status)) {
			case "in_progress":
				hasInFlight = true
			case "pending":
				hasPending = true
			}
		}
		if hasInFlight || hasPending {
			return phase
		}
	}
	if len(rm.Phases) > 0 {
		return &rm.Phases[0]
	}
	return nil
}

func fallbackToday(status model.ProjectStatusView) []string {
	items := []string{}
	if status.Staged > 0 {
		items = append(items, fmt.Sprintf("[x] %d file(s) staged", status.Staged))
	}
	if status.Modified > 0 {
		items = append(items, fmt.Sprintf("[ ] %d modified file(s) still open", status.Modified))
	}
	if status.Untracked > 0 {
		items = append(items, fmt.Sprintf("[ ] %d new file(s) not tracked yet", status.Untracked))
	}
	if len(items) == 0 {
		items = append(items, "[x] working tree is clean")
	}
	return items
}

func fallbackTomorrow(status model.ProjectStatusView) []string {
	items := []string{}
	if status.Modified > 0 {
		items = append(items, "[ ] review modified files")
	}
	if status.Untracked > 0 {
		items = append(items, "[ ] decide whether to add new files")
	}
	if status.Behind > 0 {
		items = append(items, fmt.Sprintf("[ ] sync %d commit(s) from upstream", status.Behind))
	}
	if len(items) == 0 {
		items = append(items, "[ ] continue next milestone")
	}
	return items
}

func fallbackOverview(status model.ProjectStatusView) []string {
	lines := []string{
		fmt.Sprintf("repo: %s", status.Name),
		fmt.Sprintf("root: %s", status.Root),
	}
	return lines
}

func fallbackWeekGoals(status model.ProjectStatusView) []string {
	items := []string{
		"[ ] keep branch " + status.Branch + " moving",
		"[ ] reduce modified file count",
	}
	if status.Untracked > 0 {
		items = append(items, "[ ] decide which new files belong in git")
	}
	return items
}

func progressBar(pct, width int) string {
	return progressBarStyled(pct, width, "", "")
}

func progressBarStyled(pct, width int, filledColor, emptyColor string) string {
	if width <= 0 {
		width = 10
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := (pct * width) / 100
	if pct > 0 && filled == 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	cells := make([]string, 0, width)
	for i := 0; i < filled; i++ {
		cell := "■"
		if strings.TrimSpace(filledColor) != "" {
			cell = applyProjectColor(cell, filledColor)
		}
		cells = append(cells, cell)
	}
	for i := filled; i < width; i++ {
		cell := "□"
		if strings.TrimSpace(emptyColor) != "" {
			cell = applyProjectColor(cell, emptyColor)
		}
		cells = append(cells, cell)
	}
	return strings.Join(cells, "")
}

func renderProjectCard(card projectCard) string {
	if card.Doc != nil {
		return renderProjectDoc(card.Status, card.Doc)
	}
	status := card.Status
	overviewLines := []string{fmt.Sprintf("state: %s", stateLine(status))}
	overviewLines = append(overviewLines, card.Overview...)
	overviewLines = append(overviewLines, fmt.Sprintf("progress: [%s] %d%%", card.Progress, card.ProgressPct))
	overviewLines = append(overviewLines, fmt.Sprintf("activity: changed %d  docs %d  code %d  tests %d", status.Changed, status.Docs, status.Code, status.Tests))

	sections := []string{
		fmt.Sprintf("project status · %s · %s", status.Name, status.Branch),
		"",
		"overview",
	}
	sections = append(sections, overviewLines...)
	sections = append(sections,
		"",
		"today task",
	)
	sections = append(sections, renderTodayTaskLines(card)...)
	sections = append(sections,
		"",
		"tomorrow",
	)
	sections = append(sections, padProjectItems(card.Tomorrow)...)
	sections = append(sections,
		"",
		"week goals",
	)
	sections = append(sections, padProjectItems(card.WeekGoals)...)
	return renderProjectBox(sections)
}

func renderProjectDoc(status model.ProjectStatusView, doc *yaml.Node) string {
	root := projectDocRoot(doc)
	lines := []string{fmt.Sprintf("this is %s project status", status.Name)}
	if root == nil || root.Kind != yaml.MappingNode {
		return strings.Join(lines, "\n")
	}
	topMsgColor := ""
	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valueNode := root.Content[i+1]
		keyName := strings.TrimSpace(keyNode.Value)
		if strings.EqualFold(keyName, "top_msg") && isScalarNode(valueNode) {
			msg := strings.TrimSpace(valueNode.Value)
			if msg != "" {
				lines[0] = msg
			}
			continue
		}
		if strings.EqualFold(keyName, "top_msg_color") && isScalarNode(valueNode) {
			topMsgColor = strings.TrimSpace(valueNode.Value)
		}
	}
	if strings.TrimSpace(topMsgColor) != "" {
		lines[0] = applyProjectColor(lines[0], topMsgColor)
	}
	titleWidth := projectDocStructuredTitleWidth(root)

	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valueNode := root.Content[i+1]
		keyName := strings.TrimSpace(keyNode.Value)
		if strings.EqualFold(keyName, "top_msg") || strings.EqualFold(keyName, "top_msg_color") {
			continue
		}
		title := normalizeSectionTitle(keyNode.Value)
		body := valueNode
		if sectionColor, suffix, wrapped := sectionHeaderMeta(valueNode); wrapped != nil {
			title = applyProjectColor(title, sectionColor)
			if strings.TrimSpace(suffix) != "" {
				title += " [" + strings.TrimSpace(suffix) + "]"
			}
			body = wrapped
		}
		lines = append(lines, "")
		lines = append(lines, title)
		lines = append(lines, renderProjectValueLines(body, "  ", titleWidth)...)
	}

	return renderProjectBox(lines)
}

func projectDocRoot(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

func normalizeSectionTitle(key string) string {
	key = strings.ReplaceAll(strings.TrimSpace(key), "_", " ")
	if key == "" {
		return "section"
	}
	return key
}

func renderProjectValueLines(node *yaml.Node, indent string, titleWidth int) []string {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.MappingNode:
		lines := []string{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			keyName := strings.TrimSpace(keyNode.Value)
			if strings.EqualFold(keyName, "color") || strings.EqualFold(keyName, "date") || strings.EqualFold(keyName, "title_suffix") {
				continue
			}
			if strings.EqualFold(keyName, "items") {
				lines = append(lines, renderProjectValueLines(valueNode, indent, titleWidth)...)
				continue
			}
			key := normalizeSectionTitle(keyNode.Value)
			if isScalarNode(valueNode) {
				lines = append(lines, indent+key+": "+renderScalarValue(keyNode.Value, valueNode.Value))
				continue
			}
			lines = append(lines, indent+key+":")
			lines = append(lines, renderProjectValueLines(valueNode, indent+"  ", titleWidth)...)
		}
		return lines
	case yaml.SequenceNode:
		lines := []string{}
		for _, item := range node.Content {
			if structured, ok := renderStructuredProjectItem(item, indent, titleWidth); ok {
				lines = append(lines, structured)
				continue
			}
			if isScalarNode(item) {
				lines = append(lines, indent+"- "+item.Value)
				continue
			}
			lines = append(lines, indent+"-")
			lines = append(lines, renderProjectValueLines(item, indent+"  ", titleWidth)...)
		}
		return lines
	case yaml.ScalarNode:
		return []string{indent + renderScalarValue("", node.Value)}
	default:
		return nil
	}
}

func projectDocStructuredTitleWidth(root *yaml.Node) int {
	width := 0
	collectProjectStructuredTitleWidth(root, &width)
	if width < 12 {
		return 12
	}
	return width
}

func collectProjectStructuredTitleWidth(node *yaml.Node, width *int) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			collectProjectStructuredTitleWidth(child, width)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			if strings.EqualFold(strings.TrimSpace(keyNode.Value), "color") {
				continue
			}
			collectProjectStructuredTitleWidth(valueNode, width)
		}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item != nil && item.Kind == yaml.MappingNode {
				fields := orderedNodeFields(item)
				title := firstNonEmptyField(fields, "title", "name", "task", "label")
				if title == "" {
					title = firstScalarField(fields)
				}
				if len(title) > *width {
					*width = len(title)
				}
			}
			collectProjectStructuredTitleWidth(item, width)
		}
	}
}

func renderStructuredProjectItem(node *yaml.Node, indent string, titleWidth int) (string, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return "", false
	}
	fields := orderedNodeFields(node)
	if len(fields) == 0 {
		return "", false
	}

	title := firstNonEmptyField(fields, "title", "name", "task", "label")
	status := fieldValue(fields, "status")
	progress := fieldValue(fields, "progress")
	owner := fieldValue(fields, "owner")
	color := fieldValue(fields, "color")
	progressColor := fieldValue(fields, "progress_color")
	emptyColor := fieldValue(fields, "empty_color")

	if title == "" {
		title = firstScalarField(fields)
	}
	if title == "" {
		return "", false
	}
	if titleWidth < len(title) {
		titleWidth = len(title)
	}

	titleText := fmt.Sprintf("%-*s", titleWidth, title)
	if strings.TrimSpace(color) != "" {
		titleText = applyProjectColor(titleText, color)
	}
	parts := []string{fmt.Sprintf("%s%s %s", indent, taskStatusMarker(status), titleText)}
	if pct, ok := parsePercent(progress); ok {
		parts = append(parts, fmt.Sprintf("[%s] %3d%%", progressBarStyled(pct, 10, progressColor, emptyColor), pct))
	}
	if strings.TrimSpace(owner) != "" {
		parts = append(parts, "owner: "+owner)
	}

	for _, field := range fields {
		k := strings.ToLower(strings.TrimSpace(field.Key))
		if k == "title" || k == "name" || k == "task" || k == "label" || k == "status" || k == "progress" || k == "owner" || k == "color" || k == "progress_color" || k == "empty_color" {
			continue
		}
		if !isScalarNode(field.Value) {
			continue
		}
		parts = append(parts, normalizeSectionTitle(field.Key)+": "+field.Value.Value)
	}

	return strings.Join(parts, "  "), true
}

func structuredSequenceTitleWidth(node *yaml.Node) int {
	if node == nil || node.Kind != yaml.SequenceNode {
		return 0
	}
	width := 0
	for _, item := range node.Content {
		if item == nil || item.Kind != yaml.MappingNode {
			continue
		}
		fields := orderedNodeFields(item)
		title := firstNonEmptyField(fields, "title", "name", "task", "label")
		if title == "" {
			title = firstScalarField(fields)
		}
		if len(title) > width {
			width = len(title)
		}
	}
	return width
}

type nodeField struct {
	Key   string
	Value *yaml.Node
}

func orderedNodeFields(node *yaml.Node) []nodeField {
	fields := make([]nodeField, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		fields = append(fields, nodeField{
			Key:   node.Content[i].Value,
			Value: node.Content[i+1],
		})
	}
	return fields
}

func fieldValue(fields []nodeField, keys ...string) string {
	for _, key := range keys {
		for _, field := range fields {
			if strings.EqualFold(strings.TrimSpace(field.Key), key) && isScalarNode(field.Value) {
				return strings.TrimSpace(field.Value.Value)
			}
		}
	}
	return ""
}

func firstNonEmptyField(fields []nodeField, keys ...string) string {
	for _, key := range keys {
		if value := fieldValue(fields, key); value != "" {
			return value
		}
	}
	return ""
}

func firstScalarField(fields []nodeField) string {
	for _, field := range fields {
		if isScalarNode(field.Value) {
			return strings.TrimSpace(field.Value.Value)
		}
	}
	return ""
}

func isScalarNode(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode
}

func parsePercent(value string) (int, bool) {
	value = strings.TrimSpace(strings.TrimSuffix(value, "%"))
	if value == "" {
		return 0, false
	}
	var pct int
	if _, err := fmt.Sscanf(value, "%d", &pct); err != nil {
		return 0, false
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return pct, true
}

func renderScalarValue(key, value string) string {
	if pct, ok := parsePercent(value); ok && strings.Contains(strings.ToLower(key), "progress") {
		return fmt.Sprintf("[%s] %d%%", progressBar(pct, 10), pct)
	}
	return value
}

func renderTodayTaskLines(card projectCard) []string {
	if len(card.TodayTasks) > 0 {
		lines := make([]string, 0, len(card.TodayTasks))
		titleWidth := projectTaskTitleWidth(card.TodayTasks)
		for _, task := range card.TodayTasks {
			lines = append(lines, fmt.Sprintf("  %s %-*s | [%s] %3d%% | owner: %s",
				taskStatusMarker(task.Status),
				titleWidth,
				task.Title,
				progressBar(task.Progress, 10),
				task.Progress,
				valueOrString(task.Owner, "-"),
			))
		}
		return lines
	}
	return padProjectItems(card.Today)
}

func projectTaskTitleWidth(tasks []projectTask) int {
	width := 0
	for _, task := range tasks {
		if l := len(task.Title); l > width {
			width = l
		}
	}
	if width < 12 {
		return 12
	}
	return width
}

func taskStatusMarker(status string) string {
	switch normalizeTaskStatus(status) {
	case "done":
		return "[x]"
	case "block":
		return "[!]"
	default:
		return "[ ]"
	}
}

func sectionHeaderMeta(node *yaml.Node) (string, string, *yaml.Node) {
	if node == nil || node.Kind != yaml.MappingNode {
		return "", "", nil
	}
	color := fieldValue(orderedNodeFields(node), "color")
	suffix := firstNonEmptyField(orderedNodeFields(node), "date", "title_suffix")
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]
		if strings.EqualFold(strings.TrimSpace(keyNode.Value), "items") {
			return color, suffix, valueNode
		}
	}
	if strings.TrimSpace(color) == "" && strings.TrimSpace(suffix) == "" {
		return "", "", nil
	}
	return color, suffix, node
}

func applyProjectColor(text, color string) string {
	code, ok := projectColorCode(color)
	if !ok {
		return text
	}
	return "\x1b[38;5;" + code + "m" + text + "\x1b[0m"
}

func projectColorCode(color string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(color)) {
	case "dark_green", "green_4", "green4":
		return "28", true
	case "green":
		return "34", true
	case "yellow":
		return "220", true
	case "red":
		return "196", true
	case "blue":
		return "39", true
	case "cyan":
		return "51", true
	case "magenta":
		return "201", true
	case "white":
		return "15", true
	case "gray", "grey":
		return "244", true
	}
	color = strings.TrimSpace(color)
	for _, r := range color {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	if color == "" {
		return "", false
	}
	return color, true
}

func stateLine(status model.ProjectStatusView) string {
	state := "clean"
	if status.Dirty {
		state = "dirty"
	}
	parts := []string{
		fmt.Sprintf("%s · branch %s", state, status.Branch),
		fmt.Sprintf("staged %d", status.Staged),
		fmt.Sprintf("modified %d", status.Modified),
		fmt.Sprintf("untracked %d", status.Untracked),
	}
	if status.Ahead > 0 || status.Behind > 0 {
		parts = append(parts, fmt.Sprintf("ahead %d", status.Ahead), fmt.Sprintf("behind %d", status.Behind))
	}
	return strings.Join(parts, "  ")
}

func padProjectItems(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, "  "+item)
	}
	return out
}

func renderProjectBox(lines []string) string {
	width := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > width {
			width = w
		}
	}
	if width < 24 {
		width = 24
	}

	boxed := make([]string, 0, len(lines)+2)
	boxed = append(boxed, "╭"+strings.Repeat("─", width+2)+"╮")
	for i, line := range lines {
		visible := lipgloss.Width(line)
		if i == 0 {
			left := (width - visible) / 2
			if left < 0 {
				left = 0
			}
			right := width - visible - left
			if right < 0 {
				right = 0
			}
			boxed = append(boxed, "│ "+strings.Repeat(" ", left)+line+strings.Repeat(" ", right)+" │")
			continue
		}
		pad := width - visible
		if pad < 0 {
			pad = 0
		}
		boxed = append(boxed, "│ "+line+strings.Repeat(" ", pad)+" │")
	}
	boxed = append(boxed, "╰"+strings.Repeat("─", width+2)+"╯")
	return strings.Join(boxed, "\n")
}
