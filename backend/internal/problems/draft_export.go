package problems

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Export writes the draft as a gfmt author package that Import accepts.
func (uc *DraftUseCase) Export(ctx context.Context, id uuid.UUID) (io.ReadCloser, error) {
	d, err := uc.GetDraft(ctx, id)
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(uc.TempDir, "export-")
	if err != nil {
		return nil, fmt.Errorf("createtemp: %w", err)
	}
	// The open descriptor keeps the unlinked file readable until Close.
	_ = os.Remove(f.Name())
	open := func(oid uuid.UUID) (io.ReadCloser, error) {
		o, err := uc.Repo.GetObject(ctx, oid)
		if err != nil {
			return nil, err
		}
		r, _, err := uc.Store.DownloadFile(ctx, uc.Bucket, o.Key, nil)
		return r, err
	}
	w, err := writeDraftArchive(f, d, uc.Languages, open)
	if err == nil {
		err = uc.checkExport(f, w)
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// Refuse archives that the import would reject for size, so every export can be imported back.
func (uc *DraftUseCase) checkExport(f *os.File, w *archiveWriter) error {
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	limits := uc.BuildOptions.Limits
	if size > uc.ObjectBytes || w.files > limits.FileCount || w.bytes > int64(limits.DiskBytes)-size {
		return pkg.ErrPayloadTooLarge
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	return nil
}

// files counts directories too, as the import extraction does.
type archiveWriter struct {
	z     *zip.Writer
	dirs  map[string]bool
	files int
	bytes int64
}

func (w *archiveWriter) add(name string, r io.Reader) error {
	dst, err := w.z.Create(name)
	if err != nil {
		return fmt.Errorf("zip create: %w", err)
	}
	n, err := io.Copy(dst, r)
	if err != nil {
		return fmt.Errorf("zip %s: %w", name, err)
	}
	w.files++
	w.bytes += n
	for dir := path.Dir(name); dir != "." && !w.dirs[dir]; dir = path.Dir(dir) {
		w.dirs[dir] = true
		w.files++
	}
	return nil
}

func (w *archiveWriter) text(name, text string) error {
	return w.add(name, strings.NewReader(text))
}

func (w *archiveWriter) object(name string, oid uuid.UUID, open func(uuid.UUID) (io.ReadCloser, error)) error {
	r, err := open(oid)
	if err != nil {
		return err
	}
	defer r.Close()
	return w.add(name, r)
}

func writeDraftArchive(dst io.Writer, d model.Draft, languages *sandbox.Config, open func(uuid.UUID) (io.ReadCloser, error)) (*archiveWriter, error) {
	w := &archiveWriter{z: zip.NewWriter(dst), dirs: map[string]bool{}}
	m := gfmt.Problem{FormatVersion: "1.0", Title: d.Title, Type: d.Type, Limits: gfmt.Limits{TimeMs: d.TimeLimitMs, MemoryMb: d.MemoryLimitMb},
		Sources: map[string]gfmt.Source{}, Subtasks: map[string]gfmt.Subtask{}, Solutions: map[string]gfmt.Expectation{}}
	fileName := func(name, language string) string {
		if extensions := languages.Langs[language].Extensions; len(extensions) > 0 {
			return name + "." + extensions[0]
		}
		return name
	}
	program := func(role, name string, p model.DraftProgram, source gfmt.Source) (string, error) {
		file := fileName(name, p.Language)
		path := gfmt.RoleFolder(role) + "/" + file
		source.Language = p.Language
		m.Sources[path] = source
		return file, w.text(path, p.Source)
	}
	if c := d.Checker; c != nil {
		var err error
		if standard, ok := syslib.LookupChecker(c.Standard); ok {
			_, err = program("checker", c.Standard, model.DraftProgram{Language: c.Language, Source: standard.Content}, gfmt.Source{Standard: c.Standard})
		} else {
			_, err = program("checker", "checker", c.DraftProgram, gfmt.Source{})
		}
		if err != nil {
			return nil, err
		}
	}
	if d.Interactor != nil {
		if _, err := program("interactor", "interactor", *d.Interactor, gfmt.Source{}); err != nil {
			return nil, err
		}
	}
	for _, v := range d.Validators {
		source := gfmt.Source{}
		for _, id := range v.Subtasks {
			source.Groups = append(source.Groups, d.Subtask(id).Name)
		}
		if _, err := program("validator", v.Name, v.DraftProgram, source); err != nil {
			return nil, err
		}
	}
	solutions := map[string]string{}
	for _, list := range []struct {
		role     string
		programs []model.DraftNamedProgram
	}{{"generator", d.Generators}, {"solution", d.SolutionPrograms()}} {
		for _, p := range list.programs {
			file, err := program(list.role, p.Name, p.DraftProgram, gfmt.Source{})
			if err != nil {
				return nil, err
			}
			if list.role == "solution" {
				solutions[p.Name] = file
				s := d.Solution(p.Name)
				m.Solutions[file] = gfmt.Expectation{Verdicts: s.Verdicts, Points: s.Points}
			}
		}
	}
	if d.MainSolution != nil {
		m.MainSolution = solutions[*d.MainSolution]
	}
	for _, f := range d.Library {
		path := gfmt.LibraryFolder + "/" + f.Name
		if _, ok := syslib.Lookup(f.Name); ok {
			m.Sources[path] = gfmt.Source{AutoUpdate: &f.AutoUpdate}
		}
		if err := w.text(path, f.Text()); err != nil {
			return nil, err
		}
	}
	for _, s := range d.Statements {
		for _, section := range model.StatementSections {
			// Legend is always written so that a statement with empty sections survives.
			if text := *s.Section(section); text != "" || section == "legend" {
				if err := w.text(gfmt.StatementPath(s.Language, section), text); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, a := range d.Assets {
		if err := w.object(gfmt.AssetFolder+"/"+a.Name, a.ObjectID, open); err != nil {
			return nil, err
		}
	}
	if err := writeTests(w, &m, d, solutions, open); err != nil {
		return nil, err
	}
	manifest, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal problem.yaml: %w", err)
	}
	if len(manifest) > spec.MaxMetadataBytes {
		return nil, pkg.ErrPayloadTooLarge
	}
	if err := w.text("problem.yaml", string(manifest)); err != nil {
		return nil, err
	}
	if err := w.z.Close(); err != nil {
		return nil, fmt.Errorf("zip close: %w", err)
	}
	return w, nil
}

func writeTests(w *archiveWriter, m *gfmt.Problem, d model.Draft, solutions map[string]string, open func(uuid.UUID) (io.ReadCloser, error)) error {
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
		file := fmt.Sprintf("%03d", t.Ordinal)
		if t.Input.Mode == "generator" && t.Input.Generator != nil {
			test.Generate = spec.FormatCommand(append([]string{*t.Input.Generator}, t.Input.Args...))
		} else if t.Input.ObjectID != nil {
			test.Manual = file
			if err := w.object(gfmt.TestFolder+"/"+file, *t.Input.ObjectID, open); err != nil {
				return err
			}
		}
		switch {
		case t.Answer.Mode == "provided" && t.Answer.ObjectID != nil:
			test.Answer = file + ".a"
			if err := w.object(gfmt.TestFolder+"/"+test.Answer, *t.Answer.ObjectID, open); err != nil {
				return err
			}
		case t.Answer.Mode == "generated" && t.Answer.Solution != nil:
			test.Solution = solutions[*t.Answer.Solution]
		}
		subtask := m.Subtasks[names[t.SubtaskID]]
		subtask.Tests = append(subtask.Tests, test)
		m.Subtasks[names[t.SubtaskID]] = subtask
	}
	return nil
}
