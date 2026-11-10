package gfmt

import (
	"github.com/brawler2011/gate/backend/pkg/points"
	"sort"
	"time"
)

// Problem represents the GFMT problem specification.
type Problem struct {
	ToolchainDigest string                       `yaml:"toolchain_digest,omitempty" json:"toolchain_digest,omitempty"`
	Architecture    string                       `yaml:"architecture,omitempty" json:"architecture,omitempty"`
	ABI             string                       `yaml:"abi,omitempty" json:"abi,omitempty"`
	Languages       map[string]LanguageConfig    `yaml:"languages,omitempty" json:"languages,omitempty"`
	Compiled        map[string]CompiledComponent `yaml:"compiled,omitempty" json:"compiled,omitempty"`
	UpdatedAt       time.Time                    `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	MaxScore        *points.Value                `yaml:"max_score,omitempty" json:"max_score,omitempty"`
	Sources         map[string]Source            `yaml:"sources,omitempty" json:"sources,omitempty"`
	FormatVersion   string                       `yaml:"format_version" json:"format_version"`
	Title           string                       `yaml:"title" json:"title"`
	Type            string                       `yaml:"type" json:"type"` // "pass-fail", "interactive", "scoring"
	Limits          Limits                       `yaml:"limits" json:"limits"`
	Subtasks        map[string]Subtask           `yaml:"subtasks" json:"subtasks"`
	Solutions       map[string]string            `yaml:"solutions" json:"solutions"`
	Checker         string                       `yaml:"checker,omitempty" json:"checker,omitempty"`
	Interactor      string                       `yaml:"interactor,omitempty" json:"interactor,omitempty"`
	Validator       string                       `yaml:"validator,omitempty" json:"validator,omitempty"`
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

type LanguageConfig struct {
	Type        string `yaml:"type" json:"type"`
	Compile     string `yaml:"compile" json:"compile"`
	CodeFile    string `yaml:"code_file" json:"code_file"`
	CompileFile string `yaml:"compile_file" json:"compile_file"`
	Execute     string `yaml:"execute" json:"execute"`
}

type CompiledComponent struct {
	Role         string            `yaml:"role" json:"role"`
	Language     string            `yaml:"language" json:"language"`
	Entry        string            `yaml:"entry" json:"entry"`
	Dependencies map[string]string `yaml:"dependencies" json:"dependencies"`
}

// FileMapping maps a source file in the unpacked package directory to its target path in the gfmt layout.
type FileMapping struct {
	SourcePath string
	TargetPath string
}

// ImportPlan represents the transformed problem manifest and the list of files to copy.
type ImportPlan struct {
	Problem  *Problem
	Mappings []FileMapping
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

// Source holds explicit build settings; files and component kinds come from
// the package layout, and component selections remain in checker/validator/interactor.
type Source struct {
	DependencyMap map[string]string `yaml:"dependency_map,omitempty" json:"dependency_map,omitempty"`
	Compiler      string            `yaml:"compiler,omitempty" json:"compiler,omitempty"`
	Dependencies  []string          `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
}
