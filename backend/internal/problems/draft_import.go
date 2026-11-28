package problems

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats"
	"github.com/brawler2011/gate/backend/pkg/formats/spec"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/brawler2011/gate/backend/pkg/statement"
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
	// Package paths of text files converted to UTF-8.
	converted []string
	// Package paths of missing files the import can do without.
	skipped []string
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
		return nil, pkg.Wrap(pkg.ErrZipInvalid, err, "invalid import ZIP")
	}
	tree := filepath.Join(root, "tree")
	if err := ziputil.ExtractZipLimited(z, tree, int64(uc.BuildOptions.Limits.DiskBytes)-archive.spec.Size, uc.BuildOptions.Limits.FileCount); err != nil {
		return nil, uncoded(err, pkg.ErrZipInvalid, "the archive cannot be extracted")
	}
	packageRoot, err := detectImportRoot(tree)
	if err != nil {
		return nil, uncoded(err, pkg.ErrPackageLayout, "the package cannot be read")
	}
	problem, err := formats.Read(packageRoot)
	if err != nil {
		return nil, uncoded(err, pkg.ErrPackageLayout, "the package cannot be read")
	}
	if err := uc.convertImport(ctx, result, packageRoot, problem, id, revision); err != nil {
		return nil, err
	}
	if err := result.draft.ComputeScores(); err != nil {
		return nil, err
	}
	complete = true
	return result, nil
}
func detectImportRoot(root string) (string, error) {
	_, rootErr := formats.DetectFormat(root)
	if rootErr == nil {
		return root, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("readdir: %w", err)
	}
	found, files := "", false
	var folders []error
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "__MACOSX" {
			continue
		}
		if !entry.IsDir() {
			files = true
			continue
		}
		path := filepath.Join(root, entry.Name())
		_, err := formats.DetectFormat(path)
		if err == nil {
			if found != "" {
				return "", spec.Fail(pkg.ErrPackageLayout, "", "", "multiple packages in archive")
			}
			found = path
		}
		folders = append(folders, err)
	}
	switch {
	case found != "":
		return found, nil
	// An archive of one folder fails for what that folder lacks.
	case len(folders) == 1 && !files:
		return "", folders[0]
	}
	return "", rootErr
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
	if !info.Mode().IsRegular() || info.Size() > int64(uc.BuildOptions.Limits.ObjectBytes) {
		return importObject{}, pkg.ErrPayloadTooLarge
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return importObject{}, fmt.Errorf("copy: %w", err)
	}
	return importObject{temporaryID: id, path: path, spec: model.UploadSpec{Purpose: purpose, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}}, nil
}

func (uc *DraftUseCase) convertImport(ctx context.Context, result *draftImport, root string, p *spec.Problem, id uuid.UUID, revision int64) error {
	c := &importConverter{uc: uc, result: result, root: root, textTests: p.TextTests, objects: map[string]uuid.UUID{}}
	c.draft = model.Draft{ProblemID: id, Revision: revision, Title: p.Title, ShortName: "problem-" + id.String()[:8], Type: p.Type, Grading: p.Grading, MemoryLimitMb: p.MemoryLimitMb,
		DraftContent: model.DraftContent{TimeRule: p.TimeRule, InputFile: p.InputFile, OutputFile: p.OutputFile, Processes: p.Processes, SolutionName: p.SolutionName, Validators: []model.DraftValidator{}, Generators: []model.DraftNamedProgram{}, Solutions: []model.DraftSolution{},
			Library: []model.DraftLibraryFile{}, Graders: []model.DraftFile{}, Attachments: []model.DraftFile{}, Statements: []model.DraftStatement{}, Assets: []model.DraftAsset{}, Tests: []model.DraftTest{}}}
	if p.TimeLimitMs > 0 {
		c.draft.TimeLimitMs = &p.TimeLimitMs
	}
	if err := c.convert(ctx, p); err != nil {
		return err
	}
	result.draft = c.draft
	result.converted = c.converted
	result.skipped = c.skipped
	return nil
}

type importConverter struct {
	uc        *DraftUseCase
	result    *draftImport
	root      string
	draft     model.Draft
	textTests bool
	objects   map[string]uuid.UUID
	files     int
	programs  int
	textBytes int64
	converted []string
	skipped   []string
	// pictures maps each name a statement may give a converted picture to its SVG asset.
	pictures map[string]string
}

func (c *importConverter) convert(ctx context.Context, p *spec.Problem) error {
	if err := c.addLibrary(p); err != nil {
		return err
	}
	var err error
	if p.Checker != nil {
		if c.draft.Checker, err = c.checker(*p.Checker); err != nil {
			return err
		}
		if c.draft.Checker.Tests, err = c.checkerTests(p.Checker.CheckerTests); err != nil {
			return err
		}
	}
	if p.Interactor != nil {
		program, err := c.program(*p.Interactor)
		if err != nil {
			return err
		}
		c.draft.Interactor = &model.DraftInteractor{DraftProgram: *program, DraftJury: jury(*p.Interactor)}
	}
	for _, v := range p.Validators {
		programs := c.draft.ValidatorPrograms()
		if _, err := c.named(&programs, v); err != nil {
			return err
		}
		tests, err := c.validatorTests(v.ValidatorTests)
		if err != nil {
			return err
		}
		c.draft.Validators = append(c.draft.Validators, model.DraftValidator{DraftNamedProgram: programs[len(programs)-1], DraftJury: jury(v), Groups: []uuid.UUID{}, Tests: tests})
	}
	generators := map[string]string{}
	for _, g := range p.Generators {
		name, err := c.named(&c.draft.Generators, g)
		if err != nil {
			return err
		}
		generators[g.Path] = name
	}
	solutions := map[string]string{}
	for _, s := range p.Solutions {
		if s.Text == "" && s.Path != p.MainSolution && c.skip(s.Path) {
			continue
		}
		programs := c.draft.SolutionPrograms()
		name, err := c.named(&programs, s)
		if err != nil {
			return err
		}
		verdicts := slices.Clone(s.Verdicts)
		if verdicts == nil {
			verdicts = []string{}
		}
		c.draft.Solutions = append(c.draft.Solutions, model.DraftSolution{DraftNamedProgram: programs[len(programs)-1], Verdicts: verdicts, Points: s.Points})
		solutions[s.Path] = name
		if s.Path == p.MainSolution {
			c.draft.MainSolution = &name
		}
	}
	if p.MainSolution != "" && c.draft.MainSolution == nil {
		return spec.Fail(pkg.ErrUnknown, "", p.MainSolution, "main solution %s not found", p.MainSolution)
	}
	for _, a := range p.Assets {
		if !model.ValidFileName(a.Name) {
			return spec.Fail(pkg.ErrFormat, a.Path, a.Name, "invalid asset name %s", a.Name)
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
	if err := c.addPictures(ctx, p.Pictures); err != nil {
		return err
	}
	for _, s := range p.Statements {
		if err := c.addStatement(ctx, s); err != nil {
			return err
		}
	}
	for _, list := range []struct {
		files []spec.File
		draft *[]model.DraftFile
	}{{p.Graders, &c.draft.Graders}, {p.Attachments, &c.draft.Attachments}} {
		for _, f := range list.files {
			text, err := c.readText(f.Path, "")
			if err != nil {
				return err
			}
			if err := c.charge(len(text)); err != nil {
				return err
			}
			*list.draft = append(*list.draft, model.DraftFile{Name: f.Name, Content: text})
		}
	}
	if err := c.addGroups(p.Testset, generators, solutions); err != nil {
		return err
	}
	return c.scopeValidators(p.Validators)
}

func (c *importConverter) checkerTests(tests []spec.CheckerTest) ([]model.DraftCheckerTest, error) {
	result := []model.DraftCheckerTest{}
	for _, t := range tests {
		if c.skip(t.Input, t.Output, t.Answer) {
			continue
		}
		test := model.DraftCheckerTest{ID: uuid.New(), Verdict: t.Verdict, Points: t.Points}
		for _, f := range []struct {
			path string
			id   *uuid.UUID
		}{{t.Input, &test.Input}, {t.Output, &test.Output}, {t.Answer, &test.Answer}} {
			oid, err := c.object(f.path, "checker-test")
			if err != nil {
				return nil, err
			}
			*f.id = oid
		}
		result = append(result, test)
	}
	return result, nil
}

func (c *importConverter) validatorTests(tests []spec.ValidatorTest) ([]model.DraftValidatorTest, error) {
	result := []model.DraftValidatorTest{}
	for _, t := range tests {
		if c.skip(t.Input) {
			continue
		}
		oid, err := c.object(t.Input, "validator-test")
		if err != nil {
			return nil, err
		}
		result = append(result, model.DraftValidatorTest{ID: uuid.New(), Input: oid, Valid: t.Valid, Group: t.Group})
	}
	return result, nil
}

func (c *importConverter) scopeValidators(validators []spec.Program) error {
	ids := map[string]uuid.UUID{}
	for g := range c.draft.Testset.All() {
		ids[g.Name] = g.ID
	}
	for i, v := range validators {
		for _, group := range v.Groups {
			id, ok := ids[group]
			if !ok || group == "" {
				return spec.Fail(pkg.ErrUnknown, v.Path, group, "validator %s checks unknown test group %s", v.Path, group)
			}
			c.draft.Validators[i].Groups = append(c.draft.Validators[i].Groups, id)
		}
	}
	return nil
}

// Upstream copies of system files auto-update, so old packages build with the current compiler.
// Conflicting library texts are rejected unless all can use the system copy.
func (c *importConverter) addLibrary(p *spec.Problem) error {
	paths := map[string]spec.File{}
	for _, f := range p.Library {
		paths[f.Path] = f
	}
	var order []string
	texts, system := map[string][]string{}, map[string]bool{}
	pinned := map[string]bool{}
	// sources holds a package path of each distinct text, for errors.
	sources := map[string][]string{}
	// blobs holds an object of each distinct binary file, such as a jar, by name.
	blobs := map[string][]uuid.UUID{}
	add := func(name string, f spec.File) error {
		if !model.ValidFileName(name) {
			return spec.Fail(pkg.ErrFormat, "", name, "invalid library file name %s", name)
		}
		if _, ok := texts[name]; !ok && !system[name] {
			order = append(order, name)
			texts[name] = nil
		}
		pinned[name] = pinned[name] || f.Pinned
		// Charge text before retaining it so imports cannot exceed draft memory quotas.
		if f.System {
			if !system[name] {
				system[name] = true
				return c.charge(0)
			}
			return nil
		}
		if binary, err := c.binary(f.Path); err != nil || binary {
			if err != nil {
				return err
			}
			oid, err := c.object(f.Path, "library")
			if err != nil {
				return err
			}
			if !slices.ContainsFunc(blobs[name], func(id uuid.UUID) bool { return c.digest(id) == c.digest(oid) }) {
				if err := c.charge(0); err != nil {
					return err
				}
				blobs[name] = append(blobs[name], oid)
				sources[name] = append(sources[name], f.Path)
			}
			return nil
		}
		text, err := c.readText(f.Path, "")
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
	judge := func(program *spec.Program) error {
		if program == nil {
			return nil
		}
		if program.Dependencies == nil {
			for _, f := range p.Library {
				if !f.System && c.skip(f.Path) {
					continue
				}
				if err := add(f.Name, f); err != nil {
					return err
				}
			}
			return nil
		}
		for _, name := range slices.Sorted(maps.Keys(program.Dependencies)) {
			f, ok := paths[program.Dependencies[name]]
			if !ok {
				return spec.Fail(pkg.ErrUnknown, program.Path, program.Dependencies[name], "dependency %s of %s is not a library file", program.Dependencies[name], program.Path)
			}
			if err := add(name, f); err != nil {
				return err
			}
		}
		return nil
	}
	for _, program := range []*spec.Program{p.Checker, p.Interactor} {
		if err := judge(program); err != nil {
			return err
		}
	}
	for i := range p.Validators {
		if err := judge(&p.Validators[i]); err != nil {
			return err
		}
	}
	for i := range p.Generators {
		if err := judge(&p.Generators[i]); err != nil {
			return err
		}
	}
	// Without judge programs nothing selects library files, so keep them all for a later checker.
	if p.Checker == nil && p.Interactor == nil && len(p.Validators) == 0 && len(p.Generators) == 0 {
		for _, f := range p.Library {
			if !f.System && c.skip(f.Path) {
				continue
			}
			if err := add(f.Name, f); err != nil {
				return err
			}
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
		case len(blobs[name]) > 0 && (len(list) > 0 || system[name] || len(blobs[name]) > 1):
			return spec.Fail(pkg.ErrDuplicate, strings.Join(sources[name], ", "), name,
				"programs include different files named %s: %s; a problem has one library file of each name, so rename one of them", name, strings.Join(sources[name], ", "))
		case len(blobs[name]) == 1:
			file.ObjectID = &blobs[name][0]
		case pinned[name] && len(list) == 1:
			file.Content = list[0]
		case system[name] && !known:
			return spec.Fail(pkg.ErrUnknown, "", name, "unknown system library file %s", name)
		case system[name] && len(list) > 0:
			return spec.Fail(pkg.ErrDuplicate, "", name, "the package has its own %s and names the system one", name)
		case system[name], upstream:
			file.AutoUpdate = true
		case len(list) == 1:
			file.Content = list[0]
		default:
			return spec.Fail(pkg.ErrDuplicate, strings.Join(sources[name], ", "), name,
				"programs include different files named %s: %s; a problem has one library file of each name, so rename one of them", name, strings.Join(sources[name], ", "))
		}
		c.draft.Library = append(c.draft.Library, file)
	}
	return nil
}

// Known standard checkers use system references; unknown ones retain their source.
func (c *importConverter) checker(program spec.Program) (*model.DraftChecker, error) {
	if _, ok := syslib.LookupChecker(program.Standard); ok {
		language, err := c.language(program)
		if err != nil {
			return nil, err
		}
		return &model.DraftChecker{DraftProgram: model.DraftProgram{Language: language}, DraftJury: jury(spec.Program{}), Standard: program.Standard}, nil
	}
	p, err := c.program(program)
	if err != nil {
		return nil, err
	}
	return &model.DraftChecker{DraftProgram: *p, DraftJury: jury(program)}, nil
}

func jury(program spec.Program) model.DraftJury {
	j := model.DraftJury{Protocol: sandbox.Protocol(program.Protocol), Args: program.Args}
	if j.Protocol == "" {
		j.Protocol = sandbox.ProtocolTestlib
	}
	if j.Args == nil {
		j.Args = []string{}
	}
	return j
}

func (c *importConverter) program(program spec.Program) (*model.DraftProgram, error) {
	language, err := c.language(program)
	if err != nil {
		return nil, err
	}
	text := program.Text
	if text == "" {
		var err error
		if text, err = c.readText(program.Path, ""); err != nil {
			return nil, err
		}
	}
	if err := c.charge(len(text)); err != nil {
		return nil, err
	}
	return &model.DraftProgram{Language: language, Source: text}, nil
}

func (c *importConverter) language(program spec.Program) (string, error) {
	if c.programs++; c.programs > c.uc.DraftQuotas.Components {
		return "", pkg.ErrPayloadTooLarge
	}
	language := program.Language
	if language == "" {
		language = c.uc.Languages.ForPath(program.Name)
	}
	if language == "" {
		return "", spec.Fail(pkg.ErrUnsupported, program.Path, "", "no language configuration accepts %s", program.Path)
	}
	if _, err := c.uc.Languages.Lookup(language); err != nil {
		if _, ok := errors.AsType[*sandbox.NotInstalledError](err); ok {
			return "", spec.Fail(pkg.ErrUnsupported, program.Path, language, "%s of %s", err, program.Path)
		}
		return "", spec.Fail(pkg.ErrUnsupported, program.Path, language, "unsupported language configuration %s of %s", language, program.Path)
	}
	return language, nil
}

// Disambiguate duplicate program stems within a role, including across languages.
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

func (c *importConverter) addStatement(ctx context.Context, s spec.Statement) error {
	if c.draft.Statement(s.Language) != nil {
		return spec.Fail(pkg.ErrDuplicate, "", s.Language, "several statements in %s", s.Language)
	}
	st := model.DraftStatement{Language: s.Language, Title: strings.TrimSpace(s.Title)}
	sections := map[string]string{}
	switch {
	case s.TeX != "":
		text, err := c.readText(s.TeX, s.Charset)
		if err != nil {
			return err
		}
		sections = statement.SplitTeX(text)
	case s.Markdown != "":
		tex, err := c.latex(ctx, s.Markdown, s.Charset)
		if err != nil {
			return err
		}
		sections = statement.SplitTeX(tex)
	case s.HTML != "":
		page, err := c.readText(s.HTML, s.Charset)
		if err != nil {
			return err
		}
		fragments, err := statement.SplitHTML(page)
		if err != nil {
			return spec.Fail(pkg.ErrFormat, s.HTML, "", "%v", err)
		}
		for name, fragment := range fragments {
			tex, err := c.pandoc(ctx, s.HTML, "html", fragment.HTML)
			if err != nil {
				return err
			}
			sections[name] = fragment.LaTeX(tex)
		}
	case s.PDF != "":
		sections["legend"] = fmt.Sprintf("\\includegraphics{%s}", s.PDF)
	}
	if s.Tutorial != "" {
		var err error
		if sections["tutorial"], err = c.latex(ctx, s.Tutorial, s.Charset); err != nil {
			return err
		}
	}
	for name, path := range s.Sections {
		text, err := c.readText(path, s.Charset)
		if err != nil {
			return err
		}
		// Leading whitespace can start an indented code block, so only the end is trimmed.
		sections[name] = strings.TrimRightFunc(text, unicode.IsSpace)
	}
	for name, text := range sections {
		section := st.Section(name)
		if section == nil {
			return spec.Fail(pkg.ErrUnknown, "", name, "unknown statement section %s in %s", name, s.Language)
		}
		if len(text) > int(c.uc.DraftQuotas.FileBytes) {
			return pkg.ErrPayloadTooLarge
		}
		if err := c.charge(len(text)); err != nil {
			return err
		}
		*section = c.renamePictures(text)
	}
	c.draft.Statements = append(c.draft.Statements, st)
	return nil
}

// addPictures stores the SVG of each PostScript or MetaPost picture that converts; the rest stay missing images for the build to report.
func (c *importConverter) addPictures(ctx context.Context, pictures []spec.File) error {
	if len(pictures) == 0 {
		return nil
	}
	files := map[string][]byte{}
	for _, f := range pictures {
		full, err := c.file(f.Path)
		if err != nil {
			return err
		}
		if files[f.Name], err = os.ReadFile(full); err != nil {
			return fmt.Errorf("read %s: %w", f.Path, err)
		}
	}
	svgs, err := c.uc.Typeset.Pictures(ctx, files)
	if err != nil {
		return fmt.Errorf("convert pictures: %w", err)
	}
	dir, err := os.MkdirTemp(c.result.root, "pictures")
	if err != nil {
		return fmt.Errorf("create pictures dir: %w", err)
	}
	asset := func(name string) bool {
		return slices.ContainsFunc(c.draft.Assets, func(a model.DraftAsset) bool { return a.Name == name })
	}
	c.pictures = map[string]string{}
	for _, ps := range slices.Sorted(maps.Keys(svgs)) {
		base := strings.TrimSuffix(ps, path.Ext(ps))
		name := ps + ".svg"
		if ext := strings.ToLower(path.Ext(ps)); (ext == ".eps" || ext == ".ps" || ext == ".mps") && !asset(base+".svg") {
			name = base + ".svg"
		}
		if asset(name) || !model.ValidFileName(name) {
			continue
		}
		if err := c.charge(0); err != nil {
			return err
		}
		full := filepath.Join(dir, name)
		if err := os.WriteFile(full, svgs[ps], 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		oid := uuid.New()
		object, err := c.uc.importObject(full, "asset", oid)
		if err != nil {
			return err
		}
		c.result.objects = append(c.result.objects, object)
		c.draft.Assets = append(c.draft.Assets, model.DraftAsset{Name: name, ObjectID: oid})
		c.pictures[ps] = name
		// graphicx tries other extensions before PostScript, so an extensionless reference means the picture only without them.
		if _, other := statement.ImageFile(base, asset); base != ps && !other {
			c.pictures[base] = name
		}
	}
	return nil
}

// pictureRef is the file argument of \includegraphics and \illustration.
var pictureRef = regexp.MustCompile(`(\\includegraphics\s*(?:\[[^\]]*\]\s*)?|\\illustration\s*\{[^{}]*\}\s*)\{([^{}]*)\}`)

func (c *importConverter) renamePictures(text string) string {
	return pictureRef.ReplaceAllStringFunc(text, func(m string) string {
		parts := pictureRef.FindStringSubmatch(m)
		if svg, ok := c.pictures[strings.TrimSpace(parts[2])]; ok {
			return parts[1] + "{" + svg + "}"
		}
		return m
	})
}

// latex reads a LaTeX file, or converts a Markdown one.
func (c *importConverter) latex(ctx context.Context, path, charset string) (string, error) {
	text, err := c.readText(path, charset)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(path)) != ".md" {
		return text, nil
	}
	return c.pandoc(ctx, path, "markdown", text)
}

func (c *importConverter) pandoc(ctx context.Context, path, from, text string) (string, error) {
	tex, _, err := c.uc.Typeset.LaTeX(ctx, from, text)
	if errors.Is(err, sandbox.ErrUnavailable) {
		return "", fmt.Errorf("convert %s: %w", path, err)
	}
	if err != nil {
		return "", spec.Fail(pkg.ErrFormat, path, "", "pandoc cannot convert %s: %v", path, err)
	}
	return tex, nil
}

func (c *importConverter) addGroups(root spec.Group, generators, solutions map[string]string) error {
	ids := map[string]uuid.UUID{}
	for g := range root.All() {
		if _, ok := ids[g.Name]; ok {
			return spec.Fail(pkg.ErrDuplicate, "", g.Name, "several test groups named %q", g.Name)
		}
		ids[g.Name] = uuid.New()
	}
	if len(ids) > c.uc.DraftQuotas.Groups {
		return pkg.ErrPayloadTooLarge
	}
	var ordinals []int
	var convert func(spec.Group) (model.DraftGroup, error)
	convert = func(g spec.Group) (model.DraftGroup, error) {
		group := model.DraftGroup{ID: ids[g.Name], Name: g.Name, Aggregate: cmp.Or(g.Aggregate, "sum"), StopOnFailure: g.StopOnFailure, Feedback: cmp.Or(g.Feedback, "complete"),
			Requires: []uuid.UUID{}, CheckerArgs: append([]string{}, g.CheckerArgs...), ValidatorArgs: append([]string{}, g.ValidatorArgs...), Groups: []model.DraftGroup{}}
		for _, name := range g.Requires {
			id, ok := ids[name]
			if !ok || name == "" {
				return group, spec.Fail(pkg.ErrUnknown, "", name, "test group %s requires unknown group %s", g.Name, name)
			}
			group.Requires = append(group.Requires, id)
		}
		for _, test := range g.Tests {
			t, err := c.test(test, group.ID, generators, solutions)
			if err != nil {
				return group, err
			}
			c.draft.Tests = append(c.draft.Tests, t)
			ordinals = append(ordinals, test.Ordinal)
		}
		for _, child := range g.Groups {
			converted, err := convert(child)
			if err != nil {
				return group, err
			}
			group.Groups = append(group.Groups, converted)
		}
		return group, nil
	}
	var err error
	if c.draft.Testset, err = convert(root); err != nil {
		return err
	}
	if !slices.Contains(ordinals, 0) {
		order := make([]int, len(ordinals))
		for i := range order {
			order[i] = i
		}
		slices.SortStableFunc(order, func(a, b int) int { return ordinals[a] - ordinals[b] })
		tests := make([]model.DraftTest, len(order))
		for i, j := range order {
			tests[i] = c.draft.Tests[j]
			tests[i].Ordinal = i + 1
		}
		c.draft.Tests = tests
	}
	return nil
}

func (c *importConverter) test(test spec.Test, group uuid.UUID, generators, solutions map[string]string) (model.DraftTest, error) {
	if len(c.draft.Tests) >= c.uc.DraftQuotas.Tests {
		return model.DraftTest{}, pkg.ErrPayloadTooLarge
	}
	t := model.DraftTest{ID: uuid.New(), Ordinal: len(c.draft.Tests) + 1, GroupID: group, Points: test.Points, Sample: test.Sample, Pretest: test.Pretest}
	if len(test.Steps) > 0 {
		t.Input = model.DraftInput{Mode: "generator", Steps: []model.DraftStep{}, File: test.File}
		for _, step := range test.Steps {
			gen, ok := generators[step.Generator]
			if !ok {
				return t, spec.Fail(pkg.ErrUnknown, "", step.Generator, "generator %s not found", step.Generator)
			}
			t.Input.Steps = append(t.Input.Steps, model.DraftStep{Generator: gen, Args: append([]string{}, step.Args...)})
		}
	} else {
		oid, err := c.object(test.Input, "test-input")
		if err != nil {
			return t, err
		}
		t.Input = model.DraftInput{Mode: "manual", ObjectID: &oid, Steps: []model.DraftStep{}}
	}
	switch {
	// Only ICPC interactors read the answer; testlib ones produce it.
	case spec.Interactive(c.draft.Type) && (test.Answer == "" || c.draft.Interactor == nil || c.draft.Interactor.Protocol != sandbox.ProtocolICPC):
		t.Answer = model.DraftAnswer{Mode: "interactive"}
	case test.Answer != "":
		oid, err := c.object(test.Answer, "test-answer")
		if err != nil {
			return t, err
		}
		t.Answer = model.DraftAnswer{Mode: "provided", ObjectID: &oid}
	case test.AnswerSolution != "":
		name, ok := solutions[test.AnswerSolution]
		if !ok {
			return t, spec.Fail(pkg.ErrUnknown, "", test.AnswerSolution, "solution %s not found", test.AnswerSolution)
		}
		t.Answer = model.DraftAnswer{Mode: "generated", Solution: &name}
	default:
		t.Answer = model.DraftAnswer{Mode: "generated", Solution: c.draft.MainSolution}
	}
	for _, example := range []struct {
		path string
		text **string
	}{{test.StatementInput, &t.StatementInput}, {test.StatementOutput, &t.StatementOutput}} {
		if example.path == "" {
			continue
		}
		text, err := c.readText(example.path, "")
		if err != nil {
			return t, err
		}
		text = strings.ReplaceAll(text, "\r\n", "\n")
		if err := c.charge(len(text)); err != nil {
			return t, err
		}
		*example.text = &text
	}
	return t, nil
}

// The caller charges retained text against draft quotas.
func (c *importConverter) readText(path, charset string) (string, error) {
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
	text, converted, err := spec.DecodeText(data, charset)
	if err != nil {
		return "", spec.Fail(pkg.ErrNotUTF8, path, "", "cannot read text of %s: %v", path, err)
	}
	if strings.ContainsRune(text, 0) {
		return "", spec.Fail(pkg.ErrNotUTF8, path, "", "%s is a binary file, but Gate needs it as text", path)
	}
	if converted && !slices.Contains(c.converted, path) {
		c.converted = append(c.converted, path)
	}
	return text, nil
}

// binary reports a file with a NUL byte near its start, as Git tells binary files from text.
func (c *importConverter) binary(path string) (bool, error) {
	full, err := c.file(path)
	if err != nil {
		return false, err
	}
	f, err := os.Open(full)
	if err != nil {
		return false, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	head := make([]byte, 8000)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read: %w", err)
	}
	return bytes.IndexByte(head[:n], 0) >= 0, nil
}

func (c *importConverter) digest(oid uuid.UUID) string {
	for _, o := range c.result.objects {
		if o.temporaryID == oid {
			return o.spec.SHA256
		}
	}
	return ""
}

// Charge every draft entry, even when several entries reference the same source file.
func (c *importConverter) charge(n int) error {
	c.files++
	c.textBytes += int64(n)
	if c.files > c.uc.DraftQuotas.Files || c.textBytes > int64(c.uc.DraftQuotas.TextBytes) {
		return pkg.ErrPayloadTooLarge
	}
	return nil
}

func (c *importConverter) object(path, purpose string) (uuid.UUID, error) {
	if oid, ok := c.objects[path]; ok {
		return oid, nil
	}
	full, err := c.file(path)
	if err != nil {
		return uuid.Nil, err
	}
	if c.textTests && purpose != "asset" && purpose != "library" {
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

// Stream CRLF conversion so test size does not bound memory usage.
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

// skip records missing package paths; it is for files a package may omit without changing judging.
func (c *importConverter) skip(paths ...string) bool {
	var missing []string
	for _, path := range paths {
		if _, err := c.file(path); err != nil {
			missing = append(missing, path)
		}
	}
	for _, path := range missing {
		if !slices.Contains(c.skipped, path) {
			c.skipped = append(c.skipped, path)
		}
	}
	return len(missing) > 0
}

func (c *importConverter) file(path string) (string, error) {
	if !fs.ValidPath(path) || path == "." {
		return "", spec.Fail(pkg.ErrPackageLayout, path, "", "invalid package path %s", path)
	}
	full := filepath.Join(c.root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		return "", spec.Fail(pkg.ErrPackageLayout, path, "", "missing package file %s", path)
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
	missing := false
	i.draft.EachObject(func(_ string, oid *uuid.UUID) {
		id, ok := replace[*oid]
		missing = missing || !ok
		*oid = id
	})
	if missing {
		return pkg.ErrInvalid
	}
	return nil
}

// uncoded codes an error that has no client code yet; its cause stays private because it may name server paths.
func uncoded(err error, code *pkg.Error, message string) error {
	var coded *pkg.Error
	if errors.As(err, &coded) {
		return err
	}
	return pkg.Wrap(code, err, message)
}
