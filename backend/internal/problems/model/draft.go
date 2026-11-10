package problemsmodel

import (
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

const DraftSnapshotVersion = 1

type Draft struct {
	ProblemID      uuid.UUID         `json:"problem_id"`
	Revision       int64             `json:"revision"`
	Title          string            `json:"title"`
	ShortName      string            `json:"short_name"`
	Type           string            `json:"type"`
	TimeLimitMs    int               `json:"time_limit_ms"`
	MemoryLimitMb  int               `json:"memory_limit_mb"`
	MaxScore       *points.Value     `json:"max_score"`
	CheckerID      *uuid.UUID        `json:"checker_id"`
	ValidatorID    *uuid.UUID        `json:"validator_id"`
	InteractorID   *uuid.UUID        `json:"interactor_id"`
	MainSolutionID *uuid.UUID        `json:"main_solution_id"`
	Files          map[string]string `json:"files"`
	Components     []DraftComponent  `json:"components"`
	Subtasks       []DraftSubtask    `json:"subtasks"`
	Tests          []DraftTest       `json:"tests"`
	Assets         []DraftAsset      `json:"assets"`
}

type DraftComponent struct {
	ID           uuid.UUID         `json:"id"`
	Role         string            `json:"role"`
	Path         string            `json:"path"`
	Language     string            `json:"language"`
	Dependencies map[string]string `json:"dependencies"`
}

type DraftSubtask struct {
	ID        uuid.UUID    `json:"id"`
	Ordinal   int          `json:"ordinal"`
	Name      string       `json:"name"`
	Points    points.Value `json:"points"`
	Policy    string       `json:"policy"`
	DependsOn []uuid.UUID  `json:"depends_on"`
}

type DraftTest struct {
	Points    points.Value `json:"points"`
	ID        uuid.UUID    `json:"id"`
	Ordinal   int          `json:"ordinal"`
	SubtaskID uuid.UUID    `json:"subtask_id"`
	Sample    bool         `json:"sample"`
	Input     DraftInput   `json:"input"`
	Answer    DraftAnswer  `json:"answer"`
}

type DraftInput struct {
	Mode        string     `json:"mode"`
	ObjectID    *uuid.UUID `json:"object_id"`
	ComponentID *uuid.UUID `json:"component_id"`
	Args        []string   `json:"args"`
	// Seed is appended as one decimal argv element after Args (testlib convention).
	Seed *int64 `json:"seed"`
}

type DraftAnswer struct {
	Mode        string     `json:"mode"`
	ObjectID    *uuid.UUID `json:"object_id"`
	ComponentID *uuid.UUID `json:"component_id"`
}

type DraftAsset struct {
	Path     string    `json:"path"`
	ObjectID uuid.UUID `json:"object_id"`
}

type BuildSnapshot struct {
	SchemaVersion  int         `json:"schema_version"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	Draft          Draft       `json:"draft"`
	Toolchain      Toolchain   `json:"toolchain"`
	Limits         BuildLimits `json:"limits"`
	// InputObjects is derived from Draft, never accepted from the caller.
	InputObjects []uuid.UUID `json:"input_objects"`
}

type BuildLimits struct {
	DiskBytes       int64                 `json:"disk_bytes" yaml:"disk_bytes"`
	FileCount       int                   `json:"file_count" yaml:"file_count"`
	ObjectBytes     int64                 `json:"object_bytes" yaml:"object_bytes"`
	TestOutputBytes int64                 `json:"test_output_bytes" yaml:"test_output_bytes"`
	Compile         sandbox.FileRunLimits `json:"compile" yaml:"compile"`
	Auxiliary       sandbox.FileRunLimits `json:"auxiliary" yaml:"auxiliary"`
}

type Toolchain struct {
	Digest       string                      `json:"digest"`
	Architecture string                      `json:"architecture"`
	ABI          string                      `json:"abi"`
	Languages    map[string]sandbox.Language `json:"languages"`
}

type DraftQuotas struct {
	FileBytes  int64 `yaml:"file_bytes"`
	TextBytes  int64 `yaml:"text_bytes"`
	Files      int   `yaml:"files"`
	Tests      int   `yaml:"tests"`
	Components int   `yaml:"components"`
	Subtasks   int   `yaml:"subtasks"`
}

func ValidDraftPath(path string) bool {
	return fs.ValidPath(path) && path != "." && path != "problem.yaml" && path != ".gate" && !strings.HasPrefix(path, ".gate/") && !strings.ContainsAny(path, "\\\x00") && len(path) <= 512
}

func (d *Draft) InputObjects() []uuid.UUID {
	ids := make(map[uuid.UUID]struct{})
	for _, test := range d.Tests {
		if test.Input.ObjectID != nil {
			ids[*test.Input.ObjectID] = struct{}{}
		}
		if test.Answer.ObjectID != nil {
			ids[*test.Answer.ObjectID] = struct{}{}
		}
	}
	for _, asset := range d.Assets {
		ids[asset.ObjectID] = struct{}{}
	}
	result := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	slices.SortFunc(result, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return result
}

// Validate permits incomplete selections while editing, but never dangling or
// conflicting references. Build validation also requires runnable components.
func (d *Draft) Validate(q DraftQuotas, build bool) error {
	if d.Revision < 1 || d.ProblemID == uuid.Nil || !utf8.ValidString(d.Title) || len(d.Title) > 1024 || len(d.ShortName) > 128 {
		return fmt.Errorf("invalid draft identity or metadata")
	}
	if d.Type != "pass-fail" && d.Type != "scoring" && d.Type != "interactive" {
		return fmt.Errorf("invalid problem type")
	}
	if d.TimeLimitMs <= 0 || d.MemoryLimitMb <= 0 {
		return fmt.Errorf("limits must be positive")
	}
	if len(d.Files)+len(d.Assets) > q.Files || len(d.Tests) > q.Tests || len(d.Components) > q.Components || len(d.Subtasks) > q.Subtasks {
		return fmt.Errorf("draft count quota exceeded")
	}
	var total int64
	paths := make(map[string]bool)
	for path, text := range d.Files {
		if !ValidDraftPath(path) || !utf8.ValidString(text) || int64(len(text)) > q.FileBytes {
			return fmt.Errorf("invalid text file %q or size exceeded", path)
		}
		total += int64(len(text))
		paths[path] = true
	}
	if total > q.TextBytes {
		return fmt.Errorf("draft text quota exceeded")
	}
	for _, asset := range d.Assets {
		if !ValidDraftPath(asset.Path) || paths[asset.Path] || asset.ObjectID == uuid.Nil {
			return fmt.Errorf("invalid or duplicate asset %q", asset.Path)
		}
		paths[asset.Path] = true
	}
	components := make(map[uuid.UUID]DraftComponent)
	for _, c := range d.Components {
		if c.ID == uuid.Nil || components[c.ID].ID != uuid.Nil || !ValidDraftPath(c.Path) || c.Language == "" {
			return fmt.Errorf("invalid or duplicate component")
		}
		if !slices.Contains([]string{"checker", "validator", "interactor", "solution", "generator"}, c.Role) {
			return fmt.Errorf("invalid component role")
		}
		if _, ok := d.Files[c.Path]; !ok {
			return fmt.Errorf("missing component source %q", c.Path)
		}
		seen := map[string]bool{c.Path: true}
		for destination, dep := range c.Dependencies {
			if !ValidDraftPath(destination) {
				return fmt.Errorf("invalid dependency destination")
			}
			if _, ok := d.Files[dep]; !ok || seen[dep] {
				return fmt.Errorf("missing or duplicate dependency %q", dep)
			}
			seen[dep] = true
		}
		components[c.ID] = c
	}
	checkComponent := func(id *uuid.UUID, role string, required bool) error {
		if id == nil {
			if required {
				return fmt.Errorf("%s is required", role)
			}
			return nil
		}
		if c, ok := components[*id]; !ok || c.Role != role {
			return fmt.Errorf("invalid %s selection", role)
		}
		return nil
	}
	for _, selection := range []struct {
		id       *uuid.UUID
		role     string
		required bool
	}{
		{d.CheckerID, "checker", build && d.Type != "interactive"}, {d.ValidatorID, "validator", false},
		{d.InteractorID, "interactor", build && d.Type == "interactive"}, {d.MainSolutionID, "solution", build && d.Type == "interactive"},
	} {
		if err := checkComponent(selection.id, selection.role, selection.required); err != nil {
			return err
		}
	}
	groups := make(map[uuid.UUID]DraftSubtask)
	orders := make(map[int]bool)
	totalPoints := points.Value(0)
	for _, g := range d.Subtasks {
		if g.ID == uuid.Nil || groups[g.ID].ID != uuid.Nil || g.Ordinal <= 0 || orders[g.Ordinal] || g.Points < 0 || (g.Policy != "complete" && g.Policy != "each") {
			return fmt.Errorf("invalid subtask")
		}
		groups[g.ID] = g
		orders[g.Ordinal] = true
		totalPoints += g.Points
	}
	visited := make(map[uuid.UUID]int)
	var visit func(uuid.UUID) error
	visit = func(id uuid.UUID) error {
		if visited[id] == 1 {
			return fmt.Errorf("subtask dependency cycle")
		}
		if visited[id] == 2 {
			return nil
		}
		g, ok := groups[id]
		if !ok {
			return fmt.Errorf("unknown subtask dependency")
		}
		visited[id] = 1
		seen := make(map[uuid.UUID]bool)
		for _, dep := range g.DependsOn {
			if seen[dep] {
				return fmt.Errorf("duplicate subtask dependency")
			}
			seen[dep] = true
			if err := visit(dep); err != nil {
				return err
			}
		}
		visited[id] = 2
		return nil
	}
	for id := range groups {
		if err := visit(id); err != nil {
			return err
		}
	}
	if d.Type == "scoring" && (d.MaxScore == nil || *d.MaxScore < 0 || *d.MaxScore != totalPoints) {
		return fmt.Errorf("max score must equal subtask points")
	}
	if d.Type != "scoring" && d.MaxScore != nil {
		return fmt.Errorf("max score requires scoring type")
	}
	orders = make(map[int]bool)
	ids := make(map[uuid.UUID]bool)
	for _, test := range d.Tests {
		if test.Points < 0 || test.Points > points.Max {
			return fmt.Errorf("invalid test weight")
		}
		if test.ID == uuid.Nil || ids[test.ID] || test.Ordinal <= 0 || orders[test.Ordinal] || groups[test.SubtaskID].ID == uuid.Nil {
			return fmt.Errorf("invalid test identity, order or subtask")
		}
		ids[test.ID] = true
		orders[test.Ordinal] = true
		i := test.Input
		switch i.Mode {
		case "manual":
			if i.ObjectID == nil || *i.ObjectID == uuid.Nil || i.ComponentID != nil || i.Seed != nil || len(i.Args) != 0 {
				return fmt.Errorf("invalid manual input")
			}
		case "generator":
			if i.ObjectID != nil || i.Seed == nil {
				return fmt.Errorf("invalid generated input")
			}
			if err := checkComponent(i.ComponentID, "generator", true); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid input mode")
		}
		if len(i.Args) > 256 {
			return fmt.Errorf("too many generator arguments")
		}
		for _, arg := range i.Args {
			if len(arg) > 4096 || strings.ContainsRune(arg, 0) || !utf8.ValidString(arg) {
				return fmt.Errorf("invalid generator argument")
			}
		}
		a := test.Answer
		if d.Type == "interactive" {
			if a.Mode != "interactive" || a.ObjectID != nil || a.ComponentID != nil {
				return fmt.Errorf("interactive test cannot have ordinary answer")
			}
		} else {
			switch a.Mode {
			case "generated":
				if a.ObjectID != nil {
					return fmt.Errorf("generated answer cannot reference object")
				}
				if err := checkComponent(a.ComponentID, "solution", true); err != nil {
					return err
				}
			case "provided":
				if a.ObjectID == nil || *a.ObjectID == uuid.Nil || a.ComponentID != nil {
					return fmt.Errorf("invalid provided answer")
				}
			default:
				return fmt.Errorf("invalid answer mode")
			}
		}
	}
	if build && (strings.TrimSpace(d.Title) == "" || len(d.Tests) == 0) {
		return fmt.Errorf("build requires title and tests")
	}
	return nil
}

// ComputeScores derives summaries from explicit per-test weights. It never
// divides a group score across tests or normalizes authored points to 100.
func (d *Draft) ComputeScores() error {
	sums := make(map[uuid.UUID]points.Value)
	var total points.Value
	for _, test := range d.Tests {
		if test.Points < 0 || test.Points > points.Max || total > points.Max-test.Points {
			return fmt.Errorf("test weights exceed score limit")
		}
		sums[test.SubtaskID] += test.Points
		total += test.Points
	}
	for i := range d.Subtasks {
		d.Subtasks[i].Points = sums[d.Subtasks[i].ID]
	}
	if d.Type == "scoring" {
		d.MaxScore = &total
	} else {
		d.MaxScore = nil
	}
	return nil
}
