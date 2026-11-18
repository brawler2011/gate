package problems

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/formats/tex"
	"github.com/brawler2011/gate/backend/pkg/syslib"
	"github.com/brawler2011/gate/backend/pkg/ziputil"
	"github.com/google/uuid"
)

type importObject struct {
	temporaryID uuid.UUID
	path        string
	spec        model.UploadSpec
}
type draftImport struct {
	draft   model.Draft
	objects []importObject
	root    string
}

func (i *draftImport) Close() { _ = os.RemoveAll(i.root) }
func (uc *DraftUseCase) parseImport(ctx context.Context, reader io.Reader, id uuid.UUID, revision int64) (*draftImport, error) {
	root, err := os.MkdirTemp(uc.TempDir, "import-")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp: %w", err)
	}
	result := &draftImport{root: root}
	complete := false
	defer func() {
		if !complete {
			result.Close()
		}
	}()
	archive, err := uc.spool(ctx, root, "asset", reader)
	if err != nil {
		return nil, err
	}
	defer archive.file.Close()
	z, err := zip.NewReader(archive.file, archive.spec.Size)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid import ZIP")
	}
	tree := filepath.Join(root, "tree")
	if err := ziputil.ExtractZipLimited(z, tree, int64(uc.BuildOptions.Limits.DiskBytes)-archive.spec.Size, uc.BuildOptions.Limits.FileCount); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid import archive")
	}
	packageRoot, err := detectImportRoot(tree)
	if err != nil {
		return nil, err
	}
	// A published v2 package preserves its complete original author snapshot.
	if info, statErr := os.Stat(filepath.Join(packageRoot, ".gate/snapshot.json")); statErr == nil {
		if info.Size() > 16*1024*1024 {
			return nil, pkg.ErrPayloadTooLarge
		}
		data, err := os.ReadFile(filepath.Join(packageRoot, ".gate/snapshot.json"))
		if err != nil {
			return nil, fmt.Errorf("readfile: %w", err)
		}
		var snapshot model.BuildSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}
		result.draft = snapshot.Draft
		result.draft.ProblemID = id
		result.draft.Revision = revision
		for _, oid := range snapshot.Draft.InputObjects() {
			o, err := uc.importObject(filepath.Join(packageRoot, ".gate/objects/"+oid.String()), "asset", oid)
			if err != nil {
				return nil, err
			}
			result.objects = append(result.objects, o)
		}
	} else {
		problem, err := formats.Read(packageRoot)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid author package")
		}
		if err := uc.convertImport(result, packageRoot, problem, id, revision); err != nil {
			return nil, err
		}
	}
	if err := result.draft.ComputeScores(); err != nil {
		return nil, err
	}
	complete = true
	return result, nil
}
func detectImportRoot(root string) (string, error) {
	if _, err := formats.DetectFormat(root); err == nil {
		return root, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("readdir: %w", err)
	}
	found := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if _, err := formats.DetectFormat(path); err == nil {
			if found != "" {
				return "", pkg.Wrap(pkg.ErrBadInput, nil, "multiple packages in archive")
			}
			found = path
		}
	}
	if found == "" {
		return "", pkg.Wrap(pkg.ErrBadInput, nil, "package root not found")
	}
	return found, nil
}
func (uc *DraftUseCase) importObject(path, purpose string, id uuid.UUID) (importObject, error) {
	f, err := os.Open(path)
	if err != nil {
		return importObject{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return importObject{}, fmt.Errorf("stat: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > uc.ObjectBytes {
		return importObject{}, pkg.ErrPayloadTooLarge
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return importObject{}, fmt.Errorf("copy: %w", err)
	}
	return importObject{temporaryID: id, path: path, spec: model.UploadSpec{Purpose: purpose, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}}, nil
}

// convertImport turns a package read from an author format into draft
// entries: programs by role, library files, statement sections and assets.
// Test inputs, answers and assets become objects.
func (uc *DraftUseCase) convertImport(result *draftImport, root string, p *spec.Problem, id uuid.UUID, revision int64) error {
	c := &importConverter{uc: uc, result: result, root: root, textTests: p.TextTests, objects: map[string]uuid.UUID{}}
	c.draft = model.Draft{ProblemID: id, Revision: revision, Title: p.Title, ShortName: "problem-" + id.String()[:8], Type: p.Type, TimeLimitMs: p.TimeLimitMs, MemoryLimitMb: p.MemoryLimitMb,
		DraftContent: model.DraftContent{Validators: []model.DraftNamedProgram{}, Generators: []model.DraftNamedProgram{}, Solutions: []model.DraftNamedProgram{},
			Library: []model.DraftLibraryFile{}, Statements: []model.DraftStatement{}, Assets: []model.DraftAsset{}, Tests: []model.DraftTest{}, Subtasks: []model.DraftSubtask{}}}
	if err := c.convert(p); err != nil {
		return err
	}
	result.draft = c.draft
	return nil
}

type importConverter struct {
	uc     *DraftUseCase
	result *draftImport
	root   string
	draft  model.Draft
	// textTests converts CRLF line endings of test files to LF.
	textTests bool
	// objects maps a package path to the object holding it.
	objects map[string]uuid.UUID
	// files, programs and textBytes count the draft as it grows, against
	// DraftQuotas.
	files     int
	programs  int
	textBytes int64
}

func (c *importConverter) convert(p *spec.Problem) error {
	if err := c.addLibrary(p); err != nil {
		return err
	}
	var err error
	if p.Checker != nil {
		if c.draft.Checker, err = c.checker(*p.Checker); err != nil {
			return err
		}
	}
	if p.Interactor != nil {
		if c.draft.Interactor, err = c.program(*p.Interactor); err != nil {
			return err
		}
	}
	if p.Validator != nil {
		if _, err := c.named(&c.draft.Validators, *p.Validator); err != nil {
			return err
		}
	}
	generators := map[string]string{}
	for _, g := range p.Generators {
		name, err := c.named(&c.draft.Generators, g)
		if err != nil {
			return err
		}
		generators[g.Path] = name
	}
	for _, s := range p.Solutions {
		name, err := c.named(&c.draft.Solutions, s)
		if err != nil {
			return err
		}
		if s.Path == p.MainSolution {
			c.draft.MainSolution = &name
		}
	}
	if p.MainSolution != "" && c.draft.MainSolution == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "main solution "+p.MainSolution+" not found")
	}
	for _, s := range p.Statements {
		if err := c.addStatement(s); err != nil {
			return err
		}
	}
	for _, a := range p.Assets {
		if !model.ValidFileName(a.Name) {
			return pkg.Wrap(pkg.ErrBadInput, nil, "invalid asset name "+a.Name)
		}
		if err := c.charge(0); err != nil {
			return err
		}
		oid, err := c.object(a.Path, "asset")
		if err != nil {
			return err
		}
		c.draft.Assets = append(c.draft.Assets, model.DraftAsset{Name: a.Name, ObjectID: oid})
	}
	return c.addGroups(p.Groups, generators)
}

// addLibrary adds the library files the judge programs of p include. A
// draft has one file of each name, so different texts under one name are
// rejected, except upstream versions of a system library file that no
// generator includes: they become the auto-updated file, since checkers and
// validators take its current version anyway. A generator keeps the text of
// its package, since rnd differs between testlib versions, unless the text is
// the current system one.
func (c *importConverter) addLibrary(p *spec.Problem) error {
	paths := map[string]spec.File{}
	for _, f := range p.Library {
		paths[f.Path] = f
	}
	var order []string
	texts, system, generated := map[string][]string{}, map[string]bool{}, map[string]bool{}
	// sources holds a package path of each distinct text, for errors.
	sources := map[string][]string{}
	add := func(name string, f spec.File, generator bool) error {
		if !model.ValidFileName(name) {
			return pkg.Wrap(pkg.ErrBadInput, nil, "invalid library file name "+name)
		}
		if _, ok := texts[name]; !ok && !system[name] {
			order = append(order, name)
			texts[name] = nil
		}
		generated[name] = generated[name] || generator
		// Each distinct text is charged as it is read, so a package cannot
		// hold more text in memory than the draft quotas allow.
		if f.System {
			if !system[name] {
				system[name] = true
				return c.charge(0)
			}
			return nil
		}
		text, err := c.readText(f.Path)
		if err != nil {
			return err
		}
		if !slices.Contains(texts[name], text) {
			if err := c.charge(len(text)); err != nil {
				return err
			}
			texts[name] = append(texts[name], text)
			sources[name] = append(sources[name], f.Path)
		}
		return nil
	}
	judge := func(program *spec.Program, generator bool) error {
		if program == nil {
			return nil
		}
		if program.Dependencies == nil {
			for _, f := range p.Library {
				if err := add(f.Name, f, generator); err != nil {
					return err
				}
			}
			return nil
		}
		for _, name := range slices.Sorted(maps.Keys(program.Dependencies)) {
			f, ok := paths[program.Dependencies[name]]
			if !ok {
				return pkg.Wrap(pkg.ErrBadInput, nil, "dependency "+program.Dependencies[name]+" of "+program.Path+" is not a library file")
			}
			if err := add(name, f, generator); err != nil {
				return err
			}
		}
		return nil
	}
	for _, program := range []*spec.Program{p.Checker, p.Interactor, p.Validator} {
		if err := judge(program, false); err != nil {
			return err
		}
	}
	for i := range p.Generators {
		if err := judge(&p.Generators[i], true); err != nil {
			return err
		}
	}
	for _, name := range order {
		list := texts[name]
		file := model.DraftLibraryFile{Name: name}
		current, known := syslib.Lookup(name)
		upstream := known && len(list) > 0
		for _, text := range list {
			upstream = upstream && current.IsUpstream(text)
		}
		switch {
		case system[name] && !known:
			return pkg.Wrap(pkg.ErrBadInput, nil, "unknown system library file "+name)
		case system[name] && len(list) > 0:
			return pkg.Wrap(pkg.ErrBadInput, nil, "the package has its own "+name+" and names the system one")
		case system[name], upstream && !generated[name], upstream && len(list) == 1 && list[0] == current.Content:
			file.AutoUpdate = true
		case len(list) == 1:
			file.Content = list[0]
		default:
			return pkg.Wrap(pkg.ErrBadInput, nil, "programs include different files named "+name+": "+strings.Join(sources[name], ", ")+
				"; a problem has one library file of each name, so rename one of them")
		}
		c.draft.Library = append(c.draft.Library, file)
	}
	return nil
}

// checker reads the checker; a standard checker Gate has becomes a reference
// to it, and any other one, such as a standard checker unknown to Gate, is
// imported from its source.
func (c *importConverter) checker(program spec.Program) (*model.DraftChecker, error) {
	if _, ok := syslib.LookupChecker(program.Standard); ok {
		language, err := c.language(program)
		if err != nil {
			return nil, err
		}
		return &model.DraftChecker{DraftProgram: model.DraftProgram{Language: language}, Standard: program.Standard}, nil
	}
	p, err := c.program(program)
	if err != nil {
		return nil, err
	}
	return &model.DraftChecker{DraftProgram: *p}, nil
}

// program reads a program source.
func (c *importConverter) program(program spec.Program) (*model.DraftProgram, error) {
	language, err := c.language(program)
	if err != nil {
		return nil, err
	}
	text, err := c.readText(program.Path)
	if err != nil {
		return nil, err
	}
	if err := c.charge(len(text)); err != nil {
		return nil, err
	}
	return &model.DraftProgram{Language: language, Source: text}, nil
}

// language counts a program against the quota and returns its language
// configuration.
func (c *importConverter) language(program spec.Program) (string, error) {
	if c.programs++; c.programs > c.uc.DraftQuotas.Components {
		return "", pkg.ErrPayloadTooLarge
	}
	// The format names the configuration, or the extension suggests one.
	language := program.Language
	if language == "" {
		language = c.uc.Languages.ForPath(program.Name)
	}
	if _, ok := c.uc.Languages.Langs[language]; !ok {
		if language == "" {
			return "", pkg.Wrap(pkg.ErrBadInput, nil, "no language configuration accepts "+program.Path)
		}
		return "", pkg.Wrap(pkg.ErrBadInput, nil, "unsupported language configuration "+language+" of "+program.Path)
	}
	return language, nil
}

// named adds a program of a role a draft holds several of and returns its
// name: its file name without the extension, numbered when another program
// of the role already has it (gen.cpp and gen.py become gen and gen-2).
func (c *importConverter) named(list *[]model.DraftNamedProgram, program spec.Program) (string, error) {
	p, err := c.program(program)
	if err != nil {
		return "", err
	}
	stem := spec.Stem(program.Name)
	name := stem
	for n := 2; slices.ContainsFunc(*list, func(q model.DraftNamedProgram) bool { return q.Name == name }); n++ {
		name = fmt.Sprintf("%s-%d", stem, n)
	}
	*list = append(*list, model.DraftNamedProgram{Name: name, DraftProgram: *p})
	return name, nil
}

// addStatement adds the statement sections in one language, converting
// LaTeX to Markdown.
func (c *importConverter) addStatement(s spec.Statement) error {
	if c.draft.Statement(s.Language) != nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "several statements in "+s.Language)
	}
	statement := model.DraftStatement{Language: s.Language}
	sections := map[string]string{}
	if s.TeX != "" {
		text, err := c.readText(s.TeX)
		if err != nil {
			return err
		}
		sections = tex.ToSections(text)
	}
	for name, path := range s.Sections {
		text, err := c.readText(path)
		if err != nil {
			return err
		}
		sections[name] = strings.TrimSpace(text)
	}
	for name, text := range sections {
		section := statement.Section(name)
		if section == nil {
			return pkg.Wrap(pkg.ErrBadInput, nil, "unknown statement section "+name+" in "+s.Language)
		}
		if len(text) > int(c.uc.DraftQuotas.FileBytes) {
			return pkg.ErrPayloadTooLarge
		}
		if err := c.charge(len(text)); err != nil {
			return err
		}
		*section = text
	}
	c.draft.Statements = append(c.draft.Statements, statement)
	return nil
}

func (c *importConverter) addGroups(groups []spec.Group, generators map[string]string) error {
	if len(groups) > c.uc.DraftQuotas.Subtasks {
		return pkg.ErrPayloadTooLarge
	}
	ids := map[string]uuid.UUID{}
	for _, g := range groups {
		if _, ok := ids[g.Name]; ok {
			return pkg.Wrap(pkg.ErrBadInput, nil, "several test groups named "+g.Name)
		}
		ids[g.Name] = uuid.New()
	}
	for _, g := range groups {
		subtask := model.DraftSubtask{ID: ids[g.Name], Ordinal: len(c.draft.Subtasks) + 1, Name: g.Name, Policy: g.Policy, DependsOn: []uuid.UUID{}}
		for _, dep := range g.DependsOn {
			did, ok := ids[dep]
			if !ok {
				return pkg.Wrap(pkg.ErrBadInput, nil, "test group "+g.Name+" depends on unknown group "+dep)
			}
			subtask.DependsOn = append(subtask.DependsOn, did)
		}
		for _, test := range g.Tests {
			t, err := c.test(test, subtask.ID, generators)
			if err != nil {
				return err
			}
			c.draft.Tests = append(c.draft.Tests, t)
		}
		c.draft.Subtasks = append(c.draft.Subtasks, subtask)
	}
	return nil
}

func (c *importConverter) test(test spec.Test, subtask uuid.UUID, generators map[string]string) (model.DraftTest, error) {
	if len(c.draft.Tests) >= c.uc.DraftQuotas.Tests {
		return model.DraftTest{}, pkg.ErrPayloadTooLarge
	}
	t := model.DraftTest{ID: uuid.New(), Ordinal: len(c.draft.Tests) + 1, SubtaskID: subtask, Points: test.Points, Sample: test.Sample}
	if test.Generator != "" {
		gen, ok := generators[test.Generator]
		if !ok {
			return t, pkg.Wrap(pkg.ErrBadInput, nil, "generator "+test.Generator+" not found")
		}
		seed := int64(0)
		t.Input = model.DraftInput{Mode: "generator", Generator: &gen, Args: test.Args, Seed: &seed}
		if t.Input.Args == nil {
			t.Input.Args = []string{}
		}
	} else {
		oid, err := c.object(test.Input, "test-input")
		if err != nil {
			return t, err
		}
		t.Input = model.DraftInput{Mode: "manual", ObjectID: &oid, Args: []string{}}
	}
	switch {
	case c.draft.Type == "interactive":
		t.Answer = model.DraftAnswer{Mode: "interactive"}
	case test.Answer != "":
		oid, err := c.object(test.Answer, "test-answer")
		if err != nil {
			return t, err
		}
		t.Answer = model.DraftAnswer{Mode: "provided", ObjectID: &oid}
	default:
		t.Answer = model.DraftAnswer{Mode: "generated", Solution: c.draft.MainSolution}
	}
	return t, nil
}

// readText reads a text file of the package; the caller charges what it keeps.
func (c *importConverter) readText(path string) (string, error) {
	full, err := c.file(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}
	if info.Size() > int64(c.uc.DraftQuotas.FileBytes) {
		return "", pkg.ErrPayloadTooLarge
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("readfile: %w", err)
	}
	if !utf8.Valid(data) {
		return "", pkg.Wrap(pkg.ErrBadInput, nil, "text must be UTF-8: "+path)
	}
	return string(data), nil
}

// charge counts a draft file of n text bytes against the draft quotas as the
// draft grows, so a manifest naming one file many times cannot inflate it.
func (c *importConverter) charge(n int) error {
	c.files++
	c.textBytes += int64(n)
	if c.files > c.uc.DraftQuotas.Files || c.textBytes > int64(c.uc.DraftQuotas.TextBytes) {
		return pkg.ErrPayloadTooLarge
	}
	return nil
}

// object stores a package file as an object, once per path.
func (c *importConverter) object(path, purpose string) (uuid.UUID, error) {
	if oid, ok := c.objects[path]; ok {
		return oid, nil
	}
	full, err := c.file(path)
	if err != nil {
		return uuid.Nil, err
	}
	if c.textTests && purpose != "asset" {
		if full, err = c.toLF(full); err != nil {
			return uuid.Nil, err
		}
	}
	oid := uuid.New()
	object, err := c.uc.importObject(full, purpose, oid)
	if err != nil {
		return uuid.Nil, err
	}
	c.result.objects = append(c.result.objects, object)
	c.objects[path] = oid
	return oid, nil
}

// toLF returns a copy of the text file at full with LF line endings, or full
// itself when it has no CRLF. It streams, so test size does not bound memory.
func (c *importConverter) toLF(full string) (string, error) {
	src, err := os.Open(full)
	if err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	defer src.Close()
	dir := filepath.Join(c.result.root, "lf")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdirall: %w", err)
	}
	dst, err := os.CreateTemp(dir, "test-")
	if err != nil {
		return "", fmt.Errorf("createtemp: %w", err)
	}
	converted, err := copyLF(dst, src)
	if closeErr := dst.Close(); err == nil {
		err = closeErr
	}
	if err != nil || !converted {
		_ = os.Remove(dst.Name())
		if err != nil {
			return "", fmt.Errorf("convert line endings: %w", err)
		}
		return full, nil
	}
	return dst.Name(), nil
}

// copyLF copies src to dst with each CRLF replaced by LF and reports whether
// it replaced any.
func copyLF(dst io.Writer, src io.Reader) (bool, error) {
	crlf, lf := []byte("\r\n"), []byte("\n")
	buf := make([]byte, 64<<10)
	converted, heldCR := false, false
	for {
		n, readErr := src.Read(buf)
		chunk := buf[:n]
		if heldCR && n > 0 {
			// A CR that ended the previous chunk ends a line only before LF.
			if chunk[0] == '\n' {
				converted = true
			} else if _, err := dst.Write([]byte{'\r'}); err != nil {
				return false, fmt.Errorf("write: %w", err)
			}
			heldCR = false
		}
		if len(chunk) > 0 && chunk[len(chunk)-1] == '\r' {
			heldCR, chunk = true, chunk[:len(chunk)-1]
		}
		if bytes.Contains(chunk, crlf) {
			converted, chunk = true, bytes.ReplaceAll(chunk, crlf, lf)
		}
		if _, err := dst.Write(chunk); err != nil {
			return false, fmt.Errorf("write: %w", err)
		}
		if errors.Is(readErr, io.EOF) {
			if heldCR {
				if _, err := dst.Write([]byte{'\r'}); err != nil {
					return false, fmt.Errorf("write: %w", err)
				}
			}
			return converted, nil
		}
		if readErr != nil {
			return false, fmt.Errorf("read: %w", readErr)
		}
	}
}

// file resolves a package path to a regular file inside the package.
func (c *importConverter) file(path string) (string, error) {
	if !fs.ValidPath(path) || path == "." {
		return "", pkg.Wrap(pkg.ErrBadInput, nil, "invalid package path "+path)
	}
	full := filepath.Join(c.root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		return "", pkg.Wrap(pkg.ErrBadInput, err, "missing package file "+path)
	}
	return full, nil
}

func (i *draftImport) bind(ids []uuid.UUID) error {
	if len(ids) != len(i.objects) {
		return pkg.ErrConflict
	}
	replace := map[uuid.UUID]uuid.UUID{}
	for n, o := range i.objects {
		replace[o.temporaryID] = ids[n]
	}
	for n := range i.draft.Tests {
		t := &i.draft.Tests[n]
		if t.Input.ObjectID != nil {
			id, ok := replace[*t.Input.ObjectID]
			if !ok {
				return pkg.ErrBadInput
			}
			t.Input.ObjectID = &id
		}
		if t.Answer.ObjectID != nil {
			id, ok := replace[*t.Answer.ObjectID]
			if !ok {
				return pkg.ErrBadInput
			}
			t.Answer.ObjectID = &id
		}
	}
	for n := range i.draft.Assets {
		id, ok := replace[i.draft.Assets[n].ObjectID]
		if !ok {
			return pkg.ErrBadInput
		}
		i.draft.Assets[n].ObjectID = id
	}
	return nil
}
