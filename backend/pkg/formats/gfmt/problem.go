package gfmt

// Problem represents the GFMT problem specification.
type Problem struct {
	FormatVersion string             `yaml:"format_version" json:"format_version"`
	Title         string             `yaml:"title" json:"title"`
	Type          string             `yaml:"type" json:"type"` // "pass-fail", "interactive", "scoring"
	Limits        Limits             `yaml:"limits" json:"limits"`
	Subtasks      map[string]Subtask `yaml:"subtasks" json:"subtasks"`
	Solutions     map[string]string  `yaml:"solutions" json:"solutions"`
}

// Limits defines the resource limits for a problem.
type Limits struct {
	TimeMs   int `yaml:"time_ms" json:"time_ms"`
	MemoryMb int `yaml:"memory_mb" json:"memory_mb"`
}

// Subtask represents a group of tests with point allocations and scoring policy.
type Subtask struct {
	Points       int      `yaml:"points" json:"points"`
	Policy       string   `yaml:"policy" json:"policy"` // "complete", "each"
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Tests        []Test   `yaml:"tests" json:"tests"`
}

// Test represents an individual test case, either manual or generated.
type Test struct {
	Manual   string `yaml:"manual,omitempty" json:"manual,omitempty"`
	Generate string `yaml:"generate,omitempty" json:"generate,omitempty"`
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
