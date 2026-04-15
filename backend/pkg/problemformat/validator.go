package problemformat

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateManifest проверяет корректность манифеста
func ValidateManifest(manifest *ProblemManifest) error {
	var errs []error

	// Validate problem type
	validTypes := map[string]bool{
		"pass-fail":     true,
		"scoring":       true,
		"interactive":   true,
		"multi-pass":    true,
		"submit-answer": true,
	}
	if !validTypes[manifest.ProblemType] {
		errs = append(errs, fmt.Errorf("invalid problem_type: %s (must be one of: pass-fail, scoring, interactive, multi-pass, submit-answer)", manifest.ProblemType))
	}

	// Validate max_score for scoring problems
	if manifest.ProblemType == "scoring" && manifest.MaxScore == nil {
		errs = append(errs, errors.New("max_score is required for scoring problems"))
	}
	if manifest.ProblemType != "scoring" && manifest.MaxScore != nil {
		errs = append(errs, errors.New("max_score must be null for non-scoring problems"))
	}

	// Validate limits
	if manifest.TimeLimitMs <= 0 {
		errs = append(errs, errors.New("time_limit_ms must be positive"))
	}
	if manifest.MemoryLimitMb <= 0 {
		errs = append(errs, errors.New("memory_limit_mb must be positive"))
	}
	if manifest.StdoutLimitMb <= 0 {
		errs = append(errs, errors.New("stdout_limit_mb must be positive"))
	}
	if manifest.CodeSizeLimitKb <= 0 {
		errs = append(errs, errors.New("code_size_limit_kb must be positive"))
	}

	// Validate statement
	if strings.TrimSpace(manifest.Statement.Title) == "" {
		errs = append(errs, errors.New("statement.title is required"))
	}
	if strings.TrimSpace(manifest.Statement.Legend) == "" {
		errs = append(errs, errors.New("statement.legend is required"))
	}

	// Validate file metadata
	validFileTypes := map[string]bool{
		"checker":    true,
		"validator":  true,
		"generator":  true,
		"interactor": true,
	}
	for i, file := range manifest.FilesMetadata {
		if !validFileTypes[file.Type] {
			errs = append(errs, fmt.Errorf("meta_files[%d].type: invalid type %s", i, file.Type))
		}
		if strings.TrimSpace(file.Filename) == "" {
			errs = append(errs, fmt.Errorf("meta_files[%d].filename is required", i))
		}
		if strings.TrimSpace(file.Compiler) == "" {
			errs = append(errs, fmt.Errorf("meta_files[%d].compiler is required", i))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// ValidateTestsMetadata проверяет корректность tests.json
func ValidateTestsMetadata(tests *TestsMetadata, manifest *ProblemManifest) error {
	var errs []error

	// Validate tests
	testOrdinals := make(map[int]bool)
	for i, test := range tests.Tests {
		if test.Ordinal < 1 {
			errs = append(errs, fmt.Errorf("tests[%d].ordinal must be >= 1", i))
		}
		if testOrdinals[test.Ordinal] {
			errs = append(errs, fmt.Errorf("duplicate test ordinal: %d", test.Ordinal))
		}
		testOrdinals[test.Ordinal] = true

		// Validate method
		if test.Method != "manual" && test.Method != "generated" {
			errs = append(errs, fmt.Errorf("tests[%d].method must be 'manual' or 'generated'", i))
		}

		// Validate generator for generated tests
		if test.Method == "generated" && (test.Generator == nil || strings.TrimSpace(*test.Generator) == "") {
			errs = append(errs, fmt.Errorf("tests[%d].generator is required for generated tests", i))
		}
	}

	// Validate test ordinals are sequential (1, 2, 3, ..., n)
	if len(tests.Tests) > 0 {
		for expected := 1; expected <= len(tests.Tests); expected++ {
			if !testOrdinals[expected] {
				errs = append(errs, fmt.Errorf("test ordinals are not sequential: missing ordinal %d", expected))
			}
		}
	}

	// Validate groups
	groupOrdinals := make(map[int]bool)
	for i, group := range tests.Groups {
		if group.Ordinal < 0 {
			errs = append(errs, fmt.Errorf("groups[%d].ordinal must be >= 0", i))
		}
		if groupOrdinals[group.Ordinal] {
			errs = append(errs, fmt.Errorf("duplicate group ordinal: %d", group.Ordinal))
		}
		groupOrdinals[group.Ordinal] = true

		// Validate test range
		if group.Tests[0] > group.Tests[1] {
			errs = append(errs, fmt.Errorf("groups[%d].tests: invalid range [%d, %d]", i, group.Tests[0], group.Tests[1]))
		}

		// Validate points policy
		if group.PointsPolicy != "complete-group" && group.PointsPolicy != "each-test" {
			errs = append(errs, fmt.Errorf("groups[%d].points_policy must be 'complete-group' or 'each-test'", i))
		}

		// Validate dependencies
		for _, depOrdinal := range group.DependsOn {
			if !groupOrdinals[depOrdinal] && depOrdinal != group.Ordinal {
				// Allow forward dependencies, just check for circular
				if depOrdinal >= group.Ordinal {
					errs = append(errs, fmt.Errorf("groups[%d].depends_on: forward or circular dependency detected", i))
				}
			}
		}
	}

	// Validate group ordinals are sequential (0, 1, 2, ..., n-1)
	if len(tests.Groups) > 0 {
		for expected := 0; expected < len(tests.Groups); expected++ {
			if !groupOrdinals[expected] {
				errs = append(errs, fmt.Errorf("group ordinals are not sequential: missing ordinal %d", expected))
			}
		}
	}

	// Validate points for scoring problems
	if manifest != nil && manifest.ProblemType == "scoring" {
		totalPoints := 0
		for _, group := range tests.Groups {
			totalPoints += group.Points
		}
		if manifest.MaxScore != nil && totalPoints != *manifest.MaxScore {
			errs = append(errs, fmt.Errorf("sum of group points (%d) does not match max_score (%d)", totalPoints, *manifest.MaxScore))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
