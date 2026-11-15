package icpc

// Problem represents the ICPC problem.yaml specification.
type Problem struct {
	Type   string  `yaml:"type"` // "pass-fail" or "scoring"
	Name   string  `yaml:"name"`
	Author string  `yaml:"author"`
	Source string  `yaml:"source"`
	Rights string  `yaml:"rights"`
	Limits *Limits `yaml:"limits"`
	// Validation is "default" or "custom" followed by optional "score" and
	// "interactive" flags.
	Validation string `yaml:"validation"`
}

// Limits defines the resource limits for a problem.
type Limits struct {
	TimeMultiplier   float64 `yaml:"time_multiplier"`
	Time             float64 `yaml:"time"`   // in seconds
	Memory           int     `yaml:"memory"` // in MB
	Output           int     `yaml:"output"` // in MB
	Code             int     `yaml:"code"`   // in KB
	CompilationTime  float64 `yaml:"compilation_time"`
	ValidationTime   float64 `yaml:"validation_time"`
	ValidationOutput int     `yaml:"validation_output"`
}
