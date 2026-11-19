package gfmt

import (
	"cmp"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/brawler2011/gate/backend/pkg/points"
)

type Problem struct {
	// A built package names each judge-compiled role at most once.
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
	// System library files have no package copy; Gate provides them under lib/ names.
	SystemLibrary []string `yaml:"system_library,omitempty" json:"system_library,omitempty"`
}

// The judge compiles this source with every lib/ file using its own image.
type Program struct {
	Source   string `yaml:"source" json:"source"`
	Language string `yaml:"language" json:"language"`
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

type Test struct {
	Points     points.Value `yaml:"points" json:"points"`
	Ordinal    int          `yaml:"ordinal,omitempty" json:"ordinal,omitempty"`
	Answer     string       `yaml:"answer,omitempty" json:"answer,omitempty"`
	AnswerMode string       `yaml:"answer_mode,omitempty" json:"answer_mode,omitempty"`
	Manual     string       `yaml:"manual,omitempty" json:"manual,omitempty"`
	Generate   string       `yaml:"generate,omitempty" json:"generate,omitempty"`
	// Solution names the solutions/ file whose output is the answer.
	Solution string `yaml:"solution,omitempty" json:"solution,omitempty"`
	Sample   bool   `yaml:"sample,omitempty" json:"sample,omitempty"`
}

// Order by ordinals when every subtask has one; otherwise samples come first, then names in order.
func (p *Problem) OrderedSubtaskNames() []string {
	names := make([]string, 0, len(p.Subtasks))
	ordinals := true
	for name, s := range p.Subtasks {
		ordinals = ordinals && s.Ordinal > 0
		if name != "samples" {
			names = append(names, name)
		}
	}
	if ordinals {
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
}
