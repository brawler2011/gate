package pcms2

import "encoding/xml"

// Problem represents the XML problem representation in PCMS2 format.
type Problem struct {
	XMLName    xml.Name    `xml:"problem"`
	ID         string      `xml:"id,attr"`
	Version    string      `xml:"version,attr"`
	Names      []Name      `xml:"names>name"`
	Statements []Statement `xml:"statements>statement"`
	Judging    Judging     `xml:"judging"`
}

// Name represents a problem name in a specific language.
type Name struct {
	Language string `xml:"language,attr"`
	Value    string `xml:"value,attr"`
}

// Statement represents a statement document reference.
type Statement struct {
	Language string `xml:"language,attr"`
	Type     string `xml:"type,attr"`
	File     string `xml:"file,attr"`
}

// Judging holds limits, run verifier, and testsets.
type Judging struct {
	InputFile  string    `xml:"input-file"`
	OutputFile string    `xml:"output-file"`
	Run        Run       `xml:"run"`
	Testsets   []Testset `xml:"testset"`
}

// Run describes the execution verifier.
type Run struct {
	Verifier   Verifier    `xml:"verifier"`
	Interactor *Interactor `xml:"interactor"`
}

type Interactor struct {
	Type   string           `xml:"type,attr"`
	Binary InteractorBinary `xml:"binary"`
}

type InteractorBinary struct {
	Executable string `xml:"executable,attr"`
}

// Verifier describes the problem checker configuration.
type Verifier struct {
	Type   string         `xml:"type,attr"`
	Binary VerifierBinary `xml:"binary"`
}

// VerifierBinary holds the checker executable path.
type VerifierBinary struct {
	Executable string `xml:"executable,attr"`
}

// Testset defines resource limits and test specifications.
type Testset struct {
	Name              string `xml:"name,attr"`
	TimeLimit         int    `xml:"time-limit"`
	MemoryLimit       int64  `xml:"memory-limit"`
	TestCount         int    `xml:"test-count"`
	InputPathPattern  string `xml:"input-path-pattern"`
	AnswerPathPattern string `xml:"answer-path-pattern"`
	Tests             []Test `xml:"tests>test"`
}

// Test represents a single test configuration.
type Test struct {
	Index   int     `xml:"index,attr"`
	Input   string  `xml:"input,attr"`
	Outcome string  `xml:"outcome,attr"`
	Group   string  `xml:"group,attr"`
	Points  float64 `xml:"points,attr"`
}
