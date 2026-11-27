// Package archive lays out drafts in the gfmt format, both as author exports and as built packages.
package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"iter"
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
	// Test files by source, so tests sharing an object or a built file share one file.
	names map[string]string
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
	return size <= int64(limits.ObjectBytes) && s.Extracts(size, limits)
}

// Extracts reports whether the archive unpacks within the budgets judges and imports extract it with.
func (s Stats) Extracts(size int64, limits model.BuildLimits) bool {
	return s.Files <= limits.FileCount && s.Bytes <= int64(limits.DiskBytes)-size
}

func ProgramPath(languages *sandbox.Config, role, name, language string) string {
	if extensions := languages.Langs[language].Extensions; len(extensions) > 0 {
		name += "." + extensions[0]
	}
	return gfmt.RoleFolder(role) + "/" + name
}

// Layout writes every test from objects, or, when built is set, from the files the build produced.
func Layout(d model.Draft, languages *sandbox.Config, built map[uuid.UUID]BuiltTest) (*Package, error) {
	p := &Package{Manifest: &gfmt.Problem{FormatVersion: gfmt.FormatVersion, Title: d.Title, Type: d.Type, Grading: d.Grading, MaxScore: d.MaxScore,
		Limits: gfmt.Limits{TimeMs: d.TimeLimitMs, MemoryMb: d.MemoryLimitMb}, InputFile: d.InputFile, OutputFile: d.OutputFile, Processes: d.Processes, SolutionName: d.SolutionName,
		Sources: map[string]gfmt.Source{}, Solutions: map[string]gfmt.Expectation{}}}
	p.names = map[string]string{}
	m := p.Manifest
	program := func(role, name string, prog model.DraftProgram, source gfmt.Source) string {
		path := ProgramPath(languages, role, name, prog.Language)
		source.Language = prog.Language
		m.Sources[path] = source
		p.files = append(p.files, file{name: path, text: prog.Source})
		return strings.TrimPrefix(path, gfmt.RoleFolder(role)+"/")
	}
	if c := d.Checker; c != nil {
		source := gfmt.Source{Standard: c.Standard, Protocol: string(c.Protocol), Args: c.Args}
		for i, t := range c.Tests {
			name := fmt.Sprintf("checker/%02d", i+1)
			test := gfmt.ProgramTest{Input: p.object(name, t.Input), Output: p.object(name+".o", t.Output), Answer: p.object(name+".a", t.Answer), Verdict: t.Verdict, Points: t.Points}
			source.Tests = append(source.Tests, test)
		}
		name := "checker"
		if _, ok := syslib.LookupChecker(c.Standard); ok {
			name = c.Standard
		}
		program("checker", name, model.DraftProgram{Language: c.Language, Source: c.Text()}, source)
	}
	if d.Interactor != nil {
		program("interactor", "interactor", d.Interactor.DraftProgram, gfmt.Source{Protocol: string(d.Interactor.Protocol), Args: d.Interactor.Args})
	}
	for _, v := range d.Validators {
		source := gfmt.Source{Protocol: string(v.Protocol), Args: v.Args}
		for _, id := range v.Groups {
			source.Groups = append(source.Groups, d.Group(id).Name)
		}
		for i, t := range v.Tests {
			test := gfmt.ProgramTest{Input: p.object(fmt.Sprintf("validators/%s/%02d", v.Name, i+1), t.Input), Verdict: map[bool]string{true: "valid", false: "invalid"}[t.Valid], Group: t.Group}
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
		if f.ObjectID != nil {
			p.files = append(p.files, file{name: path, object: *f.ObjectID})
		} else {
			p.files = append(p.files, file{name: path, text: f.Text()})
		}
	}
	for folder, files := range map[string][]model.DraftFile{gfmt.GraderFolder: d.Graders, gfmt.AttachmentFolder: d.Attachments} {
		for _, f := range files {
			p.files = append(p.files, file{name: folder + "/" + f.Name, text: f.Content})
		}
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

// object adds a file under tests/ unless its source is there already, and returns the name to refer to.
func (p *Package) object(name string, oid uuid.UUID) string {
	return p.test(name, "object:"+oid.String(), file{object: oid})
}

func (p *Package) local(name, path string) string {
	return p.test(name, "local:"+path, file{local: path})
}

func (p *Package) test(name, source string, f file) string {
	if existing, ok := p.names[source]; ok {
		return existing
	}
	p.names[source] = name
	f.name = gfmt.TestFolder + "/" + name
	p.files = append(p.files, f)
	return name
}

func (p *Package) tests(d model.Draft, solutions map[string]string, built map[uuid.UUID]BuiltTest) error {
	m := p.Manifest
	names := map[uuid.UUID]string{}
	for g := range d.Testset.All() {
		names[g.ID] = g.Name
	}
	var convert func(g model.DraftGroup) gfmt.Group
	convert = func(g model.DraftGroup) gfmt.Group {
		group := gfmt.Group{Name: g.Name, Aggregate: g.Aggregate, StopOnFailure: g.StopOnFailure, Feedback: g.Feedback,
			CheckerArgs: g.CheckerArgs, ValidatorArgs: g.ValidatorArgs}
		for _, id := range g.Requires {
			group.Requires = append(group.Requires, names[id])
		}
		for _, child := range g.Groups {
			group.Groups = append(group.Groups, convert(child))
		}
		return group
	}
	m.Testset = convert(d.Testset)
	// Both trees are walked in the same order, so the n-th draft group is the n-th package group.
	groups := map[uuid.UUID]*gfmt.Group{}
	next, stop := iter.Pull(m.Testset.All())
	defer stop()
	for g := range d.Testset.All() {
		groups[g.ID], _ = next()
	}
	tests := slices.Clone(d.Tests)
	slices.SortFunc(tests, func(a, b model.DraftTest) int { return a.Ordinal - b.Ordinal })
	for _, t := range tests {
		test := gfmt.Test{Ordinal: t.Ordinal, Points: t.Points, Sample: t.Sample, Pretest: t.Pretest}
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
			test.Input = p.local(name, local.Input)
		case t.Input.ObjectID != nil:
			test.Input = p.object(name, *t.Input.ObjectID)
		}
		if t.Answer.Mode == "generated" && t.Answer.Solution != nil {
			test.Solution = solutions[*t.Answer.Solution]
		}
		switch {
		case isBuilt:
			test.Answer = p.local(name+".a", local.Answer)
		case t.Answer.Mode == "provided" && t.Answer.ObjectID != nil:
			test.Answer = p.object(name+".a", *t.Answer.ObjectID)
		}
		if t.StatementInput != nil {
			test.StatementInput = p.test(name+".example", "example:"+name, file{text: *t.StatementInput})
		}
		if t.StatementOutput != nil {
			test.StatementOutput = p.test(name+".example.a", "example.a:"+name, file{text: *t.StatementOutput})
		}
		group := groups[t.GroupID]
		group.Tests = append(group.Tests, test)
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
