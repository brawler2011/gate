package polygon

import "encoding/xml"

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
	Properties  []Property  `xml:"properties>property"`
}

type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type Name struct {
	Language string `xml:"language,attr"`
	Value    string `xml:"value,attr"`
}

type Statement struct {
	Language string `xml:"language,attr"`
	Charset  string `xml:"charset,attr"`
	Type     string `xml:"type,attr"`
	Path     string `xml:"path,attr"`
	Mathjax  bool   `xml:"mathjax,attr"`
}

type Judging struct {
	Testsets []Testset `xml:"testset"`
}

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

type Test struct {
	Method      string  `xml:"method,attr"`
	Cmd         string  `xml:"cmd,attr"`
	Sample      bool    `xml:"sample,attr"`
	Description string  `xml:"description,attr"`
	Group       string  `xml:"group,attr"`
	Points      float64 `xml:"points,attr"`
}

type Group struct {
	Name           string       `xml:"name,attr"`
	PointsPolicy   string       `xml:"points-policy,attr"`
	Points         float64      `xml:"points,attr"`
	FeedbackPolicy string       `xml:"feedback-policy,attr"`
	Dependencies   []Dependency `xml:"dependencies>dependency"`
}

type Dependency struct {
	Group string `xml:"group,attr"`
}

type Files struct {
	Resources   []Resource   `xml:"resources>file"`
	Executables []Executable `xml:"executables>executable"`
}

type Resource struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

type Executable struct {
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}

type Source struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

type Binary struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

type Assets struct {
	Checker    *Checker    `xml:"checker"`
	Validators []Validator `xml:"validators>validator"`
	Interactor *Interactor `xml:"interactor"`
	Solutions  []Solution  `xml:"solutions>solution"`
}

type Interactor struct {
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}

type Checker struct {
	Name   string `xml:"name,attr"`
	Type   string `xml:"type,attr"`
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}

type Validator struct {
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}

type Solution struct {
	Tag    string `xml:"tag,attr"`
	Source Source `xml:"source"`
	Binary Binary `xml:"binary"`
}
