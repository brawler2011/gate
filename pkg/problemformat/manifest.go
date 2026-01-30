package problemformat

import (
	"time"
)

// ProblemManifest - основной манифест задачи (manifest.json)
type ProblemManifest struct {
	LastUpdated   time.Time              `json:"last_updated"`
	ProblemType   string                 `json:"problem_type"` // "pass-fail", "scoring", "interactive"
	MaxScore      *int                   `json:"max_score"`
	FilesMetadata []FileMetadata         `json:"meta_files"`
	TimeLimitMs   int                    `json:"time_limit_ms"`
	MemoryLimitMb int                    `json:"memory_limit_mb"`
	StdoutLimitMb int                    `json:"stdout_limit_mb"`
	CodeSizeLimitKb int                  `json:"code_size_limit_kb"`
	Statements    map[string]Statement   `json:"statements"` // {"en": {...}, "ru": {...}}
}

// Statement - условие задачи на одном языке
type Statement struct {
	Title        string `json:"title"`
	Legend       string `json:"legend"`
	InputFormat  string `json:"input_format"`
	OutputFormat string `json:"output_format"`
	Notes        string `json:"notes"`
	Interaction  string `json:"interaction"`
	Scoring      string `json:"scoring"`
}

// Dependency - зависимость файла (например, testlib.h)
type Dependency struct {
	Filename string `json:"filename"` // "testlib.h"
	Version  string `json:"version"`  // "0.9.41"
}

// FileMetadata - метаданные файла (checker, validator, generator, interactor)
type FileMetadata struct {
	Type         string       `json:"type"` // "checker", "validator", "generator", "interactor"
	Filename     string       `json:"filename"`
	Compiler     string       `json:"compiler"` // "cpp17", "python3", etc
	BinarySha256 *string      `json:"binary_sha256"`
	Dependencies []Dependency `json:"dependencies"`
}

// TestsMetadata - метаданные тестов (tests/tests.json)
type TestsMetadata struct {
	Groups []TestGroup `json:"groups"`
	Tests  []TestCase  `json:"tests"`
}

// TestGroup - группа тестов
type TestGroup struct {
	Ordinal      int    `json:"ordinal"`
	Name         string `json:"name"`
	Points       int    `json:"points"`
	PointsPolicy string `json:"points_policy"` // "complete-group", "each-test"
	DependsOn    []int  `json:"depends_on"`    // номера групп или null
	Tests        [2]int `json:"tests"`         // диапазон [start, end]
}

// TestCase - тестовый случай
type TestCase struct {
	Ordinal   int     `json:"ordinal"`
	Method    string  `json:"method"`    // "manual" или "generated"
	Generator *string `json:"generator"` // "gen_border 1 2 3" или null
	IsSample  bool    `json:"is_sample"`
}

// Media - медиа-файлы задачи
type Media struct {
	Images []Image `json:"images"`
}

// Image - изображение
type Image struct {
	Filename string `json:"filename"`
}
