// Package statement typesets LaTeX statement sections: PDF through olymp.sty, HTML through pandoc.
package statement

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

//go:embed olymp.sty languages
var files embed.FS

// Languages are the statement languages: those with a file in languages/.
var Languages = func() []string {
	entries, err := fs.ReadDir(files, "languages")
	if err != nil {
		panic(err)
	}
	var codes []string
	for _, e := range entries {
		codes = append(codes, strings.TrimSuffix(e.Name(), path.Ext(e.Name())))
	}
	return codes
}()

// Files are what a document in the language needs next to it.
func Files(lang string) map[string][]byte {
	result := map[string][]byte{}
	for _, name := range []string{"olymp.sty", "languages/" + lang + ".tex"} {
		data, err := files.ReadFile(name)
		if err != nil {
			panic(err)
		}
		result[name] = data
	}
	return result
}

// Version keys stored booklets: bump it whenever NewDocument, olymp.sty or a language file print differently.
const Version = 2

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
	// TimeLimitMs is nil in a draft preview while the build derives the limit; the field stays empty.
	TimeLimitMs           *int
	MemoryLimitMb         int
	InputFile, OutputFile string
	// Sections in Order; empty ones are left out.
	Sections []Section
	Samples  []Sample
	// Assets is the directory of the problem's images, relative to the document.
	Assets string
}

type Contest struct {
	Title, Place, Date string
	// Language is one of Languages.
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
	tag := language.Make(c.Language)
	w := &writer{line: 1}
	var spans []span
	w.text("\\documentclass[11pt,a4paper,oneside]{article}\n")
	// SVG images arrive converted to PDF under their own names.
	w.text("\\usepackage{amsmath,amssymb}\n\\usepackage{graphicx}\n\\DeclareGraphicsRule{.svg}{pdf}{*}{}\n")
	w.text(fmt.Sprintf("\\usepackage[%s]{olymp}\n", c.Language))
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
		var limit string
		if p.TimeLimitMs != nil {
			limit = quantity(tag, *p.TimeLimitMs, 1000, "second")
		}
		w.text(fmt.Sprintf("\\begin{problem}{%s}{%s}{%s}{%s}{%s}\n", escape(name), stream(p.InputFile, "\\standardinput"), stream(p.OutputFile, "\\standardoutput"), limit, quantity(tag, p.MemoryLimitMb, 1, "mebibyte")))
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

func stream(file, standard string) string {
	if file == "" {
		return standard
	}
	return escape(file)
}

// quantity prints value/scale in the language with up to three decimals and the unit's word in the plural form CLDR picks for it.
func quantity(tag language.Tag, value, scale int, unit string) string {
	whole, fraction := value/scale, strconv.Itoa(value%scale+scale)[1:]
	fraction = strings.TrimRight(fraction, "0")
	digits := 0
	if fraction != "" {
		digits, _ = strconv.Atoi(fraction)
	}
	form := plural.Cardinal.MatchPlural(tag, whole, len(fraction), len(fraction), digits, digits)
	n := message.NewPrinter(tag).Sprint(number.Decimal(float64(value)/float64(scale), number.NoSeparator(), number.MaxFractionDigits(3)))
	return fmt.Sprintf("%s~\\limitunit{%s}{%s}", n, unit, forms[form])
}

var forms = map[plural.Form]string{plural.Zero: "zero", plural.One: "one", plural.Two: "two", plural.Few: "few", plural.Many: "many", plural.Other: "other"}

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
