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
	for path, text := range result.draft.Files {
		if strings.HasPrefix(path, "statements/") {
			statement := ParseStatementMarkdown(text)
			statement.Title = ""
			result.draft.Files[path] = RenderStatementMarkdown(statement)
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

// convertImport places a package read from an author format into the draft
// layout: each program in its role folder, shared sources in lib/,
// statements in statements/<language>.md and statement resources in assets/.
// Test inputs, answers and assets become objects.
func (uc *DraftUseCase) convertImport(result *draftImport, root string, p *spec.Problem, id uuid.UUID, revision int64) error {
	c := &importConverter{uc: uc, result: result, root: root, textTests: p.TextTests, objects: map[string]uuid.UUID{}, library: map[string]string{}, names: map[string][]string{}}
	c.draft = model.Draft{ProblemID: id, Revision: revision, Title: p.Title, ShortName: "problem-" + id.String()[:8], Type: p.Type, TimeLimitMs: p.TimeLimitMs, MemoryLimitMb: p.MemoryLimitMb, Files: map[string]string{}, Components: []model.DraftComponent{}, Tests: []model.DraftTest{}, Subtasks: []model.DraftSubtask{}, Assets: []model.DraftAsset{}}
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
	// library maps the package path of a library file to its draft path, and
	// names maps each library file name to the draft paths of that name.
	library map[string]string
	names   map[string][]string
	// files and textBytes count the draft as it grows, against DraftQuotas.
	files     int
	textBytes int64
}

func (c *importConverter) convert(p *spec.Problem) error {
	for _, f := range p.Library {
		if err := c.addLibrary(f); err != nil {
			return err
		}
	}
	for _, singleton := range []struct {
		role    string
		program *spec.Program
	}{{"checker", p.Checker}, {"validator", p.Validator}, {"interactor", p.Interactor}} {
		if singleton.program != nil {
			if _, err := c.addComponent(singleton.role, *singleton.program); err != nil {
				return err
			}
		}
	}
	generators := map[string]uuid.UUID{}
	for _, g := range p.Generators {
		cid, err := c.addComponent("generator", g)
		if err != nil {
			return err
		}
		generators[g.Path] = cid
	}
	for _, s := range p.Solutions {
		cid, err := c.addComponent("solution", s)
		if err != nil {
			return err
		}
		if s.Path == p.MainSolution {
			c.draft.MainSolutionID = &cid
		}
	}
	if p.MainSolution != "" && c.draft.MainSolutionID == nil {
		return pkg.Wrap(pkg.ErrBadInput, nil, "main solution "+p.MainSolution+" not found")
	}
	for _, s := range p.Statements {
		if err := c.addStatement(s); err != nil {
			return err
		}
	}
	for _, a := range p.Assets {
		path := model.AssetFolder + "/" + a.Name
		if !model.ValidDraftPath(path) {
			return pkg.Wrap(pkg.ErrBadInput, nil, "invalid asset name "+a.Name)
		}
		if err := c.charge(0); err != nil {
			return err
		}
		oid, err := c.object(a.Path, "asset")
		if err != nil {
			return err
		}
		c.draft.Assets = append(c.draft.Assets, model.DraftAsset{Path: path, ObjectID: oid})
	}
	// Components are listed by path, as the workshop shows the folders.
	slices.SortFunc(c.draft.Components, func(a, b model.DraftComponent) int { return strings.Compare(a.Path, b.Path) })
	return c.addGroups(p.Groups, generators)
}

// addLibrary stores a library file in lib/ under its name. Copies with equal
// text, such as testlib.h in every ICPC program folder, share one file; a
// different file of the same name gets a numbered one.
func (c *importConverter) addLibrary(f spec.File) error {
	text, err := c.readText(f.Path)
	if err != nil {
		return err
	}
	for _, path := range c.names[f.Name] {
		if c.draft.Files[path] == text {
			c.library[f.Path] = path
			return nil
		}
	}
	path, err := c.place(model.LibraryFolder, f.Name)
	if err != nil {
		return err
	}
	if err := c.charge(len(text)); err != nil {
		return err
	}
	c.draft.Files[path] = text
	c.library[f.Path] = path
	c.names[f.Name] = append(c.names[f.Name], path)
	return nil
}

// place returns a free draft path for name in folder, numbering the name when
// another file took it.
func (c *importConverter) place(folder, name string) (string, error) {
	path := folder + "/" + name
	ext := filepath.Ext(name)
	for n := 2; c.taken(path); n++ {
		path = fmt.Sprintf("%s/%s-%d%s", folder, strings.TrimSuffix(name, ext), n, ext)
	}
	if !model.ValidDraftPath(path) {
		return "", pkg.Wrap(pkg.ErrBadInput, nil, "invalid file name "+name)
	}
	return path, nil
}

func (c *importConverter) taken(path string) bool {
	_, ok := c.draft.Files[path]
	return ok
}

// addComponent places a program in its role folder and attaches its library
// dependencies.
func (c *importConverter) addComponent(role string, program spec.Program) (uuid.UUID, error) {
	if len(c.draft.Components) >= c.uc.DraftQuotas.Components {
		return uuid.Nil, pkg.ErrPayloadTooLarge
	}
	path, err := c.place(model.ComponentFolder(role), program.Name)
	if err != nil {
		return uuid.Nil, err
	}
	if _, ok := c.uc.BuildOptions.Languages[model.LanguageForPath(path)]; !ok {
		return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "unsupported source language: "+path)
	}
	text, err := c.readText(program.Path)
	if err != nil {
		return uuid.Nil, err
	}
	if err := c.charge(len(text)); err != nil {
		return uuid.Nil, err
	}
	deps := map[string]string{}
	if program.Dependencies == nil {
		for name, paths := range c.names {
			if len(paths) > 1 {
				return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "different library files named "+name)
			}
			deps[name] = paths[0]
		}
	}
	for destination, source := range program.Dependencies {
		lib, ok := c.library[source]
		if !ok {
			return uuid.Nil, pkg.Wrap(pkg.ErrBadInput, nil, "dependency "+source+" of "+program.Path+" is not a library file")
		}
		deps[destination] = lib
	}
	c.draft.Files[path] = text
	cid := uuid.New()
	c.draft.Components = append(c.draft.Components, model.DraftComponent{ID: cid, Role: role, Path: path, Dependencies: deps})
	return cid, nil
}

// addStatement stores a statement as statements/<language>.md, converting
// LaTeX to Markdown.
func (c *importConverter) addStatement(s spec.Statement) error {
	path := model.StatementFolder + "/" + s.Language + ".md"
	if !model.ValidDraftPath(path) || strings.Contains(s.Language, "/") {
		return pkg.Wrap(pkg.ErrBadInput, nil, "invalid statement language "+s.Language)
	}
	if c.taken(path) {
		return pkg.Wrap(pkg.ErrBadInput, nil, "several statements in "+s.Language)
	}
	text, err := c.readText(s.Path)
	if err != nil {
		return err
	}
	if s.TeX {
		text = tex.ToMarkdown(text)
	}
	if len(text) > int(c.uc.DraftQuotas.FileBytes) {
		return pkg.ErrPayloadTooLarge
	}
	if err := c.charge(len(text)); err != nil {
		return err
	}
	c.draft.Files[path] = text
	return nil
}

func (c *importConverter) addGroups(groups []spec.Group, generators map[string]uuid.UUID) error {
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

func (c *importConverter) test(test spec.Test, subtask uuid.UUID, generators map[string]uuid.UUID) (model.DraftTest, error) {
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
		t.Input = model.DraftInput{Mode: "generator", ComponentID: &gen, Args: test.Args, Seed: &seed}
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
		t.Answer = model.DraftAnswer{Mode: "generated", ComponentID: c.draft.MainSolutionID}
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
