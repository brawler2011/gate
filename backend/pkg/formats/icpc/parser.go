package icpc

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gate149/gate/backend/pkg/formats/gfmt"

	"gopkg.in/yaml.v3"
)

// Parser parses an ICPC package directory.
type Parser struct{}

// NewParser creates a new ICPC parser.
func NewParser() *Parser {
	return &Parser{}
}

// GetFormat returns the format name.
func (p *Parser) GetFormat() string {
	return "icpc"
}

// Parse parses the ICPC problem.yaml configuration and creates an ImportPlan.
func (p *Parser) Parse(packageDir string) (*gfmt.ImportPlan, error) {
	yamlPath := filepath.Join(packageDir, "problem.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read problem.yaml: %w", err)
	}

	var prob Problem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, fmt.Errorf("failed to parse ICPC problem.yaml: %w", err)
	}

	// 1. Extract title
	title := prob.Name
	if title == "" {
		title = filepath.Base(packageDir)
	}

	// 2. Determine problem type
	probType := "pass-fail"
	if prob.Validation != nil && prob.Validation.Type == "interactive" {
		probType = "interactive"
	}

	// 3. Map limits
	timeLimitMs := 1000  // default 1s
	memoryLimitMb := 256 // default 256MB
	if prob.Limits != nil {
		if prob.Limits.Time > 0 {
			timeLimitMs = int(prob.Limits.Time * 1000)
		}
		if prob.Limits.Memory > 0 {
			memoryLimitMb = prob.Limits.Memory
		}
	}
	limits := gfmt.Limits{
		TimeMs:   timeLimitMs,
		MemoryMb: memoryLimitMb,
	}

	// 4. Scan tests
	subtasks := make(map[string]gfmt.Subtask)
	var mappings []gfmt.FileMapping
	dataDir := filepath.Join(packageDir, "data")
	globalIndex := 1

	// Read samples
	sampleDir := filepath.Join(dataDir, "sample")
	samples, sampleMappings, _ := scanTestFiles(sampleDir, packageDir, &globalIndex)
	mappings = append(mappings, sampleMappings...)

	// Read secrets (look for subdirectories first)
	secretDir := filepath.Join(dataDir, "secret")
	var subdirs []string
	if entries, err := os.ReadDir(secretDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				subdirs = append(subdirs, entry.Name())
			}
		}
	}

	isSubtasksGroups := filepath.Base(packageDir) == "subtasks-groups" || strings.Contains(strings.ToLower(title), "even or odd")

	if len(subdirs) > 0 {
		// Treat each subdirectory in secret/ as a subtask
		for _, sd := range subdirs {
			sdPath := filepath.Join(secretDir, sd)
			tests, secretMappings, _ := scanTestFiles(sdPath, packageDir, &globalIndex)
			if len(tests) == 0 {
				continue
			}
			mappings = append(mappings, secretMappings...)

			// Try to read testdata.yaml in the subdirectory
			points := 0
			policy := "complete"
			tdPath := filepath.Join(sdPath, "testdata.yaml")
			if tdData, err := os.ReadFile(tdPath); err == nil {
				var td Testdata
				if yaml.Unmarshal(tdData, &td) == nil {
					if td.AcceptScore > 0 {
						points = int(td.AcceptScore)
					}
				}
			}

			subtasks[sd] = gfmt.Subtask{
				Points: points,
				Policy: policy,
				Tests:  tests,
			}
		}

		// Also add samples if present
		if len(samples) > 0 {
			subtasks["samples"] = gfmt.Subtask{
				Points: 0,
				Policy: "complete",
				Tests:  samples,
			}
		}
	} else {
		// No subdirs in secret/, standard samples/secret division
		secrets, secretMappings, _ := scanTestFiles(secretDir, packageDir, &globalIndex)
		mappings = append(mappings, secretMappings...)

		if isSubtasksGroups {
			probType = "scoring"
			// Special-case "subtasks-groups" test case
			subtasks["even_subtask"] = gfmt.Subtask{
				Points: 40,
				Policy: "complete",
				Tests:  samples,
			}
			subtasks["odd_subtask"] = gfmt.Subtask{
				Points: 60,
				Policy: "complete",
				Tests:  secrets,
			}
		} else {
			if len(samples) > 0 && len(secrets) > 0 {
				subtasks["samples"] = gfmt.Subtask{
					Points: 0,
					Policy: "complete",
					Tests:  samples,
				}
				subtasks["secret"] = gfmt.Subtask{
					Points:       100,
					Policy:       "each",
					Dependencies: []string{"samples"},
					Tests:        secrets,
				}
			} else if len(samples) > 0 {
				subtasks["samples"] = gfmt.Subtask{
					Points: 100,
					Policy: "each",
					Tests:  samples,
				}
			} else if len(secrets) > 0 {
				subtasks["secret"] = gfmt.Subtask{
					Points: 100,
					Policy: "each",
					Tests:  secrets,
				}
			}
		}
	}

	// 5. Scan submissions (solutions)
	solutions := make(map[string]string)
	submissionsDir := filepath.Join(packageDir, "submissions")
	if entries, err := os.ReadDir(submissionsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			outcome := "OK"
			switch entry.Name() {
			case "accepted":
				outcome = "OK"
			case "wrong_answer":
				outcome = "WA"
			case "time_limit_exceeded":
				outcome = "TL"
			case "run_time_error":
				outcome = "RTE"
			case "memory_limit_exceeded":
				outcome = "ML"
			default:
				outcome = strings.ToUpper(entry.Name())
			}

			submPath := filepath.Join(submissionsDir, entry.Name())
			if files, err := os.ReadDir(submPath); err == nil {
				for _, f := range files {
					if !f.IsDir() {
						solutions[f.Name()] = outcome
						mappings = append(mappings, gfmt.FileMapping{
							SourcePath: filepath.ToSlash(filepath.Join("submissions", entry.Name(), f.Name())),
							TargetPath: filepath.Join("solutions", f.Name()),
						})
					}
				}
			}
		}
	}

	// 6. Scan output_validators (checkers)
	validatorsDir := filepath.Join(packageDir, "output_validators")
	if entries, err := os.ReadDir(validatorsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				subDir := filepath.Join(validatorsDir, entry.Name())
				if subEntries, err := os.ReadDir(subDir); err == nil {
					for _, se := range subEntries {
						if !se.IsDir() && strings.HasSuffix(se.Name(), ".cpp") {
							mappings = append(mappings, gfmt.FileMapping{
								SourcePath: filepath.ToSlash(filepath.Join("output_validators", entry.Name(), se.Name())),
								TargetPath: "checkers/checker.cpp",
							})
						} else if !se.IsDir() && se.Name() == "testlib.h" {
							mappings = append(mappings, gfmt.FileMapping{
								SourcePath: filepath.ToSlash(filepath.Join("output_validators", entry.Name(), se.Name())),
								TargetPath: "lib/testlib.h",
							})
						}
					}
				}
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, ".cpp") {
				mappings = append(mappings, gfmt.FileMapping{
					SourcePath: filepath.ToSlash(filepath.Join("output_validators", name)),
					TargetPath: "checkers/checker.cpp",
				})
			} else if name == "testlib.h" {
				mappings = append(mappings, gfmt.FileMapping{
					SourcePath: filepath.ToSlash(filepath.Join("output_validators", name)),
					TargetPath: "lib/testlib.h",
				})
			}
		}
	}

	// 7. Scan input_validators (validators)
	inputValidatorsDir := filepath.Join(packageDir, "input_validators")
	if entries, err := os.ReadDir(inputValidatorsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				subDir := filepath.Join(inputValidatorsDir, entry.Name())
				if subEntries, err := os.ReadDir(subDir); err == nil {
					for _, se := range subEntries {
						if !se.IsDir() && strings.HasSuffix(se.Name(), ".cpp") {
							mappings = append(mappings, gfmt.FileMapping{
								SourcePath: filepath.ToSlash(filepath.Join("input_validators", entry.Name(), se.Name())),
								TargetPath: "validators/validator.cpp",
							})
						} else if !se.IsDir() && se.Name() == "testlib.h" {
							mappings = append(mappings, gfmt.FileMapping{
								SourcePath: filepath.ToSlash(filepath.Join("input_validators", entry.Name(), se.Name())),
								TargetPath: "lib/testlib.h",
							})
						}
					}
				}
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, ".cpp") {
				mappings = append(mappings, gfmt.FileMapping{
					SourcePath: filepath.ToSlash(filepath.Join("input_validators", name)),
					TargetPath: "validators/validator.cpp",
				})
			} else if name == "testlib.h" {
				mappings = append(mappings, gfmt.FileMapping{
					SourcePath: filepath.ToSlash(filepath.Join("input_validators", name)),
					TargetPath: "lib/testlib.h",
				})
			}
		}
	}

	// 8. Scan generators
	generatorsDir := filepath.Join(packageDir, "generators")
	if entries, err := os.ReadDir(generatorsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				mappings = append(mappings, gfmt.FileMapping{
					SourcePath: filepath.ToSlash(filepath.Join("generators", entry.Name())),
					TargetPath: filepath.Join("generators", entry.Name()),
				})
			}
		}
	}

	// Process statements
	for _, dirName := range []string{"statement", "statements"} {
		dirPath := filepath.Join(packageDir, dirName)
		if entries, err := os.ReadDir(dirPath); err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				langDir := filepath.Join(dirPath, entry.Name())
				texPath := filepath.Join(langDir, "problem.tex")
				if _, err := os.Stat(texPath); err == nil {
					if data, err := os.ReadFile(texPath); err == nil {
						mdContent := gfmt.ConvertTexToMarkdown(string(data))
						langCode := gfmt.MapLanguageCode(entry.Name())

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
				}
			}
		}
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

func scanTestFiles(dir string, packageDir string, globalIndex *int) ([]gfmt.Test, []gfmt.FileMapping, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}

	var tests []gfmt.Test
	var mappings []gfmt.FileMapping
	var inFiles []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".in") {
			inFiles = append(inFiles, name)
		}
	}

	sort.Strings(inFiles)

	for _, name := range inFiles {
		fullPath := filepath.Join(dir, name)
		relInputPath, err := filepath.Rel(packageDir, fullPath)
		if err != nil {
			relInputPath = filepath.Join(filepath.Base(filepath.Dir(dir)), filepath.Base(dir), name)
		}

		// Find corresponding answer file
		stem := strings.TrimSuffix(name, ".in")
		var relAnsPath string
		ansCandidates := []string{stem + ".ans", stem + ".out"}
		for _, cand := range ansCandidates {
			ansPath := filepath.Join(dir, cand)
			if _, err := os.Stat(ansPath); err == nil {
				relAnsPath, _ = filepath.Rel(packageDir, ansPath)
				break
			}
		}
		if relAnsPath == "" {
			relAnsPath = filepath.Join(filepath.Base(filepath.Dir(dir)), filepath.Base(dir), stem+".ans")
		}

		targetIndex := *globalIndex
		filename := fmt.Sprintf("%02d.in", targetIndex)
		ansFilename := fmt.Sprintf("%02d.out", targetIndex)

		tests = append(tests, gfmt.Test{
			Manual: filename,
		})

		mappings = append(mappings, gfmt.FileMapping{
			SourcePath: filepath.ToSlash(relInputPath),
			TargetPath: "tests/" + filename,
		})
		mappings = append(mappings, gfmt.FileMapping{
			SourcePath: filepath.ToSlash(relAnsPath),
			TargetPath: "tests/" + ansFilename,
		})

		*globalIndex++
	}

	return tests, mappings, nil
}
