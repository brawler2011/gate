package icpc

// Problem represents the ICPC problem.yaml specification.
type Problem struct {
	Name       string      `yaml:"name"`
	Author     string      `yaml:"author"`
	Source     string      `yaml:"source"`
	Rights     string      `yaml:"rights"`
	Limits     *Limits     `yaml:"limits"`
	Validation *Validation `yaml:"validation"`
}

// Limits defines the resource limits for a problem.
type Limits struct {
	TimeMultiplier   float64 `yaml:"time_multiplier"`
	Time             float64 `yaml:"time"` // in seconds
	Memory           int     `yaml:"memory"` // in MB
	Output           int     `yaml:"output"` // in MB
	Code             int     `yaml:"code"` // in KB
	CompilationTime  float64 `yaml:"compilation_time"`
	ValidationTime   float64 `yaml:"validation_time"`
	ValidationOutput int     `yaml:"validation_output"`
}

// Validation represents problem validation configuration.
type Validation struct {
	Type string `yaml:"type"` // "default", "custom", "interactive"
}

// Testdata represents the ICPC testdata.yaml specification.
type Testdata struct {
	OnReject    string  `yaml:"on_reject"` // "break", "continue"
	AcceptScore float64 `yaml:"accept_score"`
	ScoreType   string  `yaml:"score_type"` // "sum", "average", "pass-fail"
	Grading     string  `yaml:"grading"` // "default", "custom"
	Range       string  `yaml:"range"`
}
