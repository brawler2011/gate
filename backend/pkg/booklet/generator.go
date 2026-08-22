package booklet

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

//go:embed olymp.sty
var OlympSty []byte

type SampleData struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type ProblemData struct {
	Letter        string       `json:"letter"`
	Title         string       `json:"title"`
	TimeLimitMs   int          `json:"time_limit_ms"`
	MemoryLimitMb int          `json:"memory_limit_mb"`
	InputFile     string       `json:"input_file"`
	OutputFile    string       `json:"output_file"`
	Legend        string       `json:"legend"`
	InputFormat   string       `json:"input_format"`
	OutputFormat  string       `json:"output_format"`
	Interaction   string       `json:"interaction"`
	Scoring       string       `json:"scoring"`
	Notes         string       `json:"notes"`
	Samples       []SampleData `json:"samples"`
}

type ContestData struct {
	Title        string        `json:"title"`
	Organization string        `json:"organization"`
	Date         string        `json:"date"`
	Language     string        `json:"language"` // "ru" or "en"
	Problems     []ProblemData `json:"problems"`
}

// GenerateLatex generates a complete LaTeX document for the contest using olymp.sty.
func GenerateLatex(data ContestData) (string, error) {
	lang := strings.ToLower(strings.TrimSpace(data.Language))
	if lang == "" || (lang != "en" && lang != "english") {
		lang = "ru"
	}

	var sb strings.Builder
	sb.WriteString("\\documentclass[11pt,a4paper,oneside]{article}\n")
	sb.WriteString("\\usepackage[T2A]{fontenc}\n")
	sb.WriteString("\\usepackage[utf8]{inputenc}\n")
	if lang == "ru" {
		sb.WriteString("\\usepackage[russian,english]{babel}\n")
	} else {
		sb.WriteString("\\usepackage[english,russian]{babel}\n")
	}
	sb.WriteString("\\usepackage{amsmath,amssymb}\n")
	sb.WriteString("\\usepackage{graphicx}\n")
	sb.WriteString("\\usepackage{olymp}\n\n")

	contestTitle := escapeLatex(strings.TrimSpace(data.Title))
	if contestTitle == "" {
		contestTitle = "Contest"
	}
	orgOrDate := escapeLatex(strings.TrimSpace(data.Organization))
	if orgOrDate == "" {
		orgOrDate = data.Date
	}
	dateStr := escapeLatex(strings.TrimSpace(data.Date))
	if dateStr == "" {
		dateStr = "Gate"
	}

	fmt.Fprintf(&sb, "\\contest{%s}{%s}{%s}\n\n", contestTitle, orgOrDate, dateStr)
	sb.WriteString("\\binoppenalty=10000\n")
	sb.WriteString("\\relpenalty=10000\n\n")
	sb.WriteString("\\begin{document}\n")

	if lang == "ru" {
		sb.WriteString("\\selectlanguage{russian}\n\n")
	} else {
		sb.WriteString("\\selectlanguage{english}\n\n")
	}

	for i, prob := range data.Problems {
		if i > 0 {
			sb.WriteString("\\newpage\n\n")
		}

		problemName := prob.Title
		if prob.Letter != "" {
			problemName = fmt.Sprintf("%s. %s", prob.Letter, prob.Title)
		}
		escapedProblemName := escapeLatex(strings.TrimSpace(problemName))

		inputFile := prob.InputFile
		if inputFile == "" || inputFile == "stdin" {
			if lang == "ru" {
				inputFile = "стандартный ввод"
			} else {
				inputFile = "standard input"
			}
		} else {
			inputFile = escapeLatex(inputFile)
		}

		outputFile := prob.OutputFile
		if outputFile == "" || outputFile == "stdout" {
			if lang == "ru" {
				outputFile = "стандартный вывод"
			} else {
				outputFile = "standard output"
			}
		} else {
			outputFile = escapeLatex(outputFile)
		}

		timeLimitStr := formatTimeLimit(prob.TimeLimitMs, lang)
		memoryLimitStr := formatMemoryLimit(prob.MemoryLimitMb, lang)

		fmt.Fprintf(&sb, "\\begin{problem}{%s}{%s}{%s}{%s}{%s}\n\n",
			escapedProblemName, inputFile, outputFile, timeLimitStr, memoryLimitStr)

		if strings.TrimSpace(prob.Legend) != "" {
			sb.WriteString(MarkdownToLatex(prob.Legend))
			sb.WriteString("\n\n")
		}

		if strings.TrimSpace(prob.InputFormat) != "" {
			sb.WriteString("\\InputFile\n")
			sb.WriteString(MarkdownToLatex(prob.InputFormat))
			sb.WriteString("\n\n")
		}

		if strings.TrimSpace(prob.OutputFormat) != "" {
			sb.WriteString("\\OutputFile\n")
			sb.WriteString(MarkdownToLatex(prob.OutputFormat))
			sb.WriteString("\n\n")
		}

		if strings.TrimSpace(prob.Interaction) != "" {
			sb.WriteString("\\Interaction\n")
			sb.WriteString(MarkdownToLatex(prob.Interaction))
			sb.WriteString("\n\n")
		}

		if strings.TrimSpace(prob.Scoring) != "" {
			sb.WriteString("\\Scoring\n")
			sb.WriteString(MarkdownToLatex(prob.Scoring))
			sb.WriteString("\n\n")
		}

		if len(prob.Samples) > 0 {
			if len(prob.Samples) == 1 {
				sb.WriteString("\\Example\n\n")
			} else {
				sb.WriteString("\\Examples\n\n")
			}
			sb.WriteString("\\begin{example}\n")
			for _, s := range prob.Samples {
				fmt.Fprintf(&sb, "\\exmp{%s}{%s}%%\n", formatSampleContent(s.Input), formatSampleContent(s.Output))
			}
			sb.WriteString("\\end{example}\n\n")
		}

		if strings.TrimSpace(prob.Notes) != "" {
			sb.WriteString("\\Note\n")
			sb.WriteString(MarkdownToLatex(prob.Notes))
			sb.WriteString("\n\n")
		}

		sb.WriteString("\\end{problem}\n\n")
	}

	sb.WriteString("\\end{document}\n")
	return sb.String(), nil
}

func formatTimeLimit(ms int, lang string) string {
	if ms <= 0 {
		ms = 1000
	}
	sec := float64(ms) / 1000.0
	if lang == "ru" {
		if sec == float64(int(sec)) {
			return fmt.Sprintf("%d.0 сек", int(sec))
		}
		return fmt.Sprintf("%.2f сек", sec)
	}
	if sec == 1.0 {
		return "1.0 second"
	}
	if sec == float64(int(sec)) {
		return fmt.Sprintf("%d.0 seconds", int(sec))
	}
	return fmt.Sprintf("%.2f seconds", sec)
}

func formatMemoryLimit(mb int, lang string) string {
	if mb <= 0 {
		mb = 256
	}
	if lang == "ru" {
		if mb >= 1024 && mb%1024 == 0 {
			return fmt.Sprintf("%d ГБ", mb/1024)
		}
		return fmt.Sprintf("%d МБ", mb)
	}
	if mb >= 1024 && mb%1024 == 0 {
		return fmt.Sprintf("%d gigabytes", mb/1024)
	}
	return fmt.Sprintf("%d megabytes", mb)
}

func formatSampleContent(s string) string {
	// Ensure trailing newline for proper olymp.sty formatting
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

// CompilePDF compiles the given LaTeX source into a PDF file using pdflatex or xelatex.
func CompilePDF(ctx context.Context, texSource string) ([]byte, error) {
	tempDir, err := os.MkdirTemp("", "booklet-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write olymp.sty
	if err := os.WriteFile(filepath.Join(tempDir, "olymp.sty"), OlympSty, 0600); err != nil {
		return nil, fmt.Errorf("failed to write olymp.sty: %w", err)
	}

	// Write booklet.tex
	texPath := filepath.Join(tempDir, "booklet.tex")
	if err := os.WriteFile(texPath, []byte(texSource), 0600); err != nil {
		return nil, fmt.Errorf("failed to write booklet.tex: %w", err)
	}

	compiler := "pdflatex"
	if _, err := exec.LookPath("pdflatex"); err != nil {
		if _, errX := exec.LookPath("xelatex"); errX == nil {
			compiler = "xelatex"
		} else {
			return nil, fmt.Errorf("neither pdflatex nor xelatex found on system")
		}
	}

	// Run compiler twice for labels / page numbers
	for run := 1; run <= 2; run++ {
		//nolint:gosec // compiler is selected from fixed binary names pdflatex or xelatex
		cmd := exec.CommandContext(ctx, compiler,
			"-interaction=nonstopmode",
			"-halt-on-error",
			"-output-directory="+tempDir,
			texPath,
		)
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf

		if err := cmd.Run(); err != nil {
			logBytes, _ := os.ReadFile(filepath.Join(tempDir, "booklet.log"))
			logSnippet := string(logBytes)
			if len(logSnippet) > 4000 {
				logSnippet = logSnippet[len(logSnippet)-4000:]
			}
			return nil, fmt.Errorf("LaTeX compilation error on pass %d: %w\nOutput:\n%s\nLog:\n%s", run, err, outBuf.String(), logSnippet)
		}
	}

	pdfPath := filepath.Join(tempDir, "booklet.pdf")
	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated PDF: %w", err)
	}

	return pdfBytes, nil
}

// MarkdownToLatex converts Markdown text to LaTeX while preserving math and LaTeX commands.
func MarkdownToLatex(text string) string {
	if text == "" {
		return ""
	}

	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Segment text into math chunks ($...$, $$...$$, \[...\], \begin{equation}...\end{equation}), code blocks, and plain text chunks
	type chunk struct {
		isSpecial bool
		content   string
	}

	var chunks []chunk
	lastIdx := 0

	// Match display math ($$...$$, \[...\]), inline math ($...$), code blocks (```...```), and inline code (`...`)
	specialRegex := regexp.MustCompile("(?s)(\\$\\$.*?\\$\\$|\\\\\\[.*?\\\\\\]|```.*?```|`[^`\\n]+`|\\$[^$\\n]+\\$)")
	matches := specialRegex.FindAllStringIndex(text, -1)

	for _, m := range matches {
		start, end := m[0], m[1]
		if start > lastIdx {
			chunks = append(chunks, chunk{isSpecial: false, content: text[lastIdx:start]})
		}
		chunks = append(chunks, chunk{isSpecial: true, content: text[start:end]})
		lastIdx = end
	}
	if lastIdx < len(text) {
		chunks = append(chunks, chunk{isSpecial: false, content: text[lastIdx:]})
	}

	var sb strings.Builder
	for _, c := range chunks {
		if c.isSpecial {
			switch {
			case strings.HasPrefix(c.content, "```") && strings.HasSuffix(c.content, "```"):
				lines := strings.Split(c.content, "\n")
				var codeLines []string
				if len(lines) > 2 {
					codeLines = lines[1 : len(lines)-1]
				} else if len(lines) == 2 {
					codeLines = []string{strings.TrimSuffix(lines[1], "```")}
				}
				sb.WriteString("\n\\begin{verbatim}\n")
				sb.WriteString(strings.Join(codeLines, "\n"))
				sb.WriteString("\n\\end{verbatim}\n")
			case strings.HasPrefix(c.content, "`") && strings.HasSuffix(c.content, "`"):
				inner := strings.TrimPrefix(strings.TrimSuffix(c.content, "`"), "`")
				fmt.Fprintf(&sb, "\\texttt{%s}", escapeLatex(inner))
			default:
				// Math mode chunk: output as is
				sb.WriteString(c.content)
			}
		} else {
			// Convert markdown markup in regular text
			sb.WriteString(convertPlainTextToLatex(c.content))
		}
	}

	return sb.String()
}

func convertPlainTextToLatex(text string) string {
	lines := strings.Split(text, "\n")
	var resultLines []string

	inList := false
	listType := "" // "itemize" or "enumerate"

	closeList := func() {
		if inList {
			if listType == "itemize" {
				resultLines = append(resultLines, "\\end{itemize}")
			} else if listType == "enumerate" {
				resultLines = append(resultLines, "\\end{enumerate}")
			}
			inList = false
			listType = ""
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Headers
		if strings.HasPrefix(trimmed, "### ") {
			closeList()
			headerText := strings.TrimPrefix(trimmed, "### ")
			resultLines = append(resultLines, fmt.Sprintf("\\subsubsection*{%s}", formatInlineMarkdown(headerText)))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			closeList()
			headerText := strings.TrimPrefix(trimmed, "## ")
			resultLines = append(resultLines, fmt.Sprintf("\\subsection*{%s}", formatInlineMarkdown(headerText)))
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			closeList()
			headerText := strings.TrimPrefix(trimmed, "# ")
			resultLines = append(resultLines, fmt.Sprintf("\\section*{%s}", formatInlineMarkdown(headerText)))
			continue
		}

		// Bullet lists
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList || listType != "itemize" {
				closeList()
				resultLines = append(resultLines, "\\begin{itemize}")
				inList = true
				listType = "itemize"
			}
			itemText := strings.TrimSpace(trimmed[2:])
			resultLines = append(resultLines, fmt.Sprintf("  \\item %s", formatInlineMarkdown(itemText)))
			continue
		}

		// Numbered lists
		numListRegex := regexp.MustCompile(`^(\d+)\.\s+(.*)$`)
		if m := numListRegex.FindStringSubmatch(trimmed); m != nil {
			if !inList || listType != "enumerate" {
				closeList()
				resultLines = append(resultLines, "\\begin{enumerate}")
				inList = true
				listType = "enumerate"
			}
			resultLines = append(resultLines, fmt.Sprintf("  \\item %s", formatInlineMarkdown(m[2])))
			continue
		}

		// Empty line
		if trimmed == "" {
			closeList()
			resultLines = append(resultLines, "")
			continue
		}

		// Regular line
		resultLines = append(resultLines, formatInlineMarkdown(line))
	}

	closeList()
	return strings.Join(resultLines, "\n")
}

func formatInlineMarkdown(text string) string {
	// Convert bold **text** -> \textbf{text}
	boldRegex := regexp.MustCompile(`\*\*(.*?)\*\*`)
	text = boldRegex.ReplaceAllStringFunc(text, func(m string) string {
		inner := m[2 : len(m)-2]
		return fmt.Sprintf("\\textbf{%s}", escapeLatexSymbols(inner))
	})

	// Convert italic *text* -> \textit{text}
	italicRegex := regexp.MustCompile(`\*([^*]+)\*`)
	text = italicRegex.ReplaceAllStringFunc(text, func(m string) string {
		inner := m[1 : len(m)-1]
		return fmt.Sprintf("\\textit{%s}", escapeLatexSymbols(inner))
	})

	// Escape remaining latex symbols in plain text
	text = escapeLatexSymbols(text)
	return text
}

func escapeLatexSymbols(s string) string {
	var sb strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '&':
			if i > 0 && runes[i-1] == '\\' {
				sb.WriteRune(r)
			} else {
				sb.WriteString("\\&")
			}
		case '%':
			if i > 0 && runes[i-1] == '\\' {
				sb.WriteRune(r)
			} else {
				sb.WriteString("\\%")
			}
		case '#':
			if i > 0 && runes[i-1] == '\\' {
				sb.WriteRune(r)
			} else {
				sb.WriteString("\\#")
			}
		case '_':
			if i > 0 && runes[i-1] == '\\' {
				sb.WriteRune(r)
			} else {
				sb.WriteString("\\_")
			}
		case '~':
			if i > 0 && runes[i-1] == '\\' {
				sb.WriteRune(r)
			} else {
				sb.WriteString("\\textasciitilde{}")
			}
		case '^':
			if i > 0 && runes[i-1] == '\\' {
				sb.WriteRune(r)
			} else {
				sb.WriteString("\\textasciicircum{}")
			}
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func escapeLatex(s string) string {
	s = escapeLatexSymbols(s)
	s = strings.ReplaceAll(s, "{", "\\{")
	s = strings.ReplaceAll(s, "}", "\\}")
	return s
}
