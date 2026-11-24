// Package statement typesets LaTeX statement sections: PDF through olymp.sty, HTML through pandoc.
package statement

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

//go:embed olymp.sty
var OlympSty []byte

// Version keys stored booklets: bump it whenever NewDocument or olymp.sty print differently.
const Version = 1

// Order is the order of sections in a statement; macros a section defines apply in the sections after it.
var Order = []string{"legend", "input", "output", "interaction", "scoring", "notes"}

type Section struct {
	Name string
	Text string
}

type Sample struct {
	Input  string
	Output string
}

type Problem struct {
	Letter                     string
	Title                      string
	TimeLimitMs, MemoryLimitMb int
	InputFile, OutputFile      string
	// Sections in Order; empty ones are left out.
	Sections []Section
	Samples  []Sample
	// Assets is the directory of the problem's images, relative to the document.
	Assets string
}

type Contest struct {
	Title, Place, Date string
	// Language selects babel hyphenation and the words olymp.sty prints.
	Language string
	Problems []Problem
}

// Document is LaTeX source that knows which section each line comes from.
type Document struct {
	Source string
	spans  []span
}

type span struct {
	problem    int
	section    string
	start, end int
}

// Locate maps a line of Source to a problem index, a section and a line of that section.
func (d Document) Locate(line int) (problem int, section string, at int, ok bool) {
	for _, s := range d.spans {
		if line >= s.start && line < s.end {
			return s.problem, s.section, line - s.start + 1, true
		}
	}
	return 0, "", 0, false
}

// Explain turns a LaTeX error at a line of Source into an Error in its section.
func (d Document) Explain(err error) error {
	var tex *sandbox.TeXError
	if !errors.As(err, &tex) {
		return err
	}
	if _, section, line, ok := d.Locate(tex.Line); ok {
		return &Error{Section: section, Line: line, Message: tex.Message}
	}
	return &Error{Message: tex.Message}
}

var headings = map[string]string{"input": "\\InputFile", "output": "\\OutputFile", "interaction": "\\Interaction", "scoring": "\\Scoring", "notes": "\\Note"}

type writer struct {
	strings.Builder
	line int
}

func (w *writer) text(s string) {
	w.WriteString(s)
	w.line += strings.Count(s, "\n")
}

func NewDocument(c Contest) Document {
	russian := c.Language == "ru"
	w := &writer{line: 1}
	var spans []span
	w.text("\\documentclass[11pt,a4paper,oneside]{article}\n\\usepackage[T2A]{fontenc}\n\\usepackage[utf8]{inputenc}\n")
	if russian {
		w.text("\\usepackage[english,russian]{babel}\n")
	} else {
		w.text("\\usepackage[russian,english]{babel}\n")
	}
	// SVG images arrive converted to PDF under their own names.
	w.text("\\usepackage{amsmath,amssymb}\n\\usepackage{graphicx}\n\\DeclareGraphicsRule{.svg}{pdf}{*}{}\n\\usepackage{olymp}\n")
	w.text(fmt.Sprintf("\\contest{%s}{%s}{%s}\n", escape(c.Title), escape(c.Place), escape(c.Date)))
	if c.Date == "" {
		// olymp.sty ends the header line with \\, which fails after an empty paragraph.
		header := "\\thecontestname"
		if strings.TrimSpace(c.Place) != "" {
			header += "\\par\\thecontestlocation"
		}
		w.text("\\DeclareRobustCommand{\\contestname}{" + header + "\\strut}\n")
	}
	w.text("\\binoppenalty=10000\n\\relpenalty=10000\n\\begin{document}\n")
	for i, p := range c.Problems {
		if i > 0 {
			w.text("\\newpage\n")
		}
		name := p.Title
		if p.Letter != "" {
			name = p.Letter + ". " + p.Title
		}
		if p.Assets != "" {
			w.text(fmt.Sprintf("\\graphicspath{{%s/}}\n", p.Assets))
		}
		w.text(fmt.Sprintf("\\begin{problem}{%s}{%s}{%s}{%s}{%s}\n", escape(name), stream(p.InputFile, "стандартный ввод", "standard input", russian), stream(p.OutputFile, "стандартный вывод", "standard output", russian), timeLimit(p.TimeLimitMs, russian), memoryLimit(p.MemoryLimitMb, russian)))
		section := func(s Section) {
			if h, ok := headings[s.Name]; ok {
				w.text(h + "\n")
			}
			start := w.line
			w.text(s.Text + "\n")
			spans = append(spans, span{problem: i, section: s.Name, start: start, end: w.line})
			w.text("\n")
		}
		for _, s := range p.Sections {
			if s.Name != "notes" && strings.TrimSpace(s.Text) != "" {
				section(s)
			}
		}
		if len(p.Samples) > 0 {
			if len(p.Samples) == 1 {
				w.text("\\Example\n")
			} else {
				w.text("\\Examples\n")
			}
			w.text("\\begin{example}\n")
			for _, s := range p.Samples {
				w.text(fmt.Sprintf("\\exmp{%s}{%s}%%\n", sample(s.Input), sample(s.Output)))
			}
			w.text("\\end{example}\n")
		}
		for _, s := range p.Sections {
			if s.Name == "notes" && strings.TrimSpace(s.Text) != "" {
				section(s)
			}
		}
		w.text("\\end{problem}\n")
	}
	w.text("\\end{document}\n")
	return Document{Source: w.String(), spans: spans}
}

func stream(file, ru, en string, russian bool) string {
	switch {
	case file != "":
		return escape(file)
	case russian:
		return ru
	default:
		return en
	}
}

func timeLimit(ms int, russian bool) string {
	s := fmt.Sprintf("%g", float64(ms)/1000)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	switch {
	case russian:
		return s + " сек"
	case ms == 1000:
		return s + " second"
	default:
		return s + " seconds"
	}
}

func memoryLimit(mb int, russian bool) string {
	unit, value := "megabytes", mb
	if mb >= 1024 && mb%1024 == 0 {
		unit, value = "gigabytes", mb/1024
	}
	if russian {
		unit = map[string]string{"megabytes": "МБ", "gigabytes": "ГБ"}[unit]
	}
	return fmt.Sprintf("%d %s", value, unit)
}

// olymp.sty reads samples verbatim up to a final newline.
func sample(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

var escapes = strings.NewReplacer("\\", "\\textbackslash{}", "{", "\\{", "}", "\\}", "&", "\\&", "%", "\\%", "$", "\\$", "#", "\\#", "_", "\\_", "~", "\\textasciitilde{}", "^", "\\textasciicircum{}")

func escape(s string) string { return escapes.Replace(strings.TrimSpace(s)) }
