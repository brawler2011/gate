package polygon

import "encoding/xml"

// Problem represents the XML problem representation in Polygon format.
type Problem struct {
	XMLName     xml.Name    `xml:"problem"`
	Revision    int         `xml:"revision,attr"`
	ShortName   string      `xml:"short-name,attr"`
	URL         string      `xml:"url,attr"`
	Interactive bool        `xml:"interactive,attr"`
	Names       []Name      `xml:"names>name"`
	Statements  []Statement `xml:"statements>statement"`
	Judging     Judging     `xml:"judging"`
	Files       *Files      `xml:"files"`
	Assets      *Assets     `xml:"assets"`
}

// Name represents a problem name in a specific language.
type Name struct {
	Language string `xml:"language,attr"`
	Value    string `xml:"value,attr"`
}

// Statement represents a statement document reference.
type Statement struct {
	Language string `xml:"language,attr"`
	Charset  string `xml:"charset,attr"`
	Type     string `xml:"type,attr"`
	Path     string `xml:"path,attr"`
	Mathjax  bool   `xml:"mathjax,attr"`
}

// Judging holds the testsets.
type Judging struct {
	Testsets []Testset `xml:"testset"`
}

// Testset defines resource limits and test/group specifications.
type Testset struct {
	Name              string  `xml:"name,attr"`
	TimeLimit         int     `xml:"time-limit"`
	MemoryLimit       int64   `xml:"memory-limit"`
	TestCount         int     `xml:"test-count"`
	InputPathPattern  string  `xml:"input-path-pattern"`
	AnswerPathPattern string  `xml:"answer-path-pattern"`
	Tests             []Test  `xml:"tests>test"`
	Groups            []Group `xml:"groups>group"`
}

// Test represents a single test case configuration.
type Test struct {
	Method      string  `xml:"method,attr"`
	Cmd         string  `xml:"cmd,attr"`
	Sample      bool    `xml:"sample,attr"`
	Description string  `xml:"description,attr"`
	Group       string  `xml:"group,attr"`
	Points      float64 `xml:"points,attr"`
}

// Group represents a test group scoring policy and dependencies.
type Group struct {
	Name           string       `xml:"name,attr"`
	PointsPolicy   string       `xml:"points-policy,attr"`
	Points         float64      `xml:"points,attr"`
	FeedbackPolicy string       `xml:"feedback-policy,attr"`
	Dependencies   []Dependency `xml:"dependencies>dependency"`
}

// Dependency specifies a dependency on another test group.
type Dependency struct {
	Group string `xml:"group,attr"`
}

// Files describes resources and build executables.
type Files struct {
	Resources   []Resource   `xml:"resources>file"`
	Executables []Executable `xml:"executables>executable"`
}

// Resource represents resource files needed for building/validating.
type Resource struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

// Executable represents helper executables like generators.
type Executable struct {
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}

// Source represents source code information.
type Source struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

// Binary represents compiled binary information.
type Binary struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

// Assets holds checkers, validators, and solution sources.
type Assets struct {
	Checker    *Checker    `xml:"checker"`
	Validators []Validator `xml:"validators>validator"`
	Solutions  []Solution  `xml:"solutions>solution"`
}

// Checker represents a problem checker executable.
type Checker struct {
	Name   string `xml:"name,attr"`
	Type   string `xml:"type,attr"`
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}

// Validator represents a problem input validator executable.
type Validator struct {
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}

// Solution represents a candidate solution with its expected outcome tag.
type Solution struct {
	Tag    string `xml:"tag,attr"`
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}
