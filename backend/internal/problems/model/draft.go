package problemsmodel

import (
	"cmp"
	"fmt"
	"iter"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/bytesize"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/statement"
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
	Interactor   *DraftInteractor    `json:"interactor"`
	Validators   []DraftValidator    `json:"validators"`
	Generators   []DraftNamedProgram `json:"generators"`
	Solutions    []DraftSolution     `json:"solutions"`
	MainSolution *string             `json:"main_solution"`
	Library      []DraftLibraryFile  `json:"library"`
	Statements   []DraftStatement    `json:"statements"`
	Assets       []DraftAsset        `json:"assets"`
	// Testset is the root group of every test.
	Testset DraftGroup  `json:"testset"`
	Tests   []DraftTest `json:"tests"`
	// Solutions read InputFile and write OutputFile instead of a standard stream when the name is set.
	InputFile  string `json:"input_file"`
	OutputFile string `json:"output_file"`
}

type DraftProgram struct {
	Language string `json:"language"`
	Source   string `json:"source"`
}

// DraftJury is how a checker, interactor or validator is called; Args follow the protocol's own arguments.
type DraftJury struct {
	Protocol sandbox.Protocol `json:"protocol"`
	Args     []string         `json:"args"`
}

func (j DraftJury) Jury(executable sandbox.Executable, language string) sandbox.Jury {
	return sandbox.Jury{Executable: executable, Language: language, Protocol: j.Protocol, Args: j.Args}
}

type DraftInteractor struct {
	DraftProgram
	DraftJury
}

// DraftChecker uses the current system source when Standard names a checker and Source is empty.
type DraftChecker struct {
	DraftProgram
	DraftJury
	Standard string             `json:"standard"`
	Tests    []DraftCheckerTest `json:"tests"`
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

// DraftValidator checks the tests of the listed groups and their subgroups, or every test when the list is empty.
type DraftValidator struct {
	DraftNamedProgram
	DraftJury
	Groups []uuid.UUID          `json:"groups"`
	Tests  []DraftValidatorTest `json:"tests"`
}

// Validates reports whether v checks the tests held directly by group.
func (d *Draft) Validates(v *DraftValidator, group uuid.UUID) bool {
	if len(v.Groups) == 0 {
		return true
	}
	for _, g := range d.GroupPath(group) {
		if slices.Contains(v.Groups, g.ID) {
			return true
		}
	}
	return false
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
	Language string `json:"language"`
	// Empty shows the problem title.
	Title       string `json:"title"`
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

// DraftGroup scores its tests and subgroups; tests name their group by ID. Only the testset root is unnamed.
type DraftGroup struct {
	ID uuid.UUID `json:"id"`
	// Validators get --group with a non-empty name, as Polygon passes it only for grouped tests.
	Name string `json:"name"`
	// Aggregate is a spec.Aggregations entry over the tests and subgroups, taken in the order of their first tests.
	Aggregate string `json:"aggregate"`
	// StopOnFailure leaves the results after the first failed test or subgroup out of the score.
	StopOnFailure bool `json:"stop_on_failure"`
	// Requires lists groups that must pass completely, their own requirements included, for this group to score.
	Requires []uuid.UUID `json:"requires"`
	Feedback string      `json:"feedback"`
	// CheckerArgs follow those of the checker or interactor on the group's own tests; ValidatorArgs those of ICPC validators.
	CheckerArgs   []string     `json:"checker_args"`
	ValidatorArgs []string     `json:"validator_args"`
	Groups        []DraftGroup `json:"groups"`
}

// All yields the group and its subgroups, parents first.
func (g *DraftGroup) All() iter.Seq[*DraftGroup] {
	return func(yield func(*DraftGroup) bool) { g.walk(yield) }
}

func (g *DraftGroup) walk(yield func(*DraftGroup) bool) bool {
	if !yield(g) {
		return false
	}
	for i := range g.Groups {
		if !g.Groups[i].walk(yield) {
			return false
		}
	}
	return true
}

type DraftTest struct {
	Points  points.Value `json:"points"`
	ID      uuid.UUID    `json:"id"`
	Ordinal int          `json:"ordinal"`
	GroupID uuid.UUID    `json:"group_id"`
	Sample  bool         `json:"sample"`
	Pretest bool         `json:"pretest"`
	Input   DraftInput   `json:"input"`
	Answer  DraftAnswer  `json:"answer"`
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
	DiskBytes       bytesize.Size `json:"disk_bytes" yaml:"disk_bytes"`
	FileCount       int           `json:"file_count" yaml:"file_count"`
	ObjectBytes     bytesize.Size `json:"object_bytes" yaml:"object_bytes"`
	TestOutputBytes bytesize.Size `json:"test_output_bytes" yaml:"test_output_bytes"`
	// Every run's wall-clock limit is its CPU limit times this factor.
	WallFactor int                   `json:"wall_factor" yaml:"wall_factor"`
	Compile    sandbox.FileRunLimits `json:"compile" yaml:"compile"`
	Auxiliary  sandbox.FileRunLimits `json:"auxiliary" yaml:"auxiliary"`
	// Typesetting runs in its own sandbox, outside build snapshots.
	Typeset sandbox.FileRunLimits `json:"-" yaml:"typeset"`
}

// WithCPU sets a run's CPU limit and the wall-clock limit that follows from it.
func (l BuildLimits) WithCPU(r sandbox.FileRunLimits, cpu time.Duration) sandbox.FileRunLimits {
	r.CPU, r.Wall = cpu, time.Duration(l.WallFactor)*cpu
	return r
}

// DeriveWalls sets the wall-clock limits of the configured runs.
func (l *BuildLimits) DeriveWalls() {
	for _, r := range []*sandbox.FileRunLimits{&l.Compile, &l.Auxiliary, &l.Typeset} {
		*r = l.WithCPU(*r, r.CPU)
	}
}

// Solution limits a solution's run on a test.
func (l BuildLimits) Solution(timeLimit time.Duration, memory bytesize.Size) sandbox.FileRunLimits {
	r := l.WithCPU(l.Auxiliary, timeLimit)
	r.Memory, r.Output = memory, l.TestOutputBytes
	return r
}

// DraftQuotas counts library files and assets as Files, and programs as Components.
type DraftQuotas struct {
	FileBytes  bytesize.Size `yaml:"file_bytes"`
	TextBytes  bytesize.Size `yaml:"text_bytes"`
	Files      int           `yaml:"files"`
	Tests      int           `yaml:"tests"`
	Components int           `yaml:"components"`
	Groups     int           `yaml:"groups"`
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

// TrimStatements drops trailing whitespace, as the import does, so an exported draft imports back unchanged.
func (d *Draft) TrimStatements() {
	for i := range d.Statements {
		d.Statements[i].Title = strings.TrimSpace(d.Statements[i].Title)
		for _, name := range StatementSections {
			section := d.Statements[i].Section(name)
			*section = strings.TrimRightFunc(*section, unicode.IsSpace)
		}
	}
}

func (d *Draft) Streams() sandbox.Streams {
	return sandbox.Streams{Input: d.InputFile, Output: d.OutputFile}
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

func (d *Draft) Solution(name string) *DraftSolution {
	for i := range d.Solutions {
		if d.Solutions[i].Name == name {
			return &d.Solutions[i]
		}
	}
	return nil
}

func (d *Draft) Group(id uuid.UUID) *DraftGroup {
	path := d.GroupPath(id)
	if len(path) == 0 {
		return nil
	}
	return path[len(path)-1]
}

// GroupPath lists the groups from the root down to the one with id; it is empty for an unknown id.
func (d *Draft) GroupPath(id uuid.UUID) []*DraftGroup {
	var find func(*DraftGroup) []*DraftGroup
	find = func(g *DraftGroup) []*DraftGroup {
		if g.ID == id {
			return []*DraftGroup{g}
		}
		for i := range g.Groups {
			if path := find(&g.Groups[i]); path != nil {
				return append([]*DraftGroup{g}, path...)
			}
		}
		return nil
	}
	return find(&d.Testset)
}

func (d *Draft) ValidatorPrograms() []DraftNamedProgram {
	programs := make([]DraftNamedProgram, len(d.Validators))
	for i := range d.Validators {
		programs[i] = d.Validators[i].DraftNamedProgram
	}
	return programs
}

func (d *Draft) SolutionPrograms() []DraftNamedProgram {
	programs := make([]DraftNamedProgram, len(d.Solutions))
	for i := range d.Solutions {
		programs[i] = d.Solutions[i].DraftNamedProgram
	}
	return programs
}

// Normalize keeps absent lists from serializing as null in the contract.
func (d *Draft) Normalize() {
	empty := func(list *[]DraftNamedProgram) {
		if *list == nil {
			*list = []DraftNamedProgram{}
		}
	}
	if d.Validators == nil {
		d.Validators = []DraftValidator{}
	}
	for i := range d.Validators {
		if d.Validators[i].Groups == nil {
			d.Validators[i].Groups = []uuid.UUID{}
		}
		if d.Validators[i].Tests == nil {
			d.Validators[i].Tests = []DraftValidatorTest{}
		}
	}
	if d.Checker != nil && d.Checker.Tests == nil {
		d.Checker.Tests = []DraftCheckerTest{}
	}
	for _, j := range d.juries() {
		// Drafts saved before protocols existed are testlib.
		if j.Protocol == "" {
			j.Protocol = sandbox.ProtocolTestlib
		}
		if j.Args == nil {
			j.Args = []string{}
		}
	}
	empty(&d.Generators)
	if d.Solutions == nil {
		d.Solutions = []DraftSolution{}
	}
	for i := range d.Solutions {
		if d.Solutions[i].Verdicts == nil {
			d.Solutions[i].Verdicts = []string{}
		}
	}
	if d.Library == nil {
		d.Library = []DraftLibraryFile{}
	}
	if d.Statements == nil {
		d.Statements = []DraftStatement{}
	}
	if d.Assets == nil {
		d.Assets = []DraftAsset{}
	}
	// A new draft's testset takes the problem's ID, so every read of it names the same group.
	if d.Testset.ID == uuid.Nil {
		d.Testset.ID = d.ProblemID
	}
	for g := range d.Testset.All() {
		for _, list := range []*[]string{&g.CheckerArgs, &g.ValidatorArgs} {
			if *list == nil {
				*list = []string{}
			}
		}
		if g.Requires == nil {
			g.Requires = []uuid.UUID{}
		}
		if g.Groups == nil {
			g.Groups = []DraftGroup{}
		}
		g.Aggregate = cmp.Or(g.Aggregate, "sum")
		g.Feedback = cmp.Or(g.Feedback, "complete")
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

// EachObject visits every stored object reference with a description of it; f may replace the ID.
func (d *Draft) EachObject(f func(ref string, id *uuid.UUID)) {
	for i := range d.Tests {
		t := &d.Tests[i]
		if t.Input.ObjectID != nil {
			f(fmt.Sprintf("test %d input", t.Ordinal), t.Input.ObjectID)
		}
		if t.Answer.ObjectID != nil {
			f(fmt.Sprintf("test %d answer", t.Ordinal), t.Answer.ObjectID)
		}
	}
	for i := range d.Assets {
		f(fmt.Sprintf("asset %q", d.Assets[i].Name), &d.Assets[i].ObjectID)
	}
	if d.Checker != nil {
		for i := range d.Checker.Tests {
			t := &d.Checker.Tests[i]
			f(fmt.Sprintf("checker test %d input", i+1), &t.Input)
			f(fmt.Sprintf("checker test %d output", i+1), &t.Output)
			f(fmt.Sprintf("checker test %d answer", i+1), &t.Answer)
		}
	}
	for _, v := range d.Validators {
		for i := range v.Tests {
			f(fmt.Sprintf("validator %s test %d input", v.Name, i+1), &v.Tests[i].Input)
		}
	}
}

func (d *Draft) InputObjects() []uuid.UUID {
	ids := make(map[uuid.UUID]struct{})
	d.EachObject(func(_ string, id *uuid.UUID) { ids[*id] = struct{}{} })
	result := make([]uuid.UUID, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	slices.SortFunc(result, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return result
}

const maxFieldErrors = 50

type validation struct{ errs pkg.FieldErrors }

func (v *validation) fail(pointer string, code *pkg.Error, params map[string]any, format string, args ...any) {
	if len(v.errs) < maxFieldErrors {
		v.errs = append(v.errs, pkg.FieldError{Pointer: pointer, Code: code.Code, Params: params, Message: fmt.Sprintf(format, args...)})
	}
}

func value(x any) map[string]any { return map[string]any{"value": x} }

// with names the field that the failing one contradicts.
func with(pointer string) map[string]any { return map[string]any{"with": pointer} }

func maximum(n any) map[string]any { return map[string]any{"max": n} }

func positive() map[string]any { return map[string]any{"min": 1} }

// name reports an invalid or repeated file or program name.
func (v *validation) name(pointer, name string, valid bool, seen map[string]bool) {
	switch {
	case !valid:
		v.fail(pointer, pkg.ErrFormat, nil, "invalid name %q", name)
	case seen[name]:
		v.fail(pointer, pkg.ErrDuplicate, value(name), "duplicate name %q", name)
	}
	seen[name] = true
}

func (v *validation) title(pointer, title string) {
	if !utf8.ValidString(title) {
		v.fail(pointer, pkg.ErrNotUTF8, nil, "title must be UTF-8")
	}
	if len(title) > 1024 {
		v.fail(pointer, pkg.ErrLength, maximum(1024), "title exceeds 1024 bytes")
	}
}

// identity checks the ID and ordinal of a test.
func (v *validation) identity(at string, id uuid.UUID, seenID bool, ordinal int, seenOrdinal bool) {
	switch {
	case id == uuid.Nil:
		v.fail(at+"/id", pkg.ErrRequired, nil, "ID is missing")
	case seenID:
		v.fail(at+"/id", pkg.ErrDuplicate, nil, "ID is not unique")
	}
	switch {
	case ordinal <= 0:
		v.fail(at+"/ordinal", pkg.ErrRange, positive(), "ordinal must be positive")
	case seenOrdinal:
		v.fail(at+"/ordinal", pkg.ErrDuplicate, value(ordinal), "ordinal is not unique")
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
			v.fail(at+"/language", pkg.ErrUnknown, value(p.Language), "language %q is unavailable in the toolchain", p.Language)
		}
	})
	if d.Checker != nil && d.Checker.Standard != "" && !slices.Contains(languages[d.Checker.Language].Extensions, "cpp") {
		v.fail("/checker/language", pkg.ErrInconsistent, with("/checker/standard"), "a standard checker compiles as C++")
	}
	if d.Type != "interactive" && d.Checker == nil {
		v.fail("/checker", pkg.ErrRequired, nil, "checker is required")
	}
	if d.Type == "interactive" && d.Interactor == nil {
		v.fail("/interactor", pkg.ErrRequired, nil, "interactor is required")
	}
	if d.Type == "interactive" && d.MainSolution == nil {
		v.fail("/main_solution", pkg.ErrRequired, nil, "interactive problem requires a main solution")
	}
	if strings.TrimSpace(d.Title) == "" {
		v.fail("/title", pkg.ErrRequired, nil, "title is required")
	}
	if len(d.Tests) == 0 {
		v.fail("/tests", pkg.ErrRequired, nil, "at least one test is required")
	}
	return v.err()
}

func (d *Draft) validate(v *validation, q DraftQuotas) {
	if d.ProblemID == uuid.Nil {
		v.fail("/problem_id", pkg.ErrRequired, nil, "problem ID is required")
	}
	if d.Revision < 1 {
		v.fail("/revision", pkg.ErrRange, positive(), "revision must be positive")
	}
	v.title("/title", d.Title)
	if d.ShortName == "" {
		v.fail("/short_name", pkg.ErrRequired, nil, "short name is required")
	}
	if len(d.ShortName) > 128 {
		v.fail("/short_name", pkg.ErrLength, maximum(128), "short name exceeds 128 bytes")
	}
	if d.Type != "pass-fail" && d.Type != "scoring" && d.Type != "interactive" {
		v.fail("/type", pkg.ErrUnknown, value(d.Type), "invalid problem type %q", d.Type)
	}
	if d.TimeLimitMs <= 0 {
		v.fail("/time_limit_ms", pkg.ErrRange, positive(), "time limit must be positive")
	}
	if d.MemoryLimitMb <= 0 {
		v.fail("/memory_limit_mb", pkg.ErrRange, positive(), "memory limit must be positive")
	}
	programs := len(d.Validators) + len(d.Generators) + len(d.Solutions)
	if d.Checker != nil {
		programs++
	}
	if d.Interactor != nil {
		programs++
	}
	if programs > q.Components {
		v.fail("", pkg.ErrTooMany, maximum(q.Components), "more than %d programs", q.Components)
	}
	if len(d.Library)+len(d.Assets) > q.Files {
		v.fail("/library", pkg.ErrTooMany, maximum(q.Files), "more than %d library files and assets", q.Files)
	}
	if len(d.Tests) > q.Tests {
		v.fail("/tests", pkg.ErrTooMany, maximum(q.Tests), "more than %d tests", q.Tests)
	}

	var total int64
	text := func(pointer, content string) {
		if !utf8.ValidString(content) {
			v.fail(pointer, pkg.ErrNotUTF8, nil, "text must be UTF-8")
		}
		if int64(len(content)) > int64(q.FileBytes) {
			v.fail(pointer, pkg.ErrTooLarge, maximum(int64(q.FileBytes)), "text exceeds %d bytes", int64(q.FileBytes))
		}
		total += int64(len(content))
	}
	library := make(map[string]bool)
	for i, f := range d.Library {
		at := fmt.Sprintf("/library/%d", i)
		v.name(at+"/name", f.Name, ValidFileName(f.Name), library)
		text(at+"/content", f.Content)
		if _, ok := syslib.Lookup(f.Name); f.AutoUpdate && (!ok || f.Content != "") {
			v.fail(at+"/auto_update", pkg.ErrInconsistent, with(at+"/content"), "only an empty system library file can be auto-updated")
		}
	}
	program := func(at string, p DraftProgram) {
		if !languageKey.MatchString(p.Language) {
			v.fail(at+"/language", pkg.ErrUnknown, value(p.Language), "invalid language %q", p.Language)
		}
		text(at+"/source", p.Source)
	}
	if d.Checker != nil {
		program("/checker", d.Checker.DraftProgram)
		_, ok := syslib.LookupChecker(d.Checker.Standard)
		switch {
		case d.Checker.Standard == "":
		case !ok:
			v.fail("/checker/standard", pkg.ErrUnknown, value(d.Checker.Standard), "unknown standard checker %q", d.Checker.Standard)
		case d.Checker.Source != "":
			v.fail("/checker/standard", pkg.ErrInconsistent, with("/checker/source"), "a standard checker has no source")
		}
	}
	if d.Interactor != nil {
		program("/interactor", d.Interactor.DraftProgram)
	}
	for at, j := range d.juries() {
		switch {
		case j.Protocol != sandbox.ProtocolTestlib && j.Protocol != sandbox.ProtocolICPC:
			v.fail(at+"/protocol", pkg.ErrUnknown, value(string(j.Protocol)), "invalid protocol %q", j.Protocol)
		case j.Protocol == sandbox.ProtocolTestlib && len(j.Args) > 0:
			// testlib reads arguments after its own files as a result file and flags.
			v.fail(at+"/args", pkg.ErrInconsistent, with(at+"/protocol"), "only ICPC programs take arguments")
		}
	}
	if d.Checker != nil && d.Checker.Standard != "" && d.Checker.Protocol != sandbox.ProtocolTestlib {
		v.fail("/checker/protocol", pkg.ErrInconsistent, with("/checker/standard"), "a standard checker uses testlib")
	}
	for _, list := range []struct {
		role     string
		programs []DraftNamedProgram
	}{{"validators", d.ValidatorPrograms()}, {"generators", d.Generators}, {"solutions", d.SolutionPrograms()}} {
		names := make(map[string]bool)
		for i, p := range list.programs {
			at := fmt.Sprintf("/%s/%d", list.role, i)
			v.name(at+"/name", p.Name, ValidFileName(p.Name), names)
			program(at, p.DraftProgram)
		}
	}
	for i, validator := range d.Validators {
		seen := make(map[uuid.UUID]bool)
		for j, id := range validator.Groups {
			at := fmt.Sprintf("/validators/%d/groups/%d", i, j)
			switch {
			case d.Group(id) == nil:
				v.fail(at, pkg.ErrUnknown, nil, "unknown group")
			case id == d.Testset.ID:
				v.fail(at, pkg.ErrInconsistent, nil, "an empty list checks the testset")
			case seen[id]:
				v.fail(at, pkg.ErrDuplicate, nil, "duplicate group")
			}
			seen[id] = true
		}
	}
	for i, s := range d.Solutions {
		at := fmt.Sprintf("/solutions/%d", i)
		seen := make(map[string]bool)
		for j, verdict := range s.Verdicts {
			pointer := fmt.Sprintf("%s/verdicts/%d", at, j)
			switch {
			case !slices.Contains(SolutionVerdicts, verdict):
				v.fail(pointer, pkg.ErrUnknown, value(verdict), "unknown verdict %q", verdict)
			case seen[verdict]:
				v.fail(pointer, pkg.ErrDuplicate, value(verdict), "duplicate verdict %q", verdict)
			}
			seen[verdict] = true
		}
		if s.Points != nil {
			switch {
			case d.Type != "scoring":
				v.fail(at+"/points", pkg.ErrInconsistent, with("/type"), "expected points need a scoring problem")
			case len(s.Verdicts) == 0:
				v.fail(at+"/points", pkg.ErrInconsistent, with(at+"/verdicts"), "expected points need a solution that runs")
			case *s.Points < 0 || *s.Points > points.Max:
				v.fail(at+"/points", pkg.ErrRange, map[string]any{"min": 0, "max": points.Max}, "expected points are out of range")
			}
		}
	}
	if d.MainSolution != nil && d.Solution(*d.MainSolution) == nil {
		v.fail("/main_solution", pkg.ErrUnknown, value(*d.MainSolution), "unknown solution %q", *d.MainSolution)
	}
	for pointer, name := range map[string]string{"/input_file": d.InputFile, "/output_file": d.OutputFile} {
		switch {
		case name == "":
		case !ValidFileName(name) || !sandbox.ValidSolutionFile(name):
			v.fail(pointer, pkg.ErrFormat, nil, "invalid file name %q", name)
		case d.Type == "interactive":
			v.fail(pointer, pkg.ErrInconsistent, with("/type"), "an interactive problem has no file I/O")
		}
	}
	if d.InputFile != "" && d.InputFile == d.OutputFile {
		v.fail("/output_file", pkg.ErrInconsistent, with("/input_file"), "input and output files must differ")
	}

	languages := make(map[string]bool)
	for i, s := range d.Statements {
		at := fmt.Sprintf("/statements/%d", i)
		v.name(at+"/language", s.Language, statementLanguage.MatchString(s.Language), languages)
		v.title(at+"/title", s.Title)
		for _, name := range StatementSections {
			text(at+"/"+name, *s.Section(name))
		}
	}
	if total > int64(q.TextBytes) {
		v.fail("", pkg.ErrTooLarge, maximum(int64(q.TextBytes)), "texts exceed %d bytes together", int64(q.TextBytes))
	}
	assets := make(map[string]bool)
	for i, a := range d.Assets {
		at := fmt.Sprintf("/assets/%d", i)
		v.name(at+"/name", a.Name, ValidFileName(a.Name), assets)
		if a.ObjectID == uuid.Nil {
			v.fail(at+"/object_id", pkg.ErrRequired, nil, "asset has no content")
		}
	}

	d.validateGroups(v, q)
	d.validateTests(v)
	d.validateProgramTests(v, q)
}

// validateGroups checks the testset tree; requirements may not point to the group itself, its ancestors or descendants, or form a cycle.
func (d *Draft) validateGroups(v *validation, q DraftQuotas) {
	type node struct {
		at    string
		group *DraftGroup
		path  []uuid.UUID
	}
	nodes := map[uuid.UUID]node{}
	names := map[string]bool{}
	var order []uuid.UUID
	var visit func(g *DraftGroup, at string, path []uuid.UUID)
	visit = func(g *DraftGroup, at string, path []uuid.UUID) {
		_, seen := nodes[g.ID]
		switch {
		case g.ID == uuid.Nil:
			v.fail(at+"/id", pkg.ErrRequired, nil, "ID is missing")
		case seen:
			v.fail(at+"/id", pkg.ErrDuplicate, nil, "ID is not unique")
		}
		root := len(path) == 0
		switch {
		case root && g.Name != "":
			v.fail(at+"/name", pkg.ErrInconsistent, nil, "the testset is unnamed")
		case !root && g.Name == "":
			v.fail(at+"/name", pkg.ErrRequired, nil, "group name is required")
		case names[g.Name]:
			// Names are gfmt group names and validator --group values, so an export keeps them exact.
			v.fail(at+"/name", pkg.ErrDuplicate, value(g.Name), "duplicate group name %q", g.Name)
		}
		names[g.Name] = true
		if !slices.Contains(spec.Aggregations, g.Aggregate) {
			v.fail(at+"/aggregate", pkg.ErrUnknown, value(g.Aggregate), "invalid aggregation %q", g.Aggregate)
		}
		if !slices.Contains(spec.FeedbackPolicies, g.Feedback) {
			v.fail(at+"/feedback", pkg.ErrUnknown, value(g.Feedback), "invalid feedback policy %q", g.Feedback)
		}
		d.validateGroupArgs(v, g, at)
		path = append(slices.Clone(path), g.ID)
		if !seen {
			nodes[g.ID] = node{at: at, group: g, path: path}
			order = append(order, g.ID)
		}
		for i := range g.Groups {
			visit(&g.Groups[i], fmt.Sprintf("%s/groups/%d", at, i), path)
		}
	}
	visit(&d.Testset, "/testset", nil)
	if len(order) > q.Groups {
		v.fail("/testset", pkg.ErrTooMany, maximum(q.Groups), "more than %d groups", q.Groups)
	}
	state := map[uuid.UUID]int{}
	var requires func(uuid.UUID) bool
	requires = func(id uuid.UUID) bool {
		n := nodes[id]
		state[id] = 1
		seen := map[uuid.UUID]bool{}
		for j, required := range n.group.Requires {
			at := fmt.Sprintf("%s/requires/%d", n.at, j)
			other, ok := nodes[required]
			switch {
			case !ok:
				v.fail(at, pkg.ErrUnknown, nil, "unknown group")
			case seen[required]:
				v.fail(at, pkg.ErrDuplicate, nil, "duplicate requirement")
			case slices.Contains(n.path, required) || slices.Contains(other.path, id):
				v.fail(at, pkg.ErrInconsistent, nil, "a group cannot require itself, its ancestor or its subgroup")
			case state[required] == 1:
				v.fail(at, pkg.ErrCycle, nil, "group requirements form a cycle")
				return false
			case state[required] == 0 && !requires(required):
				return false
			}
			seen[required] = true
		}
		state[id] = 2
		return true
	}
	for _, id := range order {
		if state[id] == 0 && !requires(id) {
			break
		}
	}
	max, err := d.maxScore()
	switch {
	case err != nil:
		v.fail("/tests", pkg.ErrRange, maximum(points.Max), "test points exceed the score limit")
	case d.Type == "scoring" && (d.MaxScore == nil || *d.MaxScore != max):
		v.fail("/max_score", pkg.ErrInconsistent, with("/testset"), "max score must equal the testset score")
	case d.Type != "scoring" && d.MaxScore != nil:
		v.fail("/max_score", pkg.ErrInconsistent, with("/type"), "max score requires scoring type")
	}
}

// validateGroupArgs allows arguments only where the programs that take them speak the ICPC protocol.
func (d *Draft) validateGroupArgs(v *validation, g *DraftGroup, at string) {
	for name, args := range map[string][]string{"checker_args": g.CheckerArgs, "validator_args": g.ValidatorArgs} {
		for j, arg := range args {
			if len(arg) > 4096 || strings.ContainsRune(arg, 0) || !utf8.ValidString(arg) {
				v.fail(fmt.Sprintf("%s/%s/%d", at, name, j), pkg.ErrFormat, nil, "invalid argument")
			}
		}
	}
	if len(g.CheckerArgs) > 0 {
		for pointer, j := range d.juries() {
			if (pointer == "/checker" || pointer == "/interactor") && j.Protocol != sandbox.ProtocolICPC {
				v.fail(at+"/checker_args", pkg.ErrInconsistent, with(pointer+"/protocol"), "only ICPC programs take arguments")
			}
		}
	}
	if len(g.ValidatorArgs) > 0 {
		for i := range d.Validators {
			if d.Validators[i].Protocol != sandbox.ProtocolICPC && d.Validates(&d.Validators[i], g.ID) {
				v.fail(at+"/validator_args", pkg.ErrInconsistent, with(fmt.Sprintf("/validators/%d/protocol", i)), "only ICPC programs take arguments")
			}
		}
	}
}

func (d *Draft) validateTests(v *validation) {
	groups := make(map[uuid.UUID]bool)
	for g := range d.Testset.All() {
		groups[g.ID] = true
	}
	orders := make(map[int]bool)
	ids := make(map[uuid.UUID]bool)
	for n, test := range d.Tests {
		at := fmt.Sprintf("/tests/%d", n)
		if test.Points < 0 || test.Points > points.Max {
			v.fail(at+"/points", pkg.ErrRange, map[string]any{"min": 0, "max": points.Max}, "invalid test weight")
		}
		v.identity(at, test.ID, ids[test.ID], test.Ordinal, orders[test.Ordinal])
		if !groups[test.GroupID] {
			v.fail(at+"/group_id", pkg.ErrUnknown, nil, "unknown group")
		}
		ids[test.ID] = true
		orders[test.Ordinal] = true
		i := test.Input
		switch i.Mode {
		case "manual":
			if i.ObjectID == nil || *i.ObjectID == uuid.Nil || i.Generator != nil || len(i.Args) != 0 {
				v.fail(at+"/input", pkg.ErrInconsistent, with(at+"/input/mode"), "manual input needs content and no generator")
			}
		case "generator":
			if i.ObjectID != nil {
				v.fail(at+"/input/object_id", pkg.ErrInconsistent, with(at+"/input/mode"), "generated input cannot have content")
			}
			switch {
			case i.Generator == nil:
				v.fail(at+"/input/generator", pkg.ErrRequired, nil, "generator is missing")
			case d.Generator(*i.Generator) == nil:
				v.fail(at+"/input/generator", pkg.ErrUnknown, value(*i.Generator), "unknown generator %q", *i.Generator)
			}
		default:
			v.fail(at+"/input/mode", pkg.ErrUnknown, value(i.Mode), "invalid input mode %q", i.Mode)
		}
		if len(i.Args) > 256 {
			v.fail(at+"/input/args", pkg.ErrTooMany, maximum(256), "too many generator arguments")
		}
		for j, arg := range i.Args {
			if len(arg) > 4096 || strings.ContainsRune(arg, 0) || !utf8.ValidString(arg) {
				v.fail(fmt.Sprintf("%s/input/args/%d", at, j), pkg.ErrFormat, nil, "invalid generator argument")
			}
		}
		a := test.Answer
		if d.Type == "interactive" && a.Mode != "provided" {
			if a.Mode != "interactive" || a.ObjectID != nil || a.Solution != nil {
				v.fail(at+"/answer", pkg.ErrInconsistent, with("/type"), "interactive test answer is the interactor's output or provided")
			}
			continue
		}
		switch a.Mode {
		case "generated":
			if a.ObjectID != nil {
				v.fail(at+"/answer/object_id", pkg.ErrInconsistent, with(at+"/answer/mode"), "generated answer cannot have content")
			}
			switch {
			case a.Solution == nil:
				v.fail(at+"/answer/solution", pkg.ErrRequired, nil, "solution is missing")
			case d.Solution(*a.Solution) == nil:
				v.fail(at+"/answer/solution", pkg.ErrUnknown, value(*a.Solution), "unknown solution %q", *a.Solution)
			}
		case "provided":
			if a.ObjectID == nil || *a.ObjectID == uuid.Nil || a.Solution != nil {
				v.fail(at+"/answer", pkg.ErrInconsistent, with(at+"/answer/mode"), "provided answer needs content and no solution")
			}
		default:
			v.fail(at+"/answer/mode", pkg.ErrUnknown, value(a.Mode), "invalid answer mode %q", a.Mode)
		}
	}
}

// juries yields the JSON pointers of the checker, interactor and validators with how they are called.
func (d *Draft) juries() iter.Seq2[string, *DraftJury] {
	return func(yield func(string, *DraftJury) bool) {
		if d.Checker != nil && !yield("/checker", &d.Checker.DraftJury) {
			return
		}
		if d.Interactor != nil && !yield("/interactor", &d.Interactor.DraftJury) {
			return
		}
		for i := range d.Validators {
			if !yield(fmt.Sprintf("/validators/%d", i), &d.Validators[i].DraftJury) {
				return
			}
		}
	}
}

func (d *Draft) eachProgram(f func(at string, p DraftProgram)) {
	if d.Checker != nil {
		f("/checker", d.Checker.DraftProgram)
	}
	if d.Interactor != nil {
		f("/interactor", d.Interactor.DraftProgram)
	}
	for _, list := range []struct {
		role     string
		programs []DraftNamedProgram
	}{{"validators", d.ValidatorPrograms()}, {"generators", d.Generators}, {"solutions", d.SolutionPrograms()}} {
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
				if _, ok := statement.ImageFile(ref, func(name string) bool { return assets[name] }); !ok {
					v.fail(fmt.Sprintf("/statements/%d/%s", i, name), pkg.ErrUnknown, value(ref), "%s section of the %s statement shows missing asset %q", name, s.Language, ref)
				}
			}
		}
	}
}

// ComputeScores sets the max score from the testset.
func (d *Draft) ComputeScores() error {
	max, err := d.maxScore()
	if err != nil {
		return err
	}
	d.MaxScore = nil
	if d.Type == "scoring" {
		d.MaxScore = &max
	}
	return nil
}

// maxScore aggregates test points up the testset, leaving out groups without tests as grading does.
func (d *Draft) maxScore() (points.Value, error) {
	tests := map[uuid.UUID][]points.Value{}
	for _, t := range d.Tests {
		if t.Points < 0 || t.Points > points.Max {
			return 0, fmt.Errorf("test weights exceed score limit")
		}
		tests[t.GroupID] = append(tests[t.GroupID], t.Points)
	}
	var grade func(*DraftGroup) (points.Value, bool)
	grade = func(g *DraftGroup) (points.Value, bool) {
		scores := slices.Clone(tests[g.ID])
		for i := range g.Groups {
			if score, ok := grade(&g.Groups[i]); ok {
				scores = append(scores, score)
			}
		}
		return spec.Aggregate(g.Aggregate, scores), len(scores) > 0
	}
	var total points.Value
	for _, t := range d.Tests {
		if total > points.Max-t.Points {
			return 0, fmt.Errorf("test weights exceed score limit")
		}
		total += t.Points
	}
	max, _ := grade(&d.Testset)
	return max, nil
}
