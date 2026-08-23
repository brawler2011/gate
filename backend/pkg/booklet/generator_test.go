package booklet

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateLatex(t *testing.T) {
	contest := ContestData{
		Title:        "Олимпиада школьников по информатике",
		Organization: "Центр олимпиадного программирования",
		Date:         "22 августа 2026",
		Language:     "ru",
		Problems: []ProblemData{
			{
				Letter:        "A",
				Title:         "Сумма двух чисел",
				TimeLimitMs:   1000,
				MemoryLimitMb: 256,
				InputFile:     "stdin",
				OutputFile:    "stdout",
				Legend:        "Даны два целых числа $A$ и $B$. Найдите их сумму.",
				InputFormat:   "В первой строке записаны два числа $A$ и $B$ ($-10^9 \\le A, B \\le 10^9$).",
				OutputFormat:  "Выведите одно целое число --- сумму $A + B$.",
				Samples: []SampleData{
					{
						Input:  "2 2\n",
						Output: "4\n",
					},
					{
						Input:  "10 -5\n",
						Output: "5\n",
					},
				},
				Notes: "В первом примере $2 + 2 = 4$.",
			},
			{
				Letter:        "B",
				Title:         "Graph Connectivity",
				TimeLimitMs:   2000,
				MemoryLimitMb: 512,
				Legend:        "Проверьте, является ли граф связным.",
				InputFormat:   "Формат ввода.",
				OutputFormat:  "Формат вывода.",
				Samples: []SampleData{
					{
						Input:  "3 2\n1 2\n2 3\n",
						Output: "YES\n",
					},
				},
			},
		},
	}

	latex, err := GenerateLatex(contest)
	require.NoError(t, err)
	assert.Contains(t, latex, "\\usepackage{olymp}")
	assert.Contains(t, latex, "\\begin{problem}{A. Сумма двух чисел}")
	assert.Contains(t, latex, "стандартный ввод")
	assert.Contains(t, latex, "1.0 сек")
	assert.Contains(t, latex, "256 МБ")
	assert.Contains(t, latex, "\\exmp{2 2")
	assert.Contains(t, latex, "\\begin{problem}{B. Graph Connectivity}")
}

func TestCompilePDFLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping LaTeX compilation test in short mode")
	}

	latex := "\\documentclass{article}\\begin{document}Hello World\\end{document}"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pdfBytes, err := CompilePDFLocal(ctx, latex)
	if err != nil && strings.Contains(err.Error(), "neither pdflatex nor xelatex found") {
		t.Skip("neither pdflatex nor xelatex found on system, skipping local compilation test")
	}
	require.NoError(t, err)
	assert.NotEmpty(t, pdfBytes)
	assert.True(t, strings.HasPrefix(string(pdfBytes[:4]), "%PDF"))
}

func TestMarkdownToLatex(t *testing.T) {
	md := `
### Подзадача 1

Текст с **жирным** и *курсивным* начертанием, а также ` + "`код`" + ` и математикой $x_i \le 10^9$.

- Пункт 1
- Пункт 2

` + "```cpp\nint a, b;\ncin >> a >> b;\n```" + `
`
	res := MarkdownToLatex(md)
	assert.Contains(t, res, "\\subsubsection*{Подзадача 1}")
	assert.Contains(t, res, "\\textbf{жирным}")
	assert.Contains(t, res, "\\textit{курсивным}")
	assert.Contains(t, res, "\\texttt{код}")
	assert.Contains(t, res, "$x_i \\le 10^9$")
	assert.Contains(t, res, "\\begin{itemize}")
	assert.Contains(t, res, "\\item Пункт 1")
	assert.Contains(t, res, "\\begin{verbatim}")
	assert.Contains(t, res, "cin >> a >> b;")
}
