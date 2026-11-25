package icpc

type Problem struct {
	Type   string  `yaml:"type"` // "pass-fail" or "scoring", optionally with "submit-answer"
	Name   names   `yaml:"name"`
	Limits *Limits `yaml:"limits"`
	// Validation accepts default or custom followed by optional score and interactive flags.
	Validation string `yaml:"validation"`
	// ValidatorFlags are arguments of the output validator, the default one included.
	ValidatorFlags string `yaml:"validator_flags"`
	// Grading holds legacy settings that override every testdata.yaml.
	Grading Grading `yaml:"grading"`
}

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
