package tex

import (
	"sort"
	"strings"
)

func LanguageCode(lang string) string {
	switch strings.ToLower(lang) {
	case "english":
		return "en"
	case "russian":
		return "ru"
	default:
		if len(lang) > 2 {
			return lang[:2]
		}
		return lang
	}
}

// First matching section command wins so \examples is not also read as \example.
var sectionCommands = []struct{ section, command string }{
	{"input", "\\inputfile"}, {"input", "\\section*{input}"},
	{"output", "\\outputfile"}, {"output", "\\section*{output}"},
	{"interaction", "\\interaction"}, {"interaction", "\\section*{interaction}"},
	{"examples", "\\examples"}, {"examples", "\\example"}, {"examples", "\\begin{example}"},
	{"notes", "\\notes"}, {"notes", "\\note"}, {"notes", "\\section*{notes}"},
}

func ToSections(texContent string) map[string]string {
	content := body(texContent)
	type marker struct {
		section    string
		start, end int
	}
	lower := strings.ToLower(content)
	var markers []marker
	taken := map[int]bool{}
	for _, c := range sectionCommands {
		if pos := strings.Index(lower, c.command); pos != -1 && !taken[pos] {
			taken[pos] = true
			markers = append(markers, marker{c.section, pos, pos + len(c.command)})
		}
	}
	sort.Slice(markers, func(i, j int) bool { return markers[i].start < markers[j].start })
	sections := map[string]string{}
	current, start := "legend", 0
	for _, m := range markers {
		sections[current] = strings.TrimSpace(content[start:m.start])
		current, start = m.section, m.end
	}
	sections[current] = strings.TrimSpace(content[start:])
	legend, input, output, interaction, notes := sections["legend"], sections["input"], sections["output"], sections["interaction"], sections["notes"]

	cleanLatex := func(s string) string {
		if s == "" {
			return ""
		}
		s = strings.ReplaceAll(s, "\\begin{itemize}", "")
		s = strings.ReplaceAll(s, "\\end{itemize}", "")
		s = strings.ReplaceAll(s, "\\begin{enumerate}", "")
		s = strings.ReplaceAll(s, "\\end{enumerate}", "")

		var lines []string
		for _, line := range strings.Split(s, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "\\item") {
				line = strings.Replace(line, "\\item", "-", 1)
			}
			lines = append(lines, line)
		}
		s = strings.Join(lines, "\n")

		s = replaceCurlyCommand(s, "\\textbf", "**", "**")
		s = replaceCurlyCommand(s, "\\textit", "*", "*")
		s = replaceCurlyCommand(s, "\\texttt", "`", "`")
		s = replaceCurlyCommand(s, "\\t", "`", "`")

		s = strings.ReplaceAll(s, "``", "\"")
		s = strings.ReplaceAll(s, "''", "\"")

		s = strings.ReplaceAll(s, "\">\"", "`>`")
		s = strings.ReplaceAll(s, "\"<\"", "`<`")
		s = strings.ReplaceAll(s, "\"=\"", "`=`")

		return strings.TrimSpace(s)
	}

	result := map[string]string{}
	for name, text := range map[string]string{"legend": legend, "input": input, "output": output, "interaction": interaction, "notes": notes} {
		if text = cleanLatex(text); text != "" {
			result[name] = text
		}
	}
	return result
}

func body(texContent string) string {
	start := strings.Index(texContent, "\\begin{problem}")
	if start == -1 {
		for {
			i := strings.Index(texContent, "\\problemname{")
			if i == -1 {
				return texContent
			}
			end := closingBrace(texContent, i+len("\\problemname{"))
			if end == -1 {
				return texContent
			}
			texContent = texContent[:i] + texContent[end+1:]
		}
	}
	content := texContent[start+len("\\begin{problem}"):]
	for range 5 {
		open := strings.Index(content, "{")
		if open == -1 {
			return ""
		}
		end := closingBrace(content, open+1)
		if end == -1 {
			return ""
		}
		content = content[end+1:]
	}
	if end := strings.Index(content, "\\end{problem}"); end != -1 {
		content = content[:end]
	}
	return content
}

func closingBrace(s string, from int) int {
	depth := 1
	for i := from; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func replaceCurlyCommand(s, cmd, left, right string) string {
	for {
		idx := strings.Index(s, cmd+"{")
		if idx == -1 {
			break
		}
		braceCount := 1
		endIdx := -1
		startBrace := idx + len(cmd) + 1
		for i := startBrace; i < len(s); i++ {
			if s[i] == '{' {
				braceCount++
			} else if s[i] == '}' {
				braceCount--
				if braceCount == 0 {
					endIdx = i
					break
				}
			}
		}
		if endIdx == -1 {
			break
		}
		inner := s[startBrace:endIdx]
		s = s[:idx] + left + inner + right + s[endIdx+1:]
	}
	return s
}
