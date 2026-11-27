package problemsmodel

import (
	"slices"

	"github.com/brawler2011/gate/backend/pkg/statement"
)

// ImageReferences lists the asset names \includegraphics shows.
func ImageReferences(tex string) []string { return statement.Images(tex) }

// BuiltStatement is a draft statement with the HTML its build made of each nonempty section.
type BuiltStatement struct {
	DraftStatement
	HTML map[string]string `json:"html"`
}

// StatementPDF is a statement compiled for print, stored as its own object.
type StatementPDF struct {
	Language string
	Object   StoredObject
}

// Sections lists the statement's sections in statement.Order.
func (s *DraftStatement) Sections() []statement.Section {
	sections := make([]statement.Section, 0, len(statement.Order))
	for _, name := range statement.Order {
		sections = append(sections, statement.Section{Name: name, Text: *s.Section(name)})
	}
	return sections
}

// SiteSections are the sections the site shows: the printed ones and the tutorial.
func (s *DraftStatement) SiteSections() []statement.Section {
	return append(s.Sections(), statement.Section{Name: "tutorial", Text: s.Tutorial})
}

// PrintedProblem lays out a statement for olymp.sty; images are under the assets directory.
func PrintedProblem(title string, limits PrintedLimits, s DraftStatement, samples []Sample) statement.Problem {
	if s.Title != "" {
		title = s.Title
	}
	p := statement.Problem{Title: title, TimeLimitMs: limits.TimeLimitMs, MemoryLimitMb: limits.MemoryLimitMb, InputFile: limits.InputFile, OutputFile: limits.OutputFile, Sections: s.Sections(), Assets: "assets"}
	for _, sample := range samples {
		p.Samples = append(p.Samples, statement.Sample{Input: sample.Input, Output: sample.Output})
	}
	return p
}

type PrintedLimits struct {
	TimeLimitMs           *int
	MemoryLimitMb         int
	InputFile, OutputFile string
}

// StatementImages lists the assets the statement shows that exist.
func StatementImages(s DraftStatement, assets []DraftAsset) []DraftAsset {
	exists := func(name string) bool { return slices.ContainsFunc(assets, func(a DraftAsset) bool { return a.Name == name }) }
	shown := map[string]bool{}
	for _, name := range statement.Order {
		for _, ref := range ImageReferences(*s.Section(name)) {
			if file, ok := statement.ImageFile(ref, exists); ok {
				shown[file] = true
			}
		}
	}
	var result []DraftAsset
	for _, a := range assets {
		if shown[a.Name] {
			result = append(result, a)
		}
	}
	return result
}
