package models

import (
	"time"
)

// ProblemMetadata соответствует формату metadata.json в корне папки задачи
type ProblemMetadata struct {
	Version       int               `json:"version"` // Не уверен как лучше версию хранить.
	LastUpdated   time.Time         `json:"last_updated"`
	Title         map[string]string `json:"title"` // {"en": "A + B Problem", "ru": "Задача A + B"}
	ShortName     string            `json:"short_name"`
	Source        string            `json:"source,omitempty"`
	ProblemType   string            `json:"problem_type"` // "pass-fail", "scoring", etc
	TimeLimitMs   int               `json:"time_limit_ms"`
	MemoryLimitMb int               `json:"memory_limit_mb"`
	MaxScore      *int              `json:"max_score"` // null для pass-fail задач
	Languages     []string          `json:"languages"` // ["en", "ru"]
}

// SpecialFileMeta соответствует формату meta.json для checker/validator/generator/interactor
type SpecialFileMeta []SpecialFileEntry

type SpecialFileEntry struct {
	Filename     string             `json:"filename"`
	Compiler     string             `json:"compiler"`     // "cpp17", "python3", etc
	Dependencies *map[string]string `json:"dependencies"` // {"testlib.h": "0.9.41"}
	Hash         string             `json:"hash"`         // "sha256:abcd1234..."
}

// TestsMetadata соответствует формату tests/tests.json
type TestsMetadata struct {
	Groups []TestGroup `json:"groups"`
	Tests  []TestCase  `json:"tests"`
}

type TestGroup struct {
	Ordinal      int    `json:"ordinal"`
	Name         string `json:"name"`
	Points       int    `json:"points"`
	PointsPolicy string `json:"points_policy"` // "complete-group", "each-test"
	DependsOn    *int   `json:"depends_on"`    // номер группы или null
	Tests        [2]int `json:"tests"`         // диапазон [start, end]
}

type TestCase struct {
	Ordinal   int     `json:"ordinal"`
	Method    string  `json:"method"`    // "manual" или "generated"
	Generator *string `json:"generator"` // "gen_border 1 2 3" или null
	IsSample  bool    `json:"is_sample"`
}
