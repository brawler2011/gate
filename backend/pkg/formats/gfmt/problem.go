package gfmt

import (
	"iter"
	"slices"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
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
	Type     string             `yaml:"type" json:"type"` // one of spec.ProblemTypes
	Limits   Limits             `yaml:"limits" json:"limits"`
	// Testset is the root group; only it is unnamed.
	Testset Group `yaml:"testset" json:"testset"`
	// Empty names keep standard input and output.
	InputFile  string `yaml:"input_file,omitempty" json:"input_file,omitempty"`
	OutputFile string `yaml:"output_file,omitempty" json:"output_file,omitempty"`
	// Processes is the number of solution copies a communication problem runs at once.
	Processes int `yaml:"processes,omitempty" json:"processes,omitempty"`
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

// Group scores its tests and subgroups, taken in the order of their first tests, by an spec.Aggregations entry; empty means sum.
type Group struct {
	Name      string `yaml:"name,omitempty" json:"name,omitempty"`
	Aggregate string `yaml:"aggregate,omitempty" json:"aggregate,omitempty"`
	// StopOnFailure leaves the results after the first failed test or subgroup out of the score, as ICPC on_reject: break.
	StopOnFailure bool `yaml:"stop_on_failure,omitempty" json:"stop_on_failure,omitempty"`
	// Requires names groups that must pass completely, their own requirements included, for this group to score.
	Requires []string `yaml:"requires,omitempty" json:"requires,omitempty"`
	// Feedback is a Polygon feedback policy for the group's own tests during a contest; empty means complete.
	Feedback string `yaml:"feedback,omitempty" json:"feedback,omitempty"`
	// CheckerArgs follow those of the checker or interactor on the group's own tests; ValidatorArgs those of ICPC validators.
	CheckerArgs   []string `yaml:"checker_args,omitempty" json:"checker_args,omitempty"`
	ValidatorArgs []string `yaml:"validator_args,omitempty" json:"validator_args,omitempty"`
	Tests         []Test   `yaml:"tests,omitempty" json:"tests,omitempty"`
	Groups        []Group  `yaml:"groups,omitempty" json:"groups,omitempty"`
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
	// Pretest tests judge contest submissions until the contest ends.
	Pretest bool `yaml:"pretest,omitempty" json:"pretest,omitempty"`
	// StatementInput and StatementOutput name files under tests/ that the statement shows instead of the sample's own.
	StatementInput  string `yaml:"statement_input,omitempty" json:"statement_input,omitempty"`
	StatementOutput string `yaml:"statement_output,omitempty" json:"statement_output,omitempty"`
}

// ShowsInput reports whether the statement shows the test's own input; an interactive one is the interactor's.
func (p *Problem) ShowsInput(t Test) bool {
	return t.Sample && t.StatementInput == "" && !spec.Interactive(p.Type)
}

// Pretests keeps only pretests and drops groups left without tests; ok is false unless that leaves a proper subset.
func (p *Problem) Pretests() (restricted *Problem, ok bool) {
	kept, total := 0, 0
	var prune func(Group) (Group, bool)
	prune = func(g Group) (Group, bool) {
		total += len(g.Tests)
		g.Tests = slices.DeleteFunc(slices.Clone(g.Tests), func(t Test) bool { return !t.Pretest })
		kept += len(g.Tests)
		groups := []Group{}
		for _, child := range g.Groups {
			if child, ok := prune(child); ok {
				groups = append(groups, child)
			}
		}
		g.Groups = groups
		return g, len(g.Tests) > 0 || len(g.Groups) > 0
	}
	root, _ := prune(p.Testset)
	if kept == 0 || kept == total {
		return p, false
	}
	names := map[string]bool{}
	for g := range root.All() {
		names[g.Name] = true
	}
	for g := range root.All() {
		g.Requires = slices.DeleteFunc(slices.Clone(g.Requires), func(name string) bool { return !names[name] })
	}
	copy := *p
	copy.Testset = root
	return &copy, true
}

// All yields the group and its subgroups, parents first.
func (g *Group) All() iter.Seq[*Group] {
	return func(yield func(*Group) bool) {
		g.walk(yield)
	}
}

func (g *Group) walk(yield func(*Group) bool) bool {
	if !yield(g) {
		return false
	}
	for i := range g.Groups {
		if !g.Groups[i].walk(yield) {
			return false
		}
	}
	return true
}

// An absent language uses the file extension to suggest a configuration.
type Source struct {
	Language      string            `yaml:"language,omitempty" json:"language,omitempty"`
	DependencyMap map[string]string `yaml:"dependency_map,omitempty" json:"dependency_map,omitempty"`
	// Standard names a system checker that replaces this checker file on import.
	Standard string `yaml:"standard,omitempty" json:"standard,omitempty"`
	// AutoUpdate on a system library file: true replaces its text with the current system file.
	AutoUpdate *bool `yaml:"auto_update,omitempty" json:"auto_update,omitempty"`
	// Protocol of a checker, interactor or validator; empty means testlib.
	Protocol string   `yaml:"protocol,omitempty" json:"protocol,omitempty"`
	Args     []string `yaml:"args,omitempty" json:"args,omitempty"`
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
