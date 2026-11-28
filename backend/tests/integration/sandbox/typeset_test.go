//go:build integration

package sandbox_test

import (
	"context"
	"encoding/base64"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/brawler2011/gate/backend/tests/dbfixtures"
	"github.com/stretchr/testify/require"
)

func TestTypesetStatements(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	ts := dbfixtures.Typesetter(t)

	t.Run("HTML keeps macros across sections and drops scripts", func(t *testing.T) {
		html, err := statement.HTML(ctx, ts, []statement.Section{
			{Name: "legend", Text: "\\newcommand{\\NN}{\\mathbb{N}}\nGiven $a \\in \\NN$ <script>alert(1)</script>.\n\n\\includegraphics[width=2cm]{graph}\n% \\includegraphics{gone.png}\n\n\\begin{tabular}{|c|c|}\\hline a & b \\\\ \\hline\\end{tabular}"},
			{Name: "input", Text: "One number $n \\in \\NN$ and \\t{x}; \\href{javascript:alert(1)}{link}."},
		}, func(name string) bool { return name == "graph.png" })
		require.NoError(t, err)
		require.Contains(t, html["input"], `\(n \in \mathbb{N}\)`)
		require.Contains(t, html["legend"], "<table>")
		require.Contains(t, html["legend"], `src="graph.png"`)
		require.NotContains(t, html["legend"], "gone")
		require.Contains(t, html["input"], "<code>x</code>")
		require.NotContains(t, html["legend"], "<script")
		require.NotContains(t, html["input"], "javascript:")
	})

	t.Run("HTML drops layout commands and keeps Kattis and NERC macros", func(t *testing.T) {
		html, err := statement.HTML(ctx, ts, []statement.Section{{Name: "legend", Text: "\\noindent\\parindent=1cm Text\\texttrademark{}.\\vspace*{2mm}\\medskip\n\n{\\small small}\n\n" +
			"\\begin{center}\\includegraphics{pic.png}\\end{center}\n\n\\illustration{0.3}{pic.png}{By X}\n\n\\begin{shortitems}\\item one\\end{shortitems}\n\n" +
			"\\setlength{\\parskip}{1ex}\\pagebreak[2]\\begin{large}Big $a\\quad b$.\\end{large}"}}, func(string) bool { return true })
		require.NoError(t, err)
		require.Contains(t, html["legend"], "<p>Text™.</p>\n<p><span>small</span></p>")
		require.Contains(t, html["legend"], `<p class="image"><img src="pic.png"`)
		require.Contains(t, html["legend"], `<img src="pic.png" style="width: 30.0%"`)
		require.Contains(t, html["legend"], "<figcaption>By X</figcaption>")
		require.Contains(t, html["legend"], "<li><p>one</p></li>")
		require.Contains(t, html["legend"], `Big <span class="math inline">\(a\quad b\)</span>.`)
	})

	t.Run("HTML names what the site cannot show", func(t *testing.T) {
		_, err := statement.HTML(ctx, ts, []statement.Section{{Name: "legend", Text: "Fine."}, {Name: "notes", Text: "One.\nTwo \\exmp{1}{2}."}}, nil)
		var e *statement.Error
		require.ErrorAs(t, err, &e)
		require.Equal(t, statement.Error{Section: "notes", Line: 2, Message: e.Message}, *e)
		require.Contains(t, e.Message, "\\exmp")
	})

	problem := func(legend string) statement.Contest {
		return statement.Contest{Title: "Contest", Language: "ru", Problems: []statement.Problem{{Title: "Сумма", TimeLimitMs: new(1000), MemoryLimitMb: 256, Assets: "assets",
			Sections: []statement.Section{{Name: "legend", Text: legend}, {Name: "input", Text: "Два числа."}}, Samples: []statement.Sample{{Input: "1 2", Output: "3"}}}}}
	}

	t.Run("PDF prints Cyrillic outline fonts and SVG images", func(t *testing.T) {
		svg := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><text x="0" y="15">A</text></svg>`
		doc := statement.NewDocument(problem("\\pdfvariable compresslevel=0 Сложите числа.\n\n\\includegraphics{pic.svg}"))
		files := statement.Files("ru")
		files["assets/pic.svg"] = []byte(svg)
		pdf, err := ts.PDF(ctx, doc.Source, files)
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(string(pdf), "%PDF"))
		require.NotContains(t, string(pdf), "/Type3", "Cyrillic text falls back to bitmap fonts")
	})

	t.Run("PDF wraps text around an illustration", func(t *testing.T) {
		doc := statement.NewDocument(problem("\\noindent\\illustration{0.3}{pic.png}{By X}Text.\\begin{shortitems}\\item one\\end{shortitems}"))
		png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAAAAAA6fptVAAAACklEQVR4nGNgAAAAAgABSK+kcQAAAABJRU5ErkJggg==")
		require.NoError(t, err)
		files := statement.Files("ru")
		files["assets/pic.png"] = png
		_, err = ts.PDF(ctx, doc.Source, files)
		require.NoError(t, err)
	})

	t.Run("Pictures turn MetaPost and EPS into SVG", func(t *testing.T) {
		mp := "prologues := 3;\noutputtemplate := \"%j-%c.eps\";\nbeginfig(1) draw (0,0)--(1cm,1cm); label(btex $x_1$ etex, (0,0)); endfig;\nbeginfig(2) draw fullcircle scaled 1cm; endfig;\nend\n"
		eps := "%!PS-Adobe-3.0 EPSF-3.0\n%%BoundingBox: 0 0 20 20\nnewpath 0 0 moveto 20 20 lineto stroke\nshowpage\n"
		pipe := "%!PS-Adobe-3.0 EPSF-3.0\n%%BoundingBox: 0 0 20 20\n(%pipe%echo pwned > pwned.eps) (w) file closefile\nshowpage\n"
		svgs, err := ts.Pictures(ctx, map[string][]byte{"pic.mp": []byte(mp), "line.eps": []byte(eps), "pipe.eps": []byte(pipe)})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"pic-1.eps", "pic-2.eps", "line.eps"}, slices.Collect(maps.Keys(svgs)), "SAFER Ghostscript refuses pipe.eps")
		for name, svg := range svgs {
			require.Contains(t, string(svg), "<svg", name)
		}
	})

	t.Run("PDF typesets every statement language", func(t *testing.T) {
		text := "Sample: Þ ð ä ö å ø ł ą ė ū ā š ž ü ß Привіт 你好 こんにちは."
		for _, lang := range statement.Languages {
			c := problem(text)
			c.Language = lang
			doc := statement.NewDocument(c)
			pdf, err := ts.PDF(ctx, doc.Source, statement.Files(lang))
			require.NoError(t, err, lang)
			require.True(t, strings.HasPrefix(string(pdf), "%PDF"), lang)
		}
	})

	t.Run("PDF errors name the section and its line", func(t *testing.T) {
		doc := statement.NewDocument(problem("First line.\n\\undefinedmacro"))
		_, err := ts.PDF(ctx, doc.Source, statement.Files("ru"))
		var e *statement.Error
		require.ErrorAs(t, doc.Explain(err), &e)
		require.Equal(t, "legend", e.Section)
		require.Equal(t, 2, e.Line)
		require.Contains(t, e.Message, "Undefined control sequence")
	})

	t.Run("PDF names a missing image", func(t *testing.T) {
		doc := statement.NewDocument(problem("Text.\n\\includegraphics{nope}"))
		_, err := ts.PDF(ctx, doc.Source, statement.Files("ru"))
		var e *statement.Error
		require.ErrorAs(t, doc.Explain(err), &e)
		require.Equal(t, "legend", e.Section)
		require.Contains(t, e.Message, "nope")
	})

	t.Run("PCMS2 HTML keeps its math", func(t *testing.T) {
		sections, err := statement.SplitHTML("<div class=\"legend\"><p>Print $$$a+b$$$.</p></div>")
		require.NoError(t, err)
		tex, _, err := ts.LaTeX(ctx, "html", sections["legend"].HTML)
		require.NoError(t, err)
		require.Equal(t, "Print \\(a+b\\).", strings.TrimSpace(sections["legend"].LaTeX(tex)))
	})

	t.Run("ICPC Markdown becomes LaTeX that both typesetters read", func(t *testing.T) {
		tex, _, err := ts.LaTeX(ctx, "markdown", "Add **two** numbers $a+b$:\n\n- one\n- two\n\n| a | b |\n|---|--:|\n| 1 | 2 |\n\n## Input\n\nTwo integers.\n\n## Output\n\nTheir sum.\n")
		require.NoError(t, err)
		split := statement.SplitTeX(tex)
		require.Contains(t, split["legend"], "\\textbf{two}")
		require.Equal(t, "Two integers.", split["input"])
		require.Equal(t, "Their sum.", split["output"])

		var sections []statement.Section
		for _, name := range statement.Order {
			sections = append(sections, statement.Section{Name: name, Text: split[name]})
		}
		html, err := statement.HTML(ctx, ts, sections, nil)
		require.NoError(t, err)
		require.Contains(t, html["legend"], "<td style=\"text-align: right\">2</td>")
		doc := statement.NewDocument(statement.Contest{Language: "en", Problems: []statement.Problem{{Title: "Sum", Sections: sections}}})
		_, err = ts.PDF(ctx, doc.Source, statement.Files("en"))
		require.NoError(t, err)
	})
}
