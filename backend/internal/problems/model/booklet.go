package problemsmodel

import (
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/booklet"
)

// SelectStatement falls back from a missing language to English, then to alphabetical order.
func SelectStatement(title string, statements []DraftStatement, language string) *Statement {
	find := func(language string) *DraftStatement {
		i := slices.IndexFunc(statements, func(s DraftStatement) bool { return s.Language == language })
		if i < 0 {
			return nil
		}
		return &statements[i]
	}
	s := find(language)
	if s == nil {
		s = find("en")
	}
	if s == nil && len(statements) > 0 {
		first := slices.MinFunc(statements, func(a, b DraftStatement) int { return strings.Compare(a.Language, b.Language) })
		s = &first
	}
	statement := Statement{Title: title}
	if s != nil {
		statement.Legend, statement.InputFormat, statement.OutputFormat = s.Legend, s.Input, s.Output
		statement.Notes, statement.Interaction, statement.Scoring = s.Notes, s.Interaction, s.Scoring
		if s.Title != "" {
			statement.Title = s.Title
		}
	}
	return &statement
}

// BookletProblem lays a problem out for LaTeX; statement PDFs and contest booklets share it so that a package that builds also prints.
type BookletProblem struct {
	Letter                     string
	Title                      string
	TimeLimitMs, MemoryLimitMb int
	InputFile, OutputFile      string
	Statements                 []DraftStatement
	Samples                    []Sample
}

func (p BookletProblem) Data(language string) booklet.ProblemData {
	s := SelectStatement(p.Title, p.Statements, language)
	samples := make([]booklet.SampleData, len(p.Samples))
	for i, sample := range p.Samples {
		samples[i] = booklet.SampleData{Input: sample.Input, Output: sample.Output}
	}
	return booklet.ProblemData{
		Letter: p.Letter, Title: strings.TrimSpace(s.Title), TimeLimitMs: p.TimeLimitMs, MemoryLimitMb: p.MemoryLimitMb,
		InputFile: p.InputFile, OutputFile: p.OutputFile, Legend: s.Legend, InputFormat: s.InputFormat, OutputFormat: s.OutputFormat,
		Interaction: s.Interaction, Scoring: s.Scoring, Notes: s.Notes, Samples: samples,
	}
}
