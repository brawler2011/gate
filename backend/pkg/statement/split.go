package statement

import (
	"regexp"
	"strings"
)

// Headings of olymp.sty, Polygon and ICPC statements, and of pandoc's LaTeX from ICPC Markdown; the first group names the section.
var sectionHeading = regexp.MustCompile(`(?i)\\(InputFile|OutputFile|Interaction|Scoring|Examples?|Notes?|Explanations?)\b|\\(?:sub)*section\*?\{\s*(Input|Output|Interaction|Scoring|Examples?|Samples?|Sample input|Notes?|Explanations?)\s*\}(?:\\label\{[^}]*\})?|\\begin\{example\}`)

var sectionNames = map[string]string{
	"inputfile": "input", "input": "input", "outputfile": "output", "output": "output",
	"interaction": "interaction", "scoring": "scoring", "note": "notes", "notes": "notes", "explanation": "notes", "explanations": "notes",
}

// SplitTeX splits a whole-problem LaTeX statement into sections; examples are left out because samples come from tests.
func SplitTeX(tex string) map[string]string {
	content := body(tex)
	sections := map[string]string{}
	current, start := "legend", 0
	add := func(end int) {
		if text := strings.TrimSpace(content[start:end]); text != "" && current != "" {
			if sections[current] != "" {
				text = sections[current] + "\n\n" + text
			}
			sections[current] = text
		}
	}
	for _, m := range sectionHeading.FindAllStringSubmatchIndex(content, -1) {
		add(m[0])
		name := ""
		switch {
		case m[2] >= 0:
			name = content[m[2]:m[3]]
		case m[4] >= 0:
			name = content[m[4]:m[5]]
		}
		// Unknown names are examples, which are dropped.
		current, start = sectionNames[strings.ToLower(name)], m[1]
	}
	add(len(content))
	return sections
}

// body drops the olymp.sty problem header or ICPC's \problemname.
func body(tex string) string {
	start := strings.Index(tex, "\\begin{problem}")
	if start == -1 {
		for {
			i := strings.Index(tex, "\\problemname{")
			if i == -1 {
				return tex
			}
			end := closingBrace(tex, i+len("\\problemname{"))
			if end == -1 {
				return tex
			}
			tex = tex[:i] + tex[end+1:]
		}
	}
	content := tex[start+len("\\begin{problem}"):]
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

var includeGraphics = regexp.MustCompile(`\\includegraphics\s*(?:\[[^\]]*\])?\s*\{([^}]*)\}`)

// Images lists the files \includegraphics shows.
func Images(tex string) []string {
	var names []string
	for _, m := range includeGraphics.FindAllStringSubmatch(uncommented(tex), -1) {
		names = append(names, strings.TrimSpace(m[1]))
	}
	return names
}

// uncommented drops TeX comments: an unescaped % hides the rest of its line.
func uncommented(tex string) string {
	var b strings.Builder
	for i := 0; i < len(tex); i++ {
		switch tex[i] {
		case '\\':
			b.WriteByte(tex[i])
			if i+1 < len(tex) {
				i++
				b.WriteByte(tex[i])
			}
			continue
		case '%':
			for i < len(tex) && tex[i] != '\n' {
				i++
			}
			if i == len(tex) {
				return b.String()
			}
		}
		b.WriteByte(tex[i])
	}
	return b.String()
}

// graphicx tries these extensions after the name as given.
var imageExtensions = []string{"", ".pdf", ".png", ".jpg", ".jpeg"}

// ImageFile finds the file \includegraphics{ref} shows among those exists reports.
func ImageFile(ref string, exists func(name string) bool) (string, bool) {
	for _, ext := range imageExtensions {
		if exists(ref + ext) {
			return ref + ext, true
		}
	}
	return "", false
}
