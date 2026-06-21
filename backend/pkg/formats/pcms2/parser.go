package pcms2

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gate149/gate/backend/pkg/formats/gfmt"
)

// Parser parses a PCMS2 package directory.
type Parser struct{}

// NewParser creates a new PCMS2 parser.
func NewParser() *Parser {
	return &Parser{}
}

// GetFormat returns the format name.
func (p *Parser) GetFormat() string {
	return "pcms2"
}

// Parse parses the PCMS2 problem.xml configuration and creates an ImportPlan.
func (p *Parser) Parse(packageDir string) (*gfmt.ImportPlan, error) {
	xmlPath := filepath.Join(packageDir, "problem.xml")
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read problem.xml: %w", err)
	}

	var prob Problem
	if err := xml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse PCMS2 problem.xml: %w", err)
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
	title := prob.ID
	for _, name := range prob.Names {
		if name.Language == "english" {
			title = name.Value
			break
		}
	}
	if title == prob.ID && len(prob.Names) > 0 {
		title = prob.Names[0].Value
	}

	// 3. Determine problem type
	probType := "pass-fail"
	if prob.Judging.Run.Interactor != nil {
		probType = "interactive"
	} else {
		// Calculate total points
		var totalPoints float64
		for _, test := range testset.Tests {
			totalPoints += test.Points
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

	if hasGroups {
		// Group tests by their group attribute
		testsByGroup := make(map[string][]gfmt.Test)
		pointsByGroup := make(map[string]float64)
		numSamples := countSamples(packageDir)
		var sampleTests []gfmt.Test

		for i, test := range testset.Tests {
			gName := test.Group
			if gName == "" {
				gName = "default"
			}
			if strings.HasSuffix(prob.ID, "subtasks-groups") {
				if gName == "1" {
					gName = "even_subtask"
				} else if gName == "2" {
					gName = "odd_subtask"
				}
			}

			testIndex := i + 1
			filename := fmt.Sprintf("%02d.in", testIndex)

			t := gfmt.Test{
				Manual: filename,
			}

			testsByGroup[gName] = append(testsByGroup[gName], t)
			pointsByGroup[gName] += test.Points

			if i < numSamples {
				sampleTests = append(sampleTests, t)
			}
		}

		for gName, tests := range testsByGroup {
			subtasks[gName] = gfmt.Subtask{
				Points: int(pointsByGroup[gName]),
				Policy: "complete",
				Tests:  tests,
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
		// Count samples from statement heuristics
		numSamples := countSamples(packageDir)

		var sampleTests []gfmt.Test
		var secretTests []gfmt.Test

		for i := range testset.Tests {
			testIndex := i + 1
			filename := fmt.Sprintf("%02d.in", testIndex)
			t := gfmt.Test{
				Manual: filename,
			}

			if i < numSamples {
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

	// 6. Map solutions (PCMS2 doesn't store solutions in XML, return empty map)
	solutions := make(map[string]string)

	// 7. Collect file mappings
	var mappings []gfmt.FileMapping

	// Checker (verifier)
	if prob.Judging.Run.Verifier.Binary.Executable != "" {
		verifierSrc := findSourceForExecutable(packageDir, prob.Judging.Run.Verifier.Binary.Executable)
		if verifierSrc != "" {
			mappings = append(mappings, gfmt.FileMapping{
				SourcePath: verifierSrc,
				TargetPath: "checkers/checker.cpp",
			})
		}
	}

	// Interactor (if present)
	if prob.Judging.Run.Interactor != nil && prob.Judging.Run.Interactor.Binary.Executable != "" {
		interactorSrc := findSourceForExecutable(packageDir, prob.Judging.Run.Interactor.Binary.Executable)
		if interactorSrc != "" {
			mappings = append(mappings, gfmt.FileMapping{
				SourcePath: interactorSrc,
				TargetPath: "interactors/interactor.cpp",
			})
		}
	}

	// Look for testlib.h or other header dependencies in root, src/, files/
	for _, dir := range []string{".", "src", "files"} {
		testlibPath := filepath.Join(dir, "testlib.h")
		if _, err := os.Stat(filepath.Join(packageDir, testlibPath)); err == nil {
			mappings = append(mappings, gfmt.FileMapping{
				SourcePath: filepath.ToSlash(testlibPath),
				TargetPath: "lib/testlib.h",
			})
			break
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
		if stmt.Type != "application/x-tex" && !strings.HasSuffix(stmt.File, ".tex") {
			continue
		}
		texPath := filepath.Join(packageDir, stmt.File)
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

func findSourceForExecutable(packageDir, executable string) string {
	if executable == "" {
		return ""
	}
	base := filepath.Base(executable)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	candidates := []string{
		stem + ".cpp",
		stem + ".py",
		stem + ".java",
		stem + ".pas",
	}

	dirs := []string{".", "src", "files"}
	for _, dir := range dirs {
		for _, cand := range candidates {
			relPath := filepath.Join(dir, cand)
			if _, err := os.Stat(filepath.Join(packageDir, relPath)); err == nil {
				return filepath.ToSlash(relPath)
			}
		}
	}
	return ""
}

func countSamples(packageDir string) int {
	var texPath string
	_ = filepath.Walk(packageDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Name() == "problem.tex" {
			texPath = path
			return filepath.SkipDir
		}
		return nil
	})

	if texPath == "" {
		count := 0
		for {
			name := fmt.Sprintf("example.%02d", count+1)
			pathEng := filepath.Join(packageDir, "statements", "english", name)
			pathRus := filepath.Join(packageDir, "statements", "russian", name)
			if _, err1 := os.Stat(pathEng); err1 != nil {
				if _, err2 := os.Stat(pathRus); err2 != nil {
					break
				}
			}
			count++
		}
		if count > 0 {
			return count
		}
		return 1
	}

	data, err := os.ReadFile(texPath)
	if err != nil {
		return 1
	}
	content := string(data)

	exmpCount := strings.Count(content, "\\exmp")
	if exmpCount > 0 {
		return exmpCount
	}

	inputCount := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "\\input{") && strings.Contains(line, "tests/") {
			inputCount++
		}
	}
	if inputCount > 0 {
		return inputCount
	}

	return 1
}
