package gfmt

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"gopkg.in/yaml.v3"
)

// Read reads an author gfmt package: problem.yaml and the files of its folders.
func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := spec.ReadFile(fsys, "problem.yaml")
	if err != nil {
		return nil, fmt.Errorf("read problem.yaml: %w", err)
	}
	var manifest *Problem
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse problem.yaml: %w", err)
	}
	if manifest == nil {
		return nil, fmt.Errorf("problem.yaml must contain a mapping")
	}
	p := &spec.Problem{Title: manifest.Title, Type: manifest.Type, TimeLimitMs: manifest.Limits.TimeMs, MemoryLimitMb: manifest.Limits.MemoryMb}
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
			deps, err := dependencies(manifest.Sources[rel])
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			programs[folder] = append(programs[folder], spec.Program{File: spec.File{Name: name, Path: rel}, Dependencies: deps})
		case folder == LibraryFolder && name != "" && !strings.Contains(name, "/"):
			p.Library = append(p.Library, spec.File{Name: name, Path: rel})
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
			return fmt.Errorf("unexpected file %s", rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read package: %w", err)
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
			return nil, fmt.Errorf("more than one %s in %s/", role, folder)
		}
	}
	if p.Checker, err = singleton(RoleFolder("checker"), "checker"); err != nil {
		return nil, err
	}
	if p.Validator, err = singleton(RoleFolder("validator"), "validator"); err != nil {
		return nil, err
	}
	if p.Interactor, err = singleton(RoleFolder("interactor"), "interactor"); err != nil {
		return nil, err
	}
	p.Generators, p.Solutions = programs[RoleFolder("generator")], programs[RoleFolder("solution")]
	p.MainSolution = mainSolution(manifest.Solutions, p.Solutions)
	// Tests name generators by file stem, so the stem must identify one file.
	generators := map[string]string{}
	for _, g := range p.Generators {
		stem := spec.Stem(g.Name)
		if _, ok := generators[stem]; ok {
			return nil, fmt.Errorf("several generators named %s", stem)
		}
		generators[stem] = g.Path
	}
	if p.Groups, err = groups(manifest, generators, files); err != nil {
		return nil, err
	}
	return p, nil
}

// dependencies converts gfmt build settings into library dependencies; a
// program without them includes the whole library.
func dependencies(s Source) (map[string]string, error) {
	if len(s.DependencyMap) == 0 {
		return nil, nil
	}
	for _, source := range s.DependencyMap {
		if !strings.HasPrefix(source, LibraryFolder+"/") {
			return nil, fmt.Errorf("dependency %s is outside lib/", source)
		}
	}
	return s.DependencyMap, nil
}

// mainSolution is solutions.main, or else the first solution expected to pass.
func mainSolution(verdicts map[string]string, solutions []spec.Program) string {
	if main := verdicts["main"]; main != "" {
		return RoleFolder("solution") + "/" + main
	}
	for _, s := range solutions {
		if verdicts[s.Name] == "OK" {
			return s.Path
		}
	}
	return ""
}

// groups lists subtasks in test numbering order. A test marked as a sample in
// a scored subtask is numbered there, not again under "samples".
func groups(manifest *Problem, generators map[string]string, files map[string]bool) ([]spec.Group, error) {
	key := func(t Test) string {
		if t.Manual != "" {
			return "manual:" + t.Manual
		}
		return "generator:" + t.Generate
	}
	samples, scored := map[string]bool{}, map[string]bool{}
	for name, subtask := range manifest.Subtasks {
		for _, t := range subtask.Tests {
			if t.Sample || name == "samples" {
				samples[key(t)] = true
			}
			if name != "samples" {
				scored[key(t)] = true
			}
		}
	}
	var result []spec.Group
	dropped := ""
	for _, name := range manifest.OrderedSubtaskNames() {
		subtask := manifest.Subtasks[name]
		group := spec.Group{Name: name, Policy: subtask.Policy, DependsOn: subtask.Dependencies}
		for _, t := range subtask.Tests {
			if name == "samples" && scored[key(t)] {
				continue
			}
			test, err := convertTest(t, generators, files)
			if err != nil {
				return nil, err
			}
			test.Sample = samples[key(t)]
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
	test := spec.Test{Points: t.Points}
	if t.Manual != "" {
		test.Input = TestFolder + "/" + t.Manual
	} else {
		argv, err := spec.ParseCommand(t.Generate)
		if err != nil {
			return test, err
		}
		name, ok := generators[argv[0]]
		if !ok {
			return test, fmt.Errorf("generator %s not found", argv[0])
		}
		test.Generator, test.Args = name, argv[1:]
	}
	switch {
	case t.Answer != "":
		test.Answer = TestFolder + "/" + t.Answer
	case t.Manual != "":
		for _, candidate := range TestOutputNames(t.Manual) {
			if files[TestFolder+"/"+candidate] {
				test.Answer = TestFolder + "/" + candidate
				break
			}
		}
	}
	return test, nil
}
