package gfmt

import (
	"github.com/brawler2011/gate/backend/pkg/points"
	"sort"
	"time"
)

// Problem represents the GFMT problem specification.
type Problem struct {
	// Programs are the sources the judge compiles, by role (checker,
	// interactor); a built package names each role at most once.
	Programs      map[string]Program `yaml:"programs,omitempty" json:"programs,omitempty"`
	UpdatedAt     time.Time          `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	MaxScore      *points.Value      `yaml:"max_score,omitempty" json:"max_score,omitempty"`
	Sources       map[string]Source  `yaml:"sources,omitempty" json:"sources,omitempty"`
	FormatVersion string             `yaml:"format_version" json:"format_version"`
	Title         string             `yaml:"title" json:"title"`
	Type          string             `yaml:"type" json:"type"` // "pass-fail", "interactive", "scoring"
	Limits        Limits             `yaml:"limits" json:"limits"`
	Subtasks      map[string]Subtask `yaml:"subtasks" json:"subtasks"`
	Solutions     map[string]string  `yaml:"solutions" json:"solutions"`
}

// Program is a source file of a built package that the judge compiles with its
// own image. Dependencies map a file name in the compile directory to a package path.
type Program struct {
	Source       string            `yaml:"source" json:"source"`
	Language     string            `yaml:"language" json:"language"`
	Dependencies map[string]string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
}

// Limits defines the resource limits for a problem.
type Limits struct {
	TimeMs   int `yaml:"time_ms" json:"time_ms"`
	MemoryMb int `yaml:"memory_mb" json:"memory_mb"`
}

// Subtask represents a group of tests with point allocations and scoring policy.
type Subtask struct {
	Ordinal      int          `yaml:"ordinal,omitempty" json:"ordinal,omitempty"`
	Points       points.Value `yaml:"points" json:"points"`
	Policy       string       `yaml:"policy" json:"policy"` // "complete", "each"
	Dependencies []string     `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Tests        []Test       `yaml:"tests" json:"tests"`
}

// Test represents an individual test case, either manual or generated.
type Test struct {
	Points     points.Value `yaml:"points" json:"points"`
	Ordinal    int          `yaml:"ordinal,omitempty" json:"ordinal,omitempty"`
	Answer     string       `yaml:"answer,omitempty" json:"answer,omitempty"`
	AnswerMode string       `yaml:"answer_mode,omitempty" json:"answer_mode,omitempty"`
	Manual     string       `yaml:"manual,omitempty" json:"manual,omitempty"`
	Generate   string       `yaml:"generate,omitempty" json:"generate,omitempty"`
	Sample     bool         `yaml:"sample,omitempty" json:"sample,omitempty"`
}

// OrderedSubtaskNames defines test numbering: samples first, then group names;
// tests retain their manifest list order. Files do not determine membership.
func (p *Problem) OrderedSubtaskNames() []string {
	names := make([]string, 0, len(p.Subtasks))
	for name := range p.Subtasks {
		if name != "samples" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if _, ok := p.Subtasks["samples"]; ok {
		names = append([]string{"samples"}, names...)
	}
	return names
}

// Source holds explicit build settings of a program; files and component
// kinds come from the package layout. Language is the Gate language
// configuration; when absent, the file extension suggests one.
type Source struct {
	Language      string            `yaml:"language,omitempty" json:"language,omitempty"`
	DependencyMap map[string]string `yaml:"dependency_map,omitempty" json:"dependency_map,omitempty"`
}
