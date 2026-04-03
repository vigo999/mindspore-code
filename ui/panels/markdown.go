package panels

import (
	"regexp"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

const maxTextWidth = 120

var (
	mdRendererMu sync.Mutex
	mdRenderer   *glamour.TermRenderer
)

var orderedListPattern = regexp.MustCompile(`^\d+\.\s+`)

// markdownStyle returns a custom glamour StyleConfig based on the dark theme
// but with Document.Margin zeroed out so that external callers control
// indentation via lipgloss prefixes. Code block margin is preserved.
func markdownStyle() ansi.StyleConfig {
	s := ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("252"),
			},
			// No margin or block prefix/suffix — callers control layout.
		},
		BlockQuote: ansi.StyleBlock{
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       stringPtr("39"),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           stringPtr("228"),
				BackgroundColor: stringPtr("63"),
				Bold:            boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Prefix: "## "},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Prefix: "### "},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Prefix: "#### "},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{Prefix: "##### "},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  stringPtr("35"),
				Bold:   boolPtr(false),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  stringPtr("240"),
			Format: "\n--------\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     stringPtr("30"),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: stringPtr("35"),
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     stringPtr("212"),
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  stringPtr("243"),
			Format: "Image: {{.text}} →",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           stringPtr("203"),
				BackgroundColor: stringPtr("236"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: stringPtr("244"),
				},
				Margin: uintPtr(2),
			},
			Chroma: &ansi.Chroma{
				Text:                ansi.StylePrimitive{Color: stringPtr("#C4C4C4")},
				Error:               ansi.StylePrimitive{Color: stringPtr("#F1F1F1"), BackgroundColor: stringPtr("#F05B5B")},
				Comment:             ansi.StylePrimitive{Color: stringPtr("#676767")},
				CommentPreproc:      ansi.StylePrimitive{Color: stringPtr("#FF875F")},
				Keyword:             ansi.StylePrimitive{Color: stringPtr("#00AAFF")},
				KeywordReserved:     ansi.StylePrimitive{Color: stringPtr("#FF5FD2")},
				KeywordNamespace:    ansi.StylePrimitive{Color: stringPtr("#FF5F87")},
				KeywordType:         ansi.StylePrimitive{Color: stringPtr("#6E6ED8")},
				Operator:            ansi.StylePrimitive{Color: stringPtr("#EF8080")},
				Punctuation:         ansi.StylePrimitive{Color: stringPtr("#E8E8A8")},
				Name:                ansi.StylePrimitive{Color: stringPtr("#C4C4C4")},
				NameBuiltin:         ansi.StylePrimitive{Color: stringPtr("#FF8EC7")},
				NameTag:             ansi.StylePrimitive{Color: stringPtr("#B083EA")},
				NameAttribute:       ansi.StylePrimitive{Color: stringPtr("#7A7AE6")},
				NameClass:           ansi.StylePrimitive{Color: stringPtr("#F1F1F1"), Underline: boolPtr(true), Bold: boolPtr(true)},
				NameDecorator:       ansi.StylePrimitive{Color: stringPtr("#FFFF87")},
				NameFunction:        ansi.StylePrimitive{Color: stringPtr("#00D787")},
				LiteralNumber:       ansi.StylePrimitive{Color: stringPtr("#6EEFC0")},
				LiteralString:       ansi.StylePrimitive{Color: stringPtr("#C69669")},
				LiteralStringEscape: ansi.StylePrimitive{Color: stringPtr("#AFFFD7")},
				GenericDeleted:      ansi.StylePrimitive{Color: stringPtr("#FD5B5B")},
				GenericEmph:         ansi.StylePrimitive{Italic: boolPtr(true)},
				GenericInserted:     ansi.StylePrimitive{Color: stringPtr("#00D787")},
				GenericStrong:       ansi.StylePrimitive{Bold: boolPtr(true)},
				GenericSubheading:   ansi.StylePrimitive{Color: stringPtr("#777777")},
				Background:          ansi.StylePrimitive{BackgroundColor: stringPtr("#373737")},
			},
		},
		Table: ansi.StyleTable{},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n🠶 ",
		},
	}
	return s
}

// prepareForGlamour adds markdown hard-break markers (two trailing spaces)
// to non-blank lines outside fenced code blocks, so that glamour's
// paragraph renderer preserves the original line structure instead of
// collapsing consecutive lines into re-flowed paragraphs.
func prepareForGlamour(content string) string {
	lines := strings.Split(content, "\n")
	inFence := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if trimmed == "" {
			continue
		}
		if isMarkdownBlockLine(trimmed) {
			continue
		}
		if strings.HasSuffix(line, "  ") {
			continue
		}
		lines[i] = line + "  "
	}
	return strings.Join(lines, "\n")
}

func isMarkdownBlockLine(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	if strings.HasPrefix(trimmed, ">") {
		return true
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}
	if strings.HasPrefix(trimmed, "- [") || strings.HasPrefix(trimmed, "* [") || strings.HasPrefix(trimmed, "+ [") {
		return true
	}
	if orderedListPattern.MatchString(trimmed) {
		return true
	}
	if strings.HasPrefix(trimmed, "|") || isMarkdownTableSeparator(trimmed) {
		return true
	}
	if trimmed == "---" || trimmed == "***" || trimmed == "___" {
		return true
	}
	return false
}

func isMarkdownTableSeparator(trimmed string) bool {
	if !strings.Contains(trimmed, "|") {
		return false
	}
	candidate := strings.ReplaceAll(trimmed, "|", "")
	candidate = strings.ReplaceAll(candidate, ":", "")
	candidate = strings.ReplaceAll(candidate, "-", "")
	candidate = strings.ReplaceAll(candidate, " ", "")
	return candidate == ""
}

// RenderMarkdown converts markdown text to styled ANSI terminal output.
// The result is word-wrapped to the given width. Falls back to raw content
// on any rendering error.
func RenderMarkdown(content string, width int) string {
	if strings.TrimSpace(content) == "" {
		return content
	}
	if width < 10 {
		width = 10
	}

	// Pre-process: add hard breaks so glamour preserves line structure
	// instead of collapsing consecutive lines into re-flowed paragraphs.
	prepared := prepareForGlamour(content)

	r := getRenderer(width)
	out, err := r.Render(prepared)
	if err != nil {
		return content
	}
	return strings.TrimSpace(out)
}

// cappedMessageWidth returns width capped at maxTextWidth.
func cappedMessageWidth(width int) int {
	if width > maxTextWidth {
		return maxTextWidth
	}
	return width
}

var mdRendererWidth int

func getRenderer(width int) *glamour.TermRenderer {
	mdRendererMu.Lock()
	defer mdRendererMu.Unlock()

	if mdRenderer != nil && mdRendererWidth == width {
		return mdRenderer
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(markdownStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		r, _ = glamour.NewTermRenderer(glamour.WithStyles(markdownStyle()))
	}
	mdRenderer = r
	mdRendererWidth = width
	return mdRenderer
}

func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string  { return &s }
func uintPtr(u uint) *uint       { return &u }
