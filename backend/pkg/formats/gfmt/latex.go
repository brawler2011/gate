package gfmt

import (
	"sort"
	"strings"
)

// MapLanguageCode maps language names to standard two-letter codes.
func MapLanguageCode(lang string) string {
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

// ConvertTexToMarkdown converts a standard Polygon/PCMS2/ICPC LaTeX problem statement into GFMT markdown.
func ConvertTexToMarkdown(texContent string) string {
	startIdx := strings.Index(texContent, "\\begin{problem}")
	if startIdx == -1 {
		return ""
	}

	content := texContent[startIdx:]
	idx := strings.Index(content, "{")
	if idx == -1 {
		return ""
	}

	// Skip 5 braced arguments: {title}{input}{output}{time}{memory}
	braceCount := 0
	foundArgs := 0
	for i := idx; i < len(content); i++ {
		if content[i] == '{' {
			braceCount++
		} else if content[i] == '}' {
			braceCount--
			if braceCount == 0 {
				foundArgs++
				if foundArgs == 5 {
					content = content[i+1:]
					break
				}
			}
		}
	}

	// Truncate at \end{problem}
	if endIdx := strings.Index(content, "\\end{problem}"); endIdx != -1 {
		content = content[:endIdx]
	}

	type marker struct {
		name string
		pos  int
	}

	var markers []marker
	lowerContent := strings.ToLower(content)

	addMarker := func(mName string, command string) {
		if pos := strings.Index(lowerContent, command); pos != -1 {
			markers = append(markers, marker{name: mName, pos: pos})
		}
	}

	addMarker("input", "\\inputfile")
	addMarker("output", "\\outputfile")
	addMarker("interaction", "\\interaction")
	addMarker("examples", "\\example")
	addMarker("examples", "\\begin{example}")
	addMarker("notes", "\\note")

	sort.Slice(markers, func(i, j int) bool {
		return markers[i].pos < markers[j].pos
	})

	getSectionText := func(start int, end int) string {
		text := content[start:end]
		text = strings.TrimSpace(text)
		
		// Strip the command itself
		lowerText := strings.ToLower(text)
		if strings.HasPrefix(lowerText, "\\inputfile") {
			text = text[10:]
		} else if strings.HasPrefix(lowerText, "\\outputfile") {
			text = text[11:]
		} else if strings.HasPrefix(lowerText, "\\interaction") {
			text = text[12:]
		} else if strings.HasPrefix(lowerText, "\\example") {
			if strings.HasPrefix(lowerText, "\\examples") {
				text = text[9:]
			} else {
				text = text[8:]
			}
		} else if strings.HasPrefix(lowerText, "\\begin{example}") {
			text = text[15:]
		} else if strings.HasPrefix(lowerText, "\\note") {
			if strings.HasPrefix(lowerText, "\\notes") {
				text = text[6:]
			} else {
				text = text[5:]
			}
		}
		return strings.TrimSpace(text)
	}

	var legend, input, output, interaction, notes string
	lastPos := 0
	currentSection := "legend"

	for _, m := range markers {
		sectionText := getSectionText(lastPos, m.pos)
		switch currentSection {
		case "legend":
			legend = sectionText
		case "input":
			input = sectionText
		case "output":
			output = sectionText
		case "interaction":
			interaction = sectionText
		case "notes":
			notes = sectionText
		}
		currentSection = m.name
		lastPos = m.pos
	}

	// Last section to the end of content
	sectionText := getSectionText(lastPos, len(content))
	switch currentSection {
	case "legend":
		legend = sectionText
	case "input":
		input = sectionText
	case "output":
		output = sectionText
	case "interaction":
		interaction = sectionText
	case "notes":
		notes = sectionText
	}

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
