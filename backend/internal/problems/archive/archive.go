// Package archive lays out drafts in the gfmt format, both as author exports and as built packages.
package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// BuiltTest names the local files a build produced for a test.
type BuiltTest struct{ Input, Answer string }

// Package is a laid-out draft; exactly one source of each file is set.
type Package struct {
	Manifest *gfmt.Problem
	files    []file
}

type file struct {
	name, text, local string
	object            uuid.UUID
}

// Stats count directories as files, as the import extraction does.
type Stats struct {
	Files int
	Bytes int64
	Names map[string]bool
}

// Fits reports whether the import accepts an archive of this size and content.
func (s Stats) Fits(size int64, limits model.BuildLimits) bool {
	return size <= int64(limits.ObjectBytes) && s.Files <= limits.FileCount && s.Bytes <= int64(limits.DiskBytes)-size
}

func ProgramPath(languages *sandbox.Config, role, name, language string) string {
	if extensions := languages.Langs[language].Extensions; len(extensions) > 0 {
		name += "." + extensions[0]
	}
	return gfmt.RoleFolder(role) + "/" + name
}

// Layout writes every test from objects, or, when built is set, from the files the build produced.
func Layout(d model.Draft, languages *sandbox.Config, built map[uuid.UUID]BuiltTest) (*Package, error) {
	p := &Package{Manifest: &gfmt.Problem{FormatVersion: gfmt.FormatVersion, Title: d.Title, Type: d.Type, MaxScore: d.MaxScore,
		Limits: gfmt.Limits{TimeMs: d.TimeLimitMs, MemoryMb: d.MemoryLimitMb}, InputFile: d.InputFile, OutputFile: d.OutputFile,
		Sources: map[string]gfmt.Source{}, Subtasks: map[string]gfmt.Subtask{}, Solutions: map[string]gfmt.Expectation{}}}
	m := p.Manifest
	program := func(role, name string, prog model.DraftProgram, source gfmt.Source) string {
		path := ProgramPath(languages, role, name, prog.Language)
		source.Language = prog.Language
		m.Sources[path] = source
		p.files = append(p.files, file{name: path, text: prog.Source})
		return strings.TrimPrefix(path, gfmt.RoleFolder(role)+"/")
	}
	if c := d.Checker; c != nil {
		source := gfmt.Source{Standard: c.Standard}
		for i, t := range c.Tests {
			test := gfmt.ProgramTest{Input: fmt.Sprintf("checker/%02d", i+1), Verdict: t.Verdict, Points: t.Points}
			test.Output, test.Answer = test.Input+".o", test.Input+".a"
			p.object(test.Input, t.Input)
			p.object(test.Output, t.Output)
			p.object(test.Answer, t.Answer)
			source.Tests = append(source.Tests, test)
		}
		name := "checker"
		if _, ok := syslib.LookupChecker(c.Standard); ok {
			name = c.Standard
		}
		program("checker", name, model.DraftProgram{Language: c.Language, Source: c.Text()}, source)
	}
	if d.Interactor != nil {
		program("interactor", "interactor", *d.Interactor, gfmt.Source{})
	}
	for _, v := range d.Validators {
		source := gfmt.Source{}
		for _, id := range v.Subtasks {
			source.Groups = append(source.Groups, d.Subtask(id).Name)
		}
		for i, t := range v.Tests {
			test := gfmt.ProgramTest{Input: fmt.Sprintf("validators/%s/%02d", v.Name, i+1), Verdict: map[bool]string{true: "valid", false: "invalid"}[t.Valid], Group: t.Group}
			p.object(test.Input, t.Input)
			source.Tests = append(source.Tests, test)
		}
		program("validator", v.Name, v.DraftProgram, source)
	}
	for _, g := range d.Generators {
		program("generator", g.Name, g.DraftProgram, gfmt.Source{})
	}
	solutions := map[string]string{}
	for _, s := range d.Solutions {
		file := program("solution", s.Name, s.DraftProgram, gfmt.Source{})
		solutions[s.Name] = file
		m.Solutions[file] = gfmt.Expectation{Verdicts: s.Verdicts, Points: s.Points}
	}
	if d.MainSolution != nil {
		m.MainSolution = solutions[*d.MainSolution]
	}
	for _, f := range d.Library {
		path := gfmt.LibraryFolder + "/" + f.Name
		if _, ok := syslib.Lookup(f.Name); ok {
			m.Sources[path] = gfmt.Source{AutoUpdate: &f.AutoUpdate}
		}
		p.files = append(p.files, file{name: path, text: f.Text()})
	}
	for _, s := range d.Statements {
		if s.Title != "" {
			if m.Titles == nil {
				m.Titles = map[string]string{}
			}
			m.Titles[s.Language] = s.Title
		}
		for _, section := range model.StatementSections {
			// Legend is always written so that a statement with empty sections survives.
			if text := *s.Section(section); text != "" || section == "legend" {
				p.files = append(p.files, file{name: gfmt.StatementPath(s.Language, section), text: text})
			}
		}
	}
	for _, a := range d.Assets {
		p.files = append(p.files, file{name: gfmt.AssetFolder + "/" + a.Name, object: a.ObjectID})
	}
	return p, p.tests(d, solutions, built)
}

func (p *Package) object(name string, oid uuid.UUID) {
	p.files = append(p.files, file{name: gfmt.TestFolder + "/" + name, object: oid})
}

func (p *Package) tests(d model.Draft, solutions map[string]string, built map[uuid.UUID]BuiltTest) error {
	m := p.Manifest
	names := map[uuid.UUID]string{}
	for _, s := range d.Subtasks {
		names[s.ID] = s.Name
		m.Subtasks[s.Name] = gfmt.Subtask{Ordinal: s.Ordinal, Points: s.Points, Policy: s.Policy, Tests: []gfmt.Test{}}
	}
	for _, s := range d.Subtasks {
		subtask := m.Subtasks[names[s.ID]]
		for _, dep := range s.DependsOn {
			subtask.Dependencies = append(subtask.Dependencies, names[dep])
		}
		m.Subtasks[names[s.ID]] = subtask
	}
	tests := slices.Clone(d.Tests)
	slices.SortFunc(tests, func(a, b model.DraftTest) int { return a.Ordinal - b.Ordinal })
	for _, t := range tests {
		test := gfmt.Test{Ordinal: t.Ordinal, Points: t.Points, Sample: t.Sample}
		name := fmt.Sprintf("%03d", t.Ordinal)
		local, isBuilt := built[t.ID]
		if built != nil && !isBuilt {
			return fmt.Errorf("test %d was not built", t.Ordinal)
		}
		if t.Input.Mode == "generator" && t.Input.Generator != nil {
			test.Generate = spec.FormatCommand(append([]string{*t.Input.Generator}, t.Input.Args...))
		}
		switch {
		case isBuilt:
			test.Input = name
			p.files = append(p.files, file{name: gfmt.TestFolder + "/" + name, local: local.Input})
		case t.Input.ObjectID != nil:
			test.Input = name
			p.object(name, *t.Input.ObjectID)
		}
		if t.Answer.Mode == "generated" && t.Answer.Solution != nil {
			test.Solution = solutions[*t.Answer.Solution]
		}
		switch {
		case isBuilt:
			test.Answer = name + ".a"
			p.files = append(p.files, file{name: gfmt.TestFolder + "/" + test.Answer, local: local.Answer})
		case t.Answer.Mode == "provided" && t.Answer.ObjectID != nil:
			test.Answer = name + ".a"
			p.object(test.Answer, *t.Answer.ObjectID)
		}
		subtask := m.Subtasks[names[t.SubtaskID]]
		subtask.Tests = append(subtask.Tests, test)
		m.Subtasks[names[t.SubtaskID]] = subtask
	}
	return nil
}

// Write stores the package as a zip; open reads the objects the draft refers to.
func (p *Package) Write(dst io.Writer, open func(uuid.UUID) (io.ReadCloser, error)) (Stats, error) {
	w := &writer{z: zip.NewWriter(dst), stats: Stats{Names: map[string]bool{}}}
	for _, f := range p.files {
		var err error
		switch {
		case f.local != "":
			err = w.file(f.name, f.local)
		case f.object != uuid.Nil:
			err = w.object(f.name, f.object, open)
		default:
			err = w.add(f.name, strings.NewReader(f.text))
		}
		if err != nil {
			return w.stats, err
		}
	}
	manifest, err := yaml.Marshal(p.Manifest)
	if err != nil {
		return w.stats, fmt.Errorf("marshal problem.yaml: %w", err)
	}
	if len(manifest) > spec.MaxMetadataBytes {
		return w.stats, pkg.ErrPayloadTooLarge
	}
	if err := w.add("problem.yaml", strings.NewReader(string(manifest))); err != nil {
		return w.stats, err
	}
	if err := w.z.Close(); err != nil {
		return w.stats, fmt.Errorf("zip close: %w", err)
	}
	return w.stats, nil
}

type writer struct {
	z     *zip.Writer
	stats Stats
}

func (w *writer) add(name string, r io.Reader) error {
	// A fixed time keeps archives of the same content byte-identical.
	dst, err := w.z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		return fmt.Errorf("zip create: %w", err)
	}
	n, err := io.Copy(dst, r)
	if err != nil {
		return fmt.Errorf("zip %s: %w", name, err)
	}
	w.stats.Files++
	w.stats.Bytes += n
	w.stats.Names[name] = true
	for dir := path.Dir(name); dir != "." && !w.stats.Names[dir+"/"]; dir = path.Dir(dir) {
		w.stats.Names[dir+"/"] = true
		w.stats.Files++
	}
	return nil
}

func (w *writer) object(name string, oid uuid.UUID, open func(uuid.UUID) (io.ReadCloser, error)) error {
	r, err := open(oid)
	if err != nil {
		return err
	}
	defer r.Close()
	return w.add(name, r)
}

func (w *writer) file(name, local string) error {
	f, err := os.Open(local)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	return w.add(name, f)
}
