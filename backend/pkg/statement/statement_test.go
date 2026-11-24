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
			log = append(log, sandbox.PandocMessage{Type: "SkippedContent", Contents: p.skipped, Line: i + 1})
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
	html, err := HTML(context.Background(), pandocStub{html: out}, sections)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"legend": "<p>Text <span class=\"math inline\">\\(x\\)</span></p>\n<p><img src=\"pic.png\" style=\"width: 2cm\" alt=\"image\"/></p>",
		"input":  "<p>bad <span>y</span></p>",
	}, html)

	_, err = HTML(context.Background(), pandocStub{html: out, skipped: "\\bad{x}"}, []Section{{Name: "legend", Text: "Text."}, {Name: "input", Text: "One.\nTwo.\n\\bad{x}"}, {Name: "notes", Text: "N."}})
	require.Equal(t, &Error{Section: "input", Line: 3, Message: "\\bad{x} is not supported on the site"}, err)

	_, err = HTML(context.Background(), pandocStub{html: out}, []Section{{Name: "legend", Text: "Text."}, {Name: "notes", Text: "Missing."}})
	require.Equal(t, &Error{Section: "notes", Message: "the section is swallowed by the one before it; check its braces"}, err)
}
