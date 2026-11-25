package icpc

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/points"
	"gopkg.in/yaml.v3"
)

// CurrentProblem is the problem.yaml of version 2025-09; other keys describe the problem without changing how it is judged.
type CurrentProblem struct {
	Type   list  `yaml:"type"`
	Name   names `yaml:"name"`
	Limits struct {
		TimeLimit float64 `yaml:"time_limit"` // in seconds
		Memory    int     `yaml:"memory"`     // in MiB
	} `yaml:"limits"`
}

// list is a string or a sequence of strings.
type list []string

func (l *list) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*l = list{node.Value}
		return nil
	}
	if err := node.Decode((*[]string)(l)); err != nil {
		return fmt.Errorf("decode list: %w", err)
	}
	return nil
}

// names maps languages to problem names; a string is the English name.
type names map[string]string

func (n *names) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*n = names{"en": node.Value}
		return nil
	}
	if err := node.Decode((*map[string]string)(n)); err != nil {
		return fmt.Errorf("decode names: %w", err)
	}
	return nil
}

func readCurrent(fsys fs.FS, data []byte) (*spec.Problem, error) {
	var prob CurrentProblem
	if err := yaml.Unmarshal(data, &prob); err != nil {
		return nil, spec.Syntax("problem.yaml", err)
	}
	p := &spec.Problem{Type: "standard", Grading: "pass-fail", TimeLimitMs: 1000, MemoryLimitMb: 256}
	unsupported := func(file, value, format string, args ...any) error {
		return spec.Fail(pkg.ErrUnsupported, file, value, format, args...)
	}
	for _, t := range prob.Type {
		switch t {
		case "pass-fail":
		case "scoring":
			p.Grading = spec.GradingPoints
		case "interactive":
			p.Type = "interactive"
		case "submit-answer":
			p.Type = "output-only"
		default:
			return nil, unsupported("problem.yaml", t, "%s problems are not supported", t)
		}
	}
	if slices.Contains(prob.Type, "interactive") && slices.Contains(prob.Type, "submit-answer") {
		return nil, unsupported("problem.yaml", "submit-answer", "submit-answer problems cannot be interactive")
	}
	if prob.Limits.TimeLimit > 0 {
		p.TimeLimitMs = int(prob.Limits.TimeLimit * 1000)
	}
	if prob.Limits.Memory > 0 {
		p.MemoryLimitMb = prob.Limits.Memory
	}
	for dir, feature := range map[string]string{"include": "included files", "static_validator": "static validators"} {
		if nonEmpty(fsys, dir) {
			return nil, unsupported(dir, "", "%s are not supported", feature)
		}
	}
	collect := func(folder string) ([]spec.Program, error) {
		programs, headers, err := programs(fsys, folder)
		p.Library = append(p.Library, headers...)
		return programs, err
	}
	var err error
	if p.Validators, err = collect("input_validators"); err != nil {
		return nil, err
	}
	output, err := outputValidator(fsys, p)
	if err != nil {
		return nil, err
	}
	interactive := p.Type == "interactive"
	switch {
	case output == nil && interactive:
		return nil, spec.Fail(pkg.ErrPackageLayout, "output_validator", "", "an interactive problem needs an output validator")
	case output == nil:
		output = &spec.Program{File: spec.File{Name: "default_validator.cpp"}, Dependencies: map[string]string{}, Text: defaultValidator}
	}
	if interactive {
		p.Interactor = output
	} else {
		p.Checker = output
	}
	for _, program := range append([]*spec.Program{output}, pointers(p.Validators)...) {
		if program.Protocol, err = protocol(fsys, *program); err != nil {
			return nil, err
		}
	}
	scoring := p.Grading == spec.GradingPoints
	if p.Solutions, p.MainSolution, err = submissions(fsys, collect, scoring); err != nil {
		return nil, err
	}
	tree := currentData{fsys: fsys, scoring: scoring}
	if p.Testset, err = tree.read(); err != nil {
		return nil, err
	}
	p.Statements = statements(fsys, p)
	p.Title = prob.Name["en"]
	if p.Title == "" && len(prob.Name) > 0 {
		p.Title = prob.Name[slices.Sorted(maps.Keys(prob.Name))[0]]
	}
	for i, s := range p.Statements {
		if title := prob.Name[s.Language]; title != p.Title {
			p.Statements[i].Title = title
		}
	}
	p.Attachments, err = attachments(fsys)
	return p, err
}

func pointers(programs []spec.Program) []*spec.Program {
	var result []*spec.Program
	for i := range programs {
		result = append(result, &programs[i])
	}
	return result
}

// nonEmpty ignores dotfiles such as .gitkeep.
func nonEmpty(fsys fs.FS, dir string) bool {
	entries, _ := fs.ReadDir(fsys, dir)
	return slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return !strings.HasPrefix(e.Name(), ".") })
}

// outputValidator reads output_validator/, which is one program.
func outputValidator(fsys fs.FS, p *spec.Problem) (*spec.Program, error) {
	const dir = "output_validator"
	if !nonEmpty(fsys, dir) {
		return nil, nil
	}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	if slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return e.Name() == "build" || e.Name() == "run" }) {
		return nil, spec.Fail(pkg.ErrUnsupported, dir, "", "%s: build and run scripts are not supported", dir)
	}
	sources, headers := split(dir, entries)
	if len(sources) != 1 {
		return nil, spec.Fail(pkg.ErrPackageLayout, dir, "", "%s must hold one source file, found %d", dir, len(sources))
	}
	p.Library = append(p.Library, headers...)
	result := program(sources[0], headers)
	return &result, nil
}

// Files in attachments/ itself go to participants; code templates in its subdirectories are not imported.
func attachments(fsys fs.FS) ([]spec.File, error) {
	entries, err := fs.ReadDir(fsys, "attachments")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read attachments: %w", err)
	}
	var result []spec.File
	for _, e := range entries {
		if e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".") {
			result = append(result, spec.File{Name: e.Name(), Path: path.Join("attachments", e.Name())})
		}
	}
	return result, nil
}

// GroupConfig is a test_group.yaml, called testdata.yaml in the 2023-07 draft.
type GroupConfig struct {
	MaxScore              any  `yaml:"max_score"`
	ScoreAggregation      string `yaml:"score_aggregation"`
	ScoreMode             string `yaml:"score_mode"`
	StaticValidationScore any  `yaml:"static_validation_score"`
	RequirePass           list `yaml:"require_pass"`
	Args                  any  `yaml:"args"`
	InputValidatorArgs    any  `yaml:"input_validator_args"`
	OutputValidatorArgs   any  `yaml:"output_validator_args"`
	StaticValidatorArgs   any  `yaml:"static_validator_args"`
	InputVisualizerArgs   any  `yaml:"input_visualizer_args"`
	OutputVisualizerArgs  any  `yaml:"output_visualizer_args"`
	FullFeedback          any  `yaml:"full_feedback"`
	InputValidatorFlags   any  `yaml:"input_validator_flags"`
	OutputValidatorFlags  any  `yaml:"output_validator_flags"`
}

// TestConfig is the .yaml of one test case.
type TestConfig struct {
	Args                 any    `yaml:"args"`
	InputValidatorArgs   any    `yaml:"input_validator_args"`
	OutputValidatorArgs  any    `yaml:"output_validator_args"`
	InputVisualizerArgs  any    `yaml:"input_visualizer_args"`
	OutputVisualizerArgs any    `yaml:"output_visualizer_args"`
	FullFeedback         any    `yaml:"full_feedback"`
	Hint                 string `yaml:"hint"`
	Description          string `yaml:"description"`
}

// strict decodes YAML rejecting unknown keys, since an unknown scoring key would silently change scores.
func strict(fsys fs.FS, name string, v any) error {
	data, err := spec.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return spec.Syntax(name, err)
	}
	return nil
}

// arguments are the validator arguments of a group's own tests; groups do not inherit them.
type arguments struct{ input, output []string }

// currentData reads data/sample and data/secret; validation test data is not imported.
type currentData struct {
	fsys    fs.FS
	scoring bool
	count   int
}

func (d *currentData) config(dir string) (*GroupConfig, arguments, error) {
	var args arguments
	name := path.Join(dir, "test_group.yaml")
	if !regular(d.fsys, name) {
		name = path.Join(dir, "testdata.yaml")
	}
	if !regular(d.fsys, name) {
		return nil, args, nil
	}
	var c GroupConfig
	if err := strict(d.fsys, name, &c); err != nil {
		return nil, args, err
	}
	unsupported := func(key string) error {
		return spec.Fail(pkg.ErrUnsupported, name, key, "%s: %s is not supported", name, key)
	}
	switch {
	case c.Args != nil:
		return nil, args, unsupported("args")
	case c.StaticValidationScore != nil:
		return nil, args, unsupported("static_validation_score")
	case c.ScoreMode != "" && c.ScoreMode != "none":
		return nil, args, unsupported("score_mode " + c.ScoreMode)
	}
	var err error
	for _, a := range []struct {
		key    string
		values []any
		target *[]string
	}{{"input_validator_args", []any{c.InputValidatorArgs, c.InputValidatorFlags}, &args.input}, {"output_validator_args", []any{c.OutputValidatorArgs, c.OutputValidatorFlags}, &args.output}} {
		for _, v := range a.values {
			if v == nil {
				continue
			}
			if *a.target, err = flags(name, a.key, v); err != nil {
				return nil, args, err
			}
		}
	}
	return &c, args, nil
}

func (d *currentData) read() (spec.Group, error) {
	var root spec.Group
	if d.scoring {
		root.Aggregate = "sum"
	}
	_, args, err := d.config("data/sample")
	if err != nil {
		return root, err
	}
	sample := spec.Group{Name: "sample", ValidatorArgs: args.input, CheckerArgs: args.output}
	if sample.Tests, err = d.tests("data/sample", true); err != nil {
		return root, err
	}
	if d.scoring {
		sample.Aggregate = "sum"
	}
	if len(sample.Tests) > 0 {
		root.Groups = append(root.Groups, sample)
	}
	secret, err := d.secret()
	if err != nil {
		return root, err
	}
	root.Groups = append(root.Groups, secret)
	if len(sample.Tests) == 0 {
		for g := range root.All() {
			g.Requires = slices.DeleteFunc(g.Requires, func(name string) bool { return name == "sample" })
		}
	}
	return root, nil
}

// secret maps the ICPC scores, of whole numbers, onto Gate's group tree.
func (d *currentData) secret() (spec.Group, error) {
	const dir = "data/secret"
	config, args, err := d.config(dir)
	if err != nil {
		return spec.Group{}, err
	}
	secret := spec.Group{Name: "secret", ValidatorArgs: args.input, CheckerArgs: args.output}
	entries, err := fs.ReadDir(d.fsys, dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return secret, fmt.Errorf("read %s: %w", dir, err)
	}
	var groups []string
	for _, e := range entries {
		if e.IsDir() && (regular(d.fsys, path.Join(dir, e.Name(), "test_group.yaml")) || regular(d.fsys, path.Join(dir, e.Name(), "testdata.yaml"))) {
			groups = append(groups, e.Name())
		}
	}
	maxScore, aggregation, err := d.scoreConfig(dir, config, 100, "sum")
	if err != nil {
		return secret, err
	}
	if len(groups) == 0 {
		if secret.Tests, err = d.tests(dir, false); err != nil {
			return secret, err
		}
		d.score(&secret, aggregation, maxScore)
		return secret, d.requires(&secret, config)
	}
	if slices.ContainsFunc(entries, func(e fs.DirEntry) bool { return !e.IsDir() && strings.HasSuffix(e.Name(), ".in") }) {
		return secret, spec.Fail(pkg.ErrPackageLayout, dir, "", "%s holds both test cases and test data groups", dir)
	}
	if d.scoring {
		secret.Aggregate = "sum"
		if aggregation != "sum" {
			secret.Aggregate, secret.StopOnFailure = "min", aggregation == "pass-fail"
		}
	}
	for _, name := range groups {
		full := path.Join(dir, name)
		config, args, err := d.config(full)
		if err != nil {
			return secret, err
		}
		group := spec.Group{Name: "secret/" + name, ValidatorArgs: args.input, CheckerArgs: args.output}
		if group.Tests, err = d.tests(full, false); err != nil {
			return secret, err
		}
		groupMax, groupAggregation, err := d.scoreConfig(full, config, -1, "pass-fail")
		if err != nil {
			return secret, err
		}
		// When secret is all or nothing, so is every group, each worth the whole secret.
		if aggregation == "pass-fail" {
			groupMax, groupAggregation = maxScore, "pass-fail"
		}
		d.score(&group, groupAggregation, groupMax)
		if err := d.requires(&group, config); err != nil {
			return secret, err
		}
		secret.Groups = append(secret.Groups, group)
	}
	return secret, d.requires(&secret, config)
}

// scoreConfig reads max_score and score_aggregation; a negative max is unbounded.
func (d *currentData) scoreConfig(dir string, c *GroupConfig, maxScore int, aggregation string) (int, string, error) {
	if !d.scoring || c == nil {
		return maxScore, aggregation, d.bounded(dir, maxScore)
	}
	if c.ScoreAggregation != "" {
		aggregation = c.ScoreAggregation
	}
	switch v := c.MaxScore.(type) {
	case nil:
	case int:
		maxScore = v
	case string:
		if v != "unbounded" {
			return 0, "", spec.Fail(pkg.ErrFormat, dir, v, "%s: invalid max_score %s", dir, v)
		}
		maxScore = -1
	default:
		return 0, "", spec.Fail(pkg.ErrFormat, dir, fmt.Sprint(v), "%s: max_score must be a whole number", dir)
	}
	if !slices.Contains([]string{"pass-fail", "sum", "min"}, aggregation) {
		return 0, "", spec.Fail(pkg.ErrFormat, dir, aggregation, "%s: invalid score_aggregation %s", dir, aggregation)
	}
	return maxScore, aggregation, d.bounded(dir, maxScore)
}

func (d *currentData) bounded(dir string, maxScore int) error {
	if d.scoring && maxScore < 0 {
		return spec.Fail(pkg.ErrUnsupported, dir, "unbounded", "%s: an unbounded max_score needs score files, which are not supported", dir)
	}
	return nil
}

// score gives every test the group's max score: avg of those is the ICPC sum of max/N per test, min and all-or-nothing need no division.
func (d *currentData) score(g *spec.Group, aggregation string, maxScore int) {
	if !d.scoring {
		return
	}
	g.Aggregate, g.StopOnFailure = "min", true
	if aggregation == "sum" {
		g.Aggregate, g.StopOnFailure = "avg", false
	}
	for i := range g.Tests {
		g.Tests[i].Points = points.Value(maxScore) * points.Unit
	}
}

func (d *currentData) requires(g *spec.Group, c *GroupConfig) error {
	if !d.scoring || c == nil {
		return nil
	}
	for _, name := range c.RequirePass {
		if name != "sample" && !strings.HasPrefix(name, "secret/") {
			return spec.Fail(pkg.ErrFormat, "data/"+g.Name, name, "%s requires unknown group %s", g.Name, name)
		}
		g.Requires = append(g.Requires, name)
	}
	return nil
}

// tests reads the test cases under dir, in subdirectories too, in the lexicographic order of their names.
func (d *currentData) tests(dir string, sample bool) ([]spec.Test, error) {
	var names []string
	var nested error
	err := fs.WalkDir(d.fsys, dir, func(name string, e fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist) && name == dir:
			return fs.SkipDir
		case err != nil:
			return err
		case e.IsDir() && name != dir && (regular(d.fsys, path.Join(name, "test_group.yaml")) || regular(d.fsys, path.Join(name, "testdata.yaml"))):
			nested = spec.Fail(pkg.ErrPackageLayout, name, "", "%s: test data groups cannot nest", name)
			return fs.SkipAll
		case e.Type().IsRegular() && strings.HasSuffix(name, ".in"):
			names = append(names, strings.TrimSuffix(name, ".in"))
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	if nested != nil {
		return nil, nested
	}
	slices.Sort(names)
	var result []spec.Test
	for _, stem := range names {
		if err := d.testConfig(stem); err != nil {
			return nil, err
		}
		d.count++
		test := spec.Test{Input: stem + ".in", Sample: sample, Ordinal: d.count}
		if regular(d.fsys, stem+".ans") {
			test.Answer = stem + ".ans"
		}
		if sample {
			if regular(d.fsys, stem+".in.statement") {
				test.StatementInput = stem + ".in.statement"
			}
			for _, output := range []string{stem + ".ans.statement", stem + ".out"} {
				if regular(d.fsys, output) && test.StatementOutput == "" {
					test.StatementOutput = output
				}
			}
		}
		result = append(result, test)
	}
	return result, nil
}

// testConfig rejects what Gate cannot give a single test: its own arguments and input files.
func (d *currentData) testConfig(stem string) error {
	if _, err := fs.Stat(d.fsys, stem+".files"); err == nil {
		return spec.Fail(pkg.ErrUnsupported, stem+".files", "", "%s: input files are not supported", stem)
	}
	name := stem + ".yaml"
	if !regular(d.fsys, name) {
		return nil
	}
	var c TestConfig
	if err := strict(d.fsys, name, &c); err != nil {
		return err
	}
	for key, v := range map[string]any{"args": c.Args, "input_validator_args": c.InputValidatorArgs, "output_validator_args": c.OutputValidatorArgs} {
		if v != nil {
			return spec.Fail(pkg.ErrUnsupported, name, key, "%s: %s of a single test case are not supported", name, key)
		}
	}
	return nil
}

