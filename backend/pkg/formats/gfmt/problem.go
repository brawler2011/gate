package gfmt

import (
	"cmp"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/points"
)

type Problem struct {
	// Gate writes the max score of scoring problems for the judge; imports recompute it.
	MaxScore      *points.Value      `yaml:"max_score,omitempty" json:"max_score,omitempty"`
	Sources       map[string]Source  `yaml:"sources,omitempty" json:"sources,omitempty"`
	FormatVersion string             `yaml:"format_version" json:"format_version"`
	Title         string             `yaml:"title" json:"title"`
	// Titles by statement language override Title for that statement.
	Titles   map[string]string  `yaml:"titles,omitempty" json:"titles,omitempty"`
	Type     string             `yaml:"type" json:"type"` // "pass-fail", "interactive", "scoring"
	Limits   Limits             `yaml:"limits" json:"limits"`
	Subtasks map[string]Subtask `yaml:"subtasks" json:"subtasks"`
	// Empty names keep standard input and output.
	InputFile  string `yaml:"input_file,omitempty" json:"input_file,omitempty"`
	OutputFile string `yaml:"output_file,omitempty" json:"output_file,omitempty"`
	// MainSolution names the solutions/ file whose output answers tests without an answer.
	MainSolution string                 `yaml:"main_solution,omitempty" json:"main_solution,omitempty"`
	Solutions    map[string]Expectation `yaml:"solutions,omitempty" json:"solutions,omitempty"`
	// System library files have no package copy; Gate provides them under lib/ names.
	SystemLibrary []string `yaml:"system_library,omitempty" json:"system_library,omitempty"`
}

type Limits struct {
	TimeMs   int `yaml:"time_ms" json:"time_ms"`
	MemoryMb int `yaml:"memory_mb" json:"memory_mb"`
}

type Subtask struct {
	Ordinal      int          `yaml:"ordinal,omitempty" json:"ordinal,omitempty"`
	Points       points.Value `yaml:"points" json:"points"`
	Policy       string       `yaml:"policy" json:"policy"` // "complete", "each"
	Dependencies []string     `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Tests        []Test       `yaml:"tests" json:"tests"`
}

// A built package holds every input and answer as a file; Generate and Solution stay the source on import.
type Test struct {
	Points  points.Value `yaml:"points" json:"points"`
	Ordinal int          `yaml:"ordinal,omitempty" json:"ordinal,omitempty"`
	// Input names a file under tests/.
	Input    string `yaml:"input,omitempty" json:"input,omitempty"`
	Generate string `yaml:"generate,omitempty" json:"generate,omitempty"`
	Answer   string `yaml:"answer,omitempty" json:"answer,omitempty"`
	// Solution names the solutions/ file whose output is the answer.
	Solution string `yaml:"solution,omitempty" json:"solution,omitempty"`
	Sample   bool   `yaml:"sample,omitempty" json:"sample,omitempty"`
}

func (p *Problem) hasOrdinals() bool {
	for _, s := range p.Subtasks {
		if s.Ordinal <= 0 {
			return false
		}
	}
	return len(p.Subtasks) > 0
}

// Order by ordinals when every subtask has one; otherwise samples come first, then names in order.
func (p *Problem) OrderedSubtaskNames() []string {
	names := make([]string, 0, len(p.Subtasks))
	for name := range p.Subtasks {
		if name != "samples" {
			names = append(names, name)
		}
	}
	if p.hasOrdinals() {
		names = slices.Collect(maps.Keys(p.Subtasks))
		slices.SortFunc(names, func(a, b string) int {
			return cmp.Or(cmp.Compare(p.Subtasks[a].Ordinal, p.Subtasks[b].Ordinal), strings.Compare(a, b))
		})
		return names
	}
	sort.Strings(names)
	if _, ok := p.Subtasks["samples"]; ok {
		names = append([]string{"samples"}, names...)
	}
	return names
}

// An absent language uses the file extension to suggest a configuration.
type Source struct {
	Language      string            `yaml:"language,omitempty" json:"language,omitempty"`
	DependencyMap map[string]string `yaml:"dependency_map,omitempty" json:"dependency_map,omitempty"`
	// Standard names a system checker that replaces this checker file on import.
	Standard string `yaml:"standard,omitempty" json:"standard,omitempty"`
	// AutoUpdate on a system library file: true replaces its text with the current system file.
	AutoUpdate *bool `yaml:"auto_update,omitempty" json:"auto_update,omitempty"`
	// Groups names the subtasks a validator checks; empty means every test.
	Groups []string      `yaml:"groups,omitempty" json:"groups,omitempty"`
	Tests  []ProgramTest `yaml:"tests,omitempty" json:"tests,omitempty"`
}

// ProgramTest of a checker has an output and an answer; a validator test's verdict is valid or invalid. Files are under tests/.
type ProgramTest struct {
	Input   string        `yaml:"input" json:"input"`
	Output  string        `yaml:"output,omitempty" json:"output,omitempty"`
	Answer  string        `yaml:"answer,omitempty" json:"answer,omitempty"`
	Verdict string        `yaml:"verdict" json:"verdict"`
	Points  *points.Value `yaml:"points,omitempty" json:"points,omitempty"`
	Group   string        `yaml:"group,omitempty" json:"group,omitempty"`
}
