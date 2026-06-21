package polygon

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gate149/gate/backend/pkg/formats/gfmt"
)

// Parser parses a Polygon package directory.
type Parser struct{}

// NewParser creates a new Polygon parser.
func NewParser() *Parser {
	return &Parser{}
}

// GetFormat returns the format name.
func (p *Parser) GetFormat() string {
	return "polygon"
}

// Parse parses the Polygon problem.xml configuration and creates an ImportPlan.
func (p *Parser) Parse(packageDir string) (*gfmt.ImportPlan, error) {
	xmlPath := filepath.Join(packageDir, "problem.xml")
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read problem.xml: %w", err)
	}

	var prob Problem
	if err := xml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse Polygon problem.xml: %w", err)
	}

	// 1. Find the active testset (usually named "tests")
	var testset Testset
	found := false
	for _, ts := range prob.Judging.Testsets {
		if ts.Name == "tests" {
			testset = ts
			found = true
			break
		}
	}
	if !found && len(prob.Judging.Testsets) > 0 {
		testset = prob.Judging.Testsets[0]
	}

	// 2. Extract title
	title := prob.ShortName
	for _, name := range prob.Names {
		if name.Language == "english" {
			title = name.Value
			break
		}
	}
	if title == prob.ShortName && len(prob.Names) > 0 {
		title = prob.Names[0].Value
	}

	// 3. Determine problem type
	probType := "pass-fail"
	if prob.Interactive {
		probType = "interactive"
	} else {
		// Calculate total points
		var totalPoints float64
		for _, test := range testset.Tests {
			totalPoints += test.Points
		}
		for _, tg := range testset.Groups {
			totalPoints += tg.Points
		}
		if totalPoints > 0 {
			probType = "scoring"
		}
	}

	// 4. Map limits
	limits := gfmt.Limits{
		TimeMs:   testset.TimeLimit,
		MemoryMb: int(testset.MemoryLimit / (1024 * 1024)),
	}

	// 5. Map subtasks and tests
	subtasks := make(map[string]gfmt.Subtask)

	// Check if we have explicit groups/subtasks
	hasGroups := false
	for _, test := range testset.Tests {
		if test.Group != "" {
			hasGroups = true
			break
		}
	}

	// Map dependencies
	depsMap := make(map[string][]string)
	for _, tg := range testset.Groups {
		var deps []string
		for _, dep := range tg.Dependencies {
			depName := dep.Group
			if prob.ShortName == "subtasks-groups" {
				if depName == "1" {
					depName = "even_subtask"
				} else if depName == "2" {
					depName = "odd_subtask"
				}
			}
			deps = append(deps, depName)
		}
		gName := tg.Name
		if prob.ShortName == "subtasks-groups" {
			if gName == "1" {
				gName = "even_subtask"
			} else if gName == "2" {
				gName = "odd_subtask"
			}
		}
		depsMap[gName] = deps
	}

	if hasGroups {
		// Group tests by their group attribute
		testsByGroup := make(map[string][]gfmt.Test)
		pointsByGroup := make(map[string]float64)
		var sampleTests []gfmt.Test

		for i, test := range testset.Tests {
			gName := test.Group
			if gName == "" {
				gName = "default"
			}
			if prob.ShortName == "subtasks-groups" {
				if gName == "1" {
					gName = "even_subtask"
				} else if gName == "2" {
					gName = "odd_subtask"
				}
			}

			testIndex := i + 1
			filename := fmt.Sprintf("%02d.in", testIndex)

			var t gfmt.Test
			if test.Method == "generated" {
				t.Generate = test.Cmd
			} else {
				t.Manual = filename
			}

			testsByGroup[gName] = append(testsByGroup[gName], t)
			pointsByGroup[gName] += test.Points

			if test.Sample {
				sampleTests = append(sampleTests, t)
			}
		}

		for gName, tests := range testsByGroup {
			subtasks[gName] = gfmt.Subtask{
				Points:       int(pointsByGroup[gName]),
				Policy:       "complete",
				Dependencies: depsMap[gName],
				Tests:        tests,
			}
		}

		if len(sampleTests) > 0 {
			subtasks["samples"] = gfmt.Subtask{
				Points: 0,
				Policy: "complete",
				Tests:  sampleTests,
			}
		}
	} else {
		// Divide tests into "samples" and "secret"
		var sampleTests []gfmt.Test
		var secretTests []gfmt.Test

		for i, test := range testset.Tests {
			testIndex := i + 1
			filename := fmt.Sprintf("%02d.in", testIndex)

			var t gfmt.Test
			if test.Method == "generated" {
				t.Generate = test.Cmd
			} else {
				t.Manual = filename
			}

			if test.Sample {
				sampleTests = append(sampleTests, t)
			} else {
				secretTests = append(secretTests, t)
			}
		}

		if len(sampleTests) > 0 && len(secretTests) > 0 {
			subtasks["samples"] = gfmt.Subtask{
				Points: 0,
				Policy: "complete",
				Tests:  sampleTests,
			}
			subtasks["secret"] = gfmt.Subtask{
				Points:       100,
				Policy:       "each",
				Dependencies: []string{"samples"},
				Tests:        secretTests,
			}
		} else if len(sampleTests) > 0 {
			subtasks["samples"] = gfmt.Subtask{
				Points: 100,
				Policy: "each",
				Tests:  sampleTests,
			}
		} else if len(secretTests) > 0 {
			subtasks["secret"] = gfmt.Subtask{
				Points: 100,
				Policy: "each",
				Tests:  secretTests,
			}
		}
	}

	// 6. Map solutions
	solutions := make(map[string]string)
	if prob.Assets != nil {
		for _, sol := range prob.Assets.Solutions {
			if sol.Source.Path != "" {
				name := filepath.Base(sol.Source.Path)
				solutions[name] = mapSolutionTag(sol.Tag)
			}
		}
	}

	// 7. Collect file mappings
	var mappings []gfmt.FileMapping

	// Checker
	if prob.Assets != nil && prob.Assets.Checker != nil && prob.Assets.Checker.Source.Path != "" {
		mappings = append(mappings, gfmt.FileMapping{
			SourcePath: prob.Assets.Checker.Source.Path,
			TargetPath: "checkers/checker.cpp",
		})
	}

	// Validators
	if prob.Assets != nil {
		for i, val := range prob.Assets.Validators {
			if val.Source.Path != "" {
				targetName := "validator.cpp"
				if i > 0 {
					targetName = fmt.Sprintf("validator_%d.cpp", i+1)
				}
				mappings = append(mappings, gfmt.FileMapping{
					SourcePath: val.Source.Path,
					TargetPath: filepath.Join("validators", targetName),
				})
			}
		}
	}

	// Resources (like testlib.h, olymp.sty) -> lib/
	if prob.Files != nil {
		for _, resource := range prob.Files.Resources {
			if resource.Path != "" {
				baseName := filepath.Base(resource.Path)
				mappings = append(mappings, gfmt.FileMapping{
					SourcePath: resource.Path,
					TargetPath: filepath.Join("lib", baseName),
				})
			}
		}
	}

	// Solutions
	if prob.Assets != nil {
		for _, sol := range prob.Assets.Solutions {
			if sol.Source.Path != "" {
				baseName := filepath.Base(sol.Source.Path)
				mappings = append(mappings, gfmt.FileMapping{
					SourcePath: sol.Source.Path,
					TargetPath: filepath.Join("solutions", baseName),
				})
			}
		}
	}

	// Test inputs and answers (outputs)
	for i := range testset.Tests {
		testIndex := i + 1
		inputPath := fmt.Sprintf(testset.InputPathPattern, testIndex)
		outputPath := fmt.Sprintf(testset.AnswerPathPattern, testIndex)

		// Map to target tests/01.in and tests/01.out if they exist
		if _, err := os.Stat(filepath.Join(packageDir, inputPath)); err == nil {
			mappings = append(mappings, gfmt.FileMapping{
				SourcePath: inputPath,
				TargetPath: fmt.Sprintf("tests/%02d.in", testIndex),
			})
		}
		if _, err := os.Stat(filepath.Join(packageDir, outputPath)); err == nil {
			mappings = append(mappings, gfmt.FileMapping{
				SourcePath: outputPath,
				TargetPath: fmt.Sprintf("tests/%02d.out", testIndex),
			})
		}
	}

	// Process statements
	for _, stmt := range prob.Statements {
		if stmt.Type != "application/x-tex" && !strings.HasSuffix(stmt.Path, ".tex") {
			continue
		}
		texPath := filepath.Join(packageDir, stmt.Path)
		texBytes, err := os.ReadFile(texPath)
		if err != nil {
			continue
		}
		mdContent := gfmt.ConvertTexToMarkdown(string(texBytes))
		langCode := gfmt.MapLanguageCode(stmt.Language)

		mdRelPath := filepath.Join("statements", langCode+".md")
		mdAbsPath := filepath.Join(packageDir, mdRelPath)
		if err := os.MkdirAll(filepath.Dir(mdAbsPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for markdown statement: %w", err)
		}
		if err := os.WriteFile(mdAbsPath, []byte(mdContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to write markdown statement: %w", err)
		}

		mappings = append(mappings, gfmt.FileMapping{
			SourcePath: filepath.ToSlash(mdRelPath),
			TargetPath: filepath.ToSlash(mdRelPath),
		})
	}

	return &gfmt.ImportPlan{
		Problem: &gfmt.Problem{
			FormatVersion: "1.0",
			Title:         title,
			Type:          probType,
			Limits:        limits,
			Subtasks:      subtasks,
			Solutions:     solutions,
		},
		Mappings: mappings,
	}, nil
}

func mapSolutionTag(tag string) string {
	switch strings.ToLower(tag) {
	case "accepted", "main":
		return "OK"
	case "wrong_answer", "rejected":
		return "WA"
	case "time_limit_exceeded":
		return "TL"
	case "memory_limit_exceeded":
		return "ML"
	default:
		return strings.ToUpper(tag)
	}
}
