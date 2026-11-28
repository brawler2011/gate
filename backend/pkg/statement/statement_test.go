package statement

import (
	"context"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestSplitTeXReadsImportedStatements(t *testing.T) {
	olymp := `\documentclass{article}
\begin{document}
\begin{problem}{Sum}{standard input}{standard output}{1 second}{256 megabytes}
Add $a$ and $b$.

\InputFile
Two numbers.

\OutputFile
The sum.

\Examples
\exmp{1 2}{3}%

\Note
Easy.
\end{problem}
\end{document}`
	require.Equal(t, map[string]string{"legend": "Add $a$ and $b$.", "input": "Two numbers.", "output": "The sum.", "notes": "Easy."}, SplitTeX(olymp))

	icpc := "\\problemname{Sum}\nAdd them.\n\\section*{Input}\nTwo numbers.\n\\section*{Output}\nThe sum.\n\\section*{Interaction}\nAsk.\n"
	require.Equal(t, map[string]string{"legend": "Add them.", "input": "Two numbers.", "output": "The sum.", "interaction": "Ask."}, SplitTeX(icpc))

	// pandoc writes ICPC Markdown headings this way.
	markdown := "Add them.\n\n\\subsection{Input}\\label{input}\n\nTwo numbers.\n\n\\subsection{Sample Input}\\label{sample}\n\n1 2\n"
	require.Equal(t, map[string]string{"legend": "Add them.", "input": "Two numbers."}, SplitTeX(markdown))
}

// pandocStub answers as pandoc does and reports one skipped command, found by its text.
type pandocStub struct {
	html    string
	skipped string
}

func (p pandocStub) HTML(_ context.Context, tex string) (string, []sandbox.PandocMessage, error) {
	var log []sandbox.PandocMessage
	for i, line := range strings.Split(tex, "\n") {
		if p.skipped != "" && strings.Contains(line, p.skipped) {
			log = append(log, sandbox.PandocMessage{Type: "SkippedContent", Contents: sandbox.PandocContents(p.skipped), Line: i + 1})
		}
	}
	return p.html, log, nil
}

// Polygon HTML keeps the sections in divs and MathJax in $$$; limits and samples are left to the problem.
func TestSplitHTMLReadsPolygonSections(t *testing.T) {
	page, err := os.ReadFile("../testdata/pcms2/a-plus-b/statements/.html/english/problem.html")
	require.NoError(t, err)
	sections, err := SplitHTML(string(page))
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"legend", "input", "output", "notes"}, slices.Collect(maps.Keys(sections)))
	legend := sections["legend"]
	require.Contains(t, legend.HTML, "integers GATEMATH0END and")
	require.Equal(t, `integers \(a\) and`, legend.LaTeX("integers GATEMATH0END and"))
	require.Contains(t, sections["input"].Math, `\(-100 \le a,b \le 100\)`)
	require.NotContains(t, sections["input"].HTML, "section-title")
	for _, s := range sections {
		require.NotContains(t, s.HTML, "time limit")
		require.NotContains(t, s.HTML, "-106")
	}

	plain, err := SplitHTML("<p>Just text</p>")
	require.NoError(t, err)
	require.Equal(t, map[string]HTMLSection{"legend": {HTML: "<p>Just text</p>"}}, plain)
}

func TestHTMLSplitsSectionsAndSanitizes(t *testing.T) {
	out := `<h1 class="unnumbered" id="gate-section-legend">gate-section-legend</h1>
<p>Text <span class="math inline">\(x\)</span><script>alert(1)</script></p>
<p><img src="pic.png" style="width:2cm" alt="image" /><img src="https://tracker.example/x.png" /></p>
<h1 class="unnumbered" id="gate-section-input">gate-section-input</h1>
<p><a href="javascript:alert(1)">bad</a> <span class="evil" onclick="x()">y</span></p>`
	sections := []Section{{Name: "legend", Text: "Text $x$."}, {Name: "input", Text: "Input."}, {Name: "notes", Text: " \n"}}
	assets := func(name string) bool { return name == "pic.png" || name == "my pic.png" }
	// graphicx would find pic.png for \includegraphics{pic}.
	local := strings.Replace(out, `<img src="https://tracker.example/x.png" />`, `<img src="pic" /><img src="my%20pic.png" />`, 1)
	html, err := HTML(context.Background(), pandocStub{html: local}, sections, assets)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"legend": "<p>Text <span class=\"math inline\">\\(x\\)</span></p>\n<p><img src=\"pic.png\" style=\"width: 2cm\" alt=\"image\"/><img src=\"pic.png\"/><img src=\"my%20pic.png\"/></p>",
		"input":  "<p>bad <span>y</span></p>",
	}, html)

	_, err = HTML(context.Background(), pandocStub{html: local, skipped: "\\bad{x}"}, []Section{{Name: "legend", Text: "Text."}, {Name: "input", Text: "One.\nTwo.\n\\bad{x}"}, {Name: "notes", Text: "N."}}, assets)
	require.Equal(t, &Error{Section: "input", Line: 3, Message: "\\bad{x} is not supported on the site"}, err)

	_, err = HTML(context.Background(), pandocStub{html: local}, []Section{{Name: "legend", Text: "Text."}, {Name: "notes", Text: "Missing."}}, assets)
	require.Equal(t, &Error{Section: "notes", Message: "the section is swallowed by the one before it; check its braces"}, err)

	_, err = HTML(context.Background(), pandocStub{html: out}, sections, assets)
	require.Equal(t, &Error{Section: "legend", Message: `image "https://tracker.example/x.png" is not among the statement's assets`}, err)
}

// pandoc skips the braces after a command it does not know, so a layout command passes only with its own arguments.
func TestHTMLSkipsOnlyLayoutCommands(t *testing.T) {
	out := `<h1 class="unnumbered" id="gate-section-legend">gate-section-legend</h1><p>Text.</p>`
	for skipped, layout := range map[string]bool{
		"\\quad ": true, "\\parindent=1cm": true, "\\itemsep=-1mm plus 0.5mm": true, "\\vskip 0pt plus 1fill": true, "\\hskip\\parindent": true,
		"\\vspace*{2mm}": true, "\\bgroup ": true,
		"\\quad{hidden}": false, "\\vspace{1mm}{hidden}": false, "\\parindent=0pt{hidden}": false, "\\foo": false, "\\begin{foo}": false,
	} {
		_, err := HTML(context.Background(), pandocStub{html: out, skipped: skipped}, []Section{{Name: "legend", Text: skipped + "Text."}}, nil)
		require.Equal(t, !layout, err != nil, skipped)
	}
}

func TestQuantityPicksPluralFormAndSeparator(t *testing.T) {
	for _, c := range []struct {
		lang         string
		value, scale int
		unit, want   string
	}{
		{"ru", 2500, 1000, "second", "2,5~\\limitunit{second}{other}"},
		{"uk", 1000, 1000, "second", "1~\\limitunit{second}{one}"},
		{"en", 2500, 1000, "second", "2.5~\\limitunit{second}{other}"},
		{"en", 1000, 1000, "second", "1~\\limitunit{second}{one}"},
		{"en", 1050, 1000, "second", "1.05~\\limitunit{second}{other}"},
		{"ru", 256, 1, "mebibyte", "256~\\limitunit{mebibyte}{many}"},
		{"ru", 1024, 1, "mebibyte", "1024~\\limitunit{mebibyte}{few}"},
		{"en", 1024, 1, "mebibyte", "1024~\\limitunit{mebibyte}{other}"},
		{"lv", 10000, 1000, "second", "10~\\limitunit{second}{zero}"},
		{"lt", 1500, 1000, "second", "1,5~\\limitunit{second}{many}"},
	} {
		require.Equal(t, c.want, quantity(language.Make(c.lang), c.value, c.scale, c.unit), "%s %d", c.lang, c.value)
	}
}

// Every plural form CLDR picks for a limit must have a word in the language file.
func TestLanguageFilesHaveEveryPluralForm(t *testing.T) {
	require.Contains(t, Languages, "en")
	for _, lang := range Languages {
		text := string(Files(lang)["languages/"+lang+".tex"])
		for _, ms := range []int{0, 1000, 2000, 5000, 11000, 21000, 1500, 2500, 100, 1050} {
			for _, unit := range []string{"second", "mebibyte"} {
				q := quantity(language.Make(lang), ms, 1000, unit)
				form := q[strings.LastIndex(q, "{")+1 : len(q)-1]
				require.Contains(t, text, "\\def\\kw@"+unit+"@"+form+"{", "%s %d %s", lang, ms, unit)
			}
		}
	}
}
