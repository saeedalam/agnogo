package contrib

import (
	"context"
	"fmt"
	"strings"

	"github.com/saeedalam/agnogo"
)

// Markdown returns a tool for stripping markdown formatting.
func Markdown() []agnogo.ToolDef {
	return []agnogo.ToolDef{
		{
			Name: "markdown_strip", Desc: "Strip markdown formatting and return plain text",
			Params: agnogo.Params{
				"text": {Type: "string", Desc: "Markdown text to strip", Required: true},
			},
			Fn: func(ctx context.Context, args map[string]string) (string, error) {
				text := args["text"]
				if text == "" {
					return "", fmt.Errorf("text is required")
				}
				return stripMarkdown(text), nil
			},
		},
	}
}

func stripMarkdown(input string) string {
	lines := strings.Split(input, "\n")
	var result []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code blocks
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			result = append(result, line)
			continue
		}

		// Horizontal rules
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			continue
		}

		// Headers
		if strings.HasPrefix(trimmed, "# ") {
			trimmed = strings.TrimLeft(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "## ") {
			trimmed = strings.TrimLeft(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "### ") {
			trimmed = strings.TrimLeft(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "#### ") {
			trimmed = strings.TrimLeft(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "##### ") {
			trimmed = strings.TrimLeft(trimmed, "# ")
		} else if strings.HasPrefix(trimmed, "###### ") {
			trimmed = strings.TrimLeft(trimmed, "# ")
		}

		// Blockquotes
		for strings.HasPrefix(trimmed, "> ") {
			trimmed = trimmed[2:]
		}
		if strings.HasPrefix(trimmed, ">") {
			trimmed = trimmed[1:]
		}

		// List markers
		if strings.HasPrefix(trimmed, "- ") {
			trimmed = trimmed[2:]
		} else if strings.HasPrefix(trimmed, "* ") {
			trimmed = trimmed[2:]
		} else if strings.HasPrefix(trimmed, "+ ") {
			trimmed = trimmed[2:]
		}
		// Numbered lists
		for i, c := range trimmed {
			if c >= '0' && c <= '9' {
				continue
			}
			if c == '.' && i > 0 && i < len(trimmed)-1 && trimmed[i+1] == ' ' {
				trimmed = trimmed[i+2:]
			}
			break
		}

		// Inline formatting
		trimmed = stripInlineMarkdown(trimmed)

		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}

func stripInlineMarkdown(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		// Images ![alt](url)
		if i < len(s)-1 && s[i] == '!' && s[i+1] == '[' {
			end := strings.Index(s[i:], "](")
			if end > 0 {
				altEnd := i + end
				alt := s[i+2 : altEnd]
				urlEnd := strings.Index(s[altEnd+2:], ")")
				if urlEnd >= 0 {
					out.WriteString(alt)
					i = altEnd + 2 + urlEnd + 1
					continue
				}
			}
		}
		// Links [text](url)
		if s[i] == '[' {
			end := strings.Index(s[i:], "](")
			if end > 0 {
				textEnd := i + end
				text := s[i+1 : textEnd]
				urlEnd := strings.Index(s[textEnd+2:], ")")
				if urlEnd >= 0 {
					out.WriteString(text)
					i = textEnd + 2 + urlEnd + 1
					continue
				}
			}
		}
		// Inline code
		if s[i] == '`' {
			endIdx := strings.Index(s[i+1:], "`")
			if endIdx >= 0 {
				out.WriteString(s[i+1 : i+1+endIdx])
				i = i + 1 + endIdx + 1
				continue
			}
		}
		// Bold **text** or __text__
		if i < len(s)-1 && ((s[i] == '*' && s[i+1] == '*') || (s[i] == '_' && s[i+1] == '_')) {
			marker := s[i : i+2]
			endIdx := strings.Index(s[i+2:], marker)
			if endIdx >= 0 {
				out.WriteString(s[i+2 : i+2+endIdx])
				i = i + 2 + endIdx + 2
				continue
			}
		}
		// Italic *text* or _text_
		if s[i] == '*' || s[i] == '_' {
			marker := string(s[i])
			endIdx := strings.Index(s[i+1:], marker)
			if endIdx >= 0 {
				out.WriteString(s[i+1 : i+1+endIdx])
				i = i + 1 + endIdx + 1
				continue
			}
		}
		// Strikethrough ~~text~~
		if i < len(s)-1 && s[i] == '~' && s[i+1] == '~' {
			endIdx := strings.Index(s[i+2:], "~~")
			if endIdx >= 0 {
				out.WriteString(s[i+2 : i+2+endIdx])
				i = i + 2 + endIdx + 2
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
