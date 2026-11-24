package gfmt

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"gopkg.in/yaml.v3"
)

func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := spec.ReadFile(fsys, "problem.yaml")
	if err != nil {
		return nil, err
	}
	var manifest *Problem
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, spec.Syntax("problem.yaml", err)
	}
	if manifest == nil {
		return nil, spec.Fail(pkg.ErrPackageSyntax, "problem.yaml", "", "problem.yaml must contain a mapping")
	}
	p := &spec.Problem{Title: manifest.Title, Type: manifest.Type, TimeLimitMs: manifest.Limits.TimeMs, MemoryLimitMb: manifest.Limits.MemoryMb,
		InputFile: manifest.InputFile, OutputFile: manifest.OutputFile}
	programs := map[string][]spec.Program{}
	statements := map[string]*spec.Statement{}
	files := map[string]bool{}
	err = fs.WalkDir(fsys, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		files[rel] = true
		folder, name, _ := strings.Cut(rel, "/")
		switch {
		case rel == "problem.yaml" || folder == TestFolder:
		case folderRole(folder) != "" && name != "" && !strings.Contains(name, "/"):
			deps, err := dependencies(rel, manifest.Sources[rel])
			if err != nil {
				return err
			}
			source := manifest.Sources[rel]
			programs[folder] = append(programs[folder], spec.Program{File: spec.File{Name: name, Path: rel}, Language: source.Language, Dependencies: deps, Standard: source.Standard, Groups: source.Groups})
		case folder == LibraryFolder && name != "" && !strings.Contains(name, "/"):
			auto := manifest.Sources[rel].AutoUpdate
			p.Library = append(p.Library, spec.File{Name: name, Path: rel, System: auto != nil && *auto, Pinned: auto != nil && !*auto})
		case folder == AssetFolder && name != "" && !strings.Contains(name, "/"):
			p.Assets = append(p.Assets, spec.File{Name: name, Path: rel})
		case folder == StatementFolder && strings.Count(name, "/") == 1 && path.Ext(name) == ".md":
			language, file, _ := strings.Cut(name, "/")
			s := statements[language]
			if s == nil {
				s = &spec.Statement{Language: language, Sections: map[string]string{}}
				statements[language] = s
			}
			s.Sections[strings.TrimSuffix(file, ".md")] = rel
		default:
			return spec.Fail(pkg.ErrPackageLayout, rel, "", "unexpected file %s", rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read package: %w", err)
	}
	for _, name := range manifest.SystemLibrary {
		rel := LibraryFolder + "/" + name
		if name == "" || strings.Contains(name, "/") || files[rel] {
			return nil, spec.Fail(pkg.ErrDuplicate, rel, name, "system library file %q is invalid or also in %s/", name, LibraryFolder)
		}
		p.Library = append(p.Library, spec.File{Name: name, Path: rel, System: true})
	}
	for language, title := range manifest.Titles {
		if statements[language] == nil {
			return nil, spec.Fail(pkg.ErrUnknown, "problem.yaml", language, "title in %s has no statement", language)
		}
		statements[language].Title = title
	}
	for _, language := range slices.Sorted(maps.Keys(statements)) {
		p.Statements = append(p.Statements, *statements[language])
	}
	singleton := func(folder, role string) (*spec.Program, error) {
		switch list := programs[folder]; len(list) {
		case 0:
			return nil, nil
		case 1:
			return &list[0], nil
		default:
			return nil, spec.Fail(pkg.ErrPackageLayout, folder, "", "more than one %s in %s/", role, folder)
		}
	}
	if p.Checker, err = singleton(RoleFolder("checker"), "checker"); err != nil {
		return nil, err
	}
	if p.Interactor, err = singleton(RoleFolder("interactor"), "interactor"); err != nil {
		return nil, err
	}
	p.Validators, p.Generators, p.Solutions = programs[RoleFolder("validator")], programs[RoleFolder("generator")], programs[RoleFolder("solution")]
	for folder, list := range programs {
		for i := range list {
			if err := programTests(&list[i], folder, manifest.Sources[list[i].Path].Tests); err != nil {
				return nil, err
			}
		}
	}
	for i := range p.Solutions {
		expected := manifest.Solutions[p.Solutions[i].Name]
		p.Solutions[i].Verdicts, p.Solutions[i].Points = expected.Verdicts, expected.Points
	}
	p.MainSolution = mainSolution(manifest.MainSolution, p.Solutions)
	// Generator stems must be unique because tests refer to them without extensions.
	generators := map[string]string{}
	for _, g := range p.Generators {
		stem := spec.Stem(g.Name)
		if _, ok := generators[stem]; ok {
			return nil, spec.Fail(pkg.ErrDuplicate, g.Path, stem, "several generators named %s", stem)
		}
		generators[stem] = g.Path
	}
	if p.Groups, err = groups(manifest, generators, files); err != nil {
		return nil, err
	}
	return p, nil
}

func programTests(program *spec.Program, folder string, tests []ProgramTest) error {
	for i, t := range tests {
		input := TestFolder + "/" + t.Input
		switch {
		case t.Input == "":
			return spec.Fail(pkg.ErrRequired, program.Path, "", "%s: test %d has no input", program.Path, i+1)
		case folder == RoleFolder("checker") && t.Output != "" && t.Answer != "" && t.Group == "":
			program.CheckerTests = append(program.CheckerTests, spec.CheckerTest{Input: input, Output: TestFolder + "/" + t.Output, Answer: TestFolder + "/" + t.Answer, Verdict: t.Verdict, Points: t.Points})
		case folder == RoleFolder("validator") && (t.Verdict == "valid" || t.Verdict == "invalid") && t.Output == "" && t.Answer == "" && t.Points == nil:
			program.ValidatorTests = append(program.ValidatorTests, spec.ValidatorTest{Input: input, Valid: t.Verdict == "valid", Group: t.Group})
		default:
			return spec.Fail(pkg.ErrPackageSyntax, program.Path, "", "%s: test %d is no checker test with output and answer or validator test with a valid or invalid verdict", program.Path, i+1)
		}
	}
	return nil
}

// Programs without explicit dependencies include the whole library.
func dependencies(rel string, s Source) (map[string]string, error) {
	if len(s.DependencyMap) == 0 {
		return nil, nil
	}
	for _, source := range s.DependencyMap {
		if !strings.HasPrefix(source, LibraryFolder+"/") {
			return nil, spec.Fail(pkg.ErrPackageLayout, rel, source, "dependency %s is outside lib/", source)
		}
	}
	return s.DependencyMap, nil
}

// Use main_solution, falling back to the first solution expected to pass.
func mainSolution(main string, solutions []spec.Program) string {
	if main != "" {
		return RoleFolder("solution") + "/" + main
	}
	for _, s := range solutions {
		if slices.Equal(s.Verdicts, []string{"OK"}) {
			return s.Path
		}
	}
	return ""
}

// Samples in scored subtasks are numbered there, not duplicated under samples.
func groups(manifest *Problem, generators map[string]string, files map[string]bool) ([]spec.Group, error) {
	key := func(t Test) string {
		if t.Generate != "" {
			return "generator:" + t.Generate
		}
		return "input:" + t.Input
	}
	// Ordinals mark an exported draft, whose subtask named samples is an ordinary one.
	legacy := !manifest.hasOrdinals()
	samples, scored := map[string]bool{}, map[string]bool{}
	for name, subtask := range manifest.Subtasks {
		for _, t := range subtask.Tests {
			if legacy && name == "samples" {
				samples[key(t)] = true
			}
			if legacy && name != "samples" {
				scored[key(t)] = true
			}
		}
	}
	var result []spec.Group
	dropped := ""
	for _, name := range manifest.OrderedSubtaskNames() {
		subtask := manifest.Subtasks[name]
		group := spec.Group{Name: name, Policy: subtask.Policy, Feedback: subtask.Feedback, DependsOn: subtask.Dependencies}
		for _, t := range subtask.Tests {
			if name == "samples" && scored[key(t)] {
				continue
			}
			test, err := convertTest(t, generators, files)
			if err != nil {
				return nil, err
			}
			test.Sample = t.Sample || samples[key(t)]
			test.Pretest = t.Pretest
			group.Tests = append(group.Tests, test)
		}
		if name == "samples" && len(subtask.Tests) > 0 && len(group.Tests) == 0 {
			dropped = name
			continue
		}
		result = append(result, group)
	}
	if dropped != "" {
		for i := range result {
			result[i].DependsOn = slices.DeleteFunc(slices.Clone(result[i].DependsOn), func(dep string) bool { return dep == dropped })
		}
	}
	return result, nil
}

func convertTest(t Test, generators map[string]string, files map[string]bool) (spec.Test, error) {
	test := spec.Test{Points: t.Points, Ordinal: t.Ordinal}
	if t.Generate == "" {
		test.Input = TestFolder + "/" + t.Input
	} else {
		argv, err := spec.ParseCommand(t.Generate)
		if err != nil {
			return test, err
		}
		name, ok := generators[argv[0]]
		if !ok {
			return test, spec.Fail(pkg.ErrUnknown, "", argv[0], "generator %s not found", argv[0])
		}
		test.Generator, test.Args = name, argv[1:]
	}
	switch {
	case t.Solution != "":
		test.AnswerSolution = RoleFolder("solution") + "/" + t.Solution
	case t.Answer != "":
		test.Answer = TestFolder + "/" + t.Answer
	case t.Generate == "":
		for _, candidate := range TestOutputNames(t.Input) {
			if files[TestFolder+"/"+candidate] {
				test.Answer = TestFolder + "/" + candidate
				break
			}
		}
	}
	return test, nil
}
