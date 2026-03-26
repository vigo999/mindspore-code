package panels

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

	mdHeading1Style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Bold(true)

	mdHeading2Style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("223")).
			Bold(true)

	mdHeading3Style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Bold(true)

	mdQuoteStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true)

	mdRuleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("239"))

	mdCodeBlockStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("236")).
				PaddingLeft(1).
				PaddingRight(1)

	mdCodeInlineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("238"))

	mdBoldStyle = lipgloss.NewStyle().Bold(true)

	mdItalicStyle = lipgloss.NewStyle().Italic(true)

	mdLinkStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")).
			Underline(true)

	mdLinkURLStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))
)

func renderAgentContent(content string) string {
	if content == "" {
		return ""
	}
	if ansiEscapePattern.MatchString(content) {
		return content
	}
	return renderMarkdown(content)
}

func renderMarkdown(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	rendered := make([]string, 0, len(lines))
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			rendered = append(rendered, mdCodeBlockStyle.Render(line))
			continue
		}
		if trimmed == "" {
			rendered = append(rendered, "")
			continue
		}
		if heading, level, ok := markdownHeading(line); ok {
			rendered = append(rendered, markdownHeadingStyle(level).Render(renderInlineMarkdown(heading)))
			continue
		}
		if quote, ok := markdownQuote(line); ok {
			rendered = append(rendered, mdQuoteStyle.Render("│ "+renderInlineMarkdown(quote)))
			continue
		}
		if item, ok := markdownBullet(line); ok {
			rendered = append(rendered, agentStyle.Render("• ")+renderInlineMarkdown(item))
			continue
		}
		if index, item, ok := markdownOrderedItem(line); ok {
			rendered = append(rendered, agentStyle.Render(index+". ")+renderInlineMarkdown(item))
			continue
		}
		if markdownRule(trimmed) {
			rendered = append(rendered, mdRuleStyle.Render("────────────────────"))
			continue
		}
		rendered = append(rendered, renderInlineMarkdown(line))
	}

	return strings.Join(rendered, "\n")
}

func markdownHeading(line string) (string, int, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	level := 0
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || len(trimmed) <= level || trimmed[level] != ' ' {
		return "", 0, false
	}
	return strings.TrimSpace(trimmed[level:]), level, true
}

func markdownHeadingStyle(level int) lipgloss.Style {
	switch level {
	case 1:
		return mdHeading1Style
	case 2:
		return mdHeading2Style
	default:
		return mdHeading3Style
	}
}

func markdownQuote(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if !strings.HasPrefix(trimmed, ">") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, ">")), true
}

func markdownBullet(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) < 2 {
		return "", false
	}
	switch trimmed[0] {
	case '-', '*', '+':
		if trimmed[1] == ' ' {
			return strings.TrimSpace(trimmed[2:]), true
		}
	}
	return "", false
}

func markdownOrderedItem(line string) (string, string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	i := 0
	for i < len(trimmed) && trimmed[i] >= '0' && trimmed[i] <= '9' {
		i++
	}
	if i == 0 || i+1 >= len(trimmed) || trimmed[i] != '.' || trimmed[i+1] != ' ' {
		return "", "", false
	}
	return trimmed[:i], strings.TrimSpace(trimmed[i+2:]), true
}

func markdownRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	switch line[0] {
	case '-', '*', '_':
		for i := 1; i < len(line); i++ {
			if line[i] != line[0] {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func renderInlineMarkdown(line string) string {
	var out strings.Builder
	for len(line) > 0 {
		switch {
		case strings.HasPrefix(line, "`"):
			end := strings.Index(line[1:], "`")
			if end >= 0 {
				end++
				out.WriteString(mdCodeInlineStyle.Render(line[1:end]))
				line = line[end+1:]
				continue
			}
		case strings.HasPrefix(line, "**"):
			if text, rest, ok := markdownDelimited(line, "**"); ok {
				out.WriteString(mdBoldStyle.Render(renderInlineMarkdown(text)))
				line = rest
				continue
			}
		case strings.HasPrefix(line, "*"):
			if text, rest, ok := markdownDelimited(line, "*"); ok {
				out.WriteString(mdItalicStyle.Render(renderInlineMarkdown(text)))
				line = rest
				continue
			}
		case strings.HasPrefix(line, "["):
			if label, url, rest, ok := markdownLink(line); ok {
				out.WriteString(mdLinkStyle.Render(renderInlineMarkdown(label)))
				if url != "" {
					out.WriteString(mdLinkURLStyle.Render(" (" + url + ")"))
				}
				line = rest
				continue
			}
		}

		next := nextMarkdownToken(line)
		plain := line
		if next > 0 {
			plain = line[:next]
			line = line[next:]
		} else {
			line = ""
		}
		out.WriteString(agentStyle.Render(plain))
	}
	return out.String()
}

func markdownDelimited(line, delim string) (string, string, bool) {
	if !strings.HasPrefix(line, delim) {
		return "", "", false
	}
	end := strings.Index(line[len(delim):], delim)
	if end < 0 {
		return "", "", false
	}
	end += len(delim)
	return line[len(delim):end], line[end+len(delim):], true
}

func markdownLink(line string) (string, string, string, bool) {
	closeLabel := strings.Index(line, "](")
	if closeLabel <= 1 {
		return "", "", "", false
	}
	closeURL := strings.Index(line[closeLabel+2:], ")")
	if closeURL < 0 {
		return "", "", "", false
	}
	closeURL += closeLabel + 2
	return line[1:closeLabel], line[closeLabel+2 : closeURL], line[closeURL+1:], true
}

func nextMarkdownToken(line string) int {
	indexes := []int{
		strings.Index(line, "`"),
		strings.Index(line, "**"),
		strings.Index(line, "*"),
		strings.Index(line, "["),
	}
	best := -1
	for _, idx := range indexes {
		if idx < 0 {
			continue
		}
		if best < 0 || idx < best {
			best = idx
		}
	}
	return best
}
