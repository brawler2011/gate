package problemsmodel

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/google/uuid"
)

// Draft holds typed entries; package paths are assigned only during build, import and export.
type Draft struct {
	ProblemID     uuid.UUID     `json:"problem_id"`
	Revision      int64         `json:"revision"`
	Title         string        `json:"title"`
	ShortName     string        `json:"short_name"`
	Type          string        `json:"type"`
	TimeLimitMs   int           `json:"time_limit_ms"`
	MemoryLimitMb int           `json:"memory_limit_mb"`
	MaxScore      *points.Value `json:"max_score"`
	DraftContent
}

// DraftContent is stored as one document; catalog-query fields remain separate columns.
type DraftContent struct {
	Checker      *DraftChecker       `json:"checker"`
	Interactor   *DraftProgram       `json:"interactor"`
	Validators   []DraftNamedProgram `json:"validators"`
	Generators   []DraftNamedProgram `json:"generators"`
	Solutions    []DraftNamedProgram `json:"solutions"`
	MainSolution *string             `json:"main_solution"`
	Library      []DraftLibraryFile  `json:"library"`
	Statements   []DraftStatement    `json:"statements"`
	Assets       []DraftAsset        `json:"assets"`
	Subtasks     []DraftSubtask      `json:"subtasks"`
	Tests        []DraftTest         `json:"tests"`
}

type DraftProgram struct {
	Language string `json:"language"`
	Source   string `json:"source"`
}

// DraftChecker uses the current system source when Standard names a checker and Source is empty.
type DraftChecker struct {
	DraftProgram
	Standard string `json:"standard"`
}

// Text bundles standard checkers with system testlib, independently of the draft library.
func (c *DraftChecker) Text() string {
	if checker, ok := syslib.LookupChecker(c.Standard); ok {
		return checker.Standalone()
	}
	return c.Source
}

// DraftNamedProgram names are unique within a role and become package file stems.
type DraftNamedProgram struct {
	Name string `json:"name"`
	DraftProgram
}

// DraftLibraryFile stores no content when AutoUpdate selects the current system library.
type DraftLibraryFile struct {
	Name       string `json:"name"`
	Content    string `json:"content"`
	AutoUpdate bool   `json:"auto_update"`
}

func (f *DraftLibraryFile) Text() string {
	if system, ok := syslib.Lookup(f.Name); ok && f.AutoUpdate {
		return system.Content
	}
	return f.Content
}

type DraftStatement struct {
	Language    string `json:"language"`
	Legend      string `json:"legend"`
	Input       string `json:"input"`
	Output      string `json:"output"`
	Notes       string `json:"notes"`
	Interaction string `json:"interaction"`
	Scoring     string `json:"scoring"`
}

type DraftAsset struct {
	Name     string    `json:"name"`
	ObjectID uuid.UUID `json:"object_id"`
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
	Mode      string     `json:"mode"`
	ObjectID  *uuid.UUID `json:"object_id"`
	Generator *string    `json:"generator"`
	// Args include the full generator argv after its name; testlib derives its random seed from them.
	Args []string `json:"args"`
}

type DraftAnswer struct {
	Mode     string     `json:"mode"`
	ObjectID *uuid.UUID `json:"object_id"`
	Solution *string    `json:"solution"`
}

type BuildSnapshot struct {
	OrganizationID uuid.UUID   `json:"organization_id"`
	Draft          Draft       `json:"draft"`
	Limits         BuildLimits `json:"limits"`
	// InputObjects is derived from Draft, never accepted from the caller.
	InputObjects []uuid.UUID `json:"input_objects"`
}

type BuildLimits struct {
	DiskBytes       bytesize.Size         `json:"disk_bytes" yaml:"disk_bytes"`
	FileCount       int                   `json:"file_count" yaml:"file_count"`
	ObjectBytes     bytesize.Size         `json:"object_bytes" yaml:"object_bytes"`
	TestOutputBytes bytesize.Size         `json:"test_output_bytes" yaml:"test_output_bytes"`
	Compile         sandbox.FileRunLimits `json:"compile" yaml:"compile"`
	Auxiliary       sandbox.FileRunLimits `json:"auxiliary" yaml:"auxiliary"`
}

// DraftQuotas counts library files and assets as Files, and programs as Components.
type DraftQuotas struct {
	FileBytes  bytesize.Size `yaml:"file_bytes"`
	TextBytes  bytesize.Size `yaml:"text_bytes"`
	Files      int           `yaml:"files"`
	Tests      int           `yaml:"tests"`
	Components int           `yaml:"components"`
	Subtasks   int           `yaml:"subtasks"`
}

var StatementSections = []string{"legend", "input", "output", "notes", "interaction", "scoring"}

func (s *DraftStatement) Section(name string) *string {
	switch name {
	case "legend":
		return &s.Legend
	case "input":
		return &s.Input
	case "output":
		return &s.Output
	case "notes":
		return &s.Notes
	case "interaction":
		return &s.Interaction
	case "scoring":
		return &s.Scoring
	}
	return nil
}

// TrimStatements applies the import's trimming on save, so an exported draft imports back unchanged.
func (d *Draft) TrimStatements() {
	for i := range d.Statements {
		for _, name := range StatementSections {
			section := d.Statements[i].Section(name)
			*section = strings.TrimSpace(*section)
		}
	}
}

func (d *Draft) Statement(language string) *DraftStatement {
	for i := range d.Statements {
		if d.Statements[i].Language == language {
			return &d.Statements[i]
		}
	}
	return nil
}

func findProgram(list []DraftNamedProgram, name string) *DraftNamedProgram {
	for i := range list {
		if list[i].Name == name {
			return &list[i]
		}
	}
	return nil
}

func (d *Draft) Generator(name string) *DraftNamedProgram { return findProgram(d.Generators, name) }

func (d *Draft) Solution(name string) *DraftNamedProgram { return findProgram(d.Solutions, name) }

func (d *Draft) Validator() *DraftNamedProgram {
	if len(d.Validators) == 0 {
		return nil
	}
	return &d.Validators[0]
}

// Normalize keeps absent lists from serializing as null in the contract.
func (d *Draft) Normalize() {
	empty := func(list *[]DraftNamedProgram) {
		if *list == nil {
			*list = []DraftNamedProgram{}
		}
	}
	empty(&d.Validators)
	empty(&d.Generators)
	empty(&d.Solutions)
	if d.Library == nil {
		d.Library = []DraftLibraryFile{}
	}
	if d.Statements == nil {
		d.Statements = []DraftStatement{}
	}
	if d.Assets == nil {
		d.Assets = []DraftAsset{}
	}
	if d.Subtasks == nil {
		d.Subtasks = []DraftSubtask{}
	}
	for i := range d.Subtasks {
		if d.Subtasks[i].DependsOn == nil {
			d.Subtasks[i].DependsOn = []uuid.UUID{}
		}
	}
	if d.Tests == nil {
		d.Tests = []DraftTest{}
	}
	for i := range d.Tests {
		if d.Tests[i].Input.Args == nil {
			d.Tests[i].Input.Args = []string{}
		}
	}
}

func ValidFileName(name string) bool {
	if name == "" || name == "." || name == ".." || len(name) > 255 || !utf8.ValidString(name) || strings.ContainsAny(name, `/\`) {
		return false
	}
	return !strings.ContainsFunc(name, unicode.IsControl)
}

var (
	languageKey       = regexp.MustCompile(`^[a-z0-9][a-z0-9+_.-]{0,31}$`)
	statementLanguage = regexp.MustCompile(`^[a-z]{2,8}(-[a-z0-9]{2,8})*$`)
)

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

const maxFieldErrors = 50

type validation struct{ errs pkg.FieldErrors }

func (v *validation) fail(pointer, format string, args ...any) {
	if len(v.errs) < maxFieldErrors {
		v.errs = append(v.errs, pkg.FieldError{Pointer: pointer, Message: fmt.Sprintf(format, args...)})
	}
}

func (v *validation) err() error {
	if len(v.errs) == 0 {
		return nil
	}
	return v.errs
}

// Validate allows incomplete drafts and checks images only in changed sections, allowing staged asset renames.
// A nil previous checks all sections; violations return pkg.FieldErrors.
func (d *Draft) Validate(q DraftQuotas, previous *Draft) error {
	v := &validation{}
	d.validate(v, q)
	d.validateImages(v, previous)
	return v.err()
}

// ValidateBuild requires complete selections and checks all image references, including unchanged sections.
func (d *Draft) ValidateBuild(q DraftQuotas, languages map[string]sandbox.Language) error {
	v := &validation{}
	d.validate(v, q)
	d.validateImages(v, nil)
	d.eachProgram(func(at string, p DraftProgram) {
		if _, ok := languages[p.Language]; !ok {
			v.fail(at+"/language", "language %q is unavailable in the toolchain", p.Language)
		}
	})
	if d.Checker != nil && d.Checker.Standard != "" && !slices.Contains(languages[d.Checker.Language].Extensions, "cpp") {
		v.fail("/checker/language", "a standard checker compiles as C++")
	}
	if d.Type != "interactive" && d.Checker == nil {
		v.fail("/checker", "checker is required")
	}
	if d.Type == "interactive" && d.Interactor == nil {
		v.fail("/interactor", "interactor is required")
	}
	if d.Type == "interactive" && d.MainSolution == nil {
		v.fail("/main_solution", "interactive problem requires a main solution")
	}
	if strings.TrimSpace(d.Title) == "" {
		v.fail("/title", "title is required")
	}
	if len(d.Tests) == 0 {
		v.fail("/tests", "at least one test is required")
	}
	return v.err()
}

func (d *Draft) validate(v *validation, q DraftQuotas) {
	if d.ProblemID == uuid.Nil {
		v.fail("/problem_id", "problem ID is required")
	}
	if d.Revision < 1 {
		v.fail("/revision", "revision must be positive")
	}
	if !utf8.ValidString(d.Title) || len(d.Title) > 1024 {
		v.fail("/title", "title must be UTF-8 of at most 1024 bytes")
	}
	if d.ShortName == "" || len(d.ShortName) > 128 {
		v.fail("/short_name", "short name must have 1 to 128 bytes")
	}
	if d.Type != "pass-fail" && d.Type != "scoring" && d.Type != "interactive" {
		v.fail("/type", "invalid problem type %q", d.Type)
	}
	if d.TimeLimitMs <= 0 {
		v.fail("/time_limit_ms", "time limit must be positive")
	}
	if d.MemoryLimitMb <= 0 {
		v.fail("/memory_limit_mb", "memory limit must be positive")
	}
	programs := len(d.Validators) + len(d.Generators) + len(d.Solutions)
	if d.Checker != nil {
		programs++
	}
	if d.Interactor != nil {
		programs++
	}
	if programs > q.Components {
		v.fail("", "more than %d programs", q.Components)
	}
	if len(d.Library)+len(d.Assets) > q.Files {
		v.fail("/library", "more than %d library files and assets", q.Files)
	}
	if len(d.Tests) > q.Tests {
		v.fail("/tests", "more than %d tests", q.Tests)
	}
	if len(d.Subtasks) > q.Subtasks {
		v.fail("/subtasks", "more than %d subtasks", q.Subtasks)
	}

	var total int64
	text := func(pointer, value string) {
		if !utf8.ValidString(value) {
			v.fail(pointer, "text must be UTF-8")
		}
		if int64(len(value)) > int64(q.FileBytes) {
			v.fail(pointer, "text exceeds %d bytes", int64(q.FileBytes))
		}
		total += int64(len(value))
	}
	library := make(map[string]bool)
	for i, f := range d.Library {
		at := fmt.Sprintf("/library/%d", i)
		if !ValidFileName(f.Name) || library[f.Name] {
			v.fail(at+"/name", "invalid or duplicate file name %q", f.Name)
		}
		text(at+"/content", f.Content)
		if _, ok := syslib.Lookup(f.Name); f.AutoUpdate && (!ok || f.Content != "") {
			v.fail(at+"/auto_update", "only an empty system library file can be auto-updated")
		}
		library[f.Name] = true
	}
	program := func(at string, p DraftProgram) {
		if !languageKey.MatchString(p.Language) {
			v.fail(at+"/language", "invalid language %q", p.Language)
		}
		text(at+"/source", p.Source)
	}
	if d.Checker != nil {
		program("/checker", d.Checker.DraftProgram)
		if _, ok := syslib.LookupChecker(d.Checker.Standard); d.Checker.Standard != "" && (!ok || d.Checker.Source != "") {
			v.fail("/checker/standard", "unknown standard checker %q or a standard checker with a source", d.Checker.Standard)
		}
	}
	if d.Interactor != nil {
		program("/interactor", *d.Interactor)
	}
	for _, list := range []struct {
		role     string
		programs []DraftNamedProgram
	}{{"validators", d.Validators}, {"generators", d.Generators}, {"solutions", d.Solutions}} {
		names := make(map[string]bool)
		for i, p := range list.programs {
			at := fmt.Sprintf("/%s/%d", list.role, i)
			if !ValidFileName(p.Name) || names[p.Name] {
				v.fail(at+"/name", "invalid or duplicate name %q", p.Name)
			}
			names[p.Name] = true
			program(at, p.DraftProgram)
		}
	}
	if len(d.Validators) > 1 {
		v.fail("/validators/1", "a problem has at most one validator")
	}
	if d.MainSolution != nil && d.Solution(*d.MainSolution) == nil {
		v.fail("/main_solution", "unknown solution %q", *d.MainSolution)
	}

	languages := make(map[string]bool)
	for i, s := range d.Statements {
		at := fmt.Sprintf("/statements/%d", i)
		if !statementLanguage.MatchString(s.Language) || languages[s.Language] {
			v.fail(at+"/language", "invalid or duplicate language %q", s.Language)
		}
		languages[s.Language] = true
		for _, name := range StatementSections {
			text(at+"/"+name, *s.Section(name))
		}
	}
	if total > int64(q.TextBytes) {
		v.fail("", "texts exceed %d bytes together", int64(q.TextBytes))
	}
	assets := make(map[string]bool)
	for i, a := range d.Assets {
		at := fmt.Sprintf("/assets/%d", i)
		if !ValidFileName(a.Name) || assets[a.Name] {
			v.fail(at+"/name", "invalid or duplicate asset name %q", a.Name)
		}
		if a.ObjectID == uuid.Nil {
			v.fail(at+"/object_id", "asset has no content")
		}
		assets[a.Name] = true
	}

	d.validateSubtasks(v)
	d.validateTests(v)
}

func (d *Draft) validateSubtasks(v *validation) {
	index := make(map[uuid.UUID]int)
	orders := make(map[int]bool)
	totalPoints := points.Value(0)
	for i, g := range d.Subtasks {
		at := fmt.Sprintf("/subtasks/%d", i)
		if _, ok := index[g.ID]; g.ID == uuid.Nil || ok {
			v.fail(at+"/id", "ID is missing or not unique")
		}
		if g.Ordinal <= 0 || orders[g.Ordinal] {
			v.fail(at+"/ordinal", "ordinal must be positive and unique")
		}
		if g.Points < 0 {
			v.fail(at+"/points", "points must not be negative")
		}
		if g.Policy != "complete" && g.Policy != "each" {
			v.fail(at+"/policy", "invalid policy %q", g.Policy)
		}
		index[g.ID] = i
		orders[g.Ordinal] = true
		totalPoints += g.Points
	}
	state := make(map[uuid.UUID]int)
	var visit func(int) bool
	visit = func(i int) bool {
		g := d.Subtasks[i]
		state[g.ID] = 1
		seen := make(map[uuid.UUID]bool)
		for j, dep := range g.DependsOn {
			at := fmt.Sprintf("/subtasks/%d/depends_on/%d", i, j)
			k, ok := index[dep]
			switch {
			case !ok:
				v.fail(at, "unknown subtask")
			case seen[dep]:
				v.fail(at, "duplicate dependency")
			case state[dep] == 1:
				v.fail(at, "subtask dependencies form a cycle")
				return false
			case state[dep] == 0 && !visit(k):
				return false
			}
			seen[dep] = true
		}
		state[g.ID] = 2
		return true
	}
	for i, g := range d.Subtasks {
		if state[g.ID] == 0 && !visit(i) {
			break
		}
	}
	if d.Type == "scoring" && (d.MaxScore == nil || *d.MaxScore < 0 || *d.MaxScore != totalPoints) {
		v.fail("/max_score", "max score must equal subtask points")
	}
	if d.Type != "scoring" && d.MaxScore != nil {
		v.fail("/max_score", "max score requires scoring type")
	}
}

func (d *Draft) validateTests(v *validation) {
	subtasks := make(map[uuid.UUID]bool)
	for _, g := range d.Subtasks {
		subtasks[g.ID] = true
	}
	orders := make(map[int]bool)
	ids := make(map[uuid.UUID]bool)
	for n, test := range d.Tests {
		at := fmt.Sprintf("/tests/%d", n)
		if test.Points < 0 || test.Points > points.Max {
			v.fail(at+"/points", "invalid test weight")
		}
		if test.ID == uuid.Nil || ids[test.ID] {
			v.fail(at+"/id", "ID is missing or not unique")
		}
		if test.Ordinal <= 0 || orders[test.Ordinal] {
			v.fail(at+"/ordinal", "ordinal must be positive and unique")
		}
		if !subtasks[test.SubtaskID] {
			v.fail(at+"/subtask_id", "unknown subtask")
		}
		ids[test.ID] = true
		orders[test.Ordinal] = true
		i := test.Input
		switch i.Mode {
		case "manual":
			if i.ObjectID == nil || *i.ObjectID == uuid.Nil || i.Generator != nil || len(i.Args) != 0 {
				v.fail(at+"/input", "manual input needs content and no generator")
			}
		case "generator":
			if i.ObjectID != nil {
				v.fail(at+"/input/object_id", "generated input cannot have content")
			}
			if i.Generator == nil || d.Generator(*i.Generator) == nil {
				v.fail(at+"/input/generator", "generator is missing")
			}
		default:
			v.fail(at+"/input/mode", "invalid input mode %q", i.Mode)
		}
		if len(i.Args) > 256 {
			v.fail(at+"/input/args", "too many generator arguments")
		}
		for j, arg := range i.Args {
			if len(arg) > 4096 || strings.ContainsRune(arg, 0) || !utf8.ValidString(arg) {
				v.fail(fmt.Sprintf("%s/input/args/%d", at, j), "invalid generator argument")
			}
		}
		a := test.Answer
		if d.Type == "interactive" {
			if a.Mode != "interactive" || a.ObjectID != nil || a.Solution != nil {
				v.fail(at+"/answer", "interactive test cannot have an ordinary answer")
			}
			continue
		}
		switch a.Mode {
		case "generated":
			if a.ObjectID != nil {
				v.fail(at+"/answer/object_id", "generated answer cannot have content")
			}
			if a.Solution == nil || d.Solution(*a.Solution) == nil {
				v.fail(at+"/answer/solution", "solution is missing")
			}
		case "provided":
			if a.ObjectID == nil || *a.ObjectID == uuid.Nil || a.Solution != nil {
				v.fail(at+"/answer", "provided answer needs content and no solution")
			}
		default:
			v.fail(at+"/answer/mode", "invalid answer mode %q", a.Mode)
		}
	}
}

func (d *Draft) eachProgram(f func(at string, p DraftProgram)) {
	if d.Checker != nil {
		f("/checker", d.Checker.DraftProgram)
	}
	if d.Interactor != nil {
		f("/interactor", *d.Interactor)
	}
	for _, list := range []struct {
		role     string
		programs []DraftNamedProgram
	}{{"validators", d.Validators}, {"generators", d.Generators}, {"solutions", d.Solutions}} {
		for i, p := range list.programs {
			f(fmt.Sprintf("/%s/%d", list.role, i), p.DraftProgram)
		}
	}
}

func (d *Draft) validateImages(v *validation, previous *Draft) {
	assets := make(map[string]bool, len(d.Assets))
	for _, a := range d.Assets {
		assets[a.Name] = true
	}
	for i := range d.Statements {
		s := &d.Statements[i]
		var old *DraftStatement
		if previous != nil {
			old = previous.Statement(s.Language)
		}
		for _, name := range StatementSections {
			body := *s.Section(name)
			if old != nil && *old.Section(name) == body {
				continue
			}
			for _, ref := range ImageReferences(body) {
				if !assets[ref] {
					v.fail(fmt.Sprintf("/statements/%d/%s", i, name), "%s section of the %s statement shows missing asset %q", name, s.Language, ref)
				}
			}
		}
	}
}

// ComputeScores preserves authored test weights without rescaling group totals.
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
