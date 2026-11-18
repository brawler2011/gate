package pcms2

import "encoding/xml"

type Problem struct {
	XMLName    xml.Name    `xml:"problem"`
	ID         string      `xml:"id,attr"`
	Version    string      `xml:"version,attr"`
	Names      []Name      `xml:"names>name"`
	Statements []Statement `xml:"statements>statement"`
	Judging    Judging     `xml:"judging"`
}

type Name struct {
	Language string `xml:"language,attr"`
	Value    string `xml:"value,attr"`
}

type Statement struct {
	Language string `xml:"language,attr"`
	Type     string `xml:"type,attr"`
	File     string `xml:"file,attr"`
}

type Judging struct {
	InputFile  string    `xml:"input-file"`
	OutputFile string    `xml:"output-file"`
	Run        Run       `xml:"run"`
	Testsets   []Testset `xml:"testset"`
}

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

type Verifier struct {
	Type   string         `xml:"type,attr"`
	Binary VerifierBinary `xml:"binary"`
}

type VerifierBinary struct {
	Executable string `xml:"executable,attr"`
}

type Testset struct {
	Name              string `xml:"name,attr"`
	TimeLimit         int    `xml:"time-limit"`
	MemoryLimit       int64  `xml:"memory-limit"`
	TestCount         int    `xml:"test-count"`
	InputPathPattern  string `xml:"input-path-pattern"`
	AnswerPathPattern string `xml:"answer-path-pattern"`
	Tests             []Test `xml:"tests>test"`
}

type Test struct {
	Index   int     `xml:"index,attr"`
	Input   string  `xml:"input,attr"`
	Outcome string  `xml:"outcome,attr"`
	Group   string  `xml:"group,attr"`
	Points  float64 `xml:"points,attr"`
}
