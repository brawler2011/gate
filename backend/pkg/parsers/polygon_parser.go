package parsers

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gate149/gate/backend/pkg/problemformat"
)

// PolygonProblem represents the root structure of Polygon's problem.xml
type PolygonProblem struct {
	XMLName    xml.Name           `xml:"problem"`
	Revision   int                `xml:"revision,attr"`
	ShortName  string             `xml:"short-name,attr"`
	Judging    PolygonJudging     `xml:"judging"`
	Files      PolygonFiles       `xml:"files"`
	Statements []PolygonStatement `xml:"statements>statement"`
}

type PolygonJudging struct {
	Testset PolygonTestset `xml:"testset"`
}

type PolygonTestset struct {
	Name              string         `xml:"name,attr"`
	TimeLimit         int            `xml:"time-limit"`   // milliseconds
	MemoryLimit       int64          `xml:"memory-limit"` // bytes
	TestCount         int            `xml:"test-count"`
	InputPathPattern  string         `xml:"input-path-pattern"`
	AnswerPathPattern string         `xml:"answer-path-pattern"`
	Tests             []PolygonTest  `xml:"tests>test"`
	Groups            []PolygonGroup `xml:"groups>group"`
}

type PolygonTest struct {
	Method      string `xml:"method,attr"`
	Cmd         string `xml:"cmd,attr"`
	Sample      bool   `xml:"sample,attr"`
	Description string `xml:"description,attr"`
}

type PolygonGroup struct {
	Name           string              `xml:"name,attr"`
	PointsPolicy   string              `xml:"points-policy,attr"`
	Points         int                 `xml:"points,attr"`
	FeedbackPolicy string              `xml:"feedback-policy,attr"`
	Dependencies   []PolygonDependency `xml:"dependencies>dependency"`
}

type PolygonDependency struct {
	Group string `xml:"group,attr"`
}

type PolygonFiles struct {
	Resources   []PolygonResource   `xml:"resources>file"`
	Executables []PolygonExecutable `xml:"executables>executable"`
}

type PolygonResource struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

type PolygonExecutable struct {
	Source PolygonSource `xml:"source"`
	Binary PolygonBinary `xml:"binary"`
}

type PolygonSource struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"` // checker, validator, generator, interactor
}

type PolygonBinary struct {
	Path string `xml:"path,attr"`
	Type string `xml:"type,attr"`
}

type PolygonStatement struct {
	Language string `xml:"language,attr"`
	Charset  string `xml:"charset,attr"`
	Type     string `xml:"type,attr"`
	Path     string `xml:"path,attr"`
	Mathjax  bool   `xml:"mathjax,attr"`
}

// PolygonParser implements the ProblemParser interface for Polygon format
type PolygonParser struct{}

func NewPolygonParser() *PolygonParser {
	return &PolygonParser{}
}

func (p *PolygonParser) Parse(packageDir string) (*problemformat.ProblemManifest, *problemformat.TestsMetadata, error) {
	// Parse problem.xml
	xmlPath := filepath.Join(packageDir, "problem.xml")
	polygonProb, err := parsePolygonXML(xmlPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse problem.xml: %w", err)
	}

	// Convert to unified format
	manifest := convertPolygonToManifest(polygonProb, packageDir)
	testsMetadata := convertPolygonTests(polygonProb.Judging.Testset)

	return manifest, testsMetadata, nil
}

func (p *PolygonParser) GetFormat() string {
	return "polygon"
}

func parsePolygonXML(xmlPath string) (*PolygonProblem, error) {
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read XML file: %w", err)
	}

	var prob PolygonProblem
	if err := xml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	return &prob, nil
}

func convertPolygonToManifest(prob *PolygonProblem, packageDir string) *problemformat.ProblemManifest {
	testset := prob.Judging.Testset

	// Convert time limit from ms to ms (already in ms)
	timeLimitMs := testset.TimeLimit

	// Convert memory limit from bytes to MB
	memoryLimitMb := int(testset.MemoryLimit / (1024 * 1024))

	// Determine problem type
	problemType := "pass-fail"
	var maxScore *int
	if len(testset.Groups) > 0 {
		// Check if any group has points
		totalPoints := 0
		for _, group := range testset.Groups {
			totalPoints += group.Points
		}
		if totalPoints > 0 {
			problemType = "scoring"
			maxScore = &totalPoints
		}
	}

	// Convert statement
	statement := problemformat.Statement{}
	for _, stmt := range prob.Statements {
		if statement.Legend != "" {
			break
		}

		// Read statement content from file
		stmtPath := filepath.Join(packageDir, stmt.Path)
		content, err := os.ReadFile(stmtPath)
		if err != nil {
			continue
		}

		// Parse HTML/LaTeX statement (simplified - in production you'd parse properly)
		statement = problemformat.Statement{
			Title:        prob.ShortName,
			Legend:       string(content), // Simplified
			InputFormat:  "",
			OutputFormat: "",
			Notes:        "",
			Interaction:  "",
			Scoring:      "",
		}
	}

	// Convert file metadata
	filesMetadata := make([]problemformat.FileMetadata, 0)
	for _, exec := range prob.Files.Executables {
		fileType := exec.Source.Type
		if fileType == "" {
			// Try to infer from path
			if strings.Contains(exec.Source.Path, "checker") {
				fileType = "checker"
			} else if strings.Contains(exec.Source.Path, "validator") {
				fileType = "validator"
			} else if strings.Contains(exec.Source.Path, "generator") {
				fileType = "generator"
			} else if strings.Contains(exec.Source.Path, "interactor") {
				fileType = "interactor"
			}
		}

		if fileType != "" {
			compiler := detectCompiler(exec.Source.Path)
			filesMetadata = append(filesMetadata, problemformat.FileMetadata{
				Type:         fileType,
				Filename:     exec.Source.Path,
				Compiler:     compiler,
				BinarySha256: nil,
				Dependencies: []problemformat.Dependency{},
			})
		}
	}

	return &problemformat.ProblemManifest{
		LastUpdated:     time.Now(),
		ProblemType:     problemType,
		MaxScore:        maxScore,
		FilesMetadata:   filesMetadata,
		TimeLimitMs:     timeLimitMs,
		MemoryLimitMb:   memoryLimitMb,
		StdoutLimitMb:   64,  // Default
		CodeSizeLimitKb: 256, // Default
		Statement:       statement,
	}
}

func convertPolygonTests(testset PolygonTestset) *problemformat.TestsMetadata {
	// Convert tests
	tests := make([]problemformat.TestCase, 0, len(testset.Tests))
	for i, test := range testset.Tests {
		ordinal := i + 1
		method := "manual"
		var generator *string

		if test.Method == "generated" {
			method = "generated"
			if test.Cmd != "" {
				generator = &test.Cmd
			}
		}

		tests = append(tests, problemformat.TestCase{
			Ordinal:   ordinal,
			Method:    method,
			Generator: generator,
			IsSample:  test.Sample,
		})
	}

	// Convert groups
	groups := make([]problemformat.TestGroup, 0, len(testset.Groups))
	groupNameToOrdinal := make(map[string]int)

	for i, group := range testset.Groups {
		ordinal := i
		groupNameToOrdinal[group.Name] = ordinal

		pointsPolicy := "complete-group"
		if group.PointsPolicy == "each-test" {
			pointsPolicy = "each-test"
		}

		// Parse test range from group name or use defaults
		// In Polygon, groups typically reference tests by index
		// For simplicity, we'll distribute tests evenly
		testsPerGroup := len(tests) / len(testset.Groups)
		if testsPerGroup == 0 {
			testsPerGroup = 1
		}
		startTest := i*testsPerGroup + 1
		endTest := startTest + testsPerGroup - 1
		if i == len(testset.Groups)-1 {
			endTest = len(tests)
		}

		groups = append(groups, problemformat.TestGroup{
			Ordinal:      ordinal,
			Name:         group.Name,
			Points:       group.Points,
			PointsPolicy: pointsPolicy,
			DependsOn:    []int{}, // Will be filled below
			Tests:        [2]int{startTest, endTest},
		})
	}

	// Convert dependencies
	for i, group := range testset.Groups {
		deps := make([]int, 0)
		for _, dep := range group.Dependencies {
			if depOrdinal, ok := groupNameToOrdinal[dep.Group]; ok {
				deps = append(deps, depOrdinal)
			}
		}
		groups[i].DependsOn = deps
	}

	return &problemformat.TestsMetadata{
		Groups: groups,
		Tests:  tests,
	}
}

func detectCompiler(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".cpp", ".cc", ".cxx":
		return "cpp17"
	case ".c":
		return "c11"
	case ".py":
		return "python3"
	case ".java":
		return "java11"
	case ".pas":
		return "fpc"
	default:
		return "cpp17" // Default
	}
}
