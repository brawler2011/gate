// Package tex converts LaTeX problem statements of Polygon, PCMS2 and ICPC
// packages into Gate statement Markdown.
package tex

import (
	"sort"
	"strings"
)

// LanguageCode maps a language name such as "russian" to its two-letter code.
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

// sectionCommands start statement sections: olymp.sty commands and the
// \section* headings of ICPC statements. At one position the first wins, so
// \examples is not also read as \example.
var sectionCommands = []struct{ section, command string }{
	{"input", "\\inputfile"}, {"input", "\\section*{input}"},
	{"output", "\\outputfile"}, {"output", "\\section*{output}"},
	{"interaction", "\\interaction"}, {"interaction", "\\section*{interaction}"},
	{"examples", "\\examples"}, {"examples", "\\example"}, {"examples", "\\begin{example}"},
	{"notes", "\\notes"}, {"notes", "\\note"}, {"notes", "\\section*{notes}"},
}

// ToMarkdown converts a problem statement into statement Markdown: an
// olymp.sty problem environment (Polygon, PCMS2) or an ICPC problem.tex body.
func ToMarkdown(texContent string) string {
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

		// Clean quotes inside \item or text
		s = strings.ReplaceAll(s, "\">\"", "`>`")
		s = strings.ReplaceAll(s, "\"<\"", "`<`")
		s = strings.ReplaceAll(s, "\"=\"", "`=`")

		return strings.TrimSpace(s)
	}

	legend = cleanLatex(legend)
	input = cleanLatex(input)
	output = cleanLatex(output)
	interaction = cleanLatex(interaction)
	notes = cleanLatex(notes)

	var parts []string
	// In GFMT format, even if legend is empty, it starts with the delimiter
	parts = append(parts, "<!--legend -->")
	if legend != "" {
		parts[0] = parts[0] + "\n\n" + legend
	}
	if input != "" {
		parts = append(parts, "<!-- input -->\n\n"+input)
	}
	if output != "" {
		parts = append(parts, "<!-- output -->\n\n"+output)
	}
	if interaction != "" {
		parts = append(parts, "<!-- interaction -->\n\n"+interaction)
	}
	if notes != "" {
		parts = append(parts, "<!-- notes -->\n\n"+notes)
	}

	return strings.Join(parts, "\n\n") + "\n"
}

// body returns the statement text: the problem environment without its five
// header arguments, or else the whole document without \problemname.
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
	// Skip {title}{input}{output}{time}{memory}.
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

// closingBrace returns the index of the brace closing the group that starts
// at from, just after its opening brace, or -1.
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
