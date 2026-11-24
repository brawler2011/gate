//go:build integration

package sandbox_test

import (
	"context"
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
			{Name: "legend", Text: "\\newcommand{\\NN}{\\mathbb{N}}\nGiven $a \\in \\NN$ <script>alert(1)</script>.\n\n\\includegraphics[width=2cm]{graph.svg}\n\n\\begin{tabular}{|c|c|}\\hline a & b \\\\ \\hline\\end{tabular}"},
			{Name: "input", Text: "One number $n \\in \\NN$ and \\t{x}; \\href{javascript:alert(1)}{link}."},
		})
		require.NoError(t, err)
		require.Contains(t, html["input"], `\(n \in \mathbb{N}\)`)
		require.Contains(t, html["legend"], "<table>")
		require.Contains(t, html["legend"], `src="graph.svg"`)
		require.Contains(t, html["input"], "<code>x</code>")
		require.NotContains(t, html["legend"], "<script")
		require.NotContains(t, html["input"], "javascript:")
	})

	t.Run("HTML names what the site cannot show", func(t *testing.T) {
		_, err := statement.HTML(ctx, ts, []statement.Section{{Name: "legend", Text: "Fine."}, {Name: "notes", Text: "One.\nTwo \\exmp{1}{2}."}})
		var e *statement.Error
		require.ErrorAs(t, err, &e)
		require.Equal(t, statement.Error{Section: "notes", Line: 2, Message: e.Message}, *e)
		require.Contains(t, e.Message, "\\exmp")
	})

	problem := func(legend string) statement.Contest {
		return statement.Contest{Title: "Contest", Language: "ru", Problems: []statement.Problem{{Title: "Сумма", TimeLimitMs: 1000, MemoryLimitMb: 256, Assets: "assets",
			Sections: []statement.Section{{Name: "legend", Text: legend}, {Name: "input", Text: "Два числа."}}, Samples: []statement.Sample{{Input: "1 2", Output: "3"}}}}}
	}

	t.Run("PDF prints Cyrillic outline fonts and SVG images", func(t *testing.T) {
		svg := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><text x="0" y="15">A</text></svg>`
		doc := statement.NewDocument(problem("\\pdfcompresslevel=0 Сложите числа.\n\n\\includegraphics{pic.svg}"))
		pdf, err := ts.PDF(ctx, doc.Source, map[string][]byte{"olymp.sty": statement.OlympSty, "assets/pic.svg": []byte(svg)})
		require.NoError(t, err)
		require.True(t, strings.HasPrefix(string(pdf), "%PDF"))
		require.NotContains(t, string(pdf), "/Type3", "Cyrillic text falls back to bitmap fonts")
	})

	t.Run("PDF errors name the section and its line", func(t *testing.T) {
		doc := statement.NewDocument(problem("First line.\n\\undefinedmacro"))
		_, err := ts.PDF(ctx, doc.Source, map[string][]byte{"olymp.sty": statement.OlympSty})
		var e *statement.Error
		require.ErrorAs(t, doc.Explain(err), &e)
		require.Equal(t, "legend", e.Section)
		require.Equal(t, 2, e.Line)
		require.Contains(t, e.Message, "Undefined control sequence")
	})

	t.Run("ICPC Markdown becomes LaTeX that both typesetters read", func(t *testing.T) {
		tex, _, err := ts.LaTeX(ctx, "Add **two** numbers $a+b$:\n\n- one\n- two\n\n| a | b |\n|---|--:|\n| 1 | 2 |\n\n## Input\n\nTwo integers.\n\n## Output\n\nTheir sum.\n")
		require.NoError(t, err)
		split := statement.SplitTeX(tex)
		require.Contains(t, split["legend"], "\\textbf{two}")
		require.Equal(t, "Two integers.", split["input"])
		require.Equal(t, "Their sum.", split["output"])

		var sections []statement.Section
		for _, name := range statement.Order {
			sections = append(sections, statement.Section{Name: name, Text: split[name]})
		}
		html, err := statement.HTML(ctx, ts, sections)
		require.NoError(t, err)
		require.Contains(t, html["legend"], "<td style=\"text-align: right\">2</td>")
		doc := statement.NewDocument(statement.Contest{Language: "en", Problems: []statement.Problem{{Title: "Sum", Sections: sections}}})
		_, err = ts.PDF(ctx, doc.Source, map[string][]byte{"olymp.sty": statement.OlympSty})
		require.NoError(t, err)
	})
}
