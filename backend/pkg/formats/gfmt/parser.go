package gfmt

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"gopkg.in/yaml.v3"
)

// Folders of an author gfmt package. Each program lives in its role folder;
// tests/ holds the files problem.yaml refers to.
var programFolders = []string{"checkers", "validators", "interactors", "generators", "solutions"}

// Read reads an author gfmt package: problem.yaml and the files of its folders.
func Read(fsys fs.FS) (*spec.Problem, error) {
	data, err := fs.ReadFile(fsys, "problem.yaml")
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
	files := map[string]bool{}
	err = fs.WalkDir(fsys, ".", func(rel string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		files[rel] = true
		folder, name, _ := strings.Cut(rel, "/")
		switch {
		case rel == "problem.yaml" || folder == "tests":
		case slices.Contains(programFolders, folder) && name != "":
			deps, err := dependencies(manifest.Sources[rel])
			if err != nil {
				return fmt.Errorf("%s: %w", rel, err)
			}
			programs[folder] = append(programs[folder], spec.Program{File: spec.File{Name: name, Path: rel}, Dependencies: deps})
		case folder == "lib" && name != "":
			p.Library = append(p.Library, spec.File{Name: name, Path: rel})
		case folder == "assets" && name != "":
			p.Assets = append(p.Assets, spec.File{Name: name, Path: rel})
		case folder == "statements" && path.Ext(name) == ".md" && !strings.Contains(name, "/"):
			text, err := fs.ReadFile(fsys, rel)
			if err != nil {
				return fmt.Errorf("read statement: %w", err)
			}
			p.Statements = append(p.Statements, spec.Statement{Language: strings.TrimSuffix(name, ".md"), Markdown: string(text)})
		default:
			return fmt.Errorf("unexpected file %s", rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read package: %w", err)
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
	if p.Checker, err = singleton("checkers", "checker"); err != nil {
		return nil, err
	}
	if p.Validator, err = singleton("validators", "validator"); err != nil {
		return nil, err
	}
	if p.Interactor, err = singleton("interactors", "interactor"); err != nil {
		return nil, err
	}
	p.Generators, p.Solutions = programs["generators"], programs["solutions"]
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

// dependencies converts gfmt build settings into library dependencies.
func dependencies(s Source) (map[string]string, error) {
	if len(s.DependencyMap) == 0 && len(s.Dependencies) == 0 {
		return nil, nil
	}
	deps := map[string]string{}
	for destination, source := range s.DependencyMap {
		if !strings.HasPrefix(source, "lib/") {
			return nil, fmt.Errorf("dependency %s is outside lib/", source)
		}
		deps[destination] = source
	}
	for _, source := range s.Dependencies {
		name := strings.TrimPrefix(source, "lib/")
		deps[name] = "lib/" + name
	}
	return deps, nil
}

// mainSolution is solutions.main, or else the first solution expected to pass.
func mainSolution(verdicts map[string]string, solutions []spec.Program) string {
	if main := verdicts["main"]; main != "" {
		return inFolder("solutions", main)
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
		test.Input = inTests(t.Manual)
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
		test.Answer = inTests(t.Answer)
	case t.Manual != "":
		for _, candidate := range TestOutputNames(strings.TrimPrefix(t.Manual, "tests/")) {
			if files["tests/"+candidate] {
				test.Answer = "tests/" + candidate
				break
			}
		}
	}
	return test, nil
}

func inTests(name string) string { return inFolder("tests", name) }

func inFolder(folder, name string) string {
	if strings.HasPrefix(name, folder+"/") {
		return name
	}
	return folder + "/" + name
}
